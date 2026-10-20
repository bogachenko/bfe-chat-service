#!/usr/bin/env bash
# Upload and execute the reviewed Chat-only provisioning payload on the pinned VPS.
set -euo pipefail
: "${BFE_TEST_VPS_SSH_TARGET:?required}"
: "${BFE_CHAT_TEST_VPS_BASELINE:?required}"
: "${BFE_CHAT_TEST_PUBLIC_ORIGIN:?required}"
: "${BFE_CHAT_TEST_DRIVE_AUTH_UDS:?required}"
[[ "${BFE_CHAT_TEST_PROVISION_APPROVED:-}" == YES ]] || {
  echo 'ERROR: set BFE_CHAT_TEST_PROVISION_APPROVED=YES after review' >&2; exit 1;
}
[[ "$BFE_CHAT_TEST_PUBLIC_ORIGIN" == https://chat.webshopstudio.ru ]] || exit 1
[[ "$BFE_CHAT_TEST_DRIVE_AUTH_UDS" == /run/bfe-drive-internal-auth/validate.sock ]] || exit 1
[[ "$BFE_TEST_VPS_SSH_TARGET" =~ ^[a-zA-Z0-9_.@-]+$ ]] || exit 1
[[ -r "$BFE_CHAT_TEST_VPS_BASELINE" ]] || exit 1
[[ -r deploy/scripts/provision-test-runtime.sh && -r deploy/nginx/bfe-chat.conf ]] || exit 1
for bin in python3 ssh tar sha256sum git; do command -v "$bin" >/dev/null || exit 1; done
git diff --quiet && git diff --cached --quiet || {
  echo 'ERROR: tracked working tree must be clean' >&2; exit 1;
}
ssh_sha="$(printf '%s' "$BFE_TEST_VPS_SSH_TARGET" | sha256sum | awk '{print $1}')"
machine_sha="$(python3 - "$BFE_CHAT_TEST_VPS_BASELINE" "$ssh_sha" <<'PY'
import json, re, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    data = json.load(fh)
assert data["ssh_target_sha256"] == sys.argv[2], "unexpected SSH target"
assert re.fullmatch(r"[0-9a-f]{64}", data["machine_id_sha256"])
print(data["machine_id_sha256"])
PY
)"
ssh_args=(-o StrictHostKeyChecking=yes -o BatchMode=no -o PreferredAuthentications=password)
if [[ -n "${BFE_TEST_VPS_SSH_IDENTITY_FILE:-}" ]]; then
  [[ -r "$BFE_TEST_VPS_SSH_IDENTITY_FILE" ]] || exit 1
  ssh_args=(-o StrictHostKeyChecking=yes -o BatchMode=yes -o IdentitiesOnly=yes -i "$BFE_TEST_VPS_SSH_IDENTITY_FILE")
fi
remote() { ssh "${ssh_args[@]}" "$BFE_TEST_VPS_SSH_TARGET" "$@"; }
actual="$(remote 'cat /etc/machine-id')"
[[ "$(printf '%s' "$actual" | sha256sum | awk '{print $1}')" == "$machine_sha" ]] || {
  echo 'ERROR: wrong VPS machine identity' >&2; exit 1;
}
# One transport and execution: no stage left on the VPS.
tar -C deploy -cf - scripts/provision-test-runtime.sh nginx/bfe-chat.conf |
  remote 'sudo -n env BFE_CHAT_PROVISION_APPROVED=YES bash -c '"'"'
    set -euo pipefail
    stage="$(mktemp -d /var/tmp/bfe-chat-provision.XXXXXXXX)"
    trap "rm -rf -- $stage" EXIT
    tar -C "$stage" -xf -
    BFE_CHAT_NGINX_SOURCE="$stage/nginx/bfe-chat.conf" bash "$stage/scripts/provision-test-runtime.sh"
  '"'"''
