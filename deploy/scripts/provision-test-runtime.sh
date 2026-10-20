#!/usr/bin/env bash
# Chat-owned resources only. BFE Drive paths and PostgreSQL objects are read-only.
# Requires a reviewed Nginx source from the same Chat git checkout.
set -euo pipefail

[[ "$EUID" -eq 0 ]] || { echo 'ERROR: run as root' >&2; exit 1; }
[[ "${BFE_CHAT_PROVISION_APPROVED:-}" == YES ]] || {
  echo 'ERROR: BFE_CHAT_PROVISION_APPROVED=YES required' >&2; exit 1;
}
for bin in id getent useradd usermod install stat runuser psql grep; do
  command -v "$bin" >/dev/null || { echo "ERROR: missing $bin" >&2; exit 1; }
done

readonly chat_user=bfe-chat
readonly chat_group=bfe-internal-auth
readonly chat_uds=/run/bfe-drive-internal-auth/validate.sock
readonly env_dir=/etc/bfe-chat-service
readonly env_file="$env_dir/runtime.env"
readonly dsn='host=/run/postgresql user=bfe-chat dbname=bfe_chat sslmode=disable'
readonly origin=https://chat.webshopstudio.ru
readonly nginx_target=/etc/nginx/conf.d/bfe-chat.conf
readonly acme_root=/var/lib/letsencrypt
readonly tls_dir=/etc/letsencrypt/live/chat.webshopstudio.ru
: "${BFE_CHAT_NGINX_SOURCE:?required Nginx template path}"

# Accept only our exact temporary ACME vhost on interrupted first runs.
is_chat_acme_vhost() {
  [[ -f "$nginx_target" && ! -L "$nginx_target" ]] || return 1
  cmp -s "$nginx_target" <(
  cat <<'ACME'
server {
    listen 80;
    listen [::]:80;
    server_name chat.webshopstudio.ru;
    location ^~ /.well-known/acme-challenge/ {
        root /var/lib/letsencrypt;
        default_type text/plain;
        try_files $uri =404;
    }
    location / { return 503; }
}
ACME
  )
}

# Validate existing state before any write. Do not overwrite unknown settings.
[[ -f "$BFE_CHAT_NGINX_SOURCE" && ! -L "$BFE_CHAT_NGINX_SOURCE" ]] || { echo 'ERROR: Chat Nginx source invalid' >&2; exit 1; }
[[ -d /etc/nginx/conf.d && -f /etc/nginx/nginx.conf ]] || { echo 'ERROR: Nginx layout unexpected' >&2; exit 1; }
command -v nginx >/dev/null && command -v certbot >/dev/null || { echo 'ERROR: Nginx/Certbot unavailable' >&2; exit 1; }
nginx -t >/dev/null 2>&1 || { echo 'ERROR: existing Nginx config invalid' >&2; exit 1; }
if [[ -e "$nginx_target" || -L "$nginx_target" ]]; then
  [[ -f "$nginx_target" && ! -L "$nginx_target" ]] && { cmp -s "$BFE_CHAT_NGINX_SOURCE" "$nginx_target" || is_chat_acme_vhost; } || {
    echo 'ERROR: existing Chat Nginx config differs; no overwrite' >&2; exit 1;
  }
fi
# Reject overlapping Chat vhosts, excluding our own matching Chat file.
if nginx -T 2>&1 | grep -E '^[[:space:]]*server_name[[:space:]]+chat\.webshopstudio\.ru[[:space:]]*;' | grep -q .; then
  [[ -f "$nginx_target" ]] || { echo 'ERROR: Chat host already configured elsewhere' >&2; exit 1; }
fi
if [[ -e "$tls_dir" && ! -d "$tls_dir" ]]; then echo 'ERROR: invalid Chat TLS directory' >&2; exit 1; fi
getent group "$chat_group" >/dev/null || { echo 'ERROR: internal auth group absent' >&2; exit 1; }
[[ -S "$chat_uds" ]] || { echo 'ERROR: internal auth socket absent' >&2; exit 1; }
runuser -u postgres -- psql -X -v ON_ERROR_STOP=1 -d postgres -Atqc 'SELECT 1' | grep -qx 1 || {
  echo 'ERROR: PostgreSQL unavailable' >&2; exit 1;
}
if [[ -e "$env_file" ]]; then
  [[ -f "$env_file" ]] || { echo 'ERROR: runtime.env not a regular file' >&2; exit 1; }
  [[ "$(stat -c %U:%G:%a "$env_file")" == bfe-chat:bfe-chat:600 ]] || {
    echo 'ERROR: unexpected runtime.env owner/permissions' >&2; exit 1;
  }
  grep -Fxq "BFE_CHAT_DATABASE_URL=\"$dsn\"" "$env_file" || {
    echo 'ERROR: existing Chat DSN differs' >&2; exit 1;
  }
  grep -Fxq "BFE_CHAT_PUBLIC_ORIGIN=$origin" "$env_file" || {
    echo 'ERROR: existing Chat origin differs' >&2; exit 1;
  }
  grep -Fxq "BFE_CHAT_DRIVE_AUTH_UDS=$chat_uds" "$env_file" || {
    echo 'ERROR: existing Chat UDS differs' >&2; exit 1;
  }
fi
if id "$chat_user" >/dev/null 2>&1; then
  [[ "$(id -u "$chat_user")" -lt 1000 ]] || {
    echo 'ERROR: existing Chat username is a regular user' >&2; exit 1;
  }
fi
# Never replace an existing Chat role or database.
runuser -u postgres -- psql -X -v ON_ERROR_STOP=1 -d postgres -Atqc "
SELECT CASE WHEN EXISTS (
  SELECT 1 FROM pg_roles WHERE rolname='bfe-chat'
    AND (NOT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole
         OR rolreplication OR rolbypassrls)
) OR EXISTS (
  SELECT 1 FROM pg_database WHERE datname='bfe_chat'
    AND pg_get_userbyid(datdba)<>'bfe-chat'
) THEN 'FAIL' ELSE 'PASS' END" | grep -qx PASS || {
  echo 'ERROR: conflicting existing Chat DB/role' >&2; exit 1;
}

if ! id "$chat_user" >/dev/null 2>&1; then
  useradd --system --user-group --home-dir /var/lib/bfe-chat-service --shell /usr/sbin/nologin "$chat_user"
fi
usermod -aG "$chat_group" "$chat_user"
runuser -u "$chat_user" -- test -r "$chat_uds" || {
  echo 'ERROR: Chat user cannot access UDS' >&2; exit 1;
}

# Only create Chat-owned PostgreSQL objects. Never modify bfe_drive.
runuser -u postgres -- psql -X -v ON_ERROR_STOP=1 -d postgres <<'SQL'
SELECT 'CREATE ROLE "bfe-chat" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS'
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='bfe-chat') \gexec
SELECT 'CREATE DATABASE bfe_chat OWNER "bfe-chat"'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname='bfe_chat') \gexec
SQL
runuser -u "$chat_user" -- psql -X -v ON_ERROR_STOP=1 -h /run/postgresql -U "$chat_user" -d bfe_chat -Atqc 'SELECT 1' | grep -qx 1 || {
  echo 'ERROR: Chat PostgreSQL peer authentication failed' >&2; exit 1;
}

install -d -o "$chat_user" -g "$chat_user" -m 0700 /var/lib/bfe-chat-service
install -d -o root -g root -m 0755 "$env_dir"
if [[ ! -e "$env_file" ]]; then
  umask 077
  printf '%s\n' \
    "BFE_CHAT_DATABASE_URL=\"$dsn\"" \
    "BFE_CHAT_PUBLIC_ORIGIN=$origin" \
    "BFE_CHAT_DRIVE_AUTH_UDS=$chat_uds" > "$env_file"
  chown "$chat_user:$chat_user" "$env_file"
  chmod 0600 "$env_file"
fi

# Stage the Chat-only HTTP ACME vhost before requesting its certificate.
# Existing Drive Nginx files are not edited or replaced.
install -d -o root -g root -m 0755 "$acme_root"
if [[ ! -e "$nginx_target" ]]; then
  cat > "$nginx_target" <<'ACME'
server {
    listen 80;
    listen [::]:80;
    server_name chat.webshopstudio.ru;
    location ^~ /.well-known/acme-challenge/ {
        root /var/lib/letsencrypt;
        default_type text/plain;
        try_files $uri =404;
    }
    location / { return 503; }
}
ACME
  chmod 0644 "$nginx_target"
  nginx -t >/dev/null 2>&1 || {
    rm -f "$nginx_target"
    echo 'ERROR: Chat ACME vhost failed validation' >&2; exit 1;
  }
  systemctl reload nginx
fi
if [[ ! -s "$tls_dir/fullchain.pem" || ! -s "$tls_dir/privkey.pem" ]]; then
  # Certbot must have an existing account; no interactive account registration.
  certbot certonly --non-interactive --webroot -w "$acme_root" \
    --cert-name chat.webshopstudio.ru -d chat.webshopstudio.ru || {
      echo 'ERROR: Chat certificate issuance failed (HTTP ACME vhost retained)' >&2; exit 1;
    }
fi
# Replace the provisional Chat-only vhost. Fail rather than overwrite a changed file.
if ! cmp -s "$BFE_CHAT_NGINX_SOURCE" "$nginx_target"; then
  is_chat_acme_vhost || {
    echo 'ERROR: Chat Nginx config changed unexpectedly' >&2; exit 1;
  }
  install -o root -g root -m 0644 "$BFE_CHAT_NGINX_SOURCE" "$nginx_target"
fi
nginx -t >/dev/null 2>&1 || { echo 'ERROR: Chat HTTPS configuration invalid' >&2; exit 1; }
systemctl reload nginx
echo 'PASS: Chat identity, UDS, database, env, Nginx and TLS provisioned'
echo 'NOTE: Chat runtime and migrations were not started'
