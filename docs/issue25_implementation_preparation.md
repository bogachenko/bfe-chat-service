# Issue #25 — Implementation Preparation (NOT IMPLEMENTATION AUTHORIZATION)

Status: READY FOR IMPLEMENTATION AUTHORIZATION. Basis: approved Issue25 MentalScenarios at `df5e21ed571545bcedc0bc1bd6b96cab5034332e`. This document is a repository-source review and implementation proposal; no live PostgreSQL inspection, migration, test execution, production deployment or Drive operation has occurred.

## Verified repository baseline

- Go module `github.com/bogachenko/bfe-chat-service`, Go `1.27.2`. Existing dependencies: `centrifugal/centrifuge v0.39.3`, `jackc/pgx/v5 v5.11.0`, `pressly/goose/v3 v3.28.0`, testcontainers PostgreSQL.
- `cmd/bfe-chat-service/main.go` constructs one `centrifuge.New(centrifuge.Config{})`, a `storage.Store`, assignment-based authorization, `conversations.Service`, `realtime.Service`, and `transport.NewHandler`; app config selects HTTP origin, UDS and PostgreSQL connection.
- Current HTTP routing in `internal/transport/transport.go`: `GET /ws`, `POST /v1/conversations/{ownerAccountUUID}/messages`, `GET /v1/conversations/{ownerAccountUUID}/history`, `PUT /v1/conversations/{ownerAccountUUID}/assignment`. History pagination uses `X-After-ID`, `X-Limit`. Requests bind auth proof to the public origin/path; body limit `64<<10`. These are LEGACY routes, not Issue25 conversation UUID routes.
- Current `transport.publishCommitted` sends `{"messageId":id}` to realtime (protected metadata under new Issue25 notification requirements). `realtime.Channel(owner,viewer)` embeds account IDs in `chat:owner:viewer`; `DeliverCommittedMessage` checks the current assignment while locked and publishes synchronously. It does not implement epoch channels, opaque envelopes, immutable attempts, or UNKNOWN handling.
- Authentication currently validates caller using UDS and DPoP proof; reconnect invokes validator and Centrifuge OnSubscribe uses current assignment-based policy. Current authorizer is not Issue25 member/role ACL.
- Existing migrations (goose): `internal/storage/migrations/00001_chat_schema.sql` defines `chat_assignments(owner_account_id PRIMARY KEY, recipient_account_id)`, `chat_conversations(owner_account_id PRIMARY KEY)`, `chat_messages(id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, owner_account_id FK, sender_account_id, recipient_account_id, idempotency_key, body, document_id/title, created_at)`; unique `(owner_account_id,sender_account_id,idempotency_key)`; history indexed `(owner_account_id,id)`. `00002_atomic_operations.sql` adds SQL functions `chat_commit_assignment` and `chat_commit_message`, both locking `pg_advisory_xact_lock(hashtextextended(owner_uuid::text,0))`. Legacy identities are owner-scoped, not conversation UUID; IDENTITY ID is global, not Issue25 commit-visible per-conversation ordinal.
- Existing tests and QA workflows cover legacy assignments, UDS auth, realtime, migrations/storage, transport, integration and end-to-end probes. They are not evidence that Issue25 scenarios passed.
- `docs/issue25_legacy_readonly_research.sql` contains read-only inventory/classification queries and explicitly says NOT EXECUTED. Live `bfe_chat` schema/data/version, object counts, ambiguous history and deployment compatibility remain **UNVERIFIED**.

## Proposed implementation sequence and module dependencies

1. **M07 legacy boundary and baseline review** (READ-ONLY, only after separately authorized DB access): run reviewed inventory in READ ONLY transaction, capture source counts, IDs, provenance and document pairs; identify ambiguous owner/recipient streams, rollback and client compatibility gates. No automatic conversion or data rewriting.
2. **M01 authentication / authorization adaptation**: reuse UDS/DPoP and add session-derived caller for new membership-scoped operations; no Drive ADMIN privilege fallback. M01 is a prerequisite for protected M02–M09 operations.
3. **M03 foundational membership + PG serialization** (depends M01/M07): new conversation UUID, member epoch, membership interval and role records; one owner invariant; transaction gate for JOIN/REMOVE/TRANSFER_OWNER, typed role checks and interval bounds. M02 requires these foundations.
4. **M02 conversation lifecycle** (depends M01/M03): canonical unordered direct pair unique key and atomic create; group creation and owner assignment; hide/unhide per viewer and reopening rules.
5. **M09 document-context normalization** (depends M01 validation; later M04): immutable metadata pair without new Drive ACL grant or file reads.
6. **M04 messaging and history** (depends M01/M02/M03/M09): canonical payload, idempotency conflict 409, conversation-local commit-visible ordinal under same gate, keyset cursor, membership interval filters and legacy preservation.
7. **M06 unread/read state** (depends M03/M04): durable per-viewer monotonic read marker, correct visibility-window and foreign-author counts across devices.
8. **M05 realtime and revocation** (depends M01/M03/M04): per-viewer never-reused opaque epoch channel; already-COMMITTED message to immutable attempt with full internal binding, visible ordinal interval proof, transaction-gated REGISTER, durable CAS REGISTERED→CLAIMED, at most one worker invocation per attempt; external Centrifuge Publish outside transaction gate, timeout/ambiguous outcome UNKNOWN, no retry of claimed attempts, best-effort revoke, no protected metadata or message ID in envelope, no broker history. Membership-aware OnConnect/OnSubscribe and fresh HTTP read checks.
9. **M08 new HTTP routing** (depends all above): additive conversation UUID endpoints, validation/limits, typed errors and cursor. Keep legacy routes until independently approved cutover.

## Migration proposals — DO NOT APPLY

- `00003_issue25_conversations.sql` (proposed filename): new UUID conversation identities, explicit kind and canonical unordered direct pair unique key; per-viewer visibility records; avoid altering legacy owner-scoped tables in-place.
- `00004_issue25_membership.sql`: membership epochs (unique never-reused identifier), role and JOIN/REMOVE ordinal boundary intervals, exactly-one-owner transaction logic, uniqueness/checks/indexes for active membership and role transitions.
- `00005_issue25_messages.sql`: conversation-local ordinal counter, message with conversation UUID and sender, immutable canonical payload/idempotency identity and complete document snapshot; indices `(conversation_id,ordinal)`, sender+key unique, visible cursor queries; preserve original legacy provenance.
- `00006_issue25_read_state.sql`: per-account conversation read markers, per-user hidden flag if not in 00003; authorized unread views/indices.
- `00007_issue25_publish_attempts.sql`: attemptId primary key, immutable conversation/viewer/epoch/messageId/ordinal/opaqueChannel/wakeToken binding, FK to committed message, CHECK on state REGISTERED/CLAIMED/SUCCESS/UNKNOWN/FAILED as approved, CAS claim unique invariant; need application enforcement or DB trigger for immutable columns, and proof registration under transaction gate.
- Migration numbering is proposal, NOT checked against any new unlisted migrations outside the inspected branch; derive final SQL from real schema inventory and constraints with rollback rehearsal.

## Test / scenario matrix (future only)

- `MS25-01`: concurrent direct uniqueness and personal hide; `MS25-02`: owner/admin/member authorization and roles; `MS25-03`: boundary [10,20), [30,infinity), 12 visible of 22; `MS25-04`: both SEND/REMOVE serial orders and forbidden sender.
- `MS25-05`: owner transfer vs leave; `MS25-06`: canonical replay / 409; `MS25-07`: rollback / commit-visible ordinal and cursor; `MS25-08`: REGISTER(A,M1)→REMOVE→M2→late Publish(A) never binds M2.
- `MS25-09`: competing workers one CAS/one invocation; `MS25-10`: crash before/after CLAIM, during/after publish, UNKNOWN not automatically retried; `MS25-11`: stale subscribe/reconnect, opaque epoch channel; `MS25-12`: monotone read/unread two devices.
- `MS25-13`: UDS/DPoP and Drive ADMIN nonmember denial; `MS25-14`: real legacy preservation, ambiguous classification and separate cutover authorization; `MS25-15`: equal/exceed HTTP limits and document pair validation.
- Test layers to create after authorization: pure Go contract/unit tests; SQL transaction / advisory-lock tests using testcontainers; race/crash/fault injection around broker UNKNOWN and claim; HTTP contract tests; integration with Centrifuge v0.39.3 configured broker; legacy snapshot/checksum tests; Flutter Web and Android backward compatibility + production-like end-to-end. Assertions must include row counts, time/order, exact authorization failures and zero disclosure.

## Blocking risks / owner decisions before implementation

1. **Live database unknown:** repo migration DDL is verified, not actual `bfe_chat`. Require separately approved READ ONLY inventory and a signed preservation baseline before choosing migration/cutover execution.
2. **Legacy versus Issue25 identifiers:** current owner-account keyed tables, global bigint ID and assignment-based ACL cannot directly satisfy conversation UUID, per-conversation ordinal, epochs and group roles. Additive schema and explicit routing isolation are safer than destructive modification; decision required on API shapes and rollout.
3. **Centrifuge publish semantics:** v0.39.3 no cancelable Node.Publish. Only a pre-REMOVE registered immutable attempt can finish late; never infer no admission from timeout. Claimed-and-crashed attempt can cause lost wakeup, requiring history resync; acceptance requires best-effort realtime semantics.
4. **One-owner invariant / epoch boundaries:** cross-module single gate must be used by every transaction touching membership or committed ordinal; tested serial schedules and database integrity rules necessary. Hash advisory lock collision safety/contention and channel-token entropy need reviewed implementation.
5. **Immutable attempt correctness:** database-level immutable binding, message FK, interval validation, claim CAS and authorization checks must not be bypassable by another code path; no M2 reuse after remove.
6. **Client/API readiness:** existing `/v1/conversations/{owner}/...` routes used by legacy clients; group/direct and unread API endpoint specification plus Flutter/Web/Android compatibility must be separately checked before cutover.
7. **Security and documents:** never send `{"messageId":...}` or account IDs over Issue25 notification/channel; always reauthorize reads, preserve Drive ACL separation. Existing legacy behavior remains unchanged under original scoped approval.
8. **QA gate:** all MS25 scenario criteria are approved as specifications, **not executed tests**. Do not implement, migrate, test, inspect VPS or access Drive without Project Owner authorization.

**Result:** READY FOR IMPLEMENTATION AUTHORIZATION; authorization to implement is NOT granted by this document.
