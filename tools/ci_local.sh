#!/usr/bin/env bash
# Runs every merge gate that CI runs, locally. Exit non-zero if any fails. (Mirrors .github/workflows/ci.yml)
set -uo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd); cd "$root"
export GOTOOLCHAIN=${GOTOOLCHAIN:-go1.26.10}
fail=0; step() { echo; echo "== $1"; shift; if "$@"; then echo "   PASS"; else echo "   FAIL"; fail=1; fi; }
step "go fmt (services/api)"            bash -c 'cd services/api && test -z "$(gofmt -l .)"'
step "go vet"                           bash -c 'cd services/api && go vet ./...'
step "go test -race (incl. AD-1 import-direction check)" bash -c 'cd services/api && go test -race -count=1 ./... && { [ -n "${VG_TEST_MYSQL_DSN:-}" ] || echo "NOTE: VG_TEST_MYSQL_DSN unset - MySQL integration tests were SKIPPED"; } && { [ -n "${VG_TEST_WUKONGIM_BIN:-}" ] || echo "NOTE: VG_TEST_WUKONGIM_BIN unset - WuKongIM contract tests were SKIPPED (tools/build_wukongim.sh)"; }'
step "infra policy lint"                python3 -I tools/lint_infra.py
step "infra lint unit tests"            python3 -I tests/ops/test_lint_infra.py
step "OpenAPI diff gate (tests)"        python3 -I tests/ops/test_openapi_diff.py
step "OpenAPI diff vs base ref"         python3 -I tools/openapi_diff.py --dir packages/contracts/openapi --base-ref "${VG_BASE_REF:-origin/main}"
step "deploy/rollback rehearsal"        bash tests/ops/test_deploy.sh
step "nginx whitelist (real nginx)"     bash tests/ops/test_nginx.sh
step "design tokens + contrast"         python3 -I packages/design-tokens/build.py
step "emoji-in-navigation lint"         python3 -I tools/lint_no_emoji_nav.py apps
if command -v promtool >/dev/null 2>&1; then
  step "prometheus alert rules (promtool check + unit tests)" bash -c 'cd infrastructure/prometheus && promtool check rules veyrogram-alerts.yml && promtool test rules veyrogram-alerts.test.yml'
else
  echo "NOTE: promtool not installed - alert rule validation SKIPPED"
fi
echo; [[ $fail -eq 0 ]] && echo "ALL GATES PASSED" || echo "SOME GATES FAILED"; exit $fail
