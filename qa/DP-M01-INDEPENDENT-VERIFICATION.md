# DP-M01 — Independent Verification

**Baseline:** `719496a`. **BFE Drive contract reference:** `9a33850`.
**Status:** QA IN PROGRESS; MS-F01 OPEN; Developer ON HOLD; Production Quality Gate BLOCKED.

## Contract and production-diff audit

FC-01 requires UDS POST /internal/auth/validate, opaque token, accountId/JKT/administrativeStatus, independently verified DPoP JWS/JKT/ATH/HTM/HTU/IAT/JTI replay, fail-closed on expired/revoked tokens or UDS failure. FC-02 requires a new proof for each Centrifuge connection/reconnect, no secrets in URL.

- Direct regression: UDS listener, positive caller, invalid ATH/HTM/HTU/time/JWS, revoked and unavailable response, unavailable socket.
- Concurrent replay with the same JKT/JTI and fresh/reused reconnect proof.
- WSS-to-HTTPS binding, invalid WSS URL and URL-query token injection.
- `go test -race`, ≥90% auth statement gate, Gremlins mutation CI.

## Independent verification gaps — DO NOT DECLARE PASS

1. Local UDS test server **is not the actual BFE Drive process at 9a33850**. Real BFE Drive + PostgreSQL 18.6 session issuance/revocation integration remains REQUIRED before approving MS-F01.
2. Actual Centrifuge network WebSocket handshake/OnConnecting callback integration and reconnect through a live node remain REQUIRED. Direct `ValidateOnConnecting` testing is not proof of the physical handshake.
3. Authentication replay cache is process-local. Multi-instance shared replay protection is **not proven**; topology constraint must be documented or design amended by production developer.
4. Tests above need a completed CI result and independent mutation baseline before QA verdict. No production defect should be claimed without reproducible failing evidence.
5. Project Production Quality Gate remains blocking; module results do not waive it.

**Interim verdict: NOT VERIFIED.** No production code modified.
