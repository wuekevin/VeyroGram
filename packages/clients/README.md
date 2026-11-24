# API clients (generated)

TypeScript, Go, Kotlin and Swift clients, generated from `packages/contracts/openapi/*.yaml` by `python3 tools/gen_clients.py` (Story 1.11, AD-8).
**Do not edit the files here**: change the contract and regenerate. CI fails when the checked-in clients differ from what the contract generates
(`gen_clients.py --check`), so a contract change always ships with matching clients.

| Client | Path | Checked |
|---|---|---|
| Go | `go/` (module `veyrogram/clients/go`, standard library only) | `go vet`; **driven against the real API end to end** by `services/api/cmd/api/clientcontract_test.go` |
| TypeScript | `typescript/src/index.ts` (global `fetch`) | `tsc --strict` |
| Swift | `swift/` (Swift package, `URLSession`) | `swiftc -typecheck` on a Mac (not in the Linux CI) |
| Kotlin | `kotlin/` (`kotlinx.serialization`, you supply the transport; Gradle project with its own pinned wrapper) | compiled and **9 contract tests** (`ClientContractTest`: bearer handling, error envelope, reserved-word field `class`, open payload, query/path encoding) in the CI `android` job; first compile found the generator emitting `val class` (a Kotlin keyword), now escaped |

All four: bearer token comes from a caller-supplied provider (store it in the platform secure store - Keychain/Keystore - never in the client), the
standard error envelope `{code, message, trace_id, details}` becomes one typed error, unknown response fields are ignored, and certificate
validation is the platform's default and is never weakened. Objects the contract leaves open (the message payload envelope) keep their extra fields
in Go (`Extra`) and TypeScript (index signature); in Kotlin and Swift they are plain JSON objects.

The generator refuses constructs it cannot generate faithfully (`oneOf`, `allOf`, type lists other than object-or-null, anonymous objects) instead of
guessing, and refuses a path variable that is not declared as a parameter.

Publishing: the clients are versioned with the contract (`info.version`) in this repository. Publishing them to a package registry (npm, Maven, SwiftPM index)
is not done and needs registry accounts; see `docs/ops/MANUAL-CHECKLIST.md`.
