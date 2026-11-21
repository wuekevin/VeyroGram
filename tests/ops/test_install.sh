#!/usr/bin/env bash
# Rehearsal of infrastructure/scripts/install.sh in a throwaway root with the REAL binary: generated secrets, file modes, idempotence, and that every
# environment template is a configuration the service accepts (production/staging once the operator-only values are filled in).
# This does NOT replace the drill on a clean Linux host (accounts, systemd, journald) tracked in docs/ops/MANUAL-CHECKLIST.md.
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); scripts=$root/infrastructure/scripts
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
fail=0; pass=0
ok()   { pass=$((pass+1)); echo "  ok   - $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL - $1"; }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }
mode() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }
val()  { grep -E "^$2=" "$1" | head -1 | cut -d= -f2-; }

echo "building real binary"
bin=$tmp/veyrogram-api
(cd "$root/services/api" && go build -o "$bin" ./cmd/api) || { echo "build failed"; exit 1; }
. "$scripts/lib.sh"

fill_prod() { # operator-only values, as a human would fill them
  sed -i.bak -e "s#@@FILL:database DSN of the application account@@#veyrogram_app:Pa55-from-operator@tcp(127.0.0.1:3306)/$2?parseTime=true#" \
             -e 's#@@FILL:SMS provider account@@#smsuser#' -e 's#@@FILL:SMS provider API key@@#smskey-0123456789#' "$1"; rm -f "$1.bak"
}
checkcfg() { VG_ENV_FILE=$1 preflight "$bin" 2>/dev/null; }

for envname in production staging; do
  echo "== $envname"
  r=$tmp/$envname; out=$tmp/$envname.out
  VG_INSTALL_ROOT=$r bash "$scripts/install.sh" "$envname" >"$out" 2>&1; rc=$?
  f=$r/etc/veyrogram/api.env
  check "install exits 0" "[[ $rc -eq 0 ]]"
  check "env file mode is 0640" "[[ \$(mode $f) == 640 ]]"
  check "directories are 0750" "[[ \$(mode $r/etc/veyrogram) == 750 && \$(mode $r/var/log/veyrogram) == 750 && \$(mode $r/var/backups/veyrogram) == 700 && \$(mode $r/var/backups/veyrogram-im) == 700 ]]"
  check "no @@GEN placeholder is left" "! grep -q '@@GEN:' $f"
  check "operator-only values are listed by name" "grep -q 'SMS provider API key' $out && grep -q 'database DSN of the application account' $out"
  check "the service REFUSES the file while @@FILL values remain" "! checkcfg $f"
  # generated secrets: long, all different
  secrets=(); for k in VG_REDIS_PASSWORD VG_IM_TOKEN_KEY VG_IM_WEBHOOK_SECRET VG_IM_WEBHOOK_SIGNING_SECRET VG_PHONE_HASH_KEY VG_METRICS_TOKEN VG_ADMIN_TOTP_KEY; do secrets+=("$(val "$f" $k)"); done
  check "every generated secret is at least 32 characters" "[[ \$(printf '%s\n' \"\${secrets[@]}\" | awk 'length<32' | wc -l) -eq 0 ]]"
  check "generated secrets are all different" "[[ \$(printf '%s\n' \"\${secrets[@]}\" | sort -u | wc -l) -eq \${#secrets[@]} ]]"
  check "no secret value appears in the installer's output" "! grep -qF -e \"\${secrets[0]}\" -e \"\${secrets[1]}\" -e \"\${secrets[4]}\" $out"
  fill_prod "$f" "$([[ $envname == staging ]] && echo veyrogram_staging || echo veyrogram)"
  check "once the operator fills their values, the service ACCEPTS the $envname configuration" "checkcfg $f"
  sum1=$(cksum <"$f")
  VG_INSTALL_ROOT=$r bash "$scripts/install.sh" "$envname" >"$tmp/again.out" 2>&1
  check "second run leaves the env file untouched" "[[ \$(cksum <$f) == \"$sum1\" ]]"
  check "second run says it kept the file" "grep -q 'kept existing' $tmp/again.out"
  check "canary overrides file is installed 0640 and moves every listener" "[[ \$(mode $r/etc/veyrogram/canary.env) == 640 ]] && grep -q 'VG_HTTP_ADDR=127.0.0.1:8081' $r/etc/veyrogram/canary.env && grep -q 'VG_MANAGER_ENABLED=false' $r/etc/veyrogram/canary.env"
  check "/etc/veyrogram stays 0750 while releases are world-readable (no secrets there) so the backup accounts can run the binary" "[[ \$(mode $r/opt/veyrogram/api) == 755 && \$(mode $r/opt/veyrogram) == 755 ]]"
  check "backup scripts, env templates (0640), units, timers and the polkit rule are installed" "[[ -x $r/opt/veyrogram/backup/current/backup.sh && -x $r/opt/veyrogram/backup/current/restore.sh && \$(mode $r/etc/veyrogram/backup.env) == 640 && \$(mode $r/etc/veyrogram/backup-im.env) == 640 && -f $r/etc/systemd/system/veyrogram-backup-mysql-full.timer && -f $r/etc/systemd/system/veyrogram-backup-im.service && -f $r/etc/polkit-1/rules.d/50-veyrogram-backup.rules ]]"
  check "the polkit rule allows only stopping/starting the IM unit, for the IM account" "grep -q 'veyrogram-wukongim.service' $r/etc/polkit-1/rules.d/50-veyrogram-backup.rules && grep -q 'subject.user == \"veyrogram-im\"' $r/etc/polkit-1/rules.d/50-veyrogram-backup.rules"
  check "canary and probe units (and timer) are installed" "[[ -f $r/etc/systemd/system/veyrogram-api-canary.service && -f $r/etc/systemd/system/veyrogram-probe.service && -f $r/etc/systemd/system/veyrogram-probe.timer ]]"
  check "units, journald limits and logrotate are installed" "[[ -f $r/etc/systemd/system/veyrogram-api.service && -f $r/etc/systemd/system/veyrogram-worker.service && -f $r/etc/systemd/journald.conf.d/veyrogram.conf && -f $r/etc/logrotate.d/veyrogram ]]"
done

echo "== two installs never share secrets"
a=$(val "$tmp/production/etc/veyrogram/api.env" VG_PHONE_HASH_KEY); b=$(val "$tmp/staging/etc/veyrogram/api.env" VG_PHONE_HASH_KEY)
check "production and staging secrets differ" "[[ -n $a && $a != $b ]]"
check "staging is its own database, Redis and ports" "grep -q 'veyrogram_staging' $tmp/staging/etc/veyrogram/api.env && grep -q 'VG_REDIS_ADDR=127.0.0.1:6380' $tmp/staging/etc/veyrogram/api.env && grep -q 'VG_ENV=staging' $tmp/staging/etc/veyrogram/api.env"

for envname in development test; do
  echo "== $envname"
  r=$tmp/$envname
  VG_INSTALL_ROOT=$r bash "$scripts/install.sh" "$envname" >"$tmp/$envname.out" 2>&1
  check "$envname config is accepted as rendered (nothing to fill)" "checkcfg $r/etc/veyrogram/api.env"
  check "$envname has no placeholders left" "! grep -q '@@' $r/etc/veyrogram/api.env"
done
check "test and development use different databases" "! diff -q <(grep MYSQL_DSN $tmp/development/etc/veyrogram/api.env | sed 's/:[^@]*@/@/') <(grep MYSQL_DSN $tmp/test/etc/veyrogram/api.env | sed 's/:[^@]*@/@/') >/dev/null"

echo "== refusals"
VG_INSTALL_ROOT=$tmp/x bash "$scripts/install.sh" prod >/dev/null 2>&1; check "unknown environment name is refused (exit 2)" "[[ \$? -eq 2 ]]"
check "production env file never enables the fixed SMS code, the log SMS provider or the admin plane by default" "! grep -qE '^(VG_SMS_FIXED_CODE|VG_ADMIN_PASSWORD)=' $tmp/production/etc/veyrogram/api.env && grep -q '^VG_SMS_PROVIDER=smsbao' $tmp/production/etc/veyrogram/api.env && grep -q '^VG_MANAGER_ENABLED=false' $tmp/production/etc/veyrogram/api.env"

echo "== log rotation"
if command -v logrotate >/dev/null 2>&1; then
  me=$(id -un); gr=$(id -gn)
  sed -e "s#^    su .*#    su $me $gr#" -e "s#^    create .*#    create 0640 $me $gr#" -e "s#/var/log/#$tmp/logs/#g" "$root/infrastructure/logrotate/veyrogram" >"$tmp/lr.conf"
  mkdir -p "$tmp/logs/veyrogram" "$tmp/logs/veyrogram-im"; head -c 4096 /dev/urandom | base64 >"$tmp/logs/veyrogram/a.log"; chmod 600 "$tmp/lr.conf"
  check "logrotate accepts the config" "logrotate -d -s $tmp/lr.state $tmp/lr.conf >$tmp/lr.out 2>&1"
  logrotate -f -s "$tmp/lr.state" "$tmp/lr.conf" >/dev/null 2>&1
  check "a forced rotation keeps a dated copy and leaves a live (truncated) log" "ls $tmp/logs/veyrogram/ | grep -q 'a.log-' && [[ -f $tmp/logs/veyrogram/a.log ]]"
else
  echo "  skip - logrotate not installed on this machine (CI runs it)"
fi
check "journald drop-in bounds the journal" "grep -q '^SystemMaxUse=' $root/infrastructure/systemd/journald-veyrogram.conf && grep -q '^MaxRetentionSec=' $root/infrastructure/systemd/journald-veyrogram.conf"

echo; echo "install rehearsal: $pass passed, $fail failed"; exit $fail
