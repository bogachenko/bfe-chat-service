# DP-M05 — real BFE Drive E2E verification status

Decision updated: 2026-10-17. See [WebSocket lifetime decision](CHANGE-REQUEST-ACTIVE-WS-SESSION-REVOCATION.md).

## Evidence already obtained

- Real BFE Drive UDS + PostgreSQL 18.6 authentication through **Chat auth probe**: [DP-M01 full E2E 39020511743](https://github.com/bogachenko/bfe-chat-service/actions/runs/39020511743). This is **not** real Centrifuge WebSocket E2E.
- Real Centrifuge WebSocket with **controlled test UDS**: [QA 39058058990](https://github.com/bogachenko/bfe-chat-service/actions/runs/39058058990), [QA 39059213529](https://github.com/bogachenko/bfe-chat-service/actions/runs/39059213529).
- Quality Gates on QA commit `f030fd4`: [39059213544](https://github.com/bogachenko/bfe-chat-service/actions/runs/39059213544) PASS.

## Approved behavior

A WebSocket authenticated at connect time **may remain active after its BFE Drive session is revoked**. Chat continues under existing per-operation access-control/assignment rules until disconnect. New connections or reconnects with the revoked session must fail authentication. BFE Drive file requests retain their own session authorization rules. No session-specific forced disconnect or BFE Drive revocation notification feed is required.

## Remaining independent QA — real cross-service E2E

On an authorized E2E runner, use real BFE Drive UDS + PostgreSQL 18.6 and real Centrifuge with production OnConnecting/OnSubscribe handlers:

1. Create two valid BFE Drive sessions for an isolated account, with fresh DPoP proofs.
2. Open WebSocket using session A, subscribe, and confirm normal authorized message delivery.
3. Invalidate session A in real BFE Drive.
4. Attempt a **new** WebSocket connection using revoked session A and fresh DPoP proof; assert authentication failure.
5. Connect using valid session B; assert successful subscribe and authorized delivery.
6. Independently retain existing MS-F02 reassignment/access-control assertions; no requirement to disconnect the existing socket of session A because of Drive session invalidation alone.
7. Record actual Chat/Drive SHAs, assertions, CI run URLs and redacted logs. Never log secrets.

**Removed acceptance criterion:** no delivery to or forced disconnection of the already-open session-A WebSocket solely on session revocation. This was withdrawn by the user's explicit business decision.

## Verdict

Existing automated QA evidence: **PASS for tested fixture scenarios**.
Real BFE Drive + Centrifuge handshake/reconnect E2E: **PENDING / NOT YET VERIFIED**.
DP-M05 full Independent QA: **PENDING**, subject to remaining real E2E and other approved contract checks. Production release approval is not implied.
