# DP-M04 — Independent QA Verification

**Baseline:** `feat/dp-m04-authorization` commit `436668a8ff5fc4615dfd0d0cc6941ed2c993c90f`.
**QA branch:** `qa/independent-dp-m04-436668a`.
**Production Quality Gate:** BLOCKED independently of module QA.

## Evidence

- [DP-M04 Independent QA #39032906727](https://github.com/bogachenko/bfe-chat-service/actions/runs/39032906727): Authorization unit and Go Race Detection **PASS**, authorization statement coverage **96.6%** (threshold ≥90%). At initial evidence capture, PostgreSQL job was still running.
- [Prior DP-M04 PostgreSQL #39032755705](https://github.com/bogachenko/bfe-chat-service/actions/runs/39032755705): PostgreSQL 18.6 MS-F02 and Go Race Detection **PASS**, with identical PostgreSQL test file. Prior authorization unit passed but coverage was **65.5%**; QA-only additional test cases raised it without changing a threshold.

## Independent scenarios

1. SEND/READ/SUBSCRIBE/DELIVER: current owner and assigned recipient allowed, unrelated account forbidden. Authorization reads current assignment on each call.
2. Administrative: ASSIGN and ADMIN_READ require a caller whose current verified status is ADMIN; revoked status denied. ADMIN_READ does not grant SEND to an otherwise unrelated ADMIN.
3. Reassignment: previous recipient loses SEND, READ, SUBSCRIBE and DELIVER; new recipient succeeds.
4. Fail-closed: no assignment, malformed owner/caller/recipient, PostgreSQL failure, missing lock and publication callback; all forbidden/failed.
5. MS-F02: PostgreSQL 18.6 `WithAssignmentLock` serializes admitted synchronous publication with assignment changes. A previously admitted delivery finishes before reassignment commits, then old recipient cannot publish. Concurrent reassignment/delivery and publication failure/recovery are covered by dedicated integration tests with `-race`.
6. Unit critical coverage ≥90%: measured 96.6%, pass.

## Scope

- **No production code, GRACE contract, threshold, `main` or runner changes.** Only QA Go tests, workflow and verification report.
- Authentication validation is a prerequisite of the AuthorizeOperation API; these tests exercise already-verified Caller authorization. They do not independently execute an end-to-end live BFE Auth session revalidation.
- The MS-F02 test covers synchronous admission/publish callbacks under PostgreSQL lock, **not** downstream websocket/network delivery to a real client or asynchronous pubsub delivery after callback completion.
- Project-wide Production Quality Gate remains BLOCKED; module results do not waive release gates.

**Module decision:** DP-M04 **PASS within the tested authorization and synchronous delivery-lock boundaries**; external live end-to-end delivery is not verified.
