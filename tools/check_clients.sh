#!/usr/bin/env bash
# Compile-checks the generated API clients (Story 1.11). Go and TypeScript always; Swift where swiftc exists. Kotlin needs a Gradle project (compiler plus the kotlinx.serialization
# plugin), so it is compiled and tested separately in packages/clients/kotlin (gradle test, CI android job); the output says so.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
echo "go: vet + format"
(cd "$root/packages/clients/go" && go vet ./... && test -z "$(gofmt -l .)")
echo "typescript: tsc --strict"
(cd "$tmp" && npm init -y >/dev/null && npm install --silent --ignore-scripts --no-audit --no-fund typescript@5 >/dev/null)
(cd "$root/packages/clients/typescript" && "$tmp/node_modules/.bin/tsc" --strict --noEmit --target es2022 --lib es2022,dom --moduleResolution bundler --module esnext src/index.ts)
if command -v swiftc >/dev/null 2>&1; then
  echo "swift: typecheck"
  (cd "$root/packages/clients/swift" && swiftc -typecheck Sources/VeyroGramClient/VeyroGramClient.swift)
else
  echo "NOTE: swiftc not found - Swift client NOT compile-checked here"
fi
echo "NOTE: the Kotlin client is compiled and tested separately (packages/clients/kotlin, gradle test; CI android job)"
