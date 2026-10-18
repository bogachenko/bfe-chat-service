# BFE Chat Service — Shared Test VPS Runtime (Draft / Independent QA)

**Status:** DRAFT, NO VPS DEPLOYMENT AUTHORIZED. Applies ONLY to Chat Service test runtime. Source of conventions: BFE Drive `Makefile:update-test-runtime` and `deploy/scripts/update-runtime.sh`; no writes to the BFE Drive repository or its runtime resources.

## Environment: reuse existing BFE Drive test VPS

Required local environment:
- `BFE_TEST_VPS_SSH_TARGET`: **same value as BFE Drive test runtime**. Never substitute production.
- `BFE_TEST_PUBLIC_HOST`: existing test HTTPS hostname (same as BFE Drive).
- `BFE_CHAT_TEST_PUBLIC_ORIGIN`: **separate Chat HTTPS hostname** on the same VPS (for example `https://chat-test.example.org`), different from `https://$BFE_TEST_PUBLIC_HOST`. The actual DNS/TLS/virtual host must be approved by VPS operator.
- `BFE_CHAT_TEST_DRIVE_AUTH_UDS`: **actual** existing Drive Internal Auth socket absolute pathname on VPS. Never assume path.
- `BFE_CHAT_TEST_VPS_BASELINE=$HOME/.config/bfe-drive-test/vps-baseline.json`: existing approved BFE Drive test VPS baseline (SSH-target SHA and machine ID SHA). Path is explicit, never invented.
- Optional `BFE_TEST_VPS_SSH_IDENTITY_FILE`: absolute SSH private key path, used with `IdentitiesOnly=yes`. Host key must already be pinned in known_hosts.

Existing BFE Drive Makefile also requires `BFE_TEST_POSTBOX_SENDER_ADDRESS`. **Chat does not use Postbox**, so it does not require or repurpose that variable. BFE Drive state remains in `$HOME/.config/bfe-drive-test`.

## Host prerequisites — provisioned/reviewed outside this PR

1. BFE Drive, Nginx, and PostgreSQL already running; `/etc/machine-id` matches the approved BFE Drive test baseline.
2. Dedicated `bfe-chat` system user, PostgreSQL role and **separate** `bfe_chat` database with migrations permission; no access to `bfe_drive` tables or credentials.
3. Existing Chat-only `/etc/bfe-chat-service/runtime.env` (0600; owner `bfe-chat`) containing `BFE_CHAT_DATABASE_URL`, `BFE_CHAT_PUBLIC_ORIGIN`, `BFE_CHAT_DRIVE_AUTH_UDS`. The DSN is secret and MUST NOT be committed or passed as an SSH argument.
4. `bfe-chat` has OS read/connect permission to the **existing** BFE Drive Internal Auth UDS, without changing socket/service ownership globally or restarting Drive.
5. **Independent Nginx test configuration**, provisioned/reviewed by VPS operator, forwards Chat HTTP/WebSocket paths to `127.0.0.1:8081` under a **separate Chat HTTPS virtual host**, preserving DPoP HTU/Host and WebSocket Upgrade. It must not overwrite Drive locations or open port 8081 publicly. Script checks installed Nginx configuration for loopback backend and `nginx -t`, and **does not edit nginx**.
6. Free localhost port `8081`, otherwise it must already belong to the running `bfe-chat-service.service`. Make sure test DB backup/restore and PostgreSQL 18.6 compatibility were independently validated before any service startup that runs goose migrations.
7. `sudo -n` constrained to the reviewed operations; review least privilege. Separate service's binary `/usr/local/bin/bfe-chat-service`, unit `bfe-chat-service.service`, migrations `/opt/bfe-chat-service/migrations`, and provenance `/var/lib/bfe-chat-service/deploy-state` do not overlap BFE Drive.

## Commands

Read-only preflight: `make check-test-runtime` (uses SSH; makes no VPS changes).

After independent QA and explicit Command Center deployment authorization only:

```bash
BFE_CHAT_TEST_DEPLOY_APPROVED=YES make update-test-runtime
```

This builds from clean immutable Git HEAD (Go 1.27.2, target amd64/arm64), validates the VPS machine identity, service/database/UDS/Nginx prerequisites, transfers a Chat-only payload, installs the dedicated binary/unit/migrations, restarts **only** `bfe-chat-service.service`, and requires localhost listening on port 8081. Current implementation has **no automatic schema downgrade**, and cannot substitute for a reviewed backup/rollback and secrets-rotation procedure.

## Independent QA acceptance checklist

- Confirm the baseline and exact SSH/public host agree with BFE Drive test deployment; test `--check` against actual VPS **without** triggering deployment.
- Confirm no writes, restarts or ownership changes to BFE Drive, PostgreSQL, or Nginx.
- Inspect Nginx routing collisions (including `/ws` and `/v1/` paths) with BFE Drive before any rollout. Resolve conflicts via a separate reviewed ops change; this PR must not silently edit Drive routes.
- Verify dedicated PostgreSQL role/database and permissions, backup and migration safety, UDS group/ACL, unit hardening, and secret-file ownership.
- Verify Bash syntax/shellcheck and fail-closed behavior for missing vars, wrong machine, missing auth socket, occupied 8081, and absent service prerequisites.
- On a separately authorized deployment: check HTTPS/DPoP origin, HTTP unauthorized status, real WebSocket Upgrade, UDS auth, persistence, service restart/recovery, and BFE Drive availability.
- Do not mark as Production readiness or deploy before approval.

**Gate:** DRAFT PR / INDEPENDENT QA PENDING / VPS NOT TOUCHED / PRODUCTION RELEASE PROHIBITED.

## PR #24 remediation: rollback and scope checks

- Preflight is **read-only** and prints fixed PASS markers, never database rows, credentials or complete nginx configuration. It checks host baseline SHA, Drive uptime, restricted UDS, dedicated PostgreSQL role/database, **denial of CONNECT and table privileges in BFE Drive database**, nginx HTTPS `server_name` / `listen 443 ssl` / both HTTP and WebSocket upstream locations in **one** `server` block, and TCP 8081 PID matching systemd `MainPID`. A free port is allowed before initial deployment; a foreign listener is blocking.
- Candidate installation snapshots old **binary, unit and Chat migrations**. On failure it stops Chat and compares the Chat goose schema version. If unchanged, it restores old binary/unit/migrations and restarts only Chat. If schema advanced, schema check fails, or no complete old unit/binary exists, it **leaves Chat stopped** and requires manual recovery. PostgreSQL migrations are **never automatically rolled back**.
- Automated negative tests cover split vhosts, plain HTTP, incorrect proxy, duplicate hostname, foreign process, public listener, duplicate listeners and malformed config. `make test-test-runtime` is blocking within `make quality`. These tests do **not** touch the VPS.
- Limitations pending Independent QA: verify against the real existing test nginx topology, systemd unit semantics, DB privilege model and real service identity. `make check-test-runtime` must be run by the authorized test VPS operator as a separate read-only check. Review backup/migration recovery and SSH/sudo privilege minimization before any deployment.
