#!/usr/bin/env bash
set -Eeuo pipefail

# Enforce a single Nginx vhost for one domain that serves:
# - PanelX control-plane routes (/health, /panel, /v1/, /assets/)
# - WordPress for all other routes
#
# This script:
# 1) Backs up current Nginx site config state
# 2) Renders one canonical vhost config
# 3) Disables/removes conflicting domain vhosts
# 4) Validates nginx config
# 5) Reloads nginx (or rolls back on failure)
#
# Usage:
#   sudo bash tests/fix-nginx-single-vhost.sh --domain site.example.com
#
# Optional:
#   --web-root /var/www/panelx/sites/site.example.com/public_html
#   --control-plane-url http://127.0.0.1:8080
#   --php-socket /run/php/php8.3-fpm.sock
#   --no-ssl-redirect
#   --dry-run

DOMAIN=""
WEB_ROOT=""
CONTROL_PLANE_URL="http://127.0.0.1:8080"
PHP_SOCKET=""
NO_SSL_REDIRECT=0
DRY_RUN=0

NGINX_AVAILABLE_DIR="/etc/nginx/sites-available"
NGINX_ENABLED_DIR="/etc/nginx/sites-enabled"

CANONICAL_PREFIX="panelx-domain"

usage() {
  cat <<'EOF'
Usage:
  fix-nginx-single-vhost.sh --domain <fqdn> [options]

Required:
  --domain <fqdn>                  Domain to enforce as single-vhost (example: site.example.com)

Optional:
  --web-root <path>                WordPress web root (default: /var/www/panelx/sites/<domain>/public_html)
  --control-plane-url <url>        Upstream PanelX URL (default: http://127.0.0.1:8080)
  --php-socket <path>              PHP-FPM unix socket (auto-detected if omitted)
  --no-ssl-redirect                Keep HTTP serving app directly even when SSL cert exists
  --dry-run                        Print planned actions only; do not modify files
  -h, --help                       Show help
EOF
}

log() { printf "\n==> %s\n" "$*"; }
ok() { printf "✔ %s\n" "$*"; }
warn() { printf "! %s\n" "$*"; }
die() { printf "ERROR: %s\n" "$*" >&2; exit 1; }

require_root() {
  [[ "${EUID}" -eq 0 ]] || die "Run this script as root."
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --domain)
        DOMAIN="${2:-}"
        [[ -n "${DOMAIN}" ]] || die "Missing value for --domain"
        shift 2
        ;;
      --web-root)
        WEB_ROOT="${2:-}"
        [[ -n "${WEB_ROOT}" ]] || die "Missing value for --web-root"
        shift 2
        ;;
      --control-plane-url)
        CONTROL_PLANE_URL="${2:-}"
        [[ -n "${CONTROL_PLANE_URL}" ]] || die "Missing value for --control-plane-url"
        shift 2
        ;;
      --php-socket)
        PHP_SOCKET="${2:-}"
        [[ -n "${PHP_SOCKET}" ]] || die "Missing value for --php-socket"
        shift 2
        ;;
      --no-ssl-redirect)
        NO_SSL_REDIRECT=1
        shift
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
  done
}

detect_php_socket() {
  local candidates=(
    "/run/php/php8.3-fpm.sock"
    "/run/php/php8.2-fpm.sock"
    "/run/php/php8.1-fpm.sock"
    "/run/php/php-fpm.sock"
  )

  for s in "${candidates[@]}"; do
    if [[ -S "${s}" ]]; then
      echo "${s}"
      return 0
    fi
  done

  # Fallback; nginx -t will fail if wrong
  echo "/run/php/php8.3-fpm.sock"
}

render_http_server_block() {
  local domain="$1"
  local web_root="$2"
  local cp_url="$3"
  local php_socket="$4"

  cat <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${domain};

    root ${web_root};
    index index.php index.html index.htm;

    access_log /var/log/nginx/${domain}.access.log;
    error_log /var/log/nginx/${domain}.error.log;

    client_max_body_size 128M;

    # PanelX control-plane routes
    location = /health {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /panel {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /v1/ {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /assets/ {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # WordPress routes
    location / {
        try_files \$uri \$uri/ /index.php?\$args;
    }

    location ~ \\.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:${php_socket};
    }

    location ~ /\\.ht {
        deny all;
    }
}
EOF
}

render_https_server_blocks() {
  local domain="$1"
  local web_root="$2"
  local cp_url="$3"
  local php_socket="$4"
  local cert_file="$5"
  local key_file="$6"

  cat <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${domain};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${domain};

    ssl_certificate ${cert_file};
    ssl_certificate_key ${key_file};
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root ${web_root};
    index index.php index.html index.htm;

    access_log /var/log/nginx/${domain}.access.log;
    error_log /var/log/nginx/${domain}.error.log;

    client_max_body_size 128M;

    # PanelX control-plane routes
    location = /health {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /panel {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /v1/ {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /assets/ {
        proxy_pass ${cp_url};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # WordPress routes
    location / {
        try_files \$uri \$uri/ /index.php?\$args;
    }

    location ~ \\.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:${php_socket};
    }

    location ~ /\\.ht {
        deny all;
    }
}
EOF
}

main() {
  parse_args "$@"
  require_root

  require_cmd nginx
  require_cmd grep
  require_cmd sed
  require_cmd cp
  require_cmd mkdir
  require_cmd find
  require_cmd install
  require_cmd systemctl
  require_cmd mktemp
  require_cmd date

  [[ -n "${DOMAIN}" ]] || die "--domain is required"

  if [[ -z "${WEB_ROOT}" ]]; then
    WEB_ROOT="/var/www/panelx/sites/${DOMAIN}/public_html"
  fi

  if [[ -z "${PHP_SOCKET}" ]]; then
    PHP_SOCKET="$(detect_php_socket)"
  fi

  local cert_file="/etc/letsencrypt/live/${DOMAIN}/fullchain.pem"
  local key_file="/etc/letsencrypt/live/${DOMAIN}/privkey.pem"
  local has_ssl=0
  if [[ -f "${cert_file}" && -f "${key_file}" ]]; then
    has_ssl=1
  fi

  local ts backup_root backup_avail backup_enabled
  ts="$(date +%Y%m%d-%H%M%S)"
  backup_root="/var/backups/panelx/nginx-single-vhost-${DOMAIN}-${ts}"
  backup_avail="${backup_root}/sites-available"
  backup_enabled="${backup_root}/sites-enabled"

  local canonical_name="${CANONICAL_PREFIX}-${DOMAIN}.conf"
  local canonical_conf="${NGINX_AVAILABLE_DIR}/${canonical_name}"
  local canonical_link="${NGINX_ENABLED_DIR}/${canonical_name}"

  log "Domain: ${DOMAIN}"
  log "Web root: ${WEB_ROOT}"
  log "Control-plane URL: ${CONTROL_PLANE_URL}"
  log "PHP socket: ${PHP_SOCKET}"
  log "SSL cert present: ${has_ssl}"
  log "Backup path: ${backup_root}"

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    warn "Dry-run mode enabled; no files will be changed."
  fi

  if [[ "${DRY_RUN}" -eq 0 ]]; then
    mkdir -p "${backup_avail}" "${backup_enabled}"
    cp -a "${NGINX_AVAILABLE_DIR}/." "${backup_avail}/"
    cp -a "${NGINX_ENABLED_DIR}/." "${backup_enabled}/"
    ok "Backed up Nginx site state"
  fi

  local tmp_conf
  tmp_conf="$(mktemp)"
  if [[ "${has_ssl}" -eq 1 && "${NO_SSL_REDIRECT}" -eq 0 ]]; then
    render_https_server_blocks "${DOMAIN}" "${WEB_ROOT}" "${CONTROL_PLANE_URL}" "${PHP_SOCKET}" "${cert_file}" "${key_file}" > "${tmp_conf}"
  else
    render_http_server_block "${DOMAIN}" "${WEB_ROOT}" "${CONTROL_PLANE_URL}" "${PHP_SOCKET}" > "${tmp_conf}"
  fi

  log "Writing canonical vhost: ${canonical_conf}"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    install -m 0644 "${tmp_conf}" "${canonical_conf}"
  fi
  rm -f "${tmp_conf}"
  ok "Canonical vhost rendered"

  log "Disabling conflicting enabled vhosts"
  mapfile -t conflicts < <(
    grep -R -l --include '*.conf' -E "server_name[[:space:]]+.*\\b${DOMAIN}\\b" "${NGINX_ENABLED_DIR}" 2>/dev/null || true
  )

  if [[ "${#conflicts[@]}" -eq 0 ]]; then
    ok "No conflicting vhosts found in sites-enabled"
  else
    for file in "${conflicts[@]}"; do
      if [[ "${file}" == "${canonical_link}" ]]; then
        continue
      fi
      echo " - ${file}"
      if [[ "${DRY_RUN}" -eq 0 ]]; then
        rm -f "${file}"
      fi
    done
    ok "Conflicting enabled vhosts removed"
  fi

  log "Ensuring canonical symlink exists"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    rm -f "${canonical_link}"
    ln -s "${canonical_conf}" "${canonical_link}"
  fi
  ok "Canonical symlink in place"

  log "Validating nginx configuration"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    if ! nginx -t; then
      warn "nginx -t failed; restoring backup..."
      rm -rf "${NGINX_AVAILABLE_DIR}" "${NGINX_ENABLED_DIR}"
      mkdir -p "${NGINX_AVAILABLE_DIR}" "${NGINX_ENABLED_DIR}"
      cp -a "${backup_avail}/." "${NGINX_AVAILABLE_DIR}/"
      cp -a "${backup_enabled}/." "${NGINX_ENABLED_DIR}/"
      nginx -t || die "Rollback failed: nginx config still invalid"
      systemctl reload nginx || true
      die "Single-vhost enforcement failed; backup restored"
    fi

    systemctl reload nginx
  else
    echo "DRY-RUN: nginx -t"
    echo "DRY-RUN: systemctl reload nginx"
  fi

  ok "Nginx reload successful"
  ok "Single-vhost enforcement complete for ${DOMAIN}"
  echo "Backup stored at: ${backup_root}"
}

main "$@"
