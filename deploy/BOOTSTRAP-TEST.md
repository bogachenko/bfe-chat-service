# First-time TEST bootstrap — inspect only

The updater in this PR deliberately does **not** provision the first Chat installation. Run `bash deploy/scripts/preflight-test-bootstrap.sh` with `BFE_TEST_VPS_SSH_TARGET` and `BFE_TEST_PUBLIC_HOST` to report the baseline without changing remote files, services, DB, nginx or firewall.

Candidate templates under `deploy/templates/` are **not applied** by this repository. They must be adapted and reviewed against the actual test VPS configuration before authorizing any changes.

## Gating decisions

1. Confirm the real BFE Drive Internal Auth UDS location, expected HTTP protocol and test-service permission model (do not use `chmod 777`).
2. Confirm whether the configured `BFE_TEST_PUBLIC_HOST` is the Drive TLS host and whether inserting Chat routes into `bfe-drive.local.d` is appropriate. If different, use its own approved HTTPS virtual host; preserve URL paths for DPoP htu and /ws.
3. Plan dedicated `bfe-chat` Linux user and `bfe_chat` PostgreSQL database, DB backup, Goose migration bootstrap; `openChatDB` runs `goose.UpContext` on startup, so first service start is a DB-changing operation requiring approval.
4. Provide protected `/etc/bfe-chat-service/runtime.env`, with `BFE_CHAT_DATABASE_URL`, `BFE_CHAT_PUBLIC_ORIGIN` and `BFE_CHAT_DRIVE_AUTH_UDS`. Confirm that systemd passes config args and environment safely; do not expose secrets in `ExecStart` arguments or logs.
5. Plan first binary deployment and initial `runtime-release.json` provenance. Verify rollback, service health, DPoP authenticated requests and WebSocket subscription before enabling `make update-test-runtime`.

**No file here authorizes first install, migration, nginx reload, or production deployment.**

SSH uses standard authentication according to the user's SSH configuration (including password when allowed). The script does not store passwords; its `sudo -n` checks remain noninteractive and may show MISSING when passwordless sudo is not configured.
