#!/usr/bin/env bash
# Restore from the encrypted backups (run on the RESTORE machine, which holds the private key; the backup host never does).
#   restore.sh mysql  -d <backup dir> -k <private.pem> -t <my.cnf for the TARGET server> [-u 'YYYY-MM-DD HH:MM:SS' (UTC)] [-D database]
#       newest full backup not after -u (or the newest), then the binlogs after it replayed up to -u (point-in-time recovery). Refuses a target that already
#       has tables in the database: restore into an empty server or a scratch database, never over live data.
#   restore.sh im     -f <im-*.tar.gz.vgbk> -k <private.pem> -o <empty dir>      unpack a WuKongIM data directory snapshot
#   restore.sh verify -d <backup dir> -k <private.pem>                           decrypt and authenticate EVERY artifact (the quarterly drill's first step)
# Everything is decrypted into a private temp directory and used only after the whole stream authenticated; a tampered or truncated artifact aborts the restore
# before anything is loaded. Prints how long each phase took (RTO evidence). Settings: VG_BACKUP_BIN, VG_BACKUP_MYSQL_BIN_DIR.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
umask 077
VG_BACKUP_BIN=${VG_BACKUP_BIN:-/opt/veyrogram/api/current/veyrogram-api}
bd=${VG_BACKUP_MYSQL_BIN_DIR:-}; [[ -z $bd ]] || bd=${bd%/}/
MYSQL=${bd}mysql; MYSQLBINLOG=${bd}mysqlbinlog
export TZ=UTC                                    # mysqlbinlog reads --stop-datetime in the local zone; fix it so -u means UTC everywhere
mode=${1:-}; shift || true
dir=; key=; target=; until=; db=; file=; outdir=
while getopts "d:k:t:u:D:f:o:" o; do case $o in d) dir=$OPTARG;; k) key=$OPTARG;; t) target=$OPTARG;; u) until=$OPTARG;; D) db=$OPTARG;; f) file=$OPTARG;; o) outdir=$OPTARG;; *) die "bad option";; esac; done
[[ -r $key ]] || die "-k <private.pem> is required"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/vgrestore.XXXXXX"); trap 'rm -rf "$tmp"' EXIT
dec() { "$VG_BACKUP_BIN" backup decrypt -key "$key" -in "$1" -out "$2" || die "$(basename "$1"): authentication failed - the file is corrupt, truncated, tampered with, or was made for another key; nothing was restored"; }
check_manifest() { # <artifact>: the shipped copy must still be the one that was made
  local m=$1.manifest; [[ -f $m ]] || return 0
  [[ $(manifest_get "$m" sha256) == "$(sha256_of "$1")" ]] || die "$(basename "$1"): checksum differs from its manifest (damaged in storage or in transfer)"
}

case $mode in
mysql)
  [[ -d $dir && -r $target ]] || die "-d <backup dir> and -t <target my.cnf> are required"
  t0=$(now)
  # Newest full not after -u. The time comes from the file name (UTC); the manifest is NOT trusted for anything but a damage check, because it is not authenticated.
  full=; uiso=; [[ -z $until ]] || uiso=$(date -u -d "$until" +%FT%TZ 2>/dev/null || date -u -j -f '%Y-%m-%d %H:%M:%S' "$until" +%FT%TZ)
  for a in $(ls -1 "$dir"/mysql-full-*.vgbk 2>/dev/null | sort); do
    c=$(basename "$a" .sql.gz.vgbk); c=${c#mysql-full-}; c=$(sed -E 's/^([0-9]{4})([0-9]{2})([0-9]{2})T([0-9]{2})([0-9]{2})([0-9]{2})Z$/\1-\2-\3T\4:\5:\6Z/' <<<"$c")
    [[ -z $uiso || $c < $uiso || $c == "$uiso" ]] && full=$a
  done
  [[ -n $full ]] || die "no full backup at or before ${until:-now} in $dir"
  check_manifest "$full"
  log "full backup: $(basename "$full")"
  # Authenticate EVERYTHING first (full and every binlog) and check the chain; only then touch the target, so a bad artifact never leaves a half-restored database.
  dec "$full" "$tmp/full.sql.gz"; t1=$(now)
  # Where the binlog replay starts, and which database this is, come from INSIDE the authenticated dump (its header), never from the unauthenticated manifest.
  posline=$(gzip -dc "$tmp/full.sql.gz" 2>/dev/null | awk 'NR<=200 && /SOURCE_LOG_FILE|MASTER_LOG_FILE/ {print; exit}' || true)
  [[ -n $posline ]] || die "the dump has no binlog position line; it was not made by backup.sh mysql-full"
  startfile=$(sed -E "s/.*(SOURCE|MASTER)_LOG_FILE='([^']+)'.*/\2/" <<<"$posline"); startpos=$(sed -E "s/.*(SOURCE|MASTER)_LOG_POS=([0-9]+).*/\2/" <<<"$posline")
  dumpdb=$(gzip -dc "$tmp/full.sql.gz" 2>/dev/null | awk 'NR<=200 && /^CREATE DATABASE/ {print; exit}' | sed -E 's/.*`([^`]+)`.*/\1/' || true)
  dbname=${db:-$dumpdb}; [[ -n $dbname ]] || die "cannot tell which database the dump holds"
  have=$("$MYSQL" --defaults-extra-file="$target" -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$dbname'")
  [[ $have == 0 ]] || die "the target already has $have table(s) in '$dbname': restore into an empty server or a scratch database"
  log "binlog replay starts at $startfile:$startpos"
  files=(); n=0
  for b in $(ls -1 "$dir"/binlog-*.vgbk 2>/dev/null | sort); do
    f=$(basename "$b" .vgbk); f=${f#binlog-}
    [[ $f > $startfile || $f == "$startfile" ]] || continue
    check_manifest "$b"; dec "$b" "$tmp/$f"; files+=("$tmp/$f"); n=$((n+1))
  done
  # the chain must be gapless from the full's own binlog file: a missing file would make point-in-time recovery silently skip changes
  prev=$((10#${startfile##*.} - 1))
  if (( n > 0 )); then for f in "${files[@]}"; do
    num=$((10#${f##*.})); [[ $num == $((prev+1)) ]] || die "binlog chain has a gap before $(basename "$f") (expected number $((prev+1))); nothing was restored"; prev=$num
  done; fi
  t2=$(now)
  gzip -dc "$tmp/full.sql.gz" | "$MYSQL" --defaults-extra-file="$target"; t3=$(now)
  log "full restored (decrypt+verify $((t2-t0))s, load $((t3-t2))s)"
  if (( n > 0 )); then
    opts=(--database="$dbname" --start-position="$startpos"); [[ -z $until ]] || opts+=(--stop-datetime="$until")
    "$MYSQLBINLOG" "${opts[@]}" "${files[@]}" | "$MYSQL" --defaults-extra-file="$target"
  fi
  log "restored '$dbname': full $(basename "$full") + $n binlog file(s)${until:+ up to $until UTC}; total $(( $(now) - t0 ))s"
  ;;
im)
  [[ -f $file && -n $outdir ]] || die "-f <im artifact> and -o <empty dir> are required"
  mkdir -p "$outdir"; [[ -z $(ls -A "$outdir") ]] || die "$outdir is not empty"
  check_manifest "$file"; dec "$file" "$tmp/im.tar.gz"
  gzip -dc "$tmp/im.tar.gz" | tar -C "$outdir" -xf -
  log "IM data restored to $outdir/data (start the IM against it; see the checklist for the post-restore reconciliation)"
  ;;
verify)
  [[ -d $dir ]] || die "-d <backup dir> is required"
  bad=0; n=0
  for a in "$dir"/*.vgbk; do
    [[ -e $a ]] || continue; n=$((n+1))
    if [[ -f $a.manifest && $(manifest_get "$a.manifest" sha256) != "$(sha256_of "$a")" ]]; then log "BAD   $(basename "$a"): checksum differs from its manifest"; bad=$((bad+1)); continue; fi
    if "$VG_BACKUP_BIN" backup decrypt -key "$key" -in "$a" -out "$tmp/x" 2>/dev/null; then log "ok    $(basename "$a")"; rm -f "$tmp/x"; else log "BAD   $(basename "$a"): does not authenticate"; bad=$((bad+1)); fi
  done
  (( n > 0 )) || die "no artifacts in $dir"
  (( bad == 0 )) || die "$bad of $n artifact(s) are bad"
  log "all $n artifact(s) decrypt and authenticate"
  ;;
*) die "usage: restore.sh mysql|im|verify (see the header)" ;;
esac
