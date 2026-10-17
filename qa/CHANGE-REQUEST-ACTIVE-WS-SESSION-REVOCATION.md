# GRACE decision — active WebSocket after BFE Drive session invalidation

Status: **SUPERSEDED / NOT REQUIRED — USER DECISION 2026-10-17**

## Authorized business behavior

The user explicitly decided: **an already established Chat WebSocket may continue operating until it disconnects even if its original BFE Drive session is subsequently invalidated.** No forced disconnection, real-time revocation events, polling, or revalidation of BFE session for an existing Chat WebSocket is required.

- A **new** WebSocket connection MUST authenticate against currently valid BFE Drive credentials through the existing UDS/DPoP boundary. A revoked session MUST be rejected on new connect/reconnect.
- A previously authorized, still-open Chat WebSocket MAY continue receiving messages and performing chat operations, subject to **existing Chat assignment / ACL checks and other currently approved operation-level guards**. Revocation of a Drive session alone does not terminate that connection.
- Reassignment, account authorization and other existing access-control rules remain effective; this decision does not waive FC-07/FC-08/FC-09 or MS-F02 checks.
- File operations in BFE Drive continue to enforce BFE Drive's session validity independently. This decision grants no new file access.
- No change to session expiry, logout, deactivate-user or password reset semantics *inside BFE Drive* is authorized.

## GRACE scope and traceability

This document replaces the earlier proposed session-specific immediate-revocation architecture. The previous proposal for Drive-to-Chat `SessionRevoked` events, replay/reconciliation, session-ID mapping, publication ordering barrier, and MS-F04 forced-disconnect acceptance is **WITHDRAWN**.

**RequirementsAnalysis.xml / Contracts.xml / DevelopmentPlan.xml / MentalScenarios.xml**: maintain the existing HUMAN_APPROVED artifacts; propose only a minimal clarification of WebSocket authentication-at-connect vs. open-connection lifetime if ambiguity creates a contradiction. Such edits require separate GRACE review; do not silently mutate approved XML files. No new upstream BFE Drive contract is required for this business decision.

## Independent QA acceptance — DP-M05

Required:
1. Real BFE Drive UDS + PostgreSQL 18.6 + actual Centrifuge WebSocket handshake, using production handlers; valid session connects and subscribes.
2. After a real BFE Drive session is invalidated, a **new** handshake/reconnect with that invalidated session is rejected, using a fresh proof.
3. An independently valid BFE Drive session can establish a new Chat WebSocket.
4. Existing MS-F02 reassignment behavior, message authorization, race/integration/security checks, and coverage gates (80% overall / 90% critical) remain required.

**Explicitly NOT required:** force-disconnecting an already-open WebSocket on Drive session invalidation, or proving no delivery to that socket solely because its Drive session was invalidated. Continued chat access for the already-open connection is intended behavior, not a security defect under this decision.

## Change control

No production implementation of immediate WebSocket session revocation is authorized. Do not add a new Drive-to-Chat revocation feed or weaken existing connect-time authentication. DP-M05 real cross-service handshake and reconnection checks remain to be independently verified; passing synthetic UDS tests is not sufficient evidence for those checks.
