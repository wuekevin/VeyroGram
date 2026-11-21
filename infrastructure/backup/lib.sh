# shellcheck shell=bash
# Shared helpers for backup.sh / restore.sh (sourced). bash 3.2 compatible (the rehearsals run on macOS as well as Linux).
VG_BACKUP_ENV=${VG_BACKUP_ENV:-/etc/veyrogram/backup.env}

log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

# Load VG_BACKUP_* / VG_IM_* settings from the env file. Parsed line by line, never sourced: values may contain & ; ( ) and must not execute.
load_env() {
  local f=$VG_BACKUP_ENV line k v
  [[ -r $f ]] || return 0
  while IFS= read -r line || [[ -n $line ]]; do
    [[ -z ${line//[[:space:]]/} || $line == \#* ]] && continue
    k=${line%%=*}; v=${line#*=}
    v=${v%\"}; v=${v#\"}; v=${v%\'}; v=${v#\'}
    [[ $k =~ ^VG_(BACKUP|IM)_[A-Z0-9_]+$ ]] || continue
    [[ -n ${!k:-} ]] || export "$k=$v"      # an explicit environment variable wins over the file
  done <"$f"
}

# A file holding a password must not be readable by anyone else.
secret_file_ok() { local m; m=$(stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"); [[ $(( 8#$m & 8#077 )) -eq 0 ]]; }

setup() {
  load_env
  umask 077
  : "${VG_BACKUP_DIR:=/var/backups/veyrogram}"
  : "${VG_BACKUP_BIN:=/opt/veyrogram/api/current/veyrogram-api}"
  : "${VG_BACKUP_DATABASE:=veyrogram}"
  : "${VG_BACKUP_RETENTION_DAYS:=7}"
  : "${VG_BACKUP_REQUIRE_OFFHOST:=1}"
  local bd=${VG_BACKUP_MYSQL_BIN_DIR:-}; [[ -z $bd ]] || bd=${bd%/}/
  MYSQL=${bd}mysql; MYSQLDUMP=${bd}mysqldump; MYSQLBINLOG=${bd}mysqlbinlog
  mkdir -p "$VG_BACKUP_DIR/state" "$VG_BACKUP_DIR/tmp"; chmod 0700 "$VG_BACKUP_DIR" "$VG_BACKUP_DIR/state" "$VG_BACKUP_DIR/tmp"
}

sha256_of() { (sha256sum "$1" 2>/dev/null || shasum -a 256 "$1") | awk '{print $1}'; }
size_of() { wc -c <"$1" | tr -d ' '; }
now() { date +%s; }

# ---- metrics: regenerated from state files so a later success never hides an earlier failure's timestamp, and a stopped timer shows as a stale success ----
write_metrics() {
  local out=${VG_BACKUP_METRICS:-}; [[ -n $out ]] || return 0
  local tmp k f; tmp=$(mktemp "$(dirname "$out")/.backup.XXXXXX") || return 0
  {
    echo '# TYPE veyrogram_backup_last_success_timestamp_seconds gauge'
    for f in "$VG_BACKUP_DIR"/state/*.success; do [[ -e $f ]] || continue; k=$(basename "$f" .success); printf 'veyrogram_backup_last_success_timestamp_seconds{kind="%s"} %s\n' "$k" "$(cat "$f")"; done
    echo '# TYPE veyrogram_backup_last_failure_timestamp_seconds gauge'
    for f in "$VG_BACKUP_DIR"/state/*.failure; do [[ -e $f ]] || continue; k=$(basename "$f" .failure); printf 'veyrogram_backup_last_failure_timestamp_seconds{kind="%s"} %s\n' "$k" "$(cat "$f")"; done
    echo '# TYPE veyrogram_backup_last_size_bytes gauge'
    for f in "$VG_BACKUP_DIR"/state/*.size; do [[ -e $f ]] || continue; k=$(basename "$f" .size); printf 'veyrogram_backup_last_size_bytes{kind="%s"} %s\n' "$k" "$(cat "$f")"; done
  } >"$tmp"
  chmod 0644 "$tmp"; mv "$tmp" "$out"
}
record_success() { now >"$VG_BACKUP_DIR/state/$1.success"; [[ -z ${2:-} ]] || echo "$2" >"$VG_BACKUP_DIR/state/$1.size"; write_metrics; }
record_failure() { now >"$VG_BACKUP_DIR/state/$1.failure"; write_metrics; }

# encrypt_to <out file>: stdin -> encrypted file. Only the public key is needed (and present) on this host.
encrypt_to() {
  [[ -n ${VG_BACKUP_PUBKEY:-} && -r $VG_BACKUP_PUBKEY ]] || die "VG_BACKUP_PUBKEY must point at the public key (made with 'veyrogram-api backup keygen' on YOUR machine)"
  "$VG_BACKUP_BIN" backup encrypt -pub "$VG_BACKUP_PUBKEY" -out "$1"
}

# upload <file>: ship it off the host. A backup that never leaves the machine it protects is not a backup, so by default the run FAILS without a destination.
upload() {
  local f=$1
  if [[ -n ${VG_BACKUP_UPLOAD_CMD:-} ]]; then
    bash -c "$VG_BACKUP_UPLOAD_CMD" _ "$f" || die "upload of $(basename "$f") failed"
    : >"$f.uploaded"
  elif [[ $VG_BACKUP_REQUIRE_OFFHOST == 1 ]]; then
    die "VG_BACKUP_UPLOAD_CMD is not set: refusing to call a backup that stays on the host it protects a success (set VG_BACKUP_REQUIRE_OFFHOST=0 only for a rehearsal)"
  fi
}

# write_manifest <artifact> <key=value>...: a small plain-text record next to the artifact (names, times, binlog position; no secrets, no data)
write_manifest() {
  local art=$1; shift
  { printf 'artifact=%s\n' "$(basename "$art")"; printf 'created=%s\n' "$(date -u +%FT%TZ)"; printf 'size=%s\n' "$(size_of "$art")"; printf 'sha256=%s\n' "$(sha256_of "$art")"; for kv in "$@"; do printf '%s\n' "$kv"; done; } >"$art.manifest"
}
manifest_get() { grep -E "^$2=" "$1" | head -1 | cut -d= -f2-; }
