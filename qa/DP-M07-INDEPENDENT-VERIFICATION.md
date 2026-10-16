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
