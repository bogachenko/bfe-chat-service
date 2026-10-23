#!/usr/bin/env bash
set -euo pipefail
fail(){ printf 'BLOCKED: %s\n' "$*" >&2; exit 1; }
: "${BFE_TEST_VPS_SSH_TARGET:?BFE_TEST_VPS_SSH_TARGET is required}"
: "${BFE_TEST_PUBLIC_HOST:?BFE_TEST_PUBLIC_HOST is required}"
[[ "$BFE_TEST_VPS_SSH_TARGET" =~ ^[a-zA-Z0-9_.@:-]+$ ]] || fail 'invalid SSH target'
[[ "$BFE_TEST_PUBLIC_HOST" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z]{2,}$ ]] || fail 'invalid public hostname'
ssh_args=(-o BatchMode=no -o PreferredAuthentications=password,keyboard-interactive -o PubkeyAuthentication=no -o StrictHostKeyChecking=yes -o ConnectTimeout=10)
if [[ -n "${BFE_TEST_VPS_SSH_IDENTITY_FILE:-}" ]]; then
  [[ "$BFE_TEST_VPS_SSH_IDENTITY_FILE" = /* && -r "$BFE_TEST_VPS_SSH_IDENTITY_FILE" ]] || fail 'invalid SSH key path'
  ssh_args+=(-i "$BFE_TEST_VPS_SSH_IDENTITY_FILE")
fi
printf 'TEST CHAT BOOTSTRAP PREFLIGHT | public_host=%s\n' "$BFE_TEST_PUBLIC_HOST"
ssh "${ssh_args[@]}" "$BFE_TEST_VPS_SSH_TARGET" 'bash -s' <<'REMOTE'
set -eu
report(){
  label="$1"; shift
  if "$@" >/dev/null 2>&1; then printf 'PASS %s\n' "$label"; else printf 'MISSING %s\n' "$label"; fi
}
report 'non-root SSH' sh -c '[ "$(id -u)" -ne 0 ]'
report 'noninteractive sudo' sudo -n true
report 'systemd' command -v systemctl
report 'PostgreSQL client' command -v psql
report 'Nginx' command -v nginx
report 'BFE Drive active' systemctl is-active --quiet bfe-drive.service
report 'Chat service exists' systemctl cat bfe-chat-service.service
report 'Chat system user exists' id bfe-chat
report 'Chat DB exists' sh -c 'sudo -n -u postgres psql -XAt -d bfe_chat -c "SELECT 1" | grep -qx 1'
report 'Chat service binary exists' test -x /usr/local/bin/bfe-chat-service
report 'Drive Nginx extension directory' test -d /etc/nginx/snippets/bfe-drive.local.d
report 'Chat DB migrations directory' test -d /var/lib/bfe-chat-service/migrations
report 'Chat release state file' sudo -n test -s /var/lib/bfe-chat-service/runtime-release.json
printf '%s\n' 'CHECK MANUALLY: approved HTTPS host, Drive Auth UDS path and permissions, DB backups, nginx routing, public DPoP origin, and migration authorization'
REMOTE
