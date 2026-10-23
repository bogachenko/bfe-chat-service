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
- `/var/lib/bfe-chat-service/runtime-release.json` is created with mode 0600 following the first successful update of the already-running Chat service.
- `/opt/bfe-chat-service/migrations` byte-identical to the repository's SQL migration tree, with matching already-applied Goose database schema version.
- Test SSH identity permitted to read systemd state, architecture, and transfer an artifact into `/tmp`, plus appropriately scoped noninteractive sudo for Chat-only update operations. Do not modify BFE Drive service, database or secrets.

## Update semantics

The script requires an existing active Chat service and refuses to proceed when binary checksums, migration digest, database migration version, or clean checkout checks fail. It compiles a Linux binary from a committed Git archive, transfers to a private temporary path, checks integrity, atomically replaces only Chat's binary, and restarts only Chat's systemd service. It requires a healthy service and unauthenticated HTTP `401` from `127.0.0.1:8081/conversations`. If restart/readiness fails, it restores the previous binary. Provenance updates only on success.

This is an **updater**, not a bootstrap installer: if Chat has never been installed, it exits before remote writes. Do not run it against a newly provisioned service or DB until the initial baseline is reviewed and approved. The application's `openChatDB` invokes Goose at startup; although identical migration digest/version checks are enforced, initial schema provisioning, backups and any new migrations require a distinct owner-approved procedure. Production remains disabled.

## Additional release verification

Independent QA: shell syntax, fail-closed behavior and stubbed remote operations; review of least-privilege sudo and systemd settings, database read-only provenance checks, ingress/DPoP, `/ws` readiness and server-side rollback. The existing test VPS was updated successfully; local and public unauthenticated `/conversations` both returned `401 AUTH_INVALID`. Authenticated DPoP E2E and `/ws` subscription tests are still outstanding. Neither this script nor a merge authorizes production deployment.

## Versioned Chat Nginx configuration

The dedicated Chat vhost is versioned at `deploy/nginx/bfe-chat.conf`. It replaces the obsolete `/v1/` and `/v2/` locations with `/conversations` and `/conversations/`, retaining WebSocket `/ws`.

To repair only Chat ingress without rebuilding/restarting Chat:

```sh
make update-test-nginx
```

The command checks that `/etc/nginx/conf.d/bfe-chat.conf` already belongs to the expected Chat host and backend, uploads the repository template, backups the old vhost, validates `nginx -t`, reloads Nginx, and restores it on validation/reload failure. No BFE Drive file or application service is updated. Future `make update-test-runtime` runs also synchronize this vhost.
