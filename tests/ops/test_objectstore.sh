#!/usr/bin/env bash
# object-storage-setup.sh against a fake `aws` (a small stand-in that keeps a bucket's state in files). Proves the script's own logic: it applies both settings, reads
# them back, and FAILS when the provider says Enabled/Suspended wrongly or drops the rules. It cannot prove any real provider behaves; that is checklist C10.
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd); s=$root/infrastructure/backup/object-storage-setup.sh
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT; mkdir -p "$tmp/bin"
fail=0; pass=0; check() { if eval "$2"; then pass=$((pass+1)); echo "  ok   - $1"; else fail=$((fail+1)); echo "  FAIL - $1"; fi; }
cat >"$tmp/bin/aws" <<'AWS'
#!/usr/bin/env bash
st=$FAKE_STATE; args=("$@"); [[ ${args[0]} == --endpoint-url ]] && { echo "${args[1]}" >"$st/endpoint"; args=("${args[@]:2}"); }
case ${args[1]} in
  put-bucket-versioning) [[ ${FAKE_MODE:-} == ignore ]] || echo Enabled >"$st/versioning"; [[ ${FAKE_MODE:-} == suspend ]] && echo Suspended >"$st/versioning"; exit 0;;
  put-bucket-lifecycle-configuration) [[ ${FAKE_MODE:-} == droprules ]] && exit 0; for i in "${!args[@]}"; do [[ ${args[$i]} == --lifecycle-configuration ]] && cp "${args[$((i+1))]#file://}" "$st/lifecycle"; done; exit 0;;
  get-bucket-versioning) cat "$st/versioning" 2>/dev/null || echo None;;
  get-bucket-lifecycle-configuration) cat "$st/lifecycle" 2>/dev/null || { echo '{}'; };;
esac
AWS
chmod +x "$tmp/bin/aws"; export PATH=$tmp/bin:$PATH
run() { FAKE_STATE=$tmp/state; rm -rf "$tmp/state"; mkdir -p "$tmp/state"; export FAKE_STATE FAKE_MODE=$1; bash "$s" "${@:2}" >"$tmp/out" 2>&1; }
run ok backup my-backups;   check "backup bucket: applied and read back" '[[ $? -eq 0 ]] && grep -q "versioning Enabled" $tmp/out && grep -q veyrogram-backup-retention $tmp/out'
run ok media my-media;      check "media bucket: both rules active" '[[ $? -eq 0 ]] && grep -q veyrogram-media-expired-delete-markers $tmp/out'
VG_S3_ENDPOINT=https://s3.example.test run ok backup my-backups; check "a custom S3-compatible endpoint is passed through" 'grep -q https://s3.example.test $tmp/state/endpoint'
run suspend backup my-backups; check "a provider that reports versioning Suspended makes the script fail" '[[ $? -ne 0 ]] && grep -q "not Enabled" $tmp/out'
run ignore backup my-backups;  check "a provider that ignores the versioning call makes the script fail" '[[ $? -ne 0 ]]'
run droprules backup my-backups; check "a provider that drops the lifecycle rules makes the script fail" '[[ $? -ne 0 ]] && grep -q "missing or disabled" $tmp/out'
run ok backup "Bad_Bucket!";  check "an unsafe bucket name is refused" '[[ $? -eq 2 ]]'
run ok other my-backups;     check "an unknown kind is refused" '[[ $? -eq 2 ]]'
bash "$s" backup my-backups --dry-run >"$tmp/out" 2>&1; check "--dry-run only prints the two commands" 'grep -q put-bucket-versioning $tmp/out && grep -q put-bucket-lifecycle $tmp/out && [[ ! -e $tmp/state/versioning ]]'
check "retention numbers are sane: backups keep >= 14 days, old versions >= 1 day" 'python3 -I -c "
import json
d=json.load(open(\"$root/infrastructure/backup/object-storage/backup-bucket-lifecycle.json\"))[\"Rules\"][0]
assert d[\"Expiration\"][\"Days\"]>=14 and d[\"NoncurrentVersionExpiration\"][\"NoncurrentDays\"]>=1 and d[\"AbortIncompleteMultipartUpload\"][\"DaysAfterInitiation\"]>=1"'
echo; echo "object-storage rehearsal: $pass passed, $fail failed"; exit $fail
