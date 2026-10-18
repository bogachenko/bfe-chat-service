#!/usr/bin/env bash
set -euo pipefail

# BFE Chat Service test VPS deployment. Never writes to BFE Drive paths.
# --check is read-only. A deployment requires explicit QA and Command Center sign-off.
mode="${1:-}"
[[ -n "$mode" ]] || mode="--check"
[[ "$mode" == "--check" || "$mode" == "--deploy" ]] || { echo "usage: $0 --check|--deploy" >&2; exit 2; }
: "${BFE_TEST_VPS_SSH_TARGET:?required: BFE_TEST_VPS_SSH_TARGET}"
: "${BFE_TEST_PUBLIC_HOST:?required: BFE_TEST_PUBLIC_HOST}"
: "${BFE_CHAT_TEST_PUBLIC_ORIGIN:?required: BFE_CHAT_TEST_PUBLIC_ORIGIN}"
: "${BFE_CHAT_TEST_DRIVE_AUTH_UDS:?required: BFE_CHAT_TEST_DRIVE_AUTH_UDS}"
: "${BFE_CHAT_TEST_VPS_BASELINE:?required: BFE_CHAT_TEST_VPS_BASELINE (BFE Drive vps-baseline.json)}"
if [[ "$mode" == "--deploy" ]]; then
  [[ "${BFE_CHAT_TEST_DEPLOY_APPROVED:-}" == "YES" ]] || { echo "deployment blocked: BFE_CHAT_TEST_DEPLOY_APPROVED=YES required after Independent QA" >&2; exit 1; }
fi
for bin in git go ssh python3 sha256sum tar; do command -v "$bin" >/dev/null || { echo "missing: $bin" >&2; exit 1; }; done
[[ "$BFE_CHAT_TEST_PUBLIC_ORIGIN" =~ ^https://[a-zA-Z0-9.-]+$ ]] || { echo "Chat origin must be an HTTPS hostname without path/port" >&2; exit 1; }
chat_host="${BFE_CHAT_TEST_PUBLIC_ORIGIN#https://}"
[[ "$chat_host" != "$BFE_TEST_PUBLIC_HOST" ]] || { echo "Chat must use a different HTTPS virtual host to avoid Drive routing conflicts" >&2; exit 1; }
[[ "$BFE_CHAT_TEST_DRIVE_AUTH_UDS" == /* && "$BFE_CHAT_TEST_DRIVE_AUTH_UDS" != *"'"* ]] || { echo "UDS path must be absolute and contain no quote" >&2; exit 1; }
[[ "$BFE_TEST_VPS_SSH_TARGET" =~ ^[a-zA-Z0-9_.@-]+$ ]] || { echo "invalid SSH target" >&2; exit 1; }
[[ "$BFE_TEST_PUBLIC_HOST" =~ ^[a-zA-Z0-9.-]+$ ]] || { echo "invalid public host" >&2; exit 1; }
[[ -r "$BFE_CHAT_TEST_VPS_BASELINE" ]] || { echo "missing VPS baseline" >&2; exit 1; }
[[ "$(git rev-parse --is-inside-work-tree)" == true ]] || exit 1
git diff --quiet && git diff --cached --quiet || { echo "working tree must be clean" >&2; exit 1; }
commit="$(git rev-parse 'HEAD^{commit}')"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || exit 1
ssh_sha="$(printf '%s' "$BFE_TEST_VPS_SSH_TARGET" | sha256sum | awk '{print $1}')"
machine_sha="$(python3 - "$BFE_CHAT_TEST_VPS_BASELINE" "$ssh_sha" <<'PY'
import json, re, sys
x = json.load(open(sys.argv[1], encoding="utf-8"))
assert x["ssh_target_sha256"] == sys.argv[2], "SSH target differs from BFE Drive test VPS"
assert re.fullmatch(r"[0-9a-f]{64}", x["machine_id_sha256"])
print(x["machine_id_sha256"])
PY
)"
ssh_args=(-o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes)
if [[ -n "${BFE_TEST_VPS_SSH_IDENTITY_FILE:-}" ]]; then
  [[ "${BFE_TEST_VPS_SSH_IDENTITY_FILE}" == /* && -r "${BFE_TEST_VPS_SSH_IDENTITY_FILE}" ]] || exit 1
  ssh_args+=(-i "$BFE_TEST_VPS_SSH_IDENTITY_FILE")
fi
remote() { ssh "${ssh_args[@]}" "$BFE_TEST_VPS_SSH_TARGET" "$@"; }
actual="$(remote 'cat /etc/machine-id')"
[[ "$(printf '%s' "$actual" | sha256sum | awk '{print $1}')" == "$machine_sha" ]] || { echo "wrong VPS identity" >&2; exit 1; }
# The check is deliberately read-only, including the nginx and BFE Drive service checks.
remote "sudo -n env CHAT_ORIGIN='$BFE_CHAT_TEST_PUBLIC_ORIGIN' CHAT_HOST='$chat_host' CHAT_UDS='$BFE_CHAT_TEST_DRIVE_AUTH_UDS' bash -s" <<'REMOTE_CHECK'
set -euo pipefail
systemctl is-active --quiet bfe-drive.service
systemctl is-active --quiet nginx.service
systemctl is-active --quiet postgresql.service || systemctl is-active --quiet postgresql@18-main.service
nginx -t >/dev/null
test -S "$CHAT_UDS"
id bfe-chat >/dev/null
runuser -u bfe-chat -- test -r "$CHAT_UDS"
test -f /etc/bfe-chat-service/runtime.env
test "$(stat -c %a /etc/bfe-chat-service/runtime.env)" = 600
test "$(stat -c %U /etc/bfe-chat-service/runtime.env)" = bfe-chat
grep -Eq '^BFE_CHAT_DATABASE_URL=' /etc/bfe-chat-service/runtime.env
 grep -Fxq "BFE_CHAT_PUBLIC_ORIGIN=$CHAT_ORIGIN" /etc/bfe-chat-service/runtime.env
 grep -Fxq "BFE_CHAT_DRIVE_AUTH_UDS=$CHAT_UDS" /etc/bfe-chat-service/runtime.env
# Verify isolated role and database without returning credentials or database content.
role_sql="SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname='bfe-chat' AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls) AND EXISTS (SELECT 1 FROM pg_database WHERE datname='bfe_chat' AND pg_get_userbyid(datdba)='bfe-chat') AND has_database_privilege('bfe-chat','bfe_chat','CONNECT') AND NOT has_database_privilege('bfe-chat','bfe_drive','CONNECT') THEN 'PASS' ELSE 'FAIL' END"
runuser -u postgres -- psql -X -v ON_ERROR_STOP=1 -d postgres -Atqc "$role_sql" | grep -qx PASS
# Defense in depth: no grants to BFE Drive data even when the DB CONNECT ACL is restrictive.
runuser -u postgres -- psql -X -v ON_ERROR_STOP=1 -d bfe_drive -Atqc "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','S','f') AND (has_table_privilege('bfe-chat',c.oid,'SELECT') OR has_table_privilege('bfe-chat',c.oid,'INSERT') OR has_table_privilege('bfe-chat',c.oid,'UPDATE') OR has_table_privilege('bfe-chat',c.oid,'DELETE'))) THEN 'FAIL' ELSE 'PASS' END" | grep -qx PASS
runuser -u bfe-chat -- psql -X -v ON_ERROR_STOP=1 -h /run/postgresql -U bfe-chat -d bfe_chat -Atqc 'SELECT 1' | grep -qx 1
# Listener ownership and the correct HTTPS server block are evaluated locally from read-only probes.
REMOTE_CHECK
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
remote 'sudo -n nginx -T 2>&1' > "$tmp/nginx"
remote 'sudo -n ss -H -ltnp "( sport = :8081 )"' > "$tmp/ss"
pid="$(remote 'sudo -n systemctl show -p MainPID --value bfe-chat-service.service')" || pid=0
# Empty port is permitted only before initial deployment.
if [[ ! -s "$tmp/ss" ]]; then pid=2; fi
python3 deploy/scripts/check-test-runtime.py --nginx "$tmp/nginx" --host "$chat_host" --ss "$tmp/ss" --pid "$pid"
echo "PRECHECK: VPS_ID=PASS DRIVE_HEALTH=PASS UDS=PASS DB_ISOLATION=PASS NGINX=PASS PORT_OWNER=PASS"

echo "BFE Chat test VPS preflight: PASS (read-only, secret-free report)"
[[ "$mode" == "--deploy" ]] || { echo "CHECK ONLY: no VPS changes"; exit 0; }

# Reproducible candidate build; build locally, transport only binary, migrations, unit.
temp="$(mktemp -d)"; trap 'rm -rf "$temp"' EXIT
mkdir -p "$temp/source" "$temp/payload/migrations"
git archive "$commit" | tar -x -C "$temp/source"
grep -qx 'go 1.27.2' "$temp/source/go.mod" || { echo "unexpected Go version" >&2; exit 1; }
arch="$(remote 'dpkg --print-architecture')"
case "$arch" in amd64|arm64) ;; *) echo "unsupported arch" >&2; exit 1;; esac
(cd "$temp/source"; GOTOOLCHAIN=go1.27.2 CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$temp/payload/bfe-chat-service" ./cmd/bfe-chat-service)
cp -a "$temp/source/internal/storage/migrations/." "$temp/payload/migrations/"
cp "$temp/source/deploy/systemd/bfe-chat-service.service" "$temp/payload/bfe-chat-service.service"
sha="$(sha256sum "$temp/payload/bfe-chat-service" | awk '{print $1}')"
stage="/var/tmp/bfe-chat-test-runtime-$commit"
# Fixed, validated values only; payload isn't interpolated into shell commands.
remote "sudo -n install -d -o root -g root -m 0700 '$stage'"
tar -C "$temp/payload" -cf - . | remote "sudo -n tar -C '$stage' -xf -"
remote "sudo -n env CHAT_STAGE='$stage' CHAT_SHA='$sha' CHAT_COMMIT='$commit' bash -s" <<'REMOTE_DEPLOY'
set -euo pipefail
[[ "$(sha256sum "$CHAT_STAGE/bfe-chat-service" | awk '{print $1}')" == "$CHAT_SHA" ]]
# Snapshot all Chat-owned runtime assets before any mutation.
install -d -o bfe-chat -g bfe-chat -m 0700 /var/lib/bfe-chat-service
backup="$(mktemp -d /var/lib/bfe-chat-service/backup.XXXXXXXX)"
chmod 0700 "$backup"
had_binary=0; had_unit=0; had_migrations=0
if [[ -f /usr/local/bin/bfe-chat-service ]]; then
  cp -a /usr/local/bin/bfe-chat-service "$backup/binary"
  had_binary=1
fi
if [[ -f /etc/systemd/system/bfe-chat-service.service ]]; then
  cp -a /etc/systemd/system/bfe-chat-service.service "$backup/unit"
  had_unit=1
fi
if [[ -d /opt/bfe-chat-service/migrations ]]; then
  cp -a /opt/bfe-chat-service/migrations "$backup/migrations"
  had_migrations=1
fi
# Take the pre-deployment schema snapshot from the independent Chat database.
schema_before="$(runuser -u bfe-chat -- psql -X -v ON_ERROR_STOP=1 -h /run/postgresql -U bfe-chat -d bfe_chat -Atqc "SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied")"
[[ "$schema_before" =~ ^[0-9]+$ ]] || { echo "ERROR: cannot verify current Chat schema; deployment stopped" >&2; exit 1; }
rollback() {
  systemctl stop bfe-chat-service.service || true
  schema_after="$(runuser -u bfe-chat -- psql -X -v ON_ERROR_STOP=1 -h /run/postgresql -U bfe-chat -d bfe_chat -Atqc "SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied" 2>/dev/null)" || schema_after=UNKNOWN
  if [[ "$schema_after" != "$schema_before" ]]; then
    echo "ERROR: Chat schema changed or cannot be verified ($schema_before -> $schema_after); automatic rollback is UNSAFE. Chat stopped; manual recovery required." >&2
    return 1
  fi
  if [[ "$had_binary" -ne 1 || "$had_unit" -ne 1 ]]; then
    echo "ERROR: no complete previous binary/systemd unit; safe rollback impossible. Chat stopped." >&2
    return 1
  fi
  install -o root -g root -m 0755 "$backup/binary" /usr/local/bin/bfe-chat-service
  install -o root -g root -m 0644 "$backup/unit" /etc/systemd/system/bfe-chat-service.service
  if [[ "$had_migrations" -eq 1 ]]; then
    rm -rf /opt/bfe-chat-service/migrations
    cp -a "$backup/migrations" /opt/bfe-chat-service/migrations
  fi
  systemctl daemon-reload
  systemctl restart bfe-chat-service.service || { echo "ERROR: previous Chat service failed to restart" >&2; return 1; }
  systemctl is-active --quiet bfe-chat-service.service || return 1
  echo "ERROR: candidate failed; previous Chat binary and unit restored; schema unchanged" >&2
}
failure() {
  trap - ERR
  if ! rollback; then
    echo "ERROR: unsafe rollback; Chat stopped for operator intervention" >&2
  fi
  exit 1
}
trap failure ERR
install -o root -g root -m 0755 "$CHAT_STAGE/bfe-chat-service" /usr/local/bin/bfe-chat-service.new
mv -f /usr/local/bin/bfe-chat-service.new /usr/local/bin/bfe-chat-service
install -d -o root -g root -m 0755 /opt/bfe-chat-service/migrations
cp -a "$CHAT_STAGE/migrations/." /opt/bfe-chat-service/migrations/
chown -R root:root /opt/bfe-chat-service/migrations
install -o root -g root -m 0644 "$CHAT_STAGE/bfe-chat-service.service" /etc/systemd/system/bfe-chat-service.service
systemctl daemon-reload
systemctl enable bfe-chat-service.service
systemctl restart bfe-chat-service.service
systemctl is-active --quiet bfe-chat-service.service
main_pid="$(systemctl show -p MainPID --value bfe-chat-service.service)"
[[ "$main_pid" =~ ^[1-9][0-9]*$ ]]
ss -H -ltnp '( sport = :8081 )' | grep -Fq "pid=$main_pid,"
ss -H -ltnp '( sport = :8081 )' | grep -Fq '127.0.0.1:8081'
install -d -o root -g root -m 0700 /var/lib/bfe-chat-service/deploy-state
printf '%s %s\n' "$CHAT_COMMIT" "$CHAT_SHA" > /var/lib/bfe-chat-service/deploy-state/test-release.txt
chmod 0600 /var/lib/bfe-chat-service/deploy-state/test-release.txt
trap - ERR
rm -rf "$CHAT_STAGE"
echo "Chat runtime update and readiness: PASS"

REMOTE_DEPLOY
echo "BFE Chat test runtime: DEPLOYED commit=$commit binary_sha256=$sha"
