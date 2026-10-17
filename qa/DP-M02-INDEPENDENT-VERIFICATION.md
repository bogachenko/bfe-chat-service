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
