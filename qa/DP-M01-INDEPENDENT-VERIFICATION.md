## FINAL — Independent QA verification (2026-10-17)

**Decision:** DP-M01 (FC-01/FC-02) **MODULE PASS**; critical scenario **MS-F01 VERIFIED / PASS**.
**Production Quality Gate remains BLOCKED** (independent repository-wide gate; no waiver). Release Manager owns review and merge decision.

**Approved baselines:** Chat Service production fix `12abd404c0fce742cf0d7727ffbad556a018720c`; BFE Drive `9a33850a5456d8e3ff40f5cdab6ded869da7110f`; QA execution checkout `7bd2e43c5a05a6239539c018b67039298299913d`. Corrected 32-byte SHA-256 renewal-token fixture from `232629e` is present in that QA checkout.

**Full E2E evidence:** [GitHub Actions run #39020511743](https://github.com/bogachenko/bfe-chat-service/actions/runs/39020511743), job `real_bfe_drive_uds` **SUCCESS** (2026-10-17).
- QA-branch checkout explicitly verified: `7bd2e43c5a05a6239539c018b67039298299913d`; not `main`.
- Private BFE Drive read-only checkout at approved `9a33850a5456d8e3ff40f5cdab6ded869da7110f` succeeded with configured secret.
- Chat Auth Probe build succeeded.
- Compilation alongside BFE Drive `internal/httpapi` integration tests **PASS**: `go test -tags=integration -run '^$' ./internal/httpapi` — `ok` (no tests run by design).
- Actual PostgreSQL 18.6-backed BFE Drive `NewInternalAuthHandler` over Unix-domain socket **PASS**: `go test -tags=integration -count=1 -run '^TestQADPM01RealDriveUDS$' -timeout=25m ./internal/httpapi` — `ok` (15.167s). The test covers active token validation, session revocation returning invalid, and unavailable socket returning fail-closed.
- The **secret/access blocker is resolved** for that run.

**Fast QA evidence:** [GitHub Actions run #38964714359](https://github.com/bogachenko/bfe-chat-service/actions/runs/38964714359) had individual `authentication` and `mutation` jobs **SUCCESS**: `go test -race` PASS, Authentication statements **96.1%** (required ≥90%), `BindOnConnecting` **100%**, Gremlins v0.6.0 **61 killed / 0 lived / 0 not covered (100% efficacy, 100% mutant coverage in targeted production scope)**. The *aggregate historical workflow* failed on the then-unavailable E2E secret; do **not** misrepresent that historical whole run as green. Fast PR CI has since been separated from manual Full E2E.

**FC-01/FC-02 / MS-F01 acceptance evidence:** Issue #3 ordinary opaque tokens with `t/r/n` PASS; actual ASCII space/tab/CR/LF rejected before UDS; real BFE session checked before DPoP; DPoP JWS/JKT/ATH/HTM/HTU/IAT, duplicate and parallel JTI replay, real Centrifuge WebSocket `OnConnecting` handshake and fresh-proof reconnect; revoked session and unavailable UDS fail closed. No additional reproducible production defect identified in verified module scope.

**Limits:** Multi-instance cross-process JTI replay protection is not demonstrated; the implementation's in-process validator/replay store must stay within its documented supported deployment topology. Live deployment topology and repo-wide coverage/release gates are separate from this module approval.

**Scope discipline:** Only QA tests, CI and QA report changed in PR #4. Neither Chat Service nor BFE Drive production source is modified. Older interim `OPEN` / `BLOCKED` observations below are historical, superseded by this FINAL decision for MS-F01 only.

---

# DP-M01 — Independent Verification baseline 12abd40

Previous issue: [#3](https://github.com/bogachenko/bfe-chat-service/issues/3).
Production fix `12abd404c0fce742cf0d7727ffbad556a018720c` changes the token whitespace expression to actual ASCII space/tab/CR/LF.

## Regression / security audit
- Issue #3: normal opaque tokens with t/r/n must be forwarded to UDS (counter assertion), then validated.
- Real ASCII space, tab, CR and LF: fail closed before UDS (counter assertion).
- Valid session + bad ATH must return PROOF_INVALID; revoked session + same bad proof returns AUTH_INVALID first. Valid session + valid fresh proof succeeds.
- JWS EdDSA, embedded public JWK, JKT, ATH, HTM, HTU, past/future IAT; parallel same-JTI; WebSocket binding/reconnect.
- Property-based replay and good session-binding samples with rapid.
- Real physical BFE Drive UDS server and Centrifuge network handshake still require independent E2E evidence; local UDS fixture is only contract-level integration.
- Validator replay tracking is process-local; cross-process replay protection unverified.
- Required threshold Authentication ≥90% statements, race and Gremlins.
- MS-F01 remains OPEN pending real BFE Drive/Centrifuge E2E and green quality evidence. Project gate remains BLOCKED.

## Verified CI evidence / independent QA continuation (2026-10-16)

Baseline: `12abd404c0fce742cf0d7727ffbad556a018720c`, Issue #3 production patch retained unchanged.
- [DP-M01 CI run 38964366112](https://github.com/bogachenko/bfe-chat-service/actions/runs/38964366112): unit, security, property-based, concurrency, real Centrifuge WebSocket handshake/reconnect and `go test -race` **PASS**.
- `internal/authentication` **96.1% statement coverage** (unchanged mandatory 90% threshold: **PASS**). `BindOnConnecting` **100%**.
- Regression: ordinary opaque strings containing t/r/n accepted and submitted to UDS; space, tab, CR, LF rejected before UDS; DPoP verified only after accepted BFE session; JWS, JKT/ATH, HTM/HTU/IAT, JTI replay and simultaneous same-JTI submits covered.
- Actual Centrifuge Node + real HTTP/WebSocket upgrade exercised `OnConnecting`, reconnect with fresh JWS and rejection of replayed JTI; payload length 16 KiB boundary verified.
- Gremlins v0.6.0 **61 KILLED / 0 LIVED**. Prior run had 11 NOT COVERED in the executable QA probe, not in production FC-01/FC-02. The mutation workflow now excludes `qa/` from production mutation scope. Its updated mutant count/coverage must be confirmed by the next CI run.
- All three previously surviving production mutants (UDS timeout, ADMIN branch, payload length boundary) have direct regression checks and were killed in the latest measured run.
- **Real BFE Drive UDS E2E: BLOCKED** on checkout of private repository `bogachenko/bfe-drive` at approved commit `9a33850`. CI log: `Repository not found` with default cross-repository GITHUB_TOKEN. The CI workflow now explicitly requires repository secret **`BFE_DRIVE_READ_TOKEN`** (fine-grained read-only Contents permission to `bogachenko/bfe-drive`). This secret must be provisioned by an authorized maintainer. No private source or credentials are copied into the PR.
- The E2E fixture is staged as `qa/testdata/bfe_drive_auth_uds_e2e_test.go`, and the Chat probe under `qa/driveauthprobe`. After secret provisioning, CI will copy the fixture only into the ephemeral Drive checkout and run the real `NewInternalAuthHandler` on UDS backed by PostgreSQL 18.6; it will verify active, revoked and missing-UDS calls. **This E2E has not yet run.**
- External multi-instance shared replay state is not evidenced. Process-local JTI replay is tested; deployment must constrain scope to one validator or use a proven shared replay store.
- **MS-F01: OPEN. DP-M01: QA IN PROGRESS. Production Quality Gate: BLOCKED.** Scoped authenticated unit/race/coverage PASS does not imply integration E2E PASS.
- **Production defects:** previously filed [Issue #3](https://github.com/bogachenko/bfe-chat-service/issues/3) confirmed fixed by regression against `12abd40`; no new reproducible production defect in current passing regression suite.

Only test, CI and QA-report files were changed. Production code and GRACE normative documents remain unchanged.

## Fast CI / Full E2E separation — 2026-10-17

- Fast PR workflow: `.github/workflows/qa-dpm01.yml` runs authentication tests, `go test -race`, 90% critical Authentication coverage gate, and Gremlins. It has **no private BFE Drive secret** and no real BFE Drive job.
- Full manual workflow: `.github/workflows/qa-dpm01-full-e2e.yml` uses `workflow_dispatch`, BFE Drive commit `9a33850a5456d8e3ff40f5cdab6ded869da7110f`, compilation alongside all `internal/httpapi` integration tests, then real PostgreSQL 18.6 + BFE Auth UDS active/revoked/unavailable scenarios.
- The **only use** of `BFE_DRIVE_READ_TOKEN` is in the manual Full E2E. CI fails closed when this secret is missing.
- Corrected `renewal_token_hash` fixture from commit `232629e` is preserved: `sha256.Sum256(...)`, **32 bytes**, satisfying BFE Drive `login_sessions_renewal_token_hash_shape`.
- **Execution status: NOT RUN** for the updated Full E2E workflow. The connected GitHub integration has no `workflow_dispatch` action; GitHub also normally requires a manually dispatched workflow file to be registered on the repository default branch. The test-only workflow changes are in PR #4's QA branch, not merged into default. Therefore there is no PostgreSQL or real BFE Auth UDS E2E result on this current configuration.
- Required handoff: authorize the CI workflow registration/merge through repository policy, provision `BFE_DRIVE_READ_TOKEN` with scoped read-only access to the private BFE Drive repository, then explicitly dispatch the Full E2E on the QA ref and review the output.
- DP-M01 **QA IN PROGRESS**; MS-F01 **OPEN**; Production Quality Gate **BLOCKED**. Do not use old CI run results as evidence for the new workflow configuration.
