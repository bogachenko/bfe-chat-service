# DP-M05 / MS-F02 — real BFE Drive E2E verification status

QA date: 2026-10-17. QA branch: `qa/qr08-writing-coverage-verification`.

## Established evidence

- Real BFE Drive UDS integration using the actual `bogachenko/bfe-drive` private code and PostgreSQL 18.6: [DP-M01 full E2E run 39020511743](https://github.com/bogachenko/bfe-chat-service/actions/runs/39020511743). This run verifies active login session, revoked session, and unavailable socket **via the Chat authentication probe**, not a Centrifuge WebSocket. It must **not** be cited as full DP-M05 E2E.
- Real Centrifuge WebSocket acceptance and FC-09 delivery/reassignment checks against a controlled Drive UDS fixture: [QA run 39058058990](https://github.com/bogachenko/bfe-chat-service/actions/runs/39058058990); [QA run 39059213529](https://github.com/bogachenko/bfe-chat-service/actions/runs/39059213529). Mock UDS in these tests is not a deployed Drive instance.
- Quality Gates on latest QA commit prior to this report: [39059213544](https://github.com/bogachenko/bfe-chat-service/actions/runs/39059213544) PASS; production coverage previously 93.31%, thresholds 80% / 90%.

## Remaining real cross-service acceptance — BLOCKED, not simulated

Run real Drive + Chat Service together on an authorized E2E runner. Test:
1. Start actual BFE Drive `NewInternalAuthHandler` on UDS with ephemeral PostgreSQL and an isolated QA account, as in `qa/testdata/bfe_drive_auth_uds_e2e_test.go`.
2. Create **two different** Drive login sessions and separate DPoP proof JTIs. Ensure the actual Centrifuge `OnConnecting` handler is bound (no test override of `node.OnConnecting`).
3. Connect via a real WebSocket and assert the first session's account identity and authorization; subscribe to the specific viewer channel, send a committed message and observe delivery.
4. Revoke the **first** session in actual BFE Drive (the database's authoritative `login_sessions.session_invalidated` state), while its WebSocket remains open.
5. Attempt another WebSocket connection using the first session and **fresh** DPoP proof: must fail closed. Verify no delivery to revoked socket *while the recipient assignment is unchanged*, not merely after reassignment.
6. Connect using the **second** valid session with a fresh DPoP proof; verify subscription and delivery are restored.
7. Preserve redacted stdout, per-step assertions, exact Chat and Drive commit SHAs, CI run URLs, and test exit codes. Never log/access-token or DPoP private key.

## Security concern requiring explicit decision

The production `internal/authentication/centrifuge.go` authenticates on **new** connections; `internal/realtime/realtime.go` checks ACL at subscription and assignment at send time. There is no observed mechanism in these files to force-disconnect an **already authenticated, already subscribed** WebSocket when the *Drive login session itself* is revoked. Do not claim that existing live sockets are denied publications without a direct live-session-revocation test. A session revocation with unchanged assignment is distinct from an assignment change.

## Environment and triggering

This assistant environment contains **no working BFE Drive UDS**, no QA session credentials, and no action capable of dispatching the manual workflow. The existing `.github/workflows/qa-dpm01-full-e2e.yml` uses a private Drive read-only token in CI and performs real UDS checks, but does not perform this DP-M05 sequence. No full real Drive + real WebSocket CI evidence has been collected yet.

**Independent QA verdict: BLOCKED on live-session revocation/no-delivery acceptance.**
