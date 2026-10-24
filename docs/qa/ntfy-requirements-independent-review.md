# Independent QA Review — ntfy realtime requirements replacement

**Verdict: `FAIL`**  
**Human Approval: `NOT GRANTED`**  
**Technology / implementation / deployment: `BLOCKED`**

Review date: 2026-10-24  
GRACE phase: RequirementsAnalysis — Independent QA before Human Approval  
Scope: requirements and architecture artifacts only. No requirements draft, approved baseline, production repository, production configuration, database, migration, or infrastructure was changed by this review.

## 1. Executive result

The drafts express a broadly consistent target: ntfy replaces Centrifuge for the proposed realtime signal channel; PostgreSQL-backed Backend HTTP APIs remain authoritative; ntfy is not message history; document permissions remain owned by BFE Drive; and no universal Android background-delivery guarantee is made.

All 10 Backend realtime requirements and all 8 Flutter realtime requirements have IDs, acceptance criteria, and semantic traceability links. The XML and Structurizr parser gates pass on the exact current blobs listed below.

That is not sufficient for Human Approval. Five substantive blockers remain: Flutter baseline provenance does not match the specified `main` baseline; the session-bound ntfy authorization contract is incomplete; active-subscriber revocation has no measurable bound; the event-loss/catch-up contract is not deterministic; and post-commit publication failure does not define the required recovery semantics. The Backend C1 model also ambiguously places the separate administrative UI inside the ntfy subscriber set, and its report contains stale verification evidence alongside newer evidence.

## 2. Exact revisions and evidence

### Backend — `bogachenko/bfe-chat-service`

| Item | Exact identity |
|---|---|
| Specified baseline `main` | `f4d626ddaee4c691ef4c3033062f95e7953e0821` |
| Current draft HEAD | `0d07d662bcb6f4d87fa956ca40a697d87ea16dc8` |
| Baseline `RequirementsAnalysis.xml` blob | `976c8cd8dfc41a0d938f350ee46db8b0bad4d79d` |
| Draft `RequirementsAnalysis.xml` blob | `488bf468d0992b21a8f14ae04e1eb940b91dc019` |
| Baseline C1 `workspace.dsl` blob | `1db2f96a299a733ec064fba197ced623bd7bcb73` |
| Draft C1 `workspace.dsl` blob | `9da6ed068b5856c7af30ff1c3496f4142a5c1c24` |
| Draft report | [docs/requirements/issue-36-ntfy-realtime-draft.md](https://github.com/bogachenko/bfe-chat-service/blob/0d07d662bcb6f4d87fa956ca40a697d87ea16dc8/docs/requirements/issue-36-ntfy-realtime-draft.md) |
| Draft PR / Issue | [PR #37](https://github.com/bogachenko/bfe-chat-service/pull/37) / [Issue #36](https://github.com/bogachenko/bfe-chat-service/issues/36) |

### Flutter — `bogachenko/bfe-chat-flutter`

| Item | Exact identity |
|---|---|
| Specified baseline `main` | `190b6d7086f606d1894985f953ad8b9abeb5e442` |
| Current draft HEAD | `3799e2bfe032e8ac99cd018f5f1b8964ffccc21b` |
| Draft `RequirementsAnalysis.xml` blob | `53b90371b2ab29446d6d1ee79bc77348f0df540f` |
| Draft C1 `workspace.dsl` blob | `a13dbab215b7d4427820aeb3151e138deec59f40` |
| Draft report | [docs/requirements/issue-4-ntfy-realtime-draft.md](https://github.com/bogachenko/bfe-chat-flutter/blob/3799e2bfe032e8ac99cd018f5f1b8964ffccc21b/docs/requirements/issue-4-ntfy-realtime-draft.md) |
| Draft PR / Issue | [PR #5](https://github.com/bogachenko/bfe-chat-flutter/pull/5) / [Issue #4](https://github.com/bogachenko/bfe-chat-flutter/issues/4) |

### GitHub Actions checked

- Backend XML + C1 parser: [run 40109255238](https://github.com/bogachenko/bfe-chat-service/actions/runs/40109255238), job `120917335825`. The log records checkout/expected HEAD `0d07d662bcb6f4d87fa956ca40a697d87ea16dc8`, XML blob `488bf468d0992b21a8f14ae04e1eb940b91dc019`, C1 blob `9da6ed068b5856c7af30ff1c3496f4142a5c1c24`, 90 IDs / 90 unique, 263 local references resolved, and Structurizr exit code 0.
- Backend Quality Gates: [run 40109255226](https://github.com/bogachenko/bfe-chat-service/actions/runs/40109255226), job `120917335821`, conclusion `success`. This is supporting repository CI evidence; it does not prove semantic correctness of the requirements.
- Flutter XML + C1 parser: [run 40109257069](https://github.com/bogachenko/bfe-chat-flutter/actions/runs/40109257069), job `120917341668`. The log records checkout/expected HEAD `3799e2bfe032e8ac99cd018f5f1b8964ffccc21b`, XML blob `53b90371b2ab29446d6d1ee79bc77348f0df540f`, C1 blob `a13dbab215b7d4427820aeb3151e138deec59f40`, 292 IDs / 292 unique, 1246 local references resolved, and Structurizr exit code 0.

These are the exact current artifact identities. Parser success is limited to XML structure, ID uniqueness, reference integrity, and Structurizr syntax. It does not validate authorization semantics, recoverability, acceptance-criteria sufficiency, or cross-repository agreement.

## 3. Cross-repository requirement matrix

| Backend requirement | Flutter requirement | Result | QA disposition |
|---|---|---|---|
| `REQ36-RT-01` — ntfy replaces Centrifuge; HTTP/PostgreSQL remain authoritative | `NTFY-RT-01` | Aligned. Both describe ntfy as a wake-up/invalidation signal, not a source of truth. | No standalone defect. |
| `REQ36-RT-02` — server-selected recipients/topics; sender excluded; no arbitrary client authority | `NTFY-RT-02` | High-level policy aligned. Exact session-bound authorization and topic/ACL contract remains unresolved. | Blocker `NTFY-QA-002`. |
| `REQ36-RT-03` — membership/session revocation for active subscriptions | `NTFY-RT-03` | Same intended behavior, but both refer to an “approved” bound that has no value. | Blocker `NTFY-QA-003`. |
| `REQ36-RT-04` — duplicate/missing/delayed/reordered signals reconcile from history | `NTFY-RT-04` | Same intent. Event identity/cursor, dedupe, and silent-gap detection/catch-up trigger are not specified. | Blocker `NTFY-QA-004`. |
| `REQ36-RT-05` — private ntfy payload; retain document ID/title in authorized backend history/internal events | `NTFY-RT-05` | Aligned. Both distinguish backend history from ntfy payload and preserve independent Drive ACLs. The exact payload allowlist is still part of the unresolved event contract. | No separate defect; see `NTFY-QA-004`. |
| `REQ36-RT-06` — committed send remains successful if publish fails | `NTFY-RT-06` | Persistence outcome aligned. Retry/outbox/equivalent and eventual refresh/loss semantics are unresolved. | Blocker `NTFY-QA-005`. |
| `REQ36-RT-07` — session-scoped subscriptions; old-account state cleared; each history request independently authorized | `NTFY-RT-02` + `NTFY-RT-03` + `NTFY-RT-04` | Covered across three Flutter requirements, but the session credential and revocation contract remains incomplete. | Blockers `NTFY-QA-002`, `NTFY-QA-003`. |
| `REQ36-RT-08` — Chat membership does not grant Drive access | `NTFY-RT-05` | Aligned; acceptance explicitly denies document open/download without Drive permission. | No standalone defect. |
| `REQ36-RT-09` — production boundary, integration inventory, compatibility, rollout/rollback | `NTFY-RT-07` | Aligned as a gate before implementation/release. Actual deployed-client integration remains unverified, correctly not assumed. | Later-stage blocker; see Section 7. |
| `REQ36-RT-10` — supported Web/Android transport and lifecycle constraints | `NTFY-RT-08` | Aligned on no universal background-delivery guarantee and reconciliation on resume/reconnect. Supported browser matrix and platform/auth choices remain open. | Technology/implementation blocker; see Section 7. |

**Count:** 10/10 Backend requirements and 8/8 Flutter requirements explicitly reviewed. The target requirements each have an acceptance criterion and a `verifies` traceability edge in their own XML.

## 4. Blocking findings

### NTFY-QA-001 — Flutter draft is not based on the specified Flutter `main` baseline

- **Severity:** BLOCKER
- **GRACE/artifacts:** `RA-BFE-CHAT-FLUTTER-001`, `RA-FLUTTER-NTFY-RT-001`; Flutter PR #5, `RequirementsAnalysis.xml`, `docs/architecture/c4/workspace.dsl`, `docs/requirements/issue-4-ntfy-realtime-draft.md`.
- **Evidence:** At the specified `main` commit `190b6d7086f606d1894985f953ad8b9abeb5e442`, the repository tree contains `InitialRequirements.md` and `QualityRequirements.xml`; it does not contain `RequirementsAnalysis.xml` or `docs/architecture/c4/workspace.dsl`. The report's claimed “approved” blobs `1db699ab0c7c726af71e4aae6443229c918cd152` and `287636242eee4b4f478906bfedf2f9aa08199114` are present at commit `1569ec28ca8ef307f9cd2ea12dee40b2d06bb9ea`, the branch `docs/issue-1-requirements-c1`. PR #5 itself targets that branch, not `main`. GitHub compare reports that commit `1569...` is 27 commits ahead of the specified `main` revision.
- **Impact:** The draft's XML/C1 baseline provenance cannot be established against the exact baseline required by this review. The parser pass on the draft HEAD does not repair that provenance gap.
- **Required correction:** Project Owner must identify which baseline is authoritative. Either establish the approved Flutter RequirementsAnalysis/C1 artifacts on the specified baseline and derive the draft from it, or formally revise the declared baseline and review scope to the approved `docs/issue-1-requirements-c1` lineage. Do not silently treat the two baselines as equivalent.
- **Blocks Human Approval:** YES.

### NTFY-QA-002 — Session-bound ntfy authorization and topic isolation are not yet a testable contract

- **Severity:** BLOCKER
- **GRACE IDs:** Backend `REQ36-RT-02`, `REQ36-RT-07`, `OQ36-NTFY-01`, `OQ36-NTFY-02`; Flutter `NTFY-RT-02`, `OQ-02`, `OQ-03`, `OQ-20`.
- **Evidence:** Backend says the exact ntfy ACL/auth mechanism is unresolved. The open questions still ask how the current BFE session/DPoP is bound, who provisions topics and ACLs, and how capability expiry/replay/rotation work. Flutter says direct use of BFE DPoP as ntfy authentication is not assumed, but does not specify the abstract capability/session contract either.
- **Impact:** “Cross-account subscriptions fail” is a necessary negative test, but without a normative session-binding, scope, expiry, replay and revocation contract it does not fully establish that an old or copied subscription credential cannot be used after logout, account switch, session expiry, or membership change.
- **Required correction:** Before approval, define the required security properties independently of a specific library: server-owned topic/recipient mapping; who authorizes publish and subscribe; capability/session binding and scope; expiry and replay behavior; logout/account-switch/session-revocation behavior; topic-secret/credential rotation; and explicit prohibition on credentials in URLs/logs. The implementation mechanism may be selected in Technology, but these observable security obligations must not remain undecided.
- **Blocks Human Approval:** YES.

### NTFY-QA-003 — Active-subscriber revocation has no measurable bound

- **Severity:** BLOCKER
- **GRACE IDs:** Backend `REQ36-RT-03`, `OQ36-NTFY-03`; Flutter `NTFY-RT-03`, `OQ-14`.
- **Evidence:** Both acceptance criteria say the subscriber must stop receiving protected events after an “approved revocation bound”; the bound is not stated. The associated open questions explicitly request that bound.
- **Impact:** “Immediate” revocation is not assumed, but no maximum permitted exposure window exists. The acceptance criteria are therefore not falsifiable and the security risk cannot be evaluated.
- **Required correction:** Define a measurable maximum revocation propagation time and the exact semantics for member removal, membership-epoch change, rejoin, session revocation, logout and account switch. Specify whether already-in-flight events are excluded and what state must be cleared at the bound. The value must be approved before the requirement is accepted; it cannot be invented by implementation.
- **Blocks Human Approval:** YES.

### NTFY-QA-004 — Event identity and silent-gap recovery are underspecified

- **Severity:** BLOCKER
- **GRACE IDs:** Backend `REQ36-RT-04`, `REQ36-RT-05`, `OQ36-NTFY-06`; Flutter `NTFY-RT-04`, `NTFY-RT-05`, `OQ-09`, `OQ-17`.
- **Evidence:** Duplicate, delayed, reordered and omitted events are declared harmless, but both drafts leave the event schema/version, event ID/cursor, deduplication key and catch-up trigger open. Recovery is described on connect/reconnect or “suspected gaps”; no observable rule detects a silently dropped event while a stream remains connected.
- **Impact:** A client may miss a signal without reconnecting and remain stale indefinitely. The current acceptance criteria cannot demonstrate convergence for that case. The exact “minimum opaque identifiers” allowed in ntfy payload also cannot be reviewed until an allowlist/schema is specified.
- **Required correction:** Define the observable reconciliation contract: versioned event shape and explicit payload allowlist; event identity/cursor semantics or an equivalent strategy; duplicate handling; how a client detects a gap; and the catch-up behavior that guarantees eventual authoritative refresh. If periodic reconciliation is the intended alternative to event cursors, state its trigger/bound explicitly. Do not rely on undocumented ntfy ordering, replay or durability.
- **Blocks Human Approval:** YES.

### NTFY-QA-005 — Post-commit publish failure does not define recovery or freshness semantics

- **Severity:** BLOCKER
- **GRACE IDs:** Backend `REQ36-RT-06`, `OQ36-NTFY-07`; Flutter `NTFY-RT-06`, `OQ-18`.
- **Evidence:** Both drafts correctly state that ntfy failure must not reverse an already committed message. Both defer retry/outbox/equivalent and dedupe behavior. They do not define whether notification delivery is best-effort, eventually retried, or recovered by another bounded reconciliation path.
- **Impact:** A committed message can be absent from a continuously open client's view if publication fails and no later event/reconnect occurs. “Send succeeded” is defined, but the observable realtime recovery outcome is not.
- **Required correction:** Specify the required user-visible and system-level outcome after post-commit failure. The specific outbox/retry technology can be selected later, but requirements must state whether signals may be permanently lost, what guarantees refreshes state, what duplicates mean, and what sanitized telemetry proves the failure/recovery path. Ensure the chosen semantics compose with `REQ36-RT-04` / `NTFY-RT-04`.
- **Blocks Human Approval:** YES.

### NTFY-QA-006 — Backend C1 groups the administrative UI with ntfy subscribers

- **Severity:** MAJOR
- **GRACE artifact:** Backend `docs/architecture/c4/workspace.dsl`, C1 System Context; `RA-BFE-CHAT-001`.
- **Evidence:** The `clients` software system is described as “Flutter Web/Android and separate administrative UI”, then the diagram draws `clients -> ntfy` with “Subscribe to authorized realtime signals”. The administrative UI is elsewhere described as legacy assignment/read-only auditing, and neither requirements draft defines it as a realtime chat subscriber.
- **Impact:** The architecture diagram can be read as extending the ntfy subscription surface to the administrative UI, an implicit consumer not established by the requirements. This is a security/scope ambiguity even though the text labels the signal stream as authorized.
- **Required correction:** Split the diagram into chat clients and administrative UI, or make the ntfy relationship explicitly apply only to authorized Flutter chat clients. Keep the current Centrifuge baseline and proposed ntfy target visibly distinct.
- **Blocks Human Approval:** YES, until the C1 scope is unambiguous.

### NTFY-QA-007 — Backend draft report contains conflicting verification evidence

- **Severity:** MAJOR
- **GRACE artifact:** Backend `docs/requirements/issue-36-ntfy-realtime-draft.md`, “Verification status” and later “Gate status” sections; `RA-BFE-CHAT-001`.
- **Evidence:** The earlier verification section calls blob `8bdd4b9b7b03e727e40e6913c609ddb5f499c444` the validated XML and links run `40107956573`. The current XML blob is `488bf468d0992b21a8f14ae04e1eb940b91dc019`. A later section reports the newer XML blob and run `40109073739`, while the current HEAD is now independently checked by run `40109255238`. The same report therefore retains an unlabelled stale “Verification status” section alongside newer results.
- **Impact:** A reader can mistake an earlier XML blob and run for current evidence. The current parser result is valid, but the report's own evidence chain is ambiguous and does not cleanly distinguish historical from current verification.
- **Required correction:** Label the earlier block as historical or remove it in a future authorized documentation update. Keep one current verification summary pinned to the exact HEAD/blob/run, and keep older runs explicitly marked historical. This QA review does not edit the draft.
- **Blocks Human Approval:** YES, as an evidence-integrity issue in the review package.

### NTFY-QA-008 — Independent human reviewer identity is not established by the available GitHub audit trail

- **Severity:** BLOCKER for the formal Independent QA gate
- **Evidence:** The connected GitHub actor is `bogachenko`, which is also the account shown as PR owner/author for the reviewed drafts. The available connector context cannot establish a distinct human reviewer identity.
- **Impact:** This report is a separate technical analysis from the Engineering Orchestrator review, but the GitHub audit trail alone does not prove that a different human performed or endorsed the review. It must not be represented as a distinct human sign-off.
- **Required correction:** Have a reviewer whose identity is distinct from the draft author/owner acknowledge the report before the organization treats the Independent QA gate as formally satisfied. No specific review tool is required.
- **Blocks Human Approval:** YES, until independent reviewer identity is established.

## 5. Security and authorization audit

| Area | Assessment |
|---|---|
| Current BFE session / DPoP binding | Partially specified; no approved ntfy credential/capability lifecycle. **BLOCKED by NTFY-QA-002.** |
| Server-side recipient/topic determination | Explicitly required by `REQ36-RT-02`; arbitrary client-selected topic/recipient is not authority. Exact ACL/auth design remains open. |
| Sender exclusion | Explicit Backend rule and acceptance criterion: sender receives no ntfy event for their own message. This is a Backend delivery obligation. |
| Conversation isolation and membership intervals | Preserved from `REQ32-MEMBERSHIP`; realtime must honor visibility intervals. Revocation bound remains missing. |
| Already-open subscriptions after removal/revocation | Desired outcome is explicit; measurable bound is not. **BLOCKED by NTFY-QA-003.** |
| Logout/account switch | Old subscription and old-session cache must be cleared before new-session data appears. This is explicit, but session capability invalidation still depends on NTFY-QA-002. |
| ntfy payload privacy | Both drafts exclude message text, document title/content and credentials/proofs/keys from ntfy. Exact payload allowlist/schema remains unresolved under NTFY-QA-004. |
| History authorization | Backend says each history request independently uses the existing session-bound DPoP HTTP authorization. ntfy events do not grant read authority. This is aligned at the requirements level; implementation is not tested because this is a requirements-only phase. |
| Chat vs BFE Drive permissions | Correctly separated. Chat membership and ntfy topic access must not grant Drive document rights; document open/download remains independently authorized by Drive. |
| Publish authorization vs subscribe authorization | Both must be server-controlled; exact mechanisms and negative-test cases remain part of NTFY-QA-002. |

## 6. Events, history, and recovery audit

- **Source of truth:** PASS at the requirements-intent level. PostgreSQL-backed Backend HTTP API owns messages, membership, history and read state; ntfy is only a wake-up/invalidation signal.
- **Duplicate/delayed/reordered signals:** The required outcome is stated, but the event identity/cursor and deduplication contract is open.
- **Missing events:** The criteria cover omitted events followed by reconnect. They do not define how to detect a silently lost event while a connection remains healthy, nor an explicit bounded refresh policy. This is NTFY-QA-004.
- **Post-commit publish failure:** A successful database commit remains successful. The repair/freshness path is not defined, so NTFY-QA-005 blocks approval.
- **Document metadata:** Document ID/title remain in authorized backend history/internal events as required by `REQ32-DOCUMENT`; this does not permit those fields in ntfy payloads. Both drafts state this distinction.
- **Cursors:** The existing history contract has conversation-scoped ordering and cursor pagination. That does not by itself define a realtime event cursor or a client-side trigger for discovering a missing signal. Do not conflate the two.

## 7. Web, Android, and production boundaries

### Web and Android

The drafts correctly do **not** promise universal background stream or notification delivery. Flutter requires reconciliation on resume/reconnect and asks for evidence covering supported browsers, transport/auth/CORS, foreground/background, Doze, process death and force-stop.

The following remain **Technology/implementation blockers**, but are not standalone blockers to approval of the high-level requirements because the drafts explicitly defer platform selection and do not promise delivery guarantees:

- Backend `OQ36-NTFY-04` / Flutter `OQ-15`: supported browser matrix, transport/auth flow, CORS/header/cookie behavior and credential-leak checks.
- Backend `OQ36-NTFY-05` / Flutter `OQ-04`, `OQ-16`, `OQ-11`: permitted Android execution model, current platform/Google Play policy review, supported API/device matrix, Doze/process-death/force-stop behavior.
- Flutter `OQ-05`: exact offline-cache storage/encryption/retention contract. The required old-session cache clearing is already stated; storage details must be settled before implementation.
- Flutter `OQ-01`, `OQ-07`, `OQ-10`: host-provided HTTP DPoP/session integration, document callback shape, and public API/lifecycle details. These belong to the later integration/API contract gate unless they force a change to the current security outcomes.

No evidence was found that supports guaranteed Android background delivery. No such guarantee should be added.

### Production boundaries

The reviewed PR changed-file lists contain requirements, C1, draft documentation, and parser-workflow files only. This review made no writes to `bfe-drive` or `bfe-drive-client`. The drafts explicitly state that production integration is not assumed and that no production code/configuration, DB, migration, permissions, infrastructure or deployment changes are authorized.

Backend `REQ36-RT-09` and Flutter `NTFY-RT-07` appropriately require deployed-client/dependency inventory, compatibility, rollout/coexistence, rollback triggers and an owner before implementation/release. Backend `OQ36-NTFY-08` and Flutter `OQ-19` are therefore **later-stage blockers**, not independent blockers to approval of the high-level requirements, provided this gate remains mandatory. The actual deployed integration inventory has not been verified in this requirements-only review.

## 8. Traceability and graph integrity

### Structural checks

| Check | Backend current XML | Flutter current XML |
|---|---:|---:|
| IDs / unique IDs (from current Actions log) | 90 / 90 | 292 / 292 |
| Local references resolved (from current Actions log) | 263 | 1246 |
| Target realtime requirements | 10 | 8 |
| Target acceptance criteria | 10 | 8 |
| Acceptance `verifies` edges | 10/10 | 8/8 |
| Structurizr C1 parser | PASS | PASS |

### Semantic traceability

- Backend `REQ36-RT-01`–`REQ36-RT-10` each trace to the relevant ISSUE32 requirement(s), an AAG scenario, a business process, and one or more invariants. `AC36-RT-01`–`AC36-RT-10` each link back to the corresponding requirement. Mental scenarios exercise the major auth, revocation, recovery, post-commit, document ACL, platform and release paths.
- Flutter `NTFY-RT-01`–`NTFY-RT-08` each have an acceptance criterion and a `verifies` edge. The graph links the requirements to existing functional/nonfunctional requirements, processes and invariants, and explicitly marks `TC-CENTRIFUGE` as `superseded_by_in_draft` by `NTFY-RT-01`.
- The Project Owner's [decision comment on Backend Issue #36](https://github.com/bogachenko/bfe-chat-service/issues/36#issuecomment-6385730270) resolves the original additive-channel wording in the issue body for this draft's target direction only. It does not grant Human Approval or authorize implementation.
- The Backend C1 and Flutter C1 both model ntfy as proposed/external and preserve the existing Centrifuge baseline as historical/current-approved context pending approval. The parser passes are current, not assumed from the old baseline.
- Semantic completeness is **not PASS** because the auth, revocation, event-loss and failure semantics identified above remain unresolved, and Backend C1 has the subscriber-scope ambiguity in NTFY-QA-006.

### Open-question duplication

Flutter XML repeats substantially the same issues under older and newer IDs: `OQ-02` / `OQ-20` (session-bound ntfy auth), `OQ-03` / `OQ-13` (topic ownership/isolation), and `OQ-09` / `OQ-17` (event schema/cursor/catch-up). The meanings are broadly aligned, but duplicate records can be closed inconsistently.

- **Finding:** `NTFY-QA-009`, **MINOR**, Flutter `RA-FLUTTER-NTFY-RT-001`.
- **Required correction:** Keep one canonical question per decision or add explicit supersession/cross-reference links and ensure both records share one status/closure record.
- **Blocks Human Approval:** NO; it is a traceability maintenance issue, not a separate semantic blocker.

## 9. Open-question classification

This review does not automatically treat every open question as a RequirementsAnalysis blocker.

| Open questions | Classification | Rationale |
|---|---|---|
| Backend `OQ36-NTFY-01`, `02`, `03`; Flutter `OQ-02`, `03`, `13`, `14`, `20` | **Block Human Approval** | Session binding/topic authorization and measurable active-revocation behavior are security invariants that must be testable in the approved requirements. See NTFY-QA-002/003. |
| Backend `OQ36-NTFY-06`; Flutter `OQ-09`, `17` | **Block Human Approval** | Event identity, dedupe and gap recovery determine whether the stated convergence criteria can be proved. See NTFY-QA-004. |
| Backend `OQ36-NTFY-07`; Flutter `OQ-18` | **Block Human Approval** | Post-commit publication failure needs explicit observable recovery/freshness semantics, even if the chosen mechanism is deferred. See NTFY-QA-005. |
| Backend `OQ36-NTFY-04`; Flutter `OQ-15` | **Technology/implementation blocker** | Exact browser transport/auth/CORS and supported browser matrix can be chosen in the later platform/technology gate, provided security outcomes and no-URL-credential rule remain normative. |
| Backend `OQ36-NTFY-05`; Flutter `OQ-04`, `OQ-11`, `OQ-16) | **Technology/implementation blocker** | The draft explicitly avoids background-delivery guarantees and requires lifecycle evidence before implementation. The permitted execution model and support matrix must be settled before implementation. |
| Backend `OQ36-NTFY-08`; Flutter `OQ-08`, `OQ-19` | **Implementation/release blocker** | Actual package integration, deployed versions, coexistence, rollout, rollback and ownership are release evidence. The drafts explicitly gate implementation/release on this evidence. |
| Flutter `OQ-05` | **Technology/implementation blocker** | Exact offline-cache storage/encryption/retention design can be decided later; logout/account-switch cache isolation is already a normative requirement. |
| Flutter `OQ-01`, `OQ-07), `OQ-10` | **Later API/integration-contract blocker** | Host API signatures, document callback shape and lifecycle disposal are downstream integration details unless they require changing the stated security/permission outcomes. |
| Flutter `OQ-06`, `OQ-12` | **Later product-behavior clarification** | Send-retry UX and detailed UI copy are not introduced by the ntfy transport replacement and do not independently invalidate the realtime requirements. |
| Backend `OQ36-NTFY-09` | **Resolved for target direction** | Owner decision comment establishes replacement, not additive ntfy. Human Approval remains ungranted. |

## 10. Unverified areas and limitations

1. No implementation, live ntfy server, real session/DPoP integration, browser, or Android device was exercised. This is intentional for RequirementsAnalysis and must not be described as runtime PASS.
2. No real deployed-client dependency inventory or rollout/rollback plan was verified. Those remain explicit later-stage gates.
3. The exact ntfy auth/ACL, revocation, event schema/cursor, retry/outbox, and supported platform contracts are not evidenced because the drafts leave them open.
4. The XML parser and Structurizr logs prove syntax/reference integrity only; they do not prove security, product behavior or requirement completeness.
5. The formal identity of a distinct human reviewer cannot be established from the available GitHub actor context; see NTFY-QA-008.
6. No `golang_tests` skill was available in the installed skill catalog during this review. No Go implementation tests were added or run because this task is requirements/architecture QA, not implementation QA.

## 11. Final independent verdict

`FAIL`

Human Approval is **not ready**. The blocking findings are:

- `NTFY-QA-001` — Flutter baseline provenance mismatch.
- `NTFY-QA-002` — session-bound ntfy authorization/topic-isolation contract incomplete.
- `NTFY-QA-003` — active-subscriber revocation bound undefined.
- `NTFY-QA-004` — event identity, silent-gap detection and catch-up contract incomplete.
- `NTFY-QA-005` — post-commit publish failure and freshness/recovery semantics incomplete.
- `NTFY-QA-006` — Backend C1 subscriber scope ambiguously includes the administrative UI.
- `NTFY-QA-007` — Backend draft report contains conflicting current/stale verification evidence.
- `NTFY-QA-008` — distinct human reviewer identity is not established for the formal Independent QA gate.

`NTFY-QA-009` is MINOR and does not independently block approval.

**Recommendation to Project Owner:** keep Human Approval `NOT GRANTED`; keep Technology, implementation and deployment `BLOCKED`. Resolve the baseline and requirements-level security/recovery blockers, correct the C1 scope and evidence-record ambiguity in an authorized revision, then request a distinct reviewer to repeat the semantic audit against the new exact HEADs and current Actions logs. The Project Owner alone decides Human Approval.
