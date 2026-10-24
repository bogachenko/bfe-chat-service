# Issue #36 — ntfy RequirementsAnalysis Revision Report

**GRACE ID:** `RA36-001`  
**Scope:** `bogachenko/bfe-chat-service` only  
**Status:** `PROPOSED / PENDING_HUMAN_APPROVAL`  
**Effective baseline:** `ISSUE32`  
**Implementation:** `BLOCKED`

## Decision and boundary

Issue #36 is titled **Replace Centrifuge with ntfy**. The Project Owner's direction is to replace Centrifuge, not add ntfy alongside it. This report covers only the Backend requirements and architecture artifacts in this repository. No other repository is modified by this work.

The approved ISSUE32 artifacts remain effective until the Project Owner grants Human Approval for RA36-001. No implementation, dependency change, database operation, production configuration change, merge to the effective baseline, or deployment is authorized by this proposal.

## Artifacts in this proposal

| Artifact | Proposed blob |
|---|---|
| `RequirementsAnalysis.xml` | `b9f7d161feeaa415a7410fc2317193e82609b765` |
| `docs/architecture/c4/workspace.dsl` | `100a140d3837280b211e1a705014edb31cf19fb1` |

The requirements proposal defines `REQ36-RT-01` through `REQ36-RT-10`, acceptance criteria, invariants, the realtime business process, and explicit proposed bounds for revocation and reconciliation. Both 30-second bounds are proposals requiring Human Approval; they are not effective requirements yet.

## Corrections against Backend-relevant Independent QA findings

| QA finding | Disposition in this revision |
|---|---|
| `NTFY-QA-001` — Flutter baseline provenance | Out of scope for this Backend-only change. No Flutter artifact, branch, PR, or Issue is changed or treated as a dependency of this Backend revision. |
| `NTFY-QA-002` — session-bound authorization | `REQ36-RT-02` defines server-owned recipient/topic mapping, session and membership-epoch scope, read-only single-use grant, maximum 30-second lifetime, replay rejection, header/relay-only credentials, and credential-log prohibition. Technology must select and prove a compliant mechanism. |
| `NTFY-QA-003` — active subscriber revocation | `REQ36-RT-03` proposes a measurable maximum 30-second revocation bound and specifies the revocation triggers and old-session cleanup. The value requires Human Approval. |
| `NTFY-QA-004` — silent event loss | `REQ36-RT-04` defines the versioned opaque wake token, deduplication/coalescing, authoritative ordinal cursor, reconnect/resume reconciliation, and proposed periodic reconciliation no less frequent than every 30 seconds while connected. |
| `NTFY-QA-005` — post-commit publish failure | `REQ36-RT-06` explicitly defines best-effort publication, durable attempt outcomes, no blind retry of ambiguous/UNKNOWN attempts, and bounded recovery through history reconciliation. |
| `NTFY-QA-006` — administrative UI in subscriber set | C1 separates Flutter chat clients from the administrative UI. Only chat clients subscribe to ntfy; the administrative UI has no ntfy relationship. |
| `NTFY-QA-007` — conflicting verification evidence | This report is the current verification summary for this proposal branch. It does not reuse old parser runs as proof for new source blobs. The earlier draft report remains historical evidence on its original draft branch and is not the current report for PR #38. |
| `NTFY-QA-008` — distinct human QA identity | Not resolved by Engineering Orchestrator edits. A distinct human reviewer must acknowledge the final Backend revision before the formal Independent QA gate is considered satisfied. |

## Security and recovery contract

- PostgreSQL-backed backend state and authenticated history remain authoritative.
- Sender is excluded from recipients.
- Each recipient/membership epoch has an opaque topic; client-supplied topic selection does not grant authority.
- The application-supplied ntfy body contains only `version` and opaque `wakeToken`; no message/document content or identifiers are included.
- The administrative UI is not an ntfy subscriber.
- Session and membership revocation must terminate active subscriptions within the proposed bound.
- Lost, duplicate, delayed, and reordered signals are reconciled through authorized backend history, not ntfy cache/order guarantees.
- Document ID/title may remain in authorized backend history, but not in ntfy payload or metadata; Drive authorization remains independent.

## Verification status

The latest requirements/C1 parser run passed on source-validation commit `c6680371d25502d1c3076233b8c8e5099a9b6e42`:

- [GitHub Actions — Requirements and C1 validation, run 40120143140](https://github.com/bogachenko/bfe-chat-service/actions/runs/40120143140): `success`.
- XML parser: `PASS`; 75 IDs, all 75 unique; 33 local references resolved; one historical external approval reference retained.
- Structurizr C1 parser: `PASS`, exit code 0.
- Exact checked blobs: `RequirementsAnalysis.xml` = `b9f7d161feeaa415a7410fc2317193e82609b765`; `workspace.dsl` = `100a140d3837280b211e1a705014edb31cf19fb1`.
- Validation workflow blob: `4af6d18a48d422724fe4f92316343eda9501a340`.

The verification report is updated after the source-validation run; this report-only change does not alter either validated source blob. Parser success establishes syntax, ID uniqueness, and local-reference integrity only; it does not prove that the proposed security/recovery contract is implementable or accepted. Independent human acknowledgement and Human Approval remain pending.

Official ntfy documentation relevant to later Technology analysis:
- [Subscribe API and stream/replay semantics](https://docs.ntfy.sh/subscribe/api/)
- [Publish and message-cache behavior](https://docs.ntfy.sh/publish/)
- [Server configuration and access control](https://docs.ntfy.sh/config/)

## Remaining gates

1. XML and Structurizr parsers must pass on the exact proposal blobs.
2. PR #38 must remain unmerged pending Human Approval.
3. A distinct human reviewer must acknowledge the final Backend revision.
4. Project Owner must approve or revise the proposed 30-second revocation and reconciliation bounds.
5. Only after Human Approval may Technology be revised; dependent GRACE artifacts must then be checked and approved before implementation.
