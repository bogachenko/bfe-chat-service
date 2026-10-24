# Issue #4 / backend Issue #36 — ntfy realtime requirements draft

**Status:** DRAFT_NOT_APPROVED. Human Approval: NOT GRANTED. Technology/implementation/deployment: BLOCKED.

## Source revisions
- Backend main: f4d626ddaee4c691ef4c3033062f95e7953e0821.
- Approved RequirementsAnalysis blob: 976c8cd8dfc41a0d938f350ee46db8b0bad4d79d.
- Approved C1 DSL blob: 1db2f96a299a733ec064fba197ced623bd7bcb73.
- Project Owner direction: bfe-chat-flutter Issue #4.
- Conflicting unchanged task: bfe-chat-service Issue #36 says ntfy is additive and Centrifuge remains; it is not changed or closed.

## Traceability matrix

| Draft requirement | Existing requirement trace | AAG scenario | Business process | Invariant | Acceptance | Mental scenario |
|---|---|---|---|---|---|---|
| REQ36-RT-01: ntfy replaces Centrifuge as target realtime transport | REQ32-MESSAGE, REQ32-HISTORY, REQ32-READ | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-NO-AUTHORITY | AC36-RT-01 | MS36-NTFY-04 |
| REQ36-RT-02: backend-controlled topics and subscription isolation | REQ32-AUTH, REQ32-ISOLATION, REQ32-MEMBERSHIP | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-SESSION | AC36-RT-02 | MS36-NTFY-01 |
| REQ36-RT-03: active revocation and membership intervals | REQ32-MEMBERSHIP, REQ32-AUTH, REQ32-ISOLATION | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-SESSION | AC36-RT-03 | MS36-NTFY-02 |
| REQ36-RT-04: missing/duplicate/reordered signals reconcile from history | REQ32-HISTORY, REQ32-MESSAGE, REQ32-READ | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-NO-AUTHORITY | AC36-RT-04 | MS36-NTFY-04 |
| REQ36-RT-05: minimum privacy-preserving payload | REQ32-DOCUMENT, REQ32-ISOLATION | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-PRIVACY, INV36-NTFY-DRIVE-ACL | AC36-RT-05 | MS36-NTFY-06 |
| REQ36-RT-06: post-commit failure/retry/dedupe | REQ32-MESSAGE, REQ32-API | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-POST-COMMIT | AC36-RT-06 | MS36-NTFY-05 |
| REQ36-RT-07: account/session isolation | REQ32-AUTH, REQ32-ISOLATION, REQ32-HISTORY | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-SESSION | AC36-RT-07 | MS36-NTFY-03 |
| REQ36-RT-08: document authorization remains independent | REQ32-DOCUMENT, REQ32-ISOLATION | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-DRIVE-ACL | AC36-RT-08 | MS36-NTFY-06 |
| REQ36-RT-09: production boundary and release compatibility | REQ32-PRESERVE | UC36-RELEASE-READINESS | BP36-NTFY-RELEASE-READINESS | INV36-NTFY-PRODUCTION-BOUNDARY | AC36-RT-09 | MS36-NTFY-08 |
| REQ36-RT-10: supported Web/Android transport and lifecycle | REQ32-AUTH, REQ32-ISOLATION, REQ32-HISTORY | UC36-REALTIME-RECOVERY | BP36-NTFY-REALTIME | INV36-NTFY-SESSION | AC36-RT-10 | MS36-NTFY-07 |

## Mental scenarios

- **MS36-NTFY-01:** cross-account or cross-conversation topic access is denied.
- **MS36-NTFY-02:** member removal/rejoin while connected respects the approved revocation bound and hides the membership gap.
- **MS36-NTFY-03:** logout/account switch clears the old subscription and account-scoped state before new data appears.
- **MS36-NTFY-04:** missing, duplicate, delayed and reordered signals converge through authoritative HTTP history.
- **MS36-NTFY-05:** post-commit ntfy failure does not reverse the committed send and does not create duplicate messages.
- **MS36-NTFY-06:** a chat signal/document reference does not bypass Drive ACLs or expose protected content.
- **MS36-NTFY-07:** unsupported browser auth or Android background delivery fails safely and reconciles on resume.
- **MS36-NTFY-08:** unknown deployed integration/rollback compatibility keeps implementation and rollout blocked.

## Blocking questions
| ID | Owner | Required evidence and closure |
|---|---|---|
| OQ36-NTFY-01 | BFE Drive/Internal Auth and backend owners | Auth flow, capability lifetime, replay/expiry/account-switch/revocation tests; session-bound auth is fail-closed |
| OQ36-NTFY-02 | Backend and ntfy infrastructure owners | Endpoint/topic/ACL ownership and negative cross-topic tests; server-controlled isolation proven |
| OQ36-NTFY-03 | Backend and ntfy infrastructure owners | Active subscriber removal/rejoin tests; approved revocation bound demonstrated |
| OQ36-NTFY-04 | Flutter Web and backend owners | Browser matrix, CORS and credential-leak tests; supported auth flow works without URL credentials |
| OQ36-NTFY-05 | Flutter Android owner and Project Owner | Current policy review and Doze/process-death tests; lifecycle limits documented |
| OQ36-NTFY-06 | Backend and Flutter owners | Versioned schema, event IDs/cursors, dedupe/gap examples; deterministic reconciliation approved |
| OQ36-NTFY-07 | Backend owner and Project Owner | Failure analysis, retry/outbox and telemetry tests; persisted send independent of ntfy |
| OQ36-NTFY-08 | Project Owner, Release Manager and client maintainers | Deployed dependency inventory and rollout/rollback plan; production impact evidenced |
| OQ36-NTFY-09 | Project Owner | Explicit owner decision recorded on Issue #36 or linked issue; issue aligned without rewriting history |

## Production boundaries
- bfe-drive is production and strictly READ-ONLY.
- bfe-drive-client is production; main and production/android must not change.
- Existing chat package integration in deployed clients is not assumed.
- No code, config, database, migration, permission, ntfy infrastructure or deployment change is authorized.
- Technology.xml, DevelopmentPlan.xml and Contracts.xml remain unchanged; their Centrifuge assumptions are downstream blockers after RequirementsAnalysis approval.
- Issue #36 is not edited or closed.

## Verification status

- Source branch heads and base SHAs were rechecked before the draft update.
- Current backend RequirementsAnalysis revision: `45d7189cf6ee871efce974ec5bc1d6f65e7bd3bf`; it includes explicit AAG, process, invariant, acceptance and mental-scenario edges. Current XML parser rerun is pending.
- Static graph check on the current XML content: 90 unique IDs, 74 traceability links, 0 unresolved traceability endpoints. All ten new requirements link to an AAG scenario, a business process, an invariant, an acceptance criterion and a mental scenario.
- The historical `APPROVAL25-REQUIREMENTS` reference is unresolved in both the draft and unchanged baseline; it was not introduced by this change.
- The previous backend XML revision at `b73e2b6a01d6e411a0ca870cf35862bba97c2417` passed Python XML parsing, but that result does not cover the current XML revision.
- Backend C1 DSL is unchanged (blob `9da6ed068b5856c7af30ff1c3496f4142a5c1c24`) and its real Structurizr parser run returned exit code 0.
- Flutter XML and C1 parser checks passed at draft HEAD `4b8d70e5ef7e59ed253d0d04b8150dbd481eff10`; the Flutter artifacts have not changed since that validation.
- Independent QA: NOT RUN. Human Approval: NOT GRANTED. Technology, implementation and deployment: BLOCKED.
