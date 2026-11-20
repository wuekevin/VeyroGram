#!/usr/bin/env bash
# Render the Nginx template: render-nginx.sh <template> <out>  (env: SERVER_NAME TLS_CERT TLS_KEY UPSTREAM; optional LISTEN, default 443)
set -euo pipefail
tpl=${1:?template}; out=${2:?output}
for v in SERVER_NAME TLS_CERT TLS_KEY UPSTREAM; do : "${!v:?$v must be set}"; done
LISTEN=${LISTEN:-443}
sed -e "s|\${LISTEN}|${LISTEN}|g" -e "s|\${SERVER_NAME}|${SERVER_NAME}|g" -e "s|\${TLS_CERT}|${TLS_CERT}|g" -e "s|\${TLS_KEY}|${TLS_KEY}|g" -e "s|\${UPSTREAM}|${UPSTREAM}|g" "$tpl" >"$out"
