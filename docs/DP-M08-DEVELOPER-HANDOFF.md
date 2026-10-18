# DP-M08 Developer handoff (FC-13 / FC-14)

Baseline: `main` at `e9c138f905665b7eead34923a9f4d3761fcc19c2`.
Approved source: `Contracts.xml` MC-08 FC-13/FC-14, `DevelopmentPlan.xml` DP-M08 and `MentalScenarios.xml`. This change does not amend GRACE.

## Implementation scope

`internal/transport.NewHandler` binds the existing DP-M05 `realtime.Service` to the Centrifuge node (DP-M01 OnConnecting and DP-M04 subscription checks), and exposes a WebSocket handler on `GET /ws`. The WebSocket client must put opaque access token and fresh DPoP proof in Centrifuge Connect frame, never URL parameters.

HTTP operations delegate to approved module services:
- `POST /v1/conversations/{ownerUUID}/messages` — FC-05, body `{idempotencyKey,text,document?:{id,title}}`.
- `GET /v1/conversations/{ownerUUID}/history` — FC-06, optional `X-After-ID` and `X-Limit` headers, to avoid credentials or payload in URL.
- `PUT /v1/conversations/{ownerUUID}/assignment` — FC-03, `{recipientAccountId}`.
- HTTP auth: `Authorization: DPoP <opaque token>` and `DPoP: <proof>`, verified by DP-M01 services with trusted configured public origin and request method/path. No client-supplied account identity is trusted.

All errors use bounded public reason codes and never echo underlying errors, tokens or proofs. Bodies have a finite size, strict JSON fields, single-object requirement and `Cache-Control: no-store`. The transport neither creates session tickets nor directly reads Drive ACL/data.

## Integration boundary

The production executable is wired to the approved UDS validator, PostgreSQL/pgx+goose, authorization, Centrifuge node, and DP-M08 HTTP router. It binds only a loopback HTTP address, for the existing external HTTPS reverse proxy, and requires explicit `-public-origin`, `-drive-auth-uds`, `-migrations` and a PostgreSQL DSN supplied via the named environment variable (`-dsn-env`). Startup fails closed on missing dependencies. Runtime deployment configuration, socket OS permissions, reverse-proxy TLS and DPoP freshness policy must be reviewed independently before production rollout.

## Independent QA

Verify FC-13/FC-14 routing and real Centrifuge handshake (token and DPoP absent from URLs/logs), unauthorized/malformed inputs, Auth unavailable classification, owner/recipient/ADMIN policies, document-only message payload, assignment conflicts, PostgreSQL persistence/idempotency, history and reconnect. Validate configured public origin matches external nginx HTTPS ingress and DPoP HTU. Client-only DOC-002, DOC-005, DOC-006 are excluded from server acceptance. QA must run full Quality Gates and independent E2E before approval. No independent QA tests were modified.
