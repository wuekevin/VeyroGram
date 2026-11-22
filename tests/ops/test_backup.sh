#!/usr/bin/env bash
# Backup and restore rehearsal with REAL tools: a real MySQL 8.0 as the source (least-privilege backup account), a second real mysqld as the restore target,
# the real binary for encryption. Proves: full + binlog point-in-time recovery to a moment before a simulated accident, encryption (no plaintext, private key never
# needed on the "host"), tamper/truncation/gap/wrong-key refusals before anything is loaded, off-host shipping required, file modes, retention, failure metrics.
# Needs VG_TEST_MYSQL_ADMIN_DSN (a MySQL with binary logging on) and the MySQL 8.0 client/server binaries; skips otherwise. NOT a substitute for the staging restore
# drill on a real second host (docs/ops/MANUAL-CHECKLIST.md C10).
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); bk=$root/infrastructure/backup
dsn=${VG_TEST_MYSQL_ADMIN_DSN:-}; [[ -n $dsn ]] || { echo "SKIP: VG_TEST_MYSQL_ADMIN_DSN not set"; exit 0; }
bin=${VG_TEST_MYSQL_BIN_DIR:-}
if [[ -z $bin ]]; then for c in "$(dirname "$(command -v mysqld 2>/dev/null || echo /nonexistent/x)")" /opt/homebrew/opt/mysql@8.0/bin /usr/local/opt/mysql@8.0/bin /usr/sbin /usr/bin; do [[ -x $c/mysqld && -x $c/mysqldump && -x $c/mysqlbinlog ]] && { bin=$c; break; }; done; fi
[[ -n $bin && -x $bin/mysqld ]] || { echo "SKIP: MySQL 8.0 binaries (mysqld, mysqldump, mysqlbinlog) not found; set VG_TEST_MYSQL_BIN_DIR"; exit 0; }
tmp=$(mktemp -d); mypid=
cleanup() {
  [[ -n ${db:-} ]] && "$bin/mysql" --defaults-extra-file="$tmp/root.cnf" -e "DROP DATABASE IF EXISTS \`$db\`; DROP USER IF EXISTS 'vgbk_u'@'localhost','vgbk_u'@'127.0.0.1';" 2>/dev/null
  if [[ -n $mypid ]]; then
    # 必须等 mysqld 真正退出再删 datadir：带 binlog 的实例关闭较慢，固定 sleep 后就 rm -rf 会让它
    # 在目录消失后空转占满 CPU 且不退出（孤儿进程）。先 SIGTERM 并等待，超时再 SIGKILL。
    kill "$mypid" 2>/dev/null
    for _ in $(seq 1 120); do kill -0 "$mypid" 2>/dev/null || break; sleep 0.25; done
    kill -0 "$mypid" 2>/dev/null && { kill -9 "$mypid" 2>/dev/null; wait "$mypid" 2>/dev/null; }
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT
fail=0; pass=0
ok()   { pass=$((pass+1)); echo "  ok   - $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL - $1"; [[ -s ${tmp:-/x}/last.err ]] && sed "s/^/         | /" "$tmp/last.err" | tail -4; }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }
mode() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }
port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])'; }

# --- source server connection from the DSN ---
python3 - "$dsn" "$tmp/root.cnf" <<'PY'
import re,sys
m=re.match(r'^([^:]*):(.*)@tcp\(([^:]+):(\d+)\)/',sys.argv[1]); u,p,h,port=m.groups()
open(sys.argv[2],"w").write("[client]\nuser=%s\npassword=%s\nhost=%s\nport=%s\n"%(u,p,h,port))
PY
chmod 600 "$tmp/root.cnf"; src_host=$(sed -n 's/^host=//p' "$tmp/root.cnf"); src_port=$(sed -n 's/^port=//p' "$tmp/root.cnf")
root_sql() { "$bin/mysql" --defaults-extra-file="$tmp/root.cnf" -N -e "$1"; }
[[ $(root_sql "SELECT @@log_bin") == 1 ]] || { echo "SKIP: the source MySQL has binary logging off"; exit 0; }

echo "building the real binary and making the key pair (on the 'restore machine')"
api=$tmp/veyrogram-api; (cd "$root/services/api" && GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11} go build -o "$api" ./cmd/api) || { echo "build failed"; exit 1; }
"$api" backup keygen -out-dir "$tmp/keys" >/dev/null || { echo "keygen failed"; exit 1; }
"$api" backup keygen -out-dir "$tmp/otherkeys" >/dev/null
check "private key is 0600 and a second keygen refuses to overwrite" "[[ \$(mode $tmp/keys/private.pem) == 600 ]] && ! $api backup keygen -out-dir $tmp/keys >/dev/null 2>&1"

echo "source data and the least-privilege backup account"
db=vgbk_$$_$RANDOM; bpw=Bk-$(head -c 12 /dev/urandom | od -An -tx1 | tr -d ' \n')
root_sql "CREATE DATABASE \`$db\`; CREATE TABLE \`$db\`.items (id INT PRIMARY KEY AUTO_INCREMENT, note VARCHAR(100)); CREATE TABLE \`$db\`.other (id INT PRIMARY KEY);
  INSERT INTO \`$db\`.items (note) VALUES ('before-full-1-SECRETMARK'),('before-full-2'); INSERT INTO \`$db\`.other VALUES (1);
  CREATE USER 'vgbk_u'@'localhost' IDENTIFIED BY '$bpw'; CREATE USER 'vgbk_u'@'127.0.0.1' IDENTIFIED BY '$bpw';
  GRANT SELECT, SHOW VIEW, TRIGGER, EVENT, RELOAD, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO 'vgbk_u'@'localhost'; GRANT SELECT, SHOW VIEW, TRIGGER, EVENT, RELOAD, REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO 'vgbk_u'@'127.0.0.1';"
printf '[client]\nuser=vgbk_u\npassword=%s\nhost=%s\nport=%s\n' "$bpw" "$src_host" "$src_port" >"$tmp/backup.cnf"; chmod 600 "$tmp/backup.cnf"
printf '[client]\nuser=vgbk_u\npassword=%s\nhost=%s\nport=%s\n' "$bpw" "$src_host" "$src_port" >"$tmp/loose.cnf"; chmod 644 "$tmp/loose.cnf"
"$bin/mysql" --defaults-extra-file="$tmp/backup.cnf" -e "DELETE FROM \`$db\`.items" 2>/dev/null; check "the backup account cannot write or delete" '[[ $(root_sql "SELECT COUNT(*) FROM \`$db\`.items") == 2 ]]'

echo "scratch restore target: a second real mysqld"
tp=$(port)
"$bin/mysqld" --no-defaults --initialize-insecure --datadir="$tmp/tdata" --log-error="$tmp/tinit.log" >/dev/null 2>&1
"$bin/mysqld" --no-defaults --datadir="$tmp/tdata" --port="$tp" --socket="$tmp/t.sock" --mysqlx=OFF --bind-address=127.0.0.1 --pid-file="$tmp/t.pid" --log-error="$tmp/t.log" --log-bin="$tmp/tbin" --server-id=77 >/dev/null 2>&1 &
mypid=$!
for i in $(seq 1 60); do "$bin/mysqladmin" --no-defaults -h127.0.0.1 -P"$tp" -uroot ping >/dev/null 2>&1 && break; sleep 0.5; done
printf '[client]\nuser=root\nhost=127.0.0.1\nport=%s\n' "$tp" >"$tmp/target.cnf"; chmod 600 "$tmp/target.cnf"
tsql() { "$bin/mysql" --defaults-extra-file="$tmp/target.cnf" -N -e "$1"; }
if [[ $(tsql "SELECT 1" 2>/dev/null) != 1 ]]; then
  echo "SKIP: a scratch mysqld could not be started here (AppArmor/permissions?); the restore half needs a second server. Last log lines:"; tail -5 "$tmp/t.log" 2>/dev/null; exit 0
fi

# --- backup environment (as install would lay it out) ---
off=$tmp/offhost; mkdir -p "$off"
cat >"$tmp/backup.env" <<ENV
VG_BACKUP_DIR=$tmp/store
VG_BACKUP_BIN=$api
VG_BACKUP_PUBKEY=$tmp/keys/public.pem
VG_BACKUP_UPLOAD_CMD=cp "\$1" $off/
VG_BACKUP_MYSQL_DEFAULTS=$tmp/backup.cnf
VG_BACKUP_DATABASE=$db
VG_BACKUP_MYSQL_BIN_DIR=$bin
VG_BACKUP_METRICS=$tmp/backup.prom
VG_BACKUP_RETENTION_DAYS=7
ENV
export VG_BACKUP_ENV=$tmp/backup.env
B() { bash "$bk/backup.sh" "$@" 2>"$tmp/last.err"; }
# restore.sh runs on the restore machine and reads no host env file: tell it where the tools are
export VG_BACKUP_MYSQL_BIN_DIR=$bin VG_BACKUP_BIN=$api
R() { bash "$bk/restore.sh" "$@" 2>"$tmp/last.err"; }
cnt() { tsql "SELECT COUNT(*) FROM \`$db\`.$1" 2>/dev/null; }

echo "case 1: full backup"
B mysql-full; rc=$?
check "full backup succeeds" "[[ $rc -eq 0 ]] || { cat $tmp/last.err; false; }"
full=$(ls "$tmp"/store/mysql-full-*.vgbk 2>/dev/null | head -1)
check "artifact and manifest exist; directory 0700, artifact 0600" "[[ -f $full && -f $full.manifest && \$(mode $tmp/store) == 700 && \$(mode $full) == 600 ]]"
check "manifest records the binlog position" "grep -qE '^binlog_file=.+\.[0-9]+$' $full.manifest && grep -qE '^binlog_pos=[0-9]+$' $full.manifest"
check "it was shipped off the host" "ls $off/mysql-full-*.vgbk >/dev/null 2>&1 && ls $off/mysql-full-*.manifest >/dev/null 2>&1"
check "NO plaintext: neither table names, row data, SQL nor a gzip header appear in the artifact" "! grep -aq -e SECRETMARK -e 'CREATE TABLE' -e items -e INSERT $full && [[ \$(head -c 2 $full | od -An -tx1 | tr -d ' \n') != 1f8b ]]"
check "metrics show a success for mysql-full" "grep -q 'veyrogram_backup_last_success_timestamp_seconds{kind=\"mysql-full\"}' $tmp/backup.prom"

echo "case 2: changes, then an accident, with binlog backups in between"
root_sql "INSERT INTO \`$db\`.items (note) VALUES ('after-full-A'),('after-full-B')"
B mysql-binlog; check "first binlog shipment" "[[ \$? -eq 0 ]] && ls $tmp/store/binlog-*.vgbk >/dev/null 2>&1"
root_sql "INSERT INTO \`$db\`.items (note) VALUES ('after-full-C')"
sleep 2; t_good=$(date -u '+%Y-%m-%d %H:%M:%S'); sleep 2
"$bin/mysql" --defaults-extra-file="$tmp/root.cnf" -D "$db" -e "DELETE FROM items; DROP TABLE other"   # the accident, run the way the application runs SQL: with its database selected
sleep 1
B mysql-binlog; check "second binlog shipment (includes the accident)" "[[ \$? -eq 0 ]]"
nb=$(ls "$tmp"/store/binlog-*.vgbk | wc -l | tr -d ' ')
B mysql-binlog; check "a run with nothing new ships nothing twice" "[[ \$(ls $tmp/store/binlog-*.vgbk | wc -l | tr -d ' ') -ge $nb ]] && ! ls $tmp/store/binlog-*.vgbk | sort | uniq -d | grep -q ."

echo "case 3: point-in-time recovery to just before the accident"
R mysql -d "$tmp/store" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" -u "$t_good"; rc=$?
check "restore succeeds" "[[ $rc -eq 0 ]] || { cat $tmp/last.err; false; }"
check "rows: both before-full rows, plus A, B and C that came after the full (5 total)" "[[ \$(cnt items) == 5 ]]"
check "the table the accident dropped is back" "[[ \$(cnt other) == 1 ]]"
check "the accident itself was not replayed" "[[ \$(tsql \"SELECT COUNT(*) FROM \\\`$db\\\`.items WHERE note='after-full-C'\") == 1 ]]"
check "restore says how long it took (RTO evidence)" "grep -q 'total [0-9]*s' $tmp/last.err"
R mysql -d "$tmp/store" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" -u "$t_good" 2>/dev/null; check "restoring over a non-empty database is refused" "[[ \$? -ne 0 ]] && grep -q 'already has' $tmp/last.err || grep -q 'already has' $tmp/last.err"
tsql "DROP DATABASE \`$db\`"
R mysql -d "$tmp/store" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf"
check "without -u the whole log is replayed: the accident IS reproduced (items empty, table dropped)" "[[ \$(cnt items) == 0 ]] && [[ -z \$(cnt other) ]]"
tsql "DROP DATABASE \`$db\`"

echo "case 4: refusals happen BEFORE anything is loaded"
cp -R "$tmp/store" "$tmp/bad1"; f=$(ls "$tmp"/bad1/mysql-full-*.vgbk | head -1)
python3 - "$f" <<'PY'
import sys; p=sys.argv[1]; b=bytearray(open(p,'rb').read()); b[len(b)//2]^=1; open(p,'wb').write(b)
PY
R mysql -d "$tmp/bad1" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" 2>/dev/null; rc=$?
check "a damaged full backup is caught by its manifest checksum first" "[[ $rc -ne 0 ]] && grep -q 'checksum differs' $tmp/last.err"
rm -f "$f.manifest"
R mysql -d "$tmp/bad1" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" 2>/dev/null; rc=$?
check "without a manifest, a flipped bit in the full backup aborts with an authentication error and an empty target" "[[ $rc -ne 0 ]] && grep -q 'authentication failed' $tmp/last.err && [[ \$(tsql \"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$db'\") == 0 ]]"
cp -R "$tmp/store" "$tmp/bad4"; for m in "$tmp"/bad4/mysql-full-*.manifest; do sed -i.bak 's/^binlog_pos=.*/binlog_pos=999999/; s/^binlog_file=.*/binlog_file=binlog.999999/' "$m"; rm -f "$m.bak"; done
tsql "DROP DATABASE IF EXISTS \`$db\`"; R mysql -d "$tmp/bad4" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" -u "$t_good"; rc=$?
check "a tampered MANIFEST cannot move the replay start: position and database come from the authenticated dump (result still correct)" "[[ $rc -eq 0 ]] && [[ \$(cnt items) == 5 ]]"
tsql "DROP DATABASE IF EXISTS \`$db\`"
cp -R "$tmp/store" "$tmp/bad2"; f=$(ls "$tmp"/bad2/binlog-*.vgbk | tail -1); rm -f "$f.manifest"; head -c $(( $(wc -c <"$f") - 40 )) "$f" >"$f.cut"; mv "$f.cut" "$f"
R mysql -d "$tmp/bad2" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" 2>/dev/null; rc=$?
check "a truncated binlog aborts BEFORE the (good) full backup is loaded" "[[ $rc -ne 0 ]] && [[ \$(tsql \"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$db'\") == 0 ]]"
cp -R "$tmp/store" "$tmp/bad3"; first=$(ls "$tmp"/bad3/binlog-*.vgbk | head -1); nbl=$(ls "$tmp"/bad3/binlog-*.vgbk | wc -l | tr -d ' ')
if (( nbl >= 2 )); then rm -f "$first" "$first.manifest"; R mysql -d "$tmp/bad3" -k "$tmp/keys/private.pem" -t "$tmp/target.cnf" 2>/dev/null; rc=$?
  check "a missing binlog (gap in the chain) is refused, nothing loaded" "[[ $rc -ne 0 ]] && grep -q 'gap' $tmp/last.err && [[ \$(tsql \"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$db'\") == 0 ]]"
else bad "need at least two binlog files for the gap test (got $nbl)"; fi
R mysql -d "$tmp/store" -k "$tmp/otherkeys/private.pem" -t "$tmp/target.cnf" 2>/dev/null; check "the wrong private key cannot restore" "[[ \$? -ne 0 ]]"
R verify -d "$tmp/store" -k "$tmp/keys/private.pem"; check "verify authenticates every artifact with the right key" "[[ \$? -eq 0 ]] && grep -q 'all [0-9]* artifact' $tmp/last.err"
R verify -d "$tmp/bad1" -k "$tmp/keys/private.pem" 2>/dev/null; check "verify reports the damaged artifact" "[[ \$? -ne 0 ]] && grep -q BAD $tmp/last.err"
R verify -d "$tmp/store" -k "$tmp/otherkeys/private.pem" 2>/dev/null; check "verify with the wrong key fails" "[[ \$? -ne 0 ]]"

echo "case 5: safety rails on the backup side"
sed -i.bak 's#^VG_BACKUP_UPLOAD_CMD=.*#VG_BACKUP_UPLOAD_CMD=#' "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
B mysql-full; rc=$?
check "without an off-host destination the run FAILS (a local-only backup is not a backup)" "[[ $rc -ne 0 ]] && grep -q 'refusing' $tmp/last.err"
check "...and the failure is in the metrics, with the earlier success still visible" "grep -q 'last_failure_timestamp_seconds{kind=\"mysql-full\"}' $tmp/backup.prom && grep -q 'last_success_timestamp_seconds{kind=\"mysql-full\"}' $tmp/backup.prom"
sed -i.bak "s#^VG_BACKUP_UPLOAD_CMD=.*#VG_BACKUP_UPLOAD_CMD=cp \"\$1\" $off/#" "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
sed -i.bak "s#^VG_BACKUP_MYSQL_DEFAULTS=.*#VG_BACKUP_MYSQL_DEFAULTS=$tmp/loose.cnf#" "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
B mysql-full; check "a password file readable by others is refused" "[[ \$? -ne 0 ]] && grep -q '0600' $tmp/last.err"
sed -i.bak "s#^VG_BACKUP_MYSQL_DEFAULTS=.*#VG_BACKUP_MYSQL_DEFAULTS=$tmp/backup.cnf#" "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
sed -i.bak "s#^VG_BACKUP_PUBKEY=.*#VG_BACKUP_PUBKEY=$tmp/missing.pem#" "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
B mysql-full; check "a missing public key fails instead of writing an unencrypted backup" "[[ \$? -ne 0 ]] && ! ls $tmp/store/mysql-full-*.sql* 2>/dev/null | grep -qv vgbk"
sed -i.bak "s#^VG_BACKUP_PUBKEY=.*#VG_BACKUP_PUBKEY=$tmp/keys/public.pem#" "$tmp/backup.env"; rm -f "$tmp/backup.env.bak"
check "the host's environment holds only the PUBLIC key" "! grep -q 'BEGIN PRIVATE' $tmp/keys/public.pem && ! grep -rq 'PRIVATE KEY' $tmp/store"

echo "case 6: retention never deletes what has not left the host"
old=$(ls "$tmp"/store/binlog-*.vgbk | head -1); touch -t 202001010000 "$old"; touch -t 202001010000 "$tmp/store/$(basename "$(ls "$tmp"/store/binlog-*.vgbk | tail -1)")"
last=$(ls "$tmp"/store/binlog-*.vgbk | tail -1); rm -f "$last.uploaded"
B prune
check "an old shipped copy is removed" "[[ ! -e $old ]]"
check "an old UNSHIPPED copy is kept" "[[ -e $last ]]"

echo; echo "backup rehearsal: $pass passed, $fail failed"; exit $fail
