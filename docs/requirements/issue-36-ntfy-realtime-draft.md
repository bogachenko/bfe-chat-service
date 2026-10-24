# Issue #4 / backend Issue #36 — ntfy realtime requirements draft

**Status:** DRAFT_NOT_APPROVED. Human Approval: NOT GRANTED. Technology/implementation/deployment: BLOCKED.

## Source revisions
- Backend main: f4d626ddaee4c691ef4c3033062f95e7953e0821.
- Approved RequirementsAnalysis blob: 976c8cd8dfc41a0d938f350ee46db8b0bad4d79d.
- Approved C1 DSL blob: 1db2f96a299a733ec064fba197ced623bd7bcb73.
- Project Owner direction: bfe-chat-flutter Issue #4.
- Conflicting unchanged task: bfe-chat-service Issue #36 says ntfy is additive and Centrifuge remains; it is not changed or closed.

## Traceability matrix
| Draft requirement | Existing requirement/invariant | Acceptance |
|---|---|---|
| REQ36-RT-01: ntfy replaces Centrifuge as target realtime transport | REQ32-MESSAGE, REQ32-HISTORY, REQ32-READ | AC36-RT-01 |
| REQ36-RT-02: backend-owned recipients/topics and subscription isolation | REQ32-AUTH, REQ32-ISOLATION, REQ32-MEMBERSHIP | AC36-RT-02 |
| REQ36-RT-03: active revocation and membership interval boundaries | REQ32-MEMBERSHIP, REQ32-AUTH, REQ32-ISOLATION | AC36-RT-03 |
| REQ36-RT-04: missed/duplicate/reordered signals reconcile from history | REQ32-HISTORY, REQ32-MESSAGE, REQ32-READ | AC36-RT-04 |
| REQ36-RT-05: minimum privacy-preserving payload | REQ32-DOCUMENT, REQ32-ISOLATION | AC36-RT-05 |
| REQ36-RT-06: post-commit failure/retry/dedupe | REQ32-MESSAGE, REQ32-API | AC36-RT-06 |
| REQ36-RT-07: account/session isolation | REQ32-AUTH, REQ32-ISOLATION, REQ32-HISTORY | AC36-RT-07 |
| REQ36-RT-08: document authorization remains independent | REQ32-DOCUMENT, REQ32-ISOLATION | AC36-RT-08 |
| REQ36-RT-09: production boundary/release compatibility | REQ32-PRESERVE | AC36-RT-09 |
| REQ36-RT-10: supported Web/Android transport/lifecycle | REQ32-AUTH, REQ32-ISOLATION, REQ32-HISTORY | AC36-RT-10 |

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
- Source SHAs and branch heads rechecked before writes.
- Backend RequirementsAnalysis static lexical check: PASS; 75 unique IDs, 0 duplicate IDs, balanced XML-like tag nesting, 0 unescaped ampersands.
- Draft reference check: 57 references checked, 0 unresolved.
- Backend C1 DSL brace-balance check: delta 0. This is not Structurizr validation.
- XML parser: NOT RUN. Structurizr parser: NOT RUN. Independent QA: NOT RUN.
- Human Approval: NOT GRANTED. Next GRACE stage: BLOCKED.
