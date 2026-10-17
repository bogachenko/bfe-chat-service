# DP-M02 — Independent QA (FC-03 / FC-04 / FC-12, MS-05 / MS-11)

**Baseline:** `f2f93bf433d1db7df7e0fd2036051905e0c9feb6` on `feat/dp-m02-assignments`.
**Current verdict:** QA IN PROGRESS / NOT VERIFIED pending CI.
**Production Quality Gate:** BLOCKED; no approval or waiver.

## Test scope
- FC-03: each assignment operation revalidates current administrator; rejects NON_ADMIN, missing role and failures from authentication including revoked access, UDS unavailable and invalid DPoP. Malformed UUID must reject without persisting a recipient.
- FC-04: read current assignment, repeat reassignments, return NO_ASSIGNMENT on sql.ErrNoRows; invalid caller input and malformed stored UUID fail closed.
- FC-12: map SQLSTATE 40001/40P01/23505 to conflict; failed mutation must preserve previous recipient.
- MS-05: PostgreSQL 18.6 concurrency with 32 parallel assignment changes and 32 distinct message commits; exactly one assignment row per owner, unique message IDs, all recipients from permitted serial order, all persisted message recipients stable.
- MS-11: transactional reassignment rolled back, no change to current recipient, subsequent message uses preceding assignment.
- Security: read path performs no automatic assignment and no recipient-status lookup through Authenticator. Changes revalidate ADMIN; target account status not queried via BFE Auth (code inspection plus spy).
- Existing DP-M07 storage integration tests independently cover cancellation, backend disconnect, idempotency and replay.
- No production Authentication/Storage/assignments files or GRACE contracts changed by this QA PR.

## CI evidence
- [Independent DP-M02 QA](https://github.com/bogachenko/bfe-chat-service/actions/workflows/qa-dpm02.yml): assignment unit, race and 90% critical coverage threshold.
- [DP-M07 Integration](https://github.com/bogachenko/bfe-chat-service/actions/workflows/qa-dpm07.yml): PostgreSQL 18.6 and storage concurrency with race; mutation test.
- [Project Quality Gates](https://github.com/bogachenko/bfe-chat-service/actions/workflows/quality.yml): separate repository-wide blocking gate.

**Do not mark DP-M02 PASS until results are checked.**

## Verified QA evidence — commit 8813ddd (2026-10-17)

- [DP-M02 Independent QA #39027828319](https://github.com/bogachenko/bfe-chat-service/actions/runs/39027828319): **SUCCESS**. FC-03/FC-04/FC-12 unit and `go test -race` PASS; assignment statement coverage **93.8%**, against unmodified 90% critical gate.
- [DP-M07 #39027828383](https://github.com/bogachenko/bfe-chat-service/actions/runs/39027828383): PostgreSQL 18.6 integration and race step PASS; storage coverage **93.8%**; mutation job **SUCCESS**. Overall integration job FAIL at independent project-wide coverage/report gate: project profile **53.2%**. No DP-M02 production or QA fix can legitimately waive this gate.
- [Quality Gates #39027828239](https://github.com/bogachenko/bfe-chat-service/actions/runs/39027828239): **FAIL** at project-wide coverage: profile **49.1%**, `coverage not measurable or critical packages missing`. Missing/unimplemented packages account for substantial zero-coverage. Do not present global Quality Gates as green.
- QA-only test compile error (`undefined: admin`, corrected by `a7af477`) and typed-nil interface test panic (corrected by `8813ddd`) are resolved as evidenced by passing DP-M02 unit job.

**Scoped independent QA finding:** FC-03 / FC-04 / FC-12 unit/race and MS-05 / MS-11 PostgreSQL integration scenarios PASS. **DP-M02 module QA evidence PASS; Production Quality Gate remains BLOCKED.** Full project release approval remains reserved to Command Center. No production or GRACE changes.
