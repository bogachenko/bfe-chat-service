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
