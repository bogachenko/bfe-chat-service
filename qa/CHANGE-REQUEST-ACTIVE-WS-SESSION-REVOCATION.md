# GRACE change request — active WebSocket session revocation

Status: **PROPOSED / REQUIRES HUMAN APPROVAL BEFORE PRODUCTION IMPLEMENTATION**

## Evidence / defect

- `internal/authentication/centrifuge.go`: `OnConnecting` uses BFE Drive UDS validation once per Connect and discards the opaque token/session identity after returning `Credentials{UserID: accountId}`.
- `internal/realtime/realtime.go`: `OnSubscribe` enforces account/assignment ACL only at subscription admission; `DeliverCommittedMessage` checks assignment serialization before publishing but **does not validate the status of the receiving BFE session**.
- Consequently, invalidating a BFE session rejects a fresh handshake, **but a previously subscribed WebSocket may still receive notifications while recipient assignment is unchanged**. Existing reassignment tests do not cover this condition.
- The separate real Drive UDS evidence [run 39020511743](https://github.com/bogachenko/bfe-chat-service/actions/runs/39020511743) tests active/revoked token through UDS, not live subscribed sockets.

## Required change propagation (GRACE)

RequirementsAnalysis.xml: extend security rule BR-026 / real-time requirements with **session revocation during active subscriptions**, including the precise temporal guarantee and reconnection semantics.

DevelopmentPlan.xml: introduce an authenticated *internal* BFE Drive -> Chat revocation notification boundary (or an approved equivalent per-message current-session authority check), owned by DP-M01 for session validity and DP-M05 for Centrifuge connections. No public JWT, no new client credentials, no direct Chat access to Drive database.

Contracts.xml:
- **BFE Drive contract (new upstream):** on committed session invalidation emit `SessionRevoked { session_id, version/sequence }` over mutually authenticated local UDS event transport; support replay/catch-up after a disconnect and full snapshot reconciliation. Resolve ordering: event committed before it is acknowledged as effective; fail-closed during event-stream loss when current validity cannot be proven.
- **FC-01/FC-02:** provide a verified, opaque per-session identity to the internal WebSocket lifecycle; bind each connection to one BFE session, not merely `accountId`. No untrusted session ID from client.
- **FC-08:** track connected clients by session ID, and disconnect + remove subscriptions on valid session-revocation events. Do not disconnect a second independently valid session of the same account.
- **FC-09:** define a revocation/publication serialization point. After the authoritative revocation effective point, no new message may be admitted for the revoked connection; publishing completed before that point is irrevocable. A Drive outage must not turn stale session grants into permanent access.
- **MC-01 / MC-05 / MC-08:** retain ownership separation: Drive owns sessions; Chat owns transient connection state; transport maps errors only. No durable copies of opaque credentials or private DPoP keys in Chat.

MentalScenarios.xml (new MS-F04):
1. Open two WebSockets from two different active sessions of the same account.
2. Both subscribe and receive baseline committed message.
3. Revoke first session in **real** BFE Drive while assignment is unchanged.
4. Publish a new committed message after revocation effective point; first socket must receive no event and must be disconnected; second session remains functional.
5. Fresh reconnect with revoked session and fresh DPoP must fail; fresh valid session connects, subscribes and receives next event.
6. During lost notification stream/restart/outage, verify catch-up and fail-closed semantics without message loss in PostgreSQL; all failures have bounded timeouts and redacted logs.

## Architecture decision required before implementation

**Recommended:** durable ordered revocation feed from BFE Drive with session-specific identity and reconnect reconciliation, plus a server-side disconnect barrier. A simple `node.Disconnect(accountID)` is not sufficient: it revokes other valid sessions of the same account, and a fire-and-forget event without replay risks leaked sessions after Chat restart/outage.

**Alternative:** authorize each delivery for each connected session against BFE Drive. This requires secure server-to-server session validation that does not reuse client DPoP proof and guarantees evaluation at a defined publication ordering point. It adds potentially high UDS I/O on the hot path and must fail closed on BFE Drive outages.

A periodic client reconnect, standalone session-expiry timer, or account-only `Disconnect` is **not** equivalent to immediate, session-specific revocation.

## Acceptance / Independent QA gates

- Approved revisions to RequirementsAnalysis.xml, DevelopmentPlan.xml, Contracts.xml, MentalScenarios.xml and upstream Drive auth protocol **before** production code.
- Red/green real WebSocket tests using the actual Centrifuge node and **real BFE Drive UDS with PostgreSQL 18.6** and two separately created sessions.
- The test asserts publication *after* revocation effective point is never observed by revoked live socket, even with **unchanged assignment**.
- Race testing, 80% total / 90% critical coverage, mutation and all GitHub quality gates remain mandatory; tests must not substitute synthetic UDS for real acceptance.
- Capture approved Chat SHA, Drive SHA, test sequence, redacted logs and run URLs.

## Handoff

No existing approved contract defines session-ID-bearing revocation events or their consistency/availability semantics. In accordance with GRACE's 'no approved contract, no production implementation' and prohibition on silently adapting contracts to code, this is a **proposal**, not an authorization to change BFE Drive or Chat production code. The user/Command Center must approve the cross-service boundary and effective-time guarantee. Independent QA currently BLOCKED for active-session revocation.
