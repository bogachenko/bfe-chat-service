# Command Center decision — FINAL (2026-10-16)

**DP-M07 (FC-11/FC-12) Independent Verification: APPROVED by Command Center.**

**Verification target:** `fadedf1977470f66e6647ce7bb1136a113485583` (test-only changes; no production code changes).

**Module-level tester verdict: PASS**, as confirmed by [DP-M07 QA workflow #38944786781](https://github.com/bogachenko/bfe-chat-service/actions/runs/38944786781):

| QA check | Verified result |
| --- | --- |
| PostgreSQL 18.6 through testcontainers; Goose migrations | PASS |
| FC-11/FC-12 integration, regression, concurrent assignment and retries (MS-05, MS-07, MS-11) | PASS |
| Backend termination of in-flight transaction, rollback and idempotent recovery | PASS |
| Property-based tests and Go race detector | PASS |
| Module Coverage `internal/storage` | 100.00% statements; 80% minimum PASS |
| Gremlins v0.6.0 with `--tags integration`, scoped to `internal/storage` | **8 killed, 0 survived, 0 not covered; efficacy 100%** |

**Project-level Production Quality Gate: BLOCKED (still mandatory).** Project Coverage **13.25%** versus 80% minimum; `internal/auth`, `internal/dpop`, `internal/access` **NOT IMPLEMENTED** (90% critical threshold unchanged). The overall DP-M07 CI workflow has a failure conclusion **by design** because the Project Coverage step correctly blocks production release. Its PostgreSQL/race and mutation steps passed.

**Production Defects:** no reproducible defects found in the exercised FC-11/FC-12 behavior. Full PostgreSQL server/container shutdown during an operation was not simulated; the verified failure injection terminates a database backend connection.

**Handoff:** QA complete for DP-M07, Command Center approved independent verification. **Release Manager owns final PR review, integration/merge decision and repository-level gate policy.** This QA approval is NOT production-release approval and does not waive mandatory gates. Do not interpret Module PASS as Project PASS.

---

_The remainder of this document preserves the chronological audit, original gap lists and previous CI outcomes as historical evidence. This final decision supersedes earlier interim verdicts._

# DP-M07 Independent QA

Baseline: `b4bf51e006d0bce66a2a2a1cc6a2e846bed75f18`. Production files unchanged.

## Coverage audit (contract -> direct regression test)

- FC-11 missing inputs -> `TestCommitMessageInvalidInputs`.
- FC-12 missing inputs -> `TestCommitAssignmentInvalidInputs`.
- FC-11 creates no conversation after failed write -> `TestMS11RollbacksAndRecovery`.
- FC-11 current assignment and MS-07 replay after reassignment -> `TestMS07IdempotencyReassignment`.
- MS-07 concurrent identical sends -> `TestMS07ConcurrentDuplicate`.
- MS-07 generated payloads and assignment transitions -> `TestFC11PropertyIdempotency` (rapid).
- MS-05 concurrent send/reassignment -> `TestMS05ConcurrentAssignment`.
- MS-11 failed assignment, SQL rollback, recovery -> `TestMS11RollbacksAndRecovery`.
- Goose Up/Down/Up and PostgreSQL 18.6 -> `TestGooseRoundTrip`.
- FC-11 validation/document pairing and idempotency scope -> `TestFC11DocumentPairAndScope`.

## Outstanding gaps (block independent PASS)

1. Tests are **NOT RUN** in the authoring environment (Docker daemon unavailable). No PostgreSQL, race, coverage or mutation results exist yet. CI must provide real execution evidence.
2. MS-05 needs a deterministic two-connection lock-order test proving no stale recipient after a completed reassignment, rather than only outcome-set validation.
3. MS-11 still needs active database termination during an operation and recovery / retry verification.
4. Gremlins killed/survived/not-covered, mutation coverage and test efficacy remain **UNMEASURED**.
5. Measured coverage is **UNAVAILABLE**. Passing pre-existing tests cannot substitute for semantic-diff coverage.
6. External authorization / actual BFE Auth UDS and Centrifuge delivery boundaries are not part of this storage-only diff and are not verified here.

**Verdict: NOT VERIFIED.** No production defect is claimed without a reproducible failing execution. Command Center should keep the module gate closed until CI evidence and uncovered failure interleavings are resolved.

## CI findings (first run)

- Existing required `make coverage`: total 4.8%; without `-tags=integration`, SQL tests do not contribute. More importantly, `tools/coverage-gate` requires actual `internal/auth`, `internal/dpop`, `internal/access` packages, absent in baseline `b4bf51e`. **No test-only change can satisfy this structural gate honestly**. This is a pre-existing scope/sequence blocker; do not weaken thresholds or fabricate critical packages.
- First integration job could not compile due to missing go.sum entry for `github.com/jackc/puddle/v2` from `pgxpool`. QA workflow now runs `go mod tidy` after installing pinned versions to resolve transitive dependencies in its ephemeral checkout. This is an unverified fix until CI runs.
- Gremlins first run reports mutator coverage 12.96%; this is not a mutation score, and the QA implementation cannot claim mutation adequacy.

## Additional verification

- `TestMS05AssignmentCommitsBeforeWaitingSend`: assignment transaction commits before the sender resolves its recipient; ensures B rather than stale A.
- `TestMS11CancelledWaitingSendLeavesNoPartialState`: canceled message blocked on owner lock, no partial conversation/message, transaction rollback retains A, retry succeeds.
- Separate DP-M07 statement coverage gate enforces **80%** for `internal/storage` using `-tags=integration`. Global required gate is **unchanged** (80% overall, 90% critical) and remains blocked until the missing critical production packages exist and adequate cross-repository test coverage is added. This scope is not a substitute for global PASS.

## Verified GitHub Actions evidence — 2026-10-16

Baseline `b4bf51e`; QA HEAD before this report update: `e43fcacdb327ddbbc8cae87c6f168260df174602`.

- [DP-M07 Independent QA run 38940274961](https://github.com/bogachenko/bfe-chat-service/actions/runs/38940274961): **SUCCESS**.
- Real `postgres:18.6` testcontainers started, `SHOW server_version` returned `18.6 (Debian 18.6-1.pgdg13+2)`; Goose migration round-trip passed.
- `go test -tags=integration -count=1 -timeout=25m ./internal/storage/...`: **PASS**.
- `go test -tags=integration -race -count=1 -timeout=25m ./internal/storage/...`: **PASS**, no reported races.
- `go test -tags=integration -covermode=atomic -coverprofile=coverage.out -timeout=25m ./internal/storage/...`: **PASS**; `CommitMessage 100.0%`, `CommitAssignment 100.0%`, `internal/storage 100.0%` statements. QA 80% threshold: **PASS**. These statement results do **not** establish 100% semantic coverage.
- QA Gremlins v0.6.0 run: tool job **SUCCESS**, mutation counts **2 killed, 5 lived, 47 not covered**, mutator coverage **12.96%**. Important: this mutation invocation uses tests without the integration build tag and is NOT sufficient evidence for mutation adequacy of FC-11/FC-12.
- Global [Quality Gates](https://github.com/bogachenko/bfe-chat-service/actions) remains a **separate FAILED gate** because critical `internal/auth`, `internal/dpop`, `internal/access` packages do not exist yet and the full repository has inadequate untagged unit coverage. The threshold remains 80% overall/90% critical, unchanged.

## Independent QA conclusions

1. **No reproducible production defect identified by this successful storage-suite run.**
2. No database process termination/restart during an in-flight write was tested. MS-11 covered SQL rollback, canceled waiting write and successful retry, not destructive DB outage.
3. MS-05 proved a committed assignment before recipient resolution, concurrent send/assignment outcomes, but randomized interleavings cannot replace full schedule exploration.
4. Pre-existing full-repository coverage quality gate remains red; no threshold, production Go code or SQL migration was changed.
5. **QA verdict: DP-M07 scoped regression/race/coverage PASS; global release/quality verdict BLOCKED.** Do not interpret this scoped PASS as module complete or merge-ready until all required gates and mutation coverage concerns are resolved.

## Staged coverage policy (Command Center)

- Module Coverage: internal/storage, 80% threshold, with unit and integration tests.
- Project Coverage: all packages, 80% overall and 90% critical; absent auth/dpop/access packages = NOT IMPLEMENTED, not PASS.
- Report files: module-coverage.out, project-coverage.out, coverage-report.md, uploaded even if gate fails.
- Production Quality Gate remains blocking. Thresholds and GRACE normative documents are unchanged.
- Module PASS never implies Project PASS; statement coverage does not prove contract coverage.

## Mutation gap audit — follow-up

Prior five LIVED mutants are the five individual operand negations in `storage.go` input guards:
`CommitMessage` line 26 owner/sender/key and `CommitAssignment` line 43 owner/recipient.
These survived because Gremlins previously did not use the `integration` build tag:
regular unit tests use a nil DB and therefore short-circuit before those operands.
Existing real-DB integration tests are now included via `--tags integration`.
`TestFC11FC12InvalidUUIDsAndFailedDB` adds direct observable failure checks
with a non-nil `*sql.DB`, invalid UUIDs and closed database.

MS-11 now exercises a real PostgreSQL backend disconnection with
`pg_terminate_backend` while FC-12 holds an open transaction, asserting
rollback, intact old assignment and idempotent recovery. This simulates lost
backend connection, **not** an entire PostgreSQL service/container shutdown.
All cases run against `postgres:18.6` through testcontainers.

Mutation job now installs pinned QA dependencies and writes
`mutation-storage.json`; it excludes unrelated production packages while
mutating **all** handwritten FC-11/FC-12 storage code. No mutation score
threshold was invented or lowered; surviving mutants remain audit items.
Module and Project Quality Gates remain unchanged.
