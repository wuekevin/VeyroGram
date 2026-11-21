#!/usr/bin/env bash
# Host installer: service account, directories, environment file (secrets generated on the host), systemd units, log rotation.
#   install.sh <development|test|staging|production>
# Idempotent: running it again changes nothing that already exists, and it NEVER overwrites /etc/veyrogram/api.env (that file holds live secrets).
# Env: VG_INSTALL_ROOT (default /) prefixes every path; with a non-/ root no accounts are created and no ownership is changed (rehearsal mode).
# This script does not touch Nginx (宝塔 manages it; see render-nginx.sh) and does not start anything.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); infra=$(cd "$here/.." && pwd)
env_name=${1:-}; [[ $env_name =~ ^(development|test|staging|production)$ ]] || { echo "usage: install.sh <development|test|staging|production>" >&2; exit 2; }
ROOT=${VG_INSTALL_ROOT:-/}; ROOT=${ROOT%/}
real=0; [[ -z $ROOT ]] && real=1
SVC_USER=${VG_SERVICE_USER:-veyrogram}
log() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }

if (( real )); then
  [[ $(id -u) -eq 0 ]] || die "run as root (sudo)"
  [[ -d /run/systemd/system ]] || die "systemd is required"
  [[ $(uname -s) == Linux ]] || die "the production target is Linux"
fi
template=$infra/config/$env_name.env.template
[[ -f $template ]] || die "template $template not found"

# --- service account (no login, no home) ---
if (( real )) && ! id "$SVC_USER" >/dev/null 2>&1; then
  useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin "$SVC_USER"; log "created service account $SVC_USER"
fi
own() { (( real )) && chown "$@" || true; }   # ownership only matters on a real host

# --- directories ---
mk() { local mode=$1 owner=$2 dir=$3; mkdir -p "$ROOT$dir"; chmod "$mode" "$ROOT$dir"; own "$owner" "$ROOT$dir"; }
mk 0750 "root:$SVC_USER"          /etc/veyrogram
mk 0750 "$SVC_USER:$SVC_USER"     /opt/veyrogram/api
mk 0750 "$SVC_USER:$SVC_USER"     /var/lib/veyrogram
mk 0750 "$SVC_USER:$SVC_USER"     /var/log/veyrogram

# --- environment file: generated once, never overwritten ---
envfile=$ROOT/etc/veyrogram/api.env
b64url() { head -c 32 /dev/urandom | base64 | tr '+/' '-_' | tr -d '=\n'; }
hex32()  { head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }
gen() { case $1 in HEX32) hex32;; B64URL32|URLSAFE32) b64url;; *) die "unknown generator $1 in $template";; esac; }
if [[ -e $envfile ]]; then
  log "kept existing $envfile (not modified)"
else
  content=$(cat "$template"); re='@@GEN:([A-Z0-9]+)@@'
  while [[ $content =~ $re ]]; do content=${content/"${BASH_REMATCH[0]}"/"$(gen "${BASH_REMATCH[1]}")"}; done
  ( umask 077; tmp=$(mktemp "$ROOT/etc/veyrogram/.api.env.XXXXXX"); printf '%s\n' "$content" >"$tmp"; chmod 0640 "$tmp"; own "root:$SVC_USER" "$tmp"; mv "$tmp" "$envfile" )
  log "created $envfile (0640, secrets generated on this host; none were printed)"
fi
# canary overrides (own listener addresses); also never overwritten
if [[ ! -e $ROOT/etc/veyrogram/canary.env ]]; then
  ( umask 077; install -m 0640 "$infra/config/canary.env.template" "$ROOT/etc/veyrogram/canary.env" ); own "root:$SVC_USER" "$ROOT/etc/veyrogram/canary.env"
fi
fill=$(grep -v '^#' "$envfile" | grep -o '@@FILL:[^@]*@@' | sort -u || true)
if [[ -n $fill ]]; then
  log "STILL TO FILL IN by hand in $envfile (the service refuses to start until they are):"
  while IFS= read -r f; do log "   $f"; done <<<"$fill"
fi

# --- systemd units, journald limits, log rotation ---
inst() { mkdir -p "$(dirname "$ROOT$2")"; install -m 0644 "$1" "$ROOT$2"; }   # no -D: BSD install lacks it
for u in "$infra"/systemd/*.service "$infra"/systemd/*.timer; do inst "$u" "/etc/systemd/system/$(basename "$u")"; done
inst "$infra/systemd/journald-veyrogram.conf" /etc/systemd/journald.conf.d/veyrogram.conf
inst "$infra/logrotate/veyrogram"             /etc/logrotate.d/veyrogram
if (( real )); then
  systemctl daemon-reload
  systemctl restart systemd-journald || log "warning: could not restart journald; the new limits apply after a reboot"
  if command -v logrotate >/dev/null; then logrotate -d /etc/logrotate.d/veyrogram >/dev/null 2>&1 || die "logrotate rejected /etc/logrotate.d/veyrogram"; fi
fi
log "installed for environment: $env_name"
cat >&2 <<NEXT
Next:
  1. fill in the @@FILL:...@@ values above, then: veyrogram-api install preflight -install -fresh
  2. as the migrator account: veyrogram-api install run -username <admin>   (creates the schema and the first administrator)
  3. infrastructure/scripts/deploy.sh <binary> <version>
NEXT
