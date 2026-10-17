# DP-M06 Document Context — Developer handoff

Status: implementation awaiting Independent QA. Baseline: main `76cde290ad8e71b1e742426aae268458d32378cf`.

## Approved GRACE traceability

- `Contracts.xml` MC-06 / FC-10: normalize message text and optional single document `ID/title`, return `VALIDATION_ERROR` on malformed content.
- `RequirementsAnalysis.xml` DOC-001..DOC-012, BR-025, INV-003, INV-006: document reference belongs to an individual message; there is no document content or ACL claim.
- `DevelopmentPlan.xml` DP-M06 / DP-E10 / DP-E17: MC-03 consumes validation; MC-07 persists the reference.
- `MentalScenarios.xml`: the approved mental checks remain unchanged.

## Existing production path

1. `internal/conversations.Service.SendMessage` authorizes the sender and invokes `documentcontext.ValidateDocumentContext`.
2. The normalized message contains text, an optional document reference, or both. Nil document means ordinary chat; invalid and blank-only messages fail.
3. `internal/storage.Store.CommitMessage` passes `document_id` and `document_title` to PostgreSQL's `chat_commit_message`. The schema enforces a paired, non-blank reference and attaches it to a message row rather than to the conversation.
4. `Store.ReadHistory` returns the original per-message reference. There is no lookup into BFE Drive and no document ACL check.

## DP-M06 hardening

`internal/documentcontext/context.go` now rejects malformed UTF-8 and NUL characters in text, document ID, or document title before database persistence. Failure remains `VALIDATION_ERROR`. No new services, external calls, schema migrations, persistent permissions, or approved contract edits.

## Independent QA acceptance checklist (not developer-authored tests)

- Text-only, reference-only, combined content; blank and malformed input including invalid UTF-8 and NUL.
- Exactly one document per message; ordinary chat messages carry no document ID/title.
- Messages from different documents coexist within one conversation, retaining their own ID/title snapshots across history and reassignment.
- Idempotent retry returns the original saved document context; document deletion or Drive ACL changes do not rewrite reference metadata.
- No BFE Drive document fetch, content storage, ACL inference, cross-service table reads, or permission bypass.
- Run `go test ./...`, `go test -race ./...`, and required Quality Gates. Existing lint warning in `internal/authorization/delivery_test.go` is QA-owned and outside DP-M06 change scope.

No QA test file was changed by this branch. Do not merge before Independent QA approval.
