#!/usr/bin/env bash
# Runs every merge gate that CI runs, locally. Exit non-zero if any fails. (Mirrors .github/workflows/ci.yml)
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd); cd "$root"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.11}
fail=0; step() { echo; echo "== $1"; shift; if "$@"; then echo "   PASS"; else echo "   FAIL"; fail=1; fi; }
step "go fmt (services/api)"            bash -c 'cd services/api && test -z "$(gofmt -l .)"'
step "go vet"                           bash -c 'cd services/api && go vet ./...'
step "go.mod is tidy for every platform (go mod tidy leaves no diff)" bash -c 'cd services/api && cp go.mod /tmp/vg.go.mod.bak && cp go.sum /tmp/vg.go.sum.bak && go mod tidy && diff -q go.mod /tmp/vg.go.mod.bak >/dev/null && diff -q go.sum /tmp/vg.go.sum.bak >/dev/null; rc=$?; cp /tmp/vg.go.mod.bak go.mod; cp /tmp/vg.go.sum.bak go.sum; exit $rc'
step "cross-build linux/amd64 (production target)" bash -c 'cd services/api && GOOS=linux GOARCH=amd64 go build ./... && GOOS=linux GOARCH=amd64 go vet ./cmd/...'
step "go test -race (incl. AD-1 import-direction check)" bash -c 'cd services/api && go test -race -count=1 ./... && { [ -n "${VG_TEST_MYSQL_DSN:-}" ] || echo "NOTE: VG_TEST_MYSQL_DSN unset - MySQL integration tests were SKIPPED"; } && { [ -n "${VG_TEST_WUKONGIM_BIN:-}" ] || echo "NOTE: VG_TEST_WUKONGIM_BIN unset - WuKongIM contract tests were SKIPPED (build the patched binary with tools/build_wukongim.sh)"; }'
step "infra policy lint"                python3 -I tools/lint_infra.py
step "infra lint unit tests"            python3 -I tests/ops/test_lint_infra.py
step "supply-chain gates (tests)"      python3 -I tests/ops/test_supplychain.py
step "SBOM (CycloneDX)"                 python3 -I tools/supplychain.py sbom -o "${TMPDIR:-/tmp}/veyrogram-api.cdx.json"
step "licence gate"                     python3 -I tools/supplychain.py licenses
step "no third-party mirrors"           python3 -I tools/supplychain.py mirrors
step "secret scan"                      python3 -I tools/secret_scan.py
if [ -x "$(cd services/api && go env GOPATH)/bin/govulncheck" ]; then
  step "govulncheck (dependencies and standard library)" bash -c 'cd services/api && "$(go env GOPATH)/bin/govulncheck" ./...'
else
  echo "NOTE: govulncheck not installed - vulnerability scan SKIPPED (go install golang.org/x/vuln/cmd/govulncheck@v1.1.4)"
fi
step "OpenAPI diff gate (tests)"        python3 -I tests/ops/test_openapi_diff.py
step "upstream compat guard"             python3 -I tools/check_upstream_compat.py
step "upstream compat guard (tests)"     python3 -I tests/ops/test_upstream_compat.py
step "OpenAPI specs parse (every file, including new ones the diff does not read)" python3 -I -c "import sys,glob,yaml; [yaml.safe_load(open(f)) for f in sorted(glob.glob('packages/contracts/openapi/*.yaml'))]; print('ok')"
step "OpenAPI diff vs base ref"         python3 -I tools/openapi_diff.py --dir packages/contracts/openapi --base-ref "${VG_BASE_REF:-origin/main}"
step "deploy/rollback rehearsal"        bash tests/ops/test_deploy.sh
step "install rehearsal (secrets, modes, idempotence, templates accepted by the real binary)" bash tests/ops/test_install.sh
step "canary rehearsal (real nginx traffic split, health gate, abort, promote)" bash tests/ops/test_canary.sh
step "object-storage setup (fake aws, read-back checks)" bash tests/ops/test_objectstore.sh
step "backup + restore rehearsal (real MySQL PITR, encryption, tamper refusals)" bash tests/ops/test_backup.sh
step "nginx whitelist (real nginx)"     bash tests/ops/test_nginx.sh
step "exposure gate (config check + scanner + real WuKongIM)" bash -c 'python3 -I tests/ops/test_exposure.py && { [ -n "${VG_TEST_WUKONGIM_BIN:-}" ] || echo "NOTE: VG_TEST_WUKONGIM_BIN unset - real WuKongIM exposure test was SKIPPED"; }'
step "design tokens + contrast"         python3 -I packages/design-tokens/build.py
step "emoji-in-navigation lint"         python3 -I tools/lint_no_emoji_nav.py apps
if command -v promtool >/dev/null 2>&1; then
  step "prometheus alert rules (promtool check + unit tests)" bash -c 'cd infrastructure/prometheus && promtool check rules veyrogram-alerts.yml && promtool test rules veyrogram-alerts.test.yml'
else
  echo "NOTE: promtool not installed - alert rule validation SKIPPED"
fi
echo; [[ $fail -eq 0 ]] && echo "ALL GATES PASSED" || echo "SOME GATES FAILED"; exit $fail
