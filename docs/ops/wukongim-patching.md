# WuKongIM: pinned revision + patch queue (Story 1.8, AD-2 / R-21)

VeyroGram does not run a stock WuKongIM. The build is **pinned upstream SHA + an ordered patch queue**, built reproducibly by `tools/build_wukongim.sh`:

- Pin: `PIN=` in `tools/build_wukongim.sh` (currently `7b810e08`, built with go1.26.10). The script refuses to build if `upstream/WuKongIM` is at another revision.
- Patches: `patches/wukongim/NNNN-*.patch` (git format-patch), applied in filename order inside a throwaway worktree; a patch that does not apply fails the build.
- A hosted fork is optional: the patch queue is the source of truth, so no GitHub fork is required to build or to review changes.

## Patch queue
| # | Patch | Why | Tests |
|---|---|---|---|
| 0001 | `webhook.signing_secret`: HMAC-SHA256 over `timestamp + "." + body` on all outbound webhooks (async + `msg.before_send`), headers `X-WK-Timestamp`, `X-WK-Signature: v1=<hex>` | Stock WuKongIM signs nothing; with only a path secret + network isolation, anyone who learns the URL can forge callbacks | upstream-side unit tests (`pkg/webhooksig`, both senders, config schema/docs consistency); VeyroGram contract test against the real binary (`TestContractSignedCallbacksAreAuthenticatedAndForgeriesFailClosed`); handler unit tests (forgery, tamper, replay, stale timestamp) |

Behaviour is unchanged when `signing_secret` is empty, so the patch can be dropped upstream-wards without data migration.

## Adding or changing a patch
1. `git -C upstream/WuKongIM checkout -b veyrogram/patches 7b810e08`, change code + tests, `go test` the touched packages, commit.
2. `git -C upstream/WuKongIM format-patch -N --stdout > patches/wukongim/NNNN-name.patch`, then `git -C upstream/WuKongIM checkout 7b810e08` and delete the branch (the checkout must stay clean at the pin).
3. `bash tools/build_wukongim.sh /tmp/wk && VG_TEST_WUKONGIM_BIN=/tmp/wk bash tools/ci_local.sh`.
4. Add a row above and a VeyroGram-side test that fails on the stock binary.

## Upgrading the pin
1. Read the upstream changelog between old and new SHA for webhook, config schema, send path, and security changes.
2. `git -C upstream/WuKongIM fetch && git -C upstream/WuKongIM checkout <new>`; set `PIN=`; run `tools/build_wukongim.sh`. Fix any patch that no longer applies (re-create it as above).
3. Run the full contract suite (`tools/ci_local.sh` with the new binary). The contract tests pin the facts we rely on (Reason codes, composite person channel ids, client_msg_no dedupe, before_send wire format); a failure there is the upgrade's real signal.
4. Re-run the exposure gate and the production-template boot test (`tests/ops/test_exposure.py`).
5. Roll back by restoring the previous `PIN=` and rebuilding; the data directory format may not be downgradable, so snapshot `data_dir` before upgrading.

## Known limits
- Not yet verified: behaviour under a multi-node cluster (single node only), long-run soak, and the upstream license/NOTICE obligations of redistributing a patched build (legal review is a business task).
- The patch touches config schema, help text and the bilingual docs tables because upstream tests enforce their consistency.
