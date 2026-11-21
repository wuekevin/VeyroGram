#!/usr/bin/env bash
# Gradual (canary) release: run the new version beside the live one and send it a share of /v1 traffic through Nginx; watch it; then promote or abort.
#   canary.sh start <binary> <version> <percent 1-50>   stage the release, start the canary instance, route <percent>% of requests to it
#   canary.sh status                                    what is running and how traffic is split
#   canary.sh check                                     compare the canary's server-error rate against its request count (exit 1 = do not promote)
#   canary.sh promote                                   check, then make the canary release the live one (auto-rollback if it fails), then end the canary
#   canary.sh abort                                     stop routing to the canary, stop it, keep the live release untouched
# Any failure while starting aborts the canary and leaves traffic exactly as it was.
# The database schema must work with BOTH versions at once (expand first, contract in a later release): both run against the same MySQL.
# Env (besides lib.sh): VG_CANARY_START_CMD VG_CANARY_STOP_CMD VG_CANARY_READY_URL VG_CANARY_METRICS_URL VG_CANARY_UPSTREAM VG_CANARY_NGINX_FILE
#      VG_NGINX_TEST_CMD VG_NGINX_RELOAD_CMD VG_METRICS_TOKEN (for check) VG_CANARY_MIN_REQUESTS (default 50) VG_CANARY_MAX_5XX_PCT (default 1)
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); . "$here/lib.sh"
VG_CANARY_START_CMD=${VG_CANARY_START_CMD:-systemctl restart veyrogram-api-canary}
VG_CANARY_STOP_CMD=${VG_CANARY_STOP_CMD:-systemctl stop veyrogram-api-canary}
VG_CANARY_READY_URL=${VG_CANARY_READY_URL:-http://127.0.0.1:9111/readyz}
VG_CANARY_METRICS_URL=${VG_CANARY_METRICS_URL:-http://127.0.0.1:9111/metrics}
VG_CANARY_UPSTREAM=${VG_CANARY_UPSTREAM:-127.0.0.1:8081}
VG_CANARY_NGINX_FILE=${VG_CANARY_NGINX_FILE:-/etc/veyrogram/nginx/canary-api.conf}
VG_NGINX_TEST_CMD=${VG_NGINX_TEST_CMD:-nginx -t}
VG_NGINX_RELOAD_CMD=${VG_NGINX_RELOAD_CMD:-nginx -s reload}
VG_CANARY_MIN_REQUESTS=${VG_CANARY_MIN_REQUESTS:-50}
VG_CANARY_MAX_5XX_PCT=${VG_CANARY_MAX_5XX_PCT:-1}
state=$VG_PREFIX/.canary

nginx_apply() { bash -c "$VG_NGINX_TEST_CMD" >/dev/null 2>&1 && bash -c "$VG_NGINX_RELOAD_CMD" >/dev/null 2>&1; }

# Stop sending traffic first, then stop the instance, so no request is routed to a process that is going away.
end_canary() {
  rm -f "$VG_CANARY_NGINX_FILE"
  nginx_apply || log "WARNING: nginx did not accept/reload the config without the canary; check it by hand"
  bash -c "$VG_CANARY_STOP_CMD" >/dev/null 2>&1 || log "WARNING: could not stop the canary instance"
  rm -f "$state" "$VG_PREFIX/canary"
}

wait_canary_ready() {
  local i=0
  while (( i < VG_READY_TIMEOUT )); do curl -fsS --max-time 2 "$VG_CANARY_READY_URL" >/dev/null 2>&1 && return 0; sleep 1; i=$((i + 1)); done
  return 1
}

# Counts from the Prometheus text: total requests and 5xx for vg_http_requests_total.
counts() { # <metrics url> -> "total fivexx"
  local body; body=$(curl -fsS --max-time 5 ${VG_METRICS_TOKEN:+-H "Authorization: Bearer $VG_METRICS_TOKEN"} "$1") || return 1
  awk '/^vg_http_requests_total\{/ { n=$NF; t+=n; if ($0 ~ /class="5xx"/) e+=n } END { printf "%d %d\n", t, e }' <<<"$body"
}

cmd=${1:-}
case $cmd in
start)
  bin=${2:?usage: canary.sh start <binary> <version> <percent>}; version=${3:?}; pct=${4:?}
  [[ $pct =~ ^[0-9]+$ ]] && (( pct >= 1 && pct <= 50 )) || die "percent must be a whole number from 1 to 50"
  [[ ! -s $state ]] || die "a canary is already running ($(cat "$state")); promote or abort it first"
  [[ -n $(current_target) ]] || die "there is no live release yet; use deploy.sh for the first deployment"
  stage_release "$bin" "$version"
  ln -sfn "releases/$version" "$VG_PREFIX/canary"
  echo "$version $pct" >"$state"
  # weight against the live member's fixed weight of 100: share = w/(100+w), so w = 100*p/(100-p)
  w=$(( (100 * pct + (100 - pct) / 2) / (100 - pct) ))
  bash -c "$VG_CANARY_START_CMD" || { end_canary; die "could not start the canary instance; traffic unchanged"; }
  if ! wait_canary_ready; then end_canary; die "canary not ready within ${VG_READY_TIMEOUT}s; aborted, traffic unchanged"; fi
  mkdir -p "$(dirname "$VG_CANARY_NGINX_FILE")"
  printf 'server %s weight=%s;\n' "$VG_CANARY_UPSTREAM" "$w" >"$VG_CANARY_NGINX_FILE"
  if ! nginx_apply; then end_canary; die "nginx rejected the canary configuration; aborted, traffic unchanged"; fi
  log "canary $version is live for about ${pct}% of requests (live release: $(current_target)). Watch it, then: canary.sh check / promote / abort"
  ;;
status)
  if [[ -s $state ]]; then read -r v p <"$state"; log "canary: $v at ~${p}% (live: $(current_target))"
    if curl -fsS --max-time 2 "$VG_CANARY_READY_URL" >/dev/null 2>&1; then log "canary instance: ready"; else log "canary instance: NOT ready"; fi
  else log "no canary (live: $(current_target))"; fi
  ;;
check)
  [[ -s $state ]] || die "no canary is running"
  read -r total errs < <(counts "$VG_CANARY_METRICS_URL") || die "could not read the canary's metrics (is VG_METRICS_TOKEN set?)"
  (( total >= VG_CANARY_MIN_REQUESTS )) || die "the canary has served only $total requests (need at least $VG_CANARY_MIN_REQUESTS); keep watching"
  # integer maths in hundredths of a percent
  bp=$(( errs * 10000 / total ))
  if (( bp * 1 > VG_CANARY_MAX_5XX_PCT * 100 )); then die "canary server-error rate is $((bp / 100)).$(printf %02d $((bp % 100)))% ($errs of $total) — above ${VG_CANARY_MAX_5XX_PCT}%; do not promote"; fi
  log "canary healthy: $errs server errors in $total requests"
  ;;
promote)
  [[ -s $state ]] || die "no canary is running"
  read -r v _ <"$state"
  "$0" check
  # Ending the canary routing first, then switching the live release; the release dir is the one already verified and preflighted at start.
  rm -f "$VG_CANARY_NGINX_FILE"; nginx_apply || log "WARNING: nginx reload failed while ending the canary"
  rc=0; activate_release "$v" || rc=$?
  bash -c "$VG_CANARY_STOP_CMD" >/dev/null 2>&1 || log "WARNING: could not stop the canary instance"
  rm -f "$state" "$VG_PREFIX/canary"
  if (( rc == 0 )); then prune_releases "$v" "$(cat "$VG_PREFIX/.previous" 2>/dev/null)"; else exit "$rc"; fi
  ;;
abort)
  [[ -s $state ]] || die "no canary is running"
  end_canary; log "canary aborted; live release unchanged: $(current_target)"
  ;;
*) die "usage: canary.sh start <binary> <version> <percent> | status | check | promote | abort" ;;
esac
