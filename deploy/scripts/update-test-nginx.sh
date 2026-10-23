#!/usr/bin/env bash
set -euo pipefail
fail(){ echo "ERROR: $*" >&2; exit 1; }
: "${BFE_TEST_VPS_SSH_TARGET:?BFE_TEST_VPS_SSH_TARGET required}"
CHAT_HOST="${BFE_TEST_CHAT_PUBLIC_HOST:-chat.webshopstudio.ru}"
[[ "$CHAT_HOST" =~ ^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[A-Za-z]{2,}$ ]] || fail 'invalid Chat hostname'
[[ "$BFE_TEST_VPS_SSH_TARGET" =~ ^[a-zA-Z0-9_.@:-]+$ ]] || fail 'invalid SSH target'
ROOT="$(git rev-parse --show-toplevel)"; cd "$ROOT"
TEMPLATE=deploy/nginx/bfe-chat.conf
[[ -f "$TEMPLATE" ]] || fail 'missing Chat nginx vhost template'
for c in ssh scp sed mktemp; do command -v "$c" >/dev/null || fail "missing $c"; done
SSH=(ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=3 -o StrictHostKeyChecking=yes)
SCP=(scp -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=3 -o StrictHostKeyChecking=yes)
TEMP="$(mktemp)"; trap 'rm -f "$TEMP"' EXIT
sed "s/__CHAT_HOST__/$CHAT_HOST/g" "$TEMPLATE" > "$TEMP"
# Existing dedicated Chat vhost only: never modify the Drive vhost.
STAGE="$( "${SSH[@]}" "$BFE_TEST_VPS_SSH_TARGET" 'mktemp /tmp/bfe-chat-nginx.XXXXXXXX' )"
[[ "$STAGE" =~ ^/tmp/bfe-chat-nginx\.[A-Za-z0-9]+$ ]] || fail 'unsafe remote stage'
"${SCP[@]}" "$TEMP" "$BFE_TEST_VPS_SSH_TARGET:$STAGE"
"${SSH[@]}" "$BFE_TEST_VPS_SSH_TARGET" "sudo -n env STAGE='$STAGE' EXPECTED_HOST='$CHAT_HOST' bash -s" <<'REMOTE'
set -euo pipefail
CONF=/etc/nginx/conf.d/bfe-chat.conf
[[ -f "$CONF" ]] || { echo 'Chat nginx vhost missing; refusing new installation' >&2; exit 1; }
grep -Fq "server_name $EXPECTED_HOST;" "$CONF" || { echo 'Chat vhost host mismatch' >&2; exit 1; }
grep -Fq 'proxy_pass http://127.0.0.1:8081;' "$CONF" || { echo 'Chat vhost ownership mismatch' >&2; exit 1; }
BACKUP="$(mktemp /etc/nginx/conf.d/.bfe-chat-backup.XXXXXXXX)"
cp -p "$CONF" "$BACKUP"
cleanup(){ rm -f "$STAGE"; }
trap cleanup EXIT
install -m 0644 -o root -g root "$STAGE" "$CONF"
if ! nginx -t; then cp -p "$BACKUP" "$CONF"; rm -f "$BACKUP"; echo 'nginx syntax invalid; restored' >&2; exit 1; fi
if ! systemctl reload nginx; then
    cp -p "$BACKUP" "$CONF"
    nginx -t && systemctl reload nginx || true
    rm -f "$BACKUP"
    echo 'nginx reload failed; restored' >&2
    exit 1
fi
rm -f "$BACKUP"
echo 'BFE Chat test Nginx update PASS'
REMOTE
