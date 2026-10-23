#!/usr/bin/env bash
set -euo pipefail
# Updates an ALREADY PROVISIONED BFE Chat test runtime. No DB migrations/provisioning.
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
for c in git ssh scp tar go python3 sha256sum; do command -v "$c" >/dev/null || fail "missing tool: $c"; done
: "${BFE_TEST_VPS_SSH_TARGET:?BFE_TEST_VPS_SSH_TARGET required}"
: "${BFE_TEST_PUBLIC_HOST:?BFE_TEST_PUBLIC_HOST required}"
[[ "$BFE_TEST_VPS_SSH_TARGET" =~ ^[A-Za-z0-9_.@:-]+$ ]] || fail 'unsafe SSH target'
[[ "$BFE_TEST_PUBLIC_HOST" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z]{2,}$ ]] || fail 'public host must be bare DNS name'
ROOT="$(git rev-parse --show-toplevel)" || fail 'not a git repository'
cd "$ROOT"
[[ "$(git remote get-url origin)" == *bogachenko/bfe-chat-service* ]] || fail 'wrong repository'
git diff --quiet && git diff --cached --quiet || fail 'dirty checkout'
CANDIDATE="$(git rev-parse HEAD)"
[[ "$CANDIDATE" =~ ^[a-f0-9]{40}$ ]] || fail 'invalid candidate SHA'
MIG=internal/storage/migrations
[[ -d "$MIG" ]] || fail 'migrations missing'
migration_digest() {
  (cd "$1" && find . -type f -name '*.sql' -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1)
}
MIG_SHA="$(migration_digest "$MIG")"
MIG_VER="$(find "$MIG" -type f -name '*.sql' -printf '%f\n' | sed -n 's/^0*\([1-9][0-9]*\)_.*/\1/p' | sort -n | tail -1)"
[[ "$MIG_VER" =~ ^[0-9]+$ ]] || fail 'invalid migrations'
SSH=(ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=3 -o StrictHostKeyChecking=yes -o ConnectTimeout=10)
SCP=(scp -o PubkeyAuthentication=no -o PreferredAuthentications=password,keyboard-interactive -o NumberOfPasswordPrompts=3 -o StrictHostKeyChecking=yes -o ConnectTimeout=10)
if [[ -n "${BFE_TEST_VPS_SSH_IDENTITY_FILE:-}" ]]; then
  [[ "$BFE_TEST_VPS_SSH_IDENTITY_FILE" = /* && -r "$BFE_TEST_VPS_SSH_IDENTITY_FILE" ]] || fail 'invalid SSH identity'
  SSH+=(-i "$BFE_TEST_VPS_SSH_IDENTITY_FILE")
  SCP+=(-i "$BFE_TEST_VPS_SSH_IDENTITY_FILE")
fi
remote() { "${SSH[@]}" "$BFE_TEST_VPS_SSH_TARGET" "$@"; }
printf 'BFE Chat test preflight: commit=%s host=%s\n' "$CANDIDATE" "$BFE_TEST_PUBLIC_HOST"
# Preflight is READ-ONLY: reject any host which has not explicitly been provisioned.
INFO="$(remote "sudo -n env EXPECTED_HOST='$BFE_TEST_PUBLIC_HOST' bash -s" <<'REMOTE'
set -euo pipefail
BIN=/usr/local/bin/bfe-chat-service
STATE=/var/lib/bfe-chat-service/runtime-release.json
MIG=/var/lib/bfe-chat-service/migrations
systemctl is-active --quiet bfe-chat-service.service
test -x "$BIN" && test -s "$STATE" && test -d "$MIG"
python3 - "$STATE" "$EXPECTED_HOST" <<'PY'
import json,re,sys
s=json.load(open(sys.argv[1]))
assert s.get('public_host')==sys.argv[2] and s.get('service')=='bfe-chat-service.service'
for key,n in (('commit',40),('binary_sha256',64),('migrations_sha256',64)):
    assert re.fullmatch('[a-f0-9]{%d}'%n,s.get(key,''))
print(s['commit'],s['binary_sha256'],s['migrations_sha256'])
PY
sha256sum "$BIN" | cut -d' ' -f1
(cd "$MIG"; find . -name '*.sql' -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1)
runuser -u bfe-chat -- psql -X -v ON_ERROR_STOP=1 -h /run/postgresql -d bfe_chat -Atqc 'SELECT COALESCE(MAX(version_id) FILTER (WHERE is_applied),0) FROM goose_db_version'
REMOTE
)" || fail 'test runtime not provisioned or preflight failed; NO CHANGES'
mapfile -t LINES <<< "$INFO"
[[ "${#LINES[@]}" == 4 ]] || fail 'unexpected preflight response'
read -r OLD_COMMIT OLD_SHA OLD_MIG <<< "${LINES[0]}"
[[ "$OLD_SHA" == "${LINES[1]}" ]] || fail 'deployed binary drift'
[[ "$OLD_MIG" == "$MIG_SHA" && "${LINES[2]}" == "$MIG_SHA" ]] || fail 'migration drift/change: separate approval required'
[[ "${LINES[3]}" == "$MIG_VER" ]] || fail 'schema version mismatch: separate migration approval required'
git cat-file -e "$OLD_COMMIT^{commit}" || fail 'deployed commit missing locally'
git merge-base --is-ancestor "$OLD_COMMIT" "$CANDIDATE" || fail 'non-forward update refused'
if [[ "$OLD_COMMIT" == "$CANDIDATE" ]]; then echo 'BFE Chat test: already current'; exit 0; fi
ARCH="$(remote 'dpkg --print-architecture')"
case "$ARCH" in amd64|arm64) ;; *) fail 'unsupported VPS architecture';; esac
TMP="$(mktemp -d)"; STAGE=''
cleanup() { [[ -z "$STAGE" ]] || remote "rm -rf '$STAGE'" >/dev/null 2>&1 || true; rm -rf "$TMP"; }
trap cleanup EXIT
mkdir "$TMP/src"
git archive "$CANDIDATE" | tar -xf - -C "$TMP/src"
(cd "$TMP/src" && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -o "$TMP/bfe-chat-service" ./cmd/bfe-chat-service)
NEW_SHA="$(sha256sum "$TMP/bfe-chat-service" | cut -d' ' -f1)"
STAGE="$(remote 'mktemp -d /tmp/bfe-chat-update.XXXXXXXX')"
[[ "$STAGE" =~ ^/tmp/bfe-chat-update\.[A-Za-z0-9]+$ ]] || fail 'unsafe staging directory'
"${SCP[@]}" "$TMP/bfe-chat-service" "$BFE_TEST_VPS_SSH_TARGET:$STAGE/new"
remote "sudo -n env STAGE='$STAGE' CANDIDATE='$CANDIDATE' NEW_SHA='$NEW_SHA' OLD_SHA='$OLD_SHA' bash -s" <<'REMOTE'
set -euo pipefail
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
BIN=/usr/local/bin/bfe-chat-service
STATE=/var/lib/bfe-chat-service/runtime-release.json
SERVICE=bfe-chat-service.service
[[ "$(sha256sum "$BIN" | cut -d' ' -f1)" == "$OLD_SHA" ]] || die 'binary changed since preflight'
[[ "$(sha256sum "$STAGE/new" | cut -d' ' -f1)" == "$NEW_SHA" ]] || die 'upload hash mismatch'
install -m 0755 -o root -g root "$BIN" "$STAGE/previous"
install -m 0755 -o root -g root "$STAGE/new" "$BIN.next"
mv -f "$BIN.next" "$BIN"
rollback() {
  install -m 0755 -o root -g root "$STAGE/previous" "$BIN.restore"
  mv -f "$BIN.restore" "$BIN"
  systemctl restart "$SERVICE" || true
}
if ! systemctl restart "$SERVICE"; then rollback; die 'restart failed; old binary restored'; fi
ready=0
for i in $(seq 1 20); do
  code="$(curl -s --max-time 2 -o /dev/null -w '%{http_code}' http://127.0.0.1:8081/conversations || true)"
  if systemctl is-active --quiet "$SERVICE" && [[ "$code" == 401 ]]; then ready=1; break; fi
  sleep 1
done
if [[ "$ready" -ne 1 ]]; then rollback; die 'readiness failed; old binary restored'; fi
python3 - "$STATE" "$CANDIDATE" "$NEW_SHA" <<'PY'
import json,os,sys,tempfile
path,commit,sha=sys.argv[1:]
s=json.load(open(path));s['commit']=commit;s['binary_sha256']=sha
fd,tmp=tempfile.mkstemp(dir=os.path.dirname(path),prefix='.runtime-',text=True)
with os.fdopen(fd,'w') as f: json.dump(s,f,indent=2,sort_keys=True);f.write('\n')
os.chmod(tmp,0o600);os.replace(tmp,path)
PY
printf 'BFE Chat TEST UPDATE PASS: %s\n' "$CANDIDATE"
REMOTE
