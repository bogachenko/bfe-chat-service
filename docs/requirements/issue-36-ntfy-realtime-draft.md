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

- The gate-status correction is recorded in commit `d06e58be0e7f8b80784be24733395ec41d93cb5e`; the report consistency correction is recorded in commit `b64600109c45e669f8591154cc3a65ec842124a4`.
- Current `RequirementsAnalysis.xml` blob: `3439f5446df2fa46c22979244b5d560b5e73f7b9`.
- The real XML parser passed on the earlier blob `c77e8f9addb95962fe2b061184d5689414f6e309` at HEAD `82836e14ff3661a4987c6a1242b93e299e8b8511`. After that run, the only XML content change was the `GATE_STATUS` line (the parser status was changed from pending to PASS). The parser was not rerun on that changed XML blob; therefore the current XML parser gate is **PENDING_RERUN_AFTER_GATE_STATUS_EDIT**, not PASS.
- Independent static audit of the current XML blob: 90 ID attributes, 90 unique IDs, 0 duplicates; 74 explicit traceability links. Of 116 `ref`/`refs` tokens, unresolved local references: 0. The sole unresolved token is the documented historical external reference `APPROVAL25-REQUIREMENTS`; it is not a local XML ID.
- Backend C1 DSL blob remains `9da6ed068b5856c7af30ff1c3496f4142a5c1c24`, unchanged from the previously parser-validated draft. Its recorded Structurizr parser result is PASS (exit code 0); this does not validate the XML.
- Cross-repository verification discrepancy: the Flutter draft report explicitly records XML parser **NOT RUN** and Structurizr parser **NOT RUN**, while an earlier backend report line claimed Flutter parser checks passed. The latter claim is not accepted as evidence. Flutter parser gates remain **NOT RUN**.
- Formal Independent QA gate: **NOT RUN**; this orchestrator's static audit is not an Independent QA acceptance. Human Approval: **NOT GRANTED**. Technology, implementation and deployment remain **BLOCKED**.
- No production code, configuration, database, migrations, permissions, or infrastructure were changed. `bfe-drive` remains READ-ONLY; `bfe-drive-client` production branches remain untouched.


## Cross-component semantic consistency review (2026-10-24)

**Result: PARTIAL ALIGNMENT — REQUIREMENTS APPROVAL BLOCKED.** This is an orchestrator content review, not the formal Independent QA gate. No RequirementsAnalysis XML, C1 DSL, production artifact or implementation was changed by this review.

Reviewed against Flutter draft HEAD `4b8d70e5ef7e59ed253d0d04b8150dbd481eff10`, including its `RequirementsAnalysis.xml` blob `8d8912fd701da001a326ff2e3aacc2f45b662433`, and the current Backend Issue #36 and Flutter Issue #4 text.

### Cross-component mapping

| Backend draft | Flutter draft | Review |
|---|---|---|
| `REQ36-RT-01` | `NTFY-RT-01` | Same proposed target (ntfy replaces Centrifuge), but blocked by the conflicting unchanged Backend Issue #36 decision below. |
| `REQ36-RT-02` | `NTFY-RT-02` | Authorization/topic isolation aligned. Sender-exclusion rule is missing from the draft requirement. |
| `REQ36-RT-03`, `REQ36-RT-07` | `NTFY-RT-03` | Membership/session revocation and account-switch isolation aligned; revocation bound remains unresolved in both. |
| `REQ36-RT-04` | `NTFY-RT-04` | Authoritative-history reconciliation aligned; event identity/cursor, deduplication and catch-up trigger remain unresolved in both. |
| `REQ36-RT-05`, `REQ36-RT-08` | `NTFY-RT-05` | Minimal ntfy payload and independent Drive ACL aligned; wording in the inherited document-event requirement needs clarification. |
| `REQ36-RT-06` | `NTFY-RT-06` | Post-commit failure behavior aligned. Backend owns publishing/retry/outbox/telemetry; Flutter must not represent notification failure as message-persistence failure. Mechanism remains open. |
| `REQ36-RT-09` | `NTFY-RT-07` | Production boundary and release-evidence intent aligned; Flutter traceability links do not directly support this requirement. |
| `REQ36-RT-10` | `NTFY-RT-01`, `OQ-15`, `OQ-16` | Web and Android scope aligned; Flutter acceptance criteria should explicitly require evidence for supported browsers and Android lifecycle states. |

### Findings requiring correction or explicit resolution

1. **BLOCKER — transport decision conflicts with the existing backend issue.** Backend Issue #36 says ntfy is an additional notification channel and Centrifuge must remain responsible for in-app realtime. Flutter Issue #4 asks to evaluate replacing Centrifuge, and both drafts currently describe ntfy as the replacement target. Record an explicit Project Owner decision on Issue #36 or a linked decision before requirements approval; do not silently treat the draft proposal as effective.

2. **BLOCKER — ambiguous document-event payload contract.** Existing `REQ32-DOCUMENT` says to preserve document ID and title in message history and “events”; `REQ36-RT-05` forbids document titles in ntfy payloads, and Flutter `NTFY-RT-05` agrees. Clarify that document metadata remains available from authoritative history/domain data, while the ntfy transport payload excludes the title and content. Flutter `FR-06` can render available metadata from the authorized conversation/history flow; it does not require the title in ntfy.

3. **REQUIREMENT GAP — sender exclusion.** Backend Issue #36 explicitly says not to notify the sender of their own message unless separately approved. `REQ36-RT-02` currently requires server-side recipient selection but does not state sender exclusion or test it. Add the rule and acceptance coverage to the backend draft; Flutter should consume the server-selected recipient behavior, not choose recipients.

4. **TRACEABILITY GAP — Flutter release requirement.** `NTFY-RT-07` concerns production boundaries, compatibility, rollout and rollback, but links to `INV-HOST-SESSION`, `INV-SESSION-CACHE-ISOLATION` and `INV-DOC-ACL`; none directly expresses the production/release boundary. Add a dedicated production-boundary invariant or link to an existing requirement with that exact meaning, and connect `NTFY-RT-07` to it.

5. **ACCEPTANCE GAP — platform evidence.** Backend `REQ36-RT-10` explicitly requires supported-browser and Android-lifecycle evidence. Flutter records browser and Android questions as blocking, but `AC-NTFY-RT-01` does not require that evidence. Add an explicit acceptance criterion or link the relevant acceptance criteria to `NTFY-RT-01` before the Technology gate.

### Aligned unresolved dependencies

Both drafts correctly leave ntfy authentication, topic/ACL ownership, active-subscription revocation, event schema/cursors, retry/deduplication and production rollout as unresolved dependencies. Those are not evidence of a contradiction; they remain blockers and must not be inferred or implemented.

**Gate effect:** no approval status changes. Backend XML parser remains `PENDING_RERUN_AFTER_GATE_STATUS_EDIT`; Flutter XML and Structurizr parsers remain `NOT RUN`; Independent QA remains `NOT RUN`; Human Approval remains `NOT GRANTED`; Technology, implementation and deployment remain `BLOCKED`.
