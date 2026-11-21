#!/usr/bin/env bash
# Encrypted, off-host backups (Story 1.22; ARCHITECTURE AD-22).
#   backup.sh mysql-full      consistent logical dump (single transaction, no locks on writers) + the binlog position it corresponds to
#   backup.sh mysql-binlog    close the active binlog and ship every closed one not yet shipped -> point-in-time recovery between fulls
#   backup.sh im              WuKongIM data directory: brief stop, local copy, restart, then encrypt + ship the copy
#   backup.sh prune           delete LOCAL copies that were shipped and are older than VG_BACKUP_RETENTION_DAYS (never an unshipped one)
# Every artifact is encrypted to a PUBLIC key before it is written (the private key is never on this host), written 0600 in a 0700 directory, and shipped by
# VG_BACKUP_UPLOAD_CMD (called as: bash -c "$CMD" _ <file>). Settings: see backup.env.template. The source is only read; a failure never touches the live service
# except that `im` stops the IM for the length of one local copy and ALWAYS starts it again.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
kind=${1:-}; setup
[[ -n ${VG_BACKUP_MYSQL_DEFAULTS:-} ]] || [[ $kind == im || $kind == prune ]] || die "VG_BACKUP_MYSQL_DEFAULTS (a my.cnf with the backup account) is required"
if [[ -n ${VG_BACKUP_MYSQL_DEFAULTS:-} ]]; then secret_file_ok "$VG_BACKUP_MYSQL_DEFAULTS" || die "$VG_BACKUP_MYSQL_DEFAULTS holds a password and must be mode 0600"; fi
mycnf() { printf -- '--defaults-extra-file=%s' "$VG_BACKUP_MYSQL_DEFAULTS"; }
ts=$(date -u +%Y%m%dT%H%M%SZ)

# One EXIT trap for everything: record a failure (die/exit never fire an ERR trap), start the IM again if this run stopped it, remove scratch files.
started=0; work=; startcmd=
on_exit() {
  local rc=$?
  if [[ $started == 1 ]]; then bash -c "$startcmd" || log "ERROR: could not start the IM again - start it by hand NOW"; fi
  [[ -z $work ]] || rm -rf "$work"
  if (( rc != 0 )); then [[ $kind != mysql-full && $kind != mysql-binlog && $kind != im ]] || record_failure "$kind"; log "FAILED ($kind), exit $rc"; fi
}
trap on_exit EXIT

case $kind in
mysql-full)
  name=mysql-full-$ts; out=$VG_BACKUP_DIR/$name.sql.gz.vgbk; pos=$VG_BACKUP_DIR/tmp/$name.pos; rm -f "$pos"
  # --flush-logs + --source-data=2: the dump starts at the first event of a fresh binlog file and says which; awk passes every line through and keeps the position line.
  "$MYSQLDUMP" "$(mycnf)" --single-transaction --routines --triggers --events --no-tablespaces --hex-blob --flush-logs --source-data=2 --set-gtid-purged=OFF \
      --databases "$VG_BACKUP_DATABASE" \
    | awk -v pos="$pos" 'NR<=200 && /SOURCE_LOG_FILE|MASTER_LOG_FILE/ && !seen { print > pos; seen=1 } { print }' \
    | gzip -c | encrypt_to "$out"
  [[ -s $pos ]] || die "the dump did not record a binlog position (is binary logging on, and the server MySQL 8.0.26 or newer?)"
  file=$(sed -E "s/.*(SOURCE|MASTER)_LOG_FILE='([^']+)'.*/\2/" "$pos"); lp=$(sed -E "s/.*(SOURCE|MASTER)_LOG_POS=([0-9]+).*/\2/" "$pos"); rm -f "$pos"
  write_manifest "$out" "kind=mysql-full" "database=$VG_BACKUP_DATABASE" "binlog_file=$file" "binlog_pos=$lp"
  upload "$out"; upload "$out.manifest"
  record_success mysql-full "$(size_of "$out")"; log "mysql-full ok: $(basename "$out") (binlog $file:$lp)"
  ;;
mysql-binlog)
  latest=$(ls -1 "$VG_BACKUP_DIR"/mysql-full-*.manifest 2>/dev/null | sort | tail -1 || true)
  [[ -n $latest ]] || die "no full backup exists yet; run mysql-full first (binlogs are only meaningful on top of one)"
  startfile=$(manifest_get "$latest" binlog_file)
  "$MYSQL" "$(mycnf)" -N -e "FLUSH BINARY LOGS"
  list=$("$MYSQL" "$(mycnf)" -N -e "SHOW BINARY LOGS" | awk '{print $1}')
  active=$(tail -1 <<<"$list"); shipped=$VG_BACKUP_DIR/state/binlog.shipped; touch "$shipped"
  n=0; bytes=0
  while IFS= read -r f; do
    [[ -n $f && $f != "$active" ]] || continue
    [[ $f > $startfile || $f == "$startfile" ]] || continue       # older than the full: not needed for restore
    grep -qxF "$f" "$shipped" && continue
    work=$VG_BACKUP_DIR/tmp/binlog.$$; rm -rf "$work"; mkdir -p "$work"
    "$MYSQLBINLOG" "$(mycnf)" --read-from-remote-server --raw --result-file="$work/" "$f" >/dev/null
    out=$VG_BACKUP_DIR/binlog-$f.vgbk
    encrypt_to "$out" <"$work/$f"; rm -rf "$work"
    write_manifest "$out" "kind=mysql-binlog" "binlog_file=$f"
    upload "$out"; upload "$out.manifest"
    echo "$f" >>"$shipped"; n=$((n+1)); bytes=$((bytes + $(size_of "$out")))
  done <<<"$list"
  record_success mysql-binlog "$bytes"; log "mysql-binlog ok: shipped $n file(s); active file $active stays on the server until it closes"
  ;;
im)
  [[ -n ${VG_IM_DATA_DIR:-} && -d $VG_IM_DATA_DIR ]] || die "VG_IM_DATA_DIR must be the WuKongIM data directory"
  stopcmd=${VG_IM_STOP_CMD:-systemctl stop veyrogram-wukongim}; startcmd=${VG_IM_START_CMD:-systemctl start veyrogram-wukongim}
  quiesce=${VG_IM_QUIESCE:-stop}
  work=$VG_BACKUP_DIR/tmp/im.$$; rm -rf "$work"; mkdir -p "$work"
  case $quiesce in
  stop)   # the only procedure that is verified end to end (see the restore rehearsal): the process is stopped, so the files on disk are a clean state
    t0=$(now); started=1; bash -c "$stopcmd"
    cp -a "$VG_IM_DATA_DIR" "$work/data"                       # local copy only: the IM is down just for this
    bash -c "$startcmd"; started=0; downtime=$(( $(now) - t0 ))
    ;;
  none)   # the operator froze the filesystem or took a volume snapshot themselves and mounted it at VG_IM_DATA_DIR; nothing is stopped here
    downtime=0; cp -a "$VG_IM_DATA_DIR" "$work/data" ;;
  *) die "VG_IM_QUIESCE must be stop or none" ;;
  esac
  out=$VG_BACKUP_DIR/im-$ts.tar.gz.vgbk
  tar -C "$work" -cf - data | gzip -c | encrypt_to "$out"
  write_manifest "$out" "kind=im" "quiesce=$quiesce" "downtime_seconds=$downtime"
  upload "$out"; upload "$out.manifest"
  record_success im "$(size_of "$out")"; log "im ok: $(basename "$out") (IM was down ${downtime}s)"
  ;;
prune)
  days=$VG_BACKUP_RETENTION_DAYS; n=0
  for f in "$VG_BACKUP_DIR"/*.vgbk; do
    [[ -e $f && -e $f.uploaded ]] || continue                  # never delete something that has not left the host
    if [[ -n $(find "$f" -mtime +"$days" -print 2>/dev/null) ]]; then rm -f "$f" "$f.manifest" "$f.uploaded" "$f.manifest.uploaded"; n=$((n+1)); fi
  done
  log "prune: removed $n local copy(ies) older than $days day(s) that were already shipped"
  ;;
*) die "usage: backup.sh mysql-full | mysql-binlog | im | prune" ;;
esac
