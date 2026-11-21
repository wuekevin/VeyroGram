#!/usr/bin/env bash
# Turn on versioning and a lifecycle policy on an S3-compatible bucket, then READ BACK what the provider reports and fail if it is not what was asked for.
#   object-storage-setup.sh <backup|media> <bucket> [--dry-run]
# Needs the AWS CLI (any S3-compatible provider: set VG_S3_ENDPOINT, e.g. https://oss-cn-hangzhou.aliyuncs.com) with credentials that may administer the bucket
# (use them once, here; the running services and the backup upload use narrower ones). The retention numbers in object-storage/*.json are defaults, not decisions:
# edit them first (PRD OQ-7). backup: current objects expire after 35 days, old versions after 7. media: old versions after 30 days, delete markers cleaned up.
set -euo pipefail
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
kind=${1:-}; bucket=${2:-}; dry=${3:-}
[[ $kind == backup || $kind == media ]] && [[ $bucket =~ ^[a-z0-9][a-z0-9.-]{2,62}$ ]] || { echo "usage: object-storage-setup.sh <backup|media> <bucket> [--dry-run]" >&2; exit 2; }
policy=$here/object-storage/$kind-bucket-lifecycle.json
python3 -I -c "import json,sys; d=json.load(open(sys.argv[1])); assert d['Rules'], 'no rules'; [r['ID'] and r['Status']=='Enabled' or sys.exit('rule not enabled') for r in d['Rules']]" "$policy" || { echo "invalid policy file $policy" >&2; exit 1; }
aws_() { if [[ -n ${VG_S3_ENDPOINT:-} ]]; then aws --endpoint-url "$VG_S3_ENDPOINT" "$@"; else aws "$@"; fi; }
if [[ $dry == --dry-run ]]; then
  echo "aws s3api put-bucket-versioning --bucket $bucket --versioning-configuration Status=Enabled"
  echo "aws s3api put-bucket-lifecycle-configuration --bucket $bucket --lifecycle-configuration file://$policy"; exit 0
fi
aws_ s3api put-bucket-versioning --bucket "$bucket" --versioning-configuration Status=Enabled
aws_ s3api put-bucket-lifecycle-configuration --bucket "$bucket" --lifecycle-configuration "file://$policy"
# Read back: a provider that accepts the call but silently ignores it is exactly the failure this guards against.
status=$(aws_ s3api get-bucket-versioning --bucket "$bucket" --query Status --output text)
[[ $status == Enabled ]] || { echo "ERROR: versioning on $bucket reports '$status', not Enabled" >&2; exit 1; }
got=$(aws_ s3api get-bucket-lifecycle-configuration --bucket "$bucket" --output json)
python3 -I -c "
import json,sys
want={r['ID'] for r in json.load(open(sys.argv[1]))['Rules']}
have={r['ID'] for r in json.loads(sys.argv[2]).get('Rules',[]) if r.get('Status')=='Enabled'}
missing=want-have
sys.exit('lifecycle rules missing or disabled on the bucket: '+', '.join(sorted(missing))) if missing else print('ok: versioning Enabled; lifecycle rules active:', ', '.join(sorted(have)))
" "$policy" "$got"
