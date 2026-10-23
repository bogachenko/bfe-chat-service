# BFE Chat: test-only runtime update

`make update-test-runtime` updates an **already provisioned** Chat runtime on the test VPS. The target comes exclusively from `BFE_TEST_VPS_SSH_TARGET`; the bare public hostname comes from `BFE_TEST_PUBLIC_HOST`. Test SSH explicitly disables public-key authentication to prevent agent key attempts and requests a password interactively. Passwords are not stored. Host-key checking remains strict. Privileged deployment operations still require separately configured noninteractive sudo (`sudo -n`).

```sh
make update-test-runtime
```

`make update-runtime` intentionally exits with an error. No production Chat deployment is authorized.

## Required test VPS baseline (not installed by this updater)

- Active dedicated `bfe-chat-service.service` with `/usr/local/bin/bfe-chat-service`, listening on loopback `127.0.0.1:8081`.
- Dedicated PostgreSQL database `bfe_chat` and local Linux identity `bfe-chat` able to query `goose_db_version`.
- Separate BFE Drive Internal Auth UDS, explicitly accessible to Chat without weakening socket permissions. The Chat service config supplies `-public-origin=https://<approved-host>`, `-drive-auth-uds=...`, `-migrations=/opt/bfe-chat-service/migrations`, and a protected DB DSN through `BFE_CHAT_DATABASE_URL` or a configured alternative.
- Host HTTPS ingress exposing `/conversations` and `/ws` without rewriting DPoP-bound URLs; WebSocket forwarding configured separately. Validate whether Drive and Chat can safely share public hostname before provisioning.
- Root-owned `/var/lib/bfe-chat-service/runtime-release.json` (mode 0600) containing `public_host`, `service` (`bfe-chat-service.service`), `commit`, `binary_sha256`, and `migrations_sha256`. These entries must identify an already working approved installation.
- `/opt/bfe-chat-service/migrations` byte-identical to the repository's SQL migration tree, with matching already-applied Goose database schema version.
- Test SSH identity permitted to read systemd state, architecture, and transfer an artifact into `/tmp`, plus appropriately scoped noninteractive sudo for Chat-only update operations. Do not modify BFE Drive service, database or secrets.

## Update semantics

The script refuses to proceed if host/provenance checks, SHA256 hashes, migration digest, database migration version, forward-only Git ancestry, or clean checkout fail. It compiles a Linux binary from a committed Git archive, transfers to a private temporary path, checks integrity, atomically replaces only Chat's binary, and restarts only Chat's systemd service. It requires a healthy service and unauthenticated HTTP `401` from `127.0.0.1:8081/conversations`. If restart/readiness fails, it restores the previous binary. Provenance updates only on success.

This is an **updater**, not a bootstrap installer: if Chat has never been installed, it exits before remote writes. Do not run it against a newly provisioned service or DB until the initial baseline is reviewed and approved. The application's `openChatDB` invokes Goose at startup; although identical migration digest/version checks are enforced, initial schema provisioning, backups and any new migrations require a distinct owner-approved procedure. Production remains disabled.

## Verification still required before first test use

Independent QA: shell syntax, fail-closed behavior and stubbed remote operations; review of least-privilege sudo and systemd settings, database read-only provenance checks, ingress/DPoP, `/ws` readiness and server-side rollback. After PR quality checks and explicit owner approval, perform first test VPS rollout followed by authenticated end-to-end tests. Neither adding this script nor merging its PR authorizes remote deployment.
