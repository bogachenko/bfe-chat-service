# DP-M08 Production Executable E2E Handoff

Status: **Developer harness implemented; independent E2E verdict pending**.

GRACE: `Contracts.xml` MC-08 / FC-13 / FC-14, DP-M01 (BFE Drive Auth UDS + DPoP), DP-M04 (current assignment ACL), DP-M05 (Centrifuge), DP-M03/DP-M07 (commit + history). Client-only DOC-002, DOC-005 and DOC-006 remain outside server acceptance.

## Reproducible infrastructure

Workflow: `.github/workflows/qa-dpm08-production-e2e.yml`.

- Chat source is checked out at immutable `github.sha`, verified against `git rev-parse HEAD`.
- Read-only `BFE_DRIVE_READ_TOKEN` is used solely for a credentials-free checkout of BFE Drive commit `9a33850a5456d8e3ff40f5cdab6ded869da7110f`.
- Compiles the **actual** `cmd/bfe-chat-service` production executable, `qa/dpm08probe` (external client), and `qa/driveauthprobe` (JKT fixture).
- Installs `qa/testdata/bfe_drive_chat_production_e2e_test.go` in the real BFE Drive `internal/httpapi` package. Testcontainers starts PostgreSQL **18.6** with separate `bfe` and `chatqa` databases and their own migrations.
- Runs real `NewInternalAuthHandler` over restricted temporary Unix socket, provisions BFE Drive sessions bound to the deterministic fixture JKT, starts the real Chat executable on loopback and sends actual HTTP requests and raw Centrifuge WebSocket protocol frames.
- No mock BFE Drive Auth endpoint, no in-process replacement of the production executable or Centrifuge node.

## Independently observable scenarios

`HTTP_ASSIGNMENT`, `HTTP_SEND`, `HTTP_HISTORY`, `HTTP_FORBIDDEN`, `WS_CONNECT`, `WS_SUBSCRIBE`, `WS_DELIVERY`, `HTTP_REVOKED`, `WS_RECONNECT_DENIED`, `HTTP_FRESH_SESSION`, `WS_FRESH_RECONNECT`, `OPEN_AFTER_REVOKE`, `WS_FRESH_DELIVERY`, `HTTP_UDS_UNAVAILABLE`, `WS_UDS_UNAVAILABLE`.

Business invariant: **revocation of a BFE Drive session MUST NOT forcibly disconnect an existing WebSocket**. The harness keeps the pre-revocation TCP/WebSocket connection open; after revocation a new BFE Drive session posts a new message and the **original connection** must still receive its publication.

Fresh proof is generated for each HTTP request and each Centrifuge Connect. Reconnect with revoked credentials must produce explicit invalid-token Centrifuge denial, not a timeout or arbitrary network error.

## Secret handling

Tokens are randomly provisioned in the fixture. The probe receives them in private environment/stdin, never command-line arguments or URL. Its stdout consists only of fixed milestone labels; stderr and production process logs are suppressed in CI. The Go test's raw stdout/stderr is redirected to a non-uploaded runner temporary file. A private marker file contains only fixed labels and PASS, and the uploaded `dpm08-e2e-safe-report.txt` is constructed **from an allowlist**, listing Chat/Drive SHA, each scenario PASS/FAIL and overall outcome. Tokens, DPoP/JWS, private keys, SQL DSNs and raw request/response payloads are not uploaded.

## Independent QA and specification decision

Verify the **actual workflow run**, compiler output, HTTP status codes, serialized PostgreSQL message persistence, Centrifuge event framing, distinct-session proof JTI, revoke/reconnect semantics and secret-free artifact before PASS. Check no regression of DP-M01 or DP-M05.

**Open specification question:** `docs/DP-M08-JSON-DUPLICATE-KEYS-QUESTION.md`. GRACE does not specify duplicate JSON key policy. The current Go decoder behavior is unchanged until the Command Center approves a policy. Do not treat duplicate-key rejection as an approved test expectation.

No changes to approved GRACE XML or production BFE Drive. Do not merge without independent QA.
