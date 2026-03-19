#!/usr/bin/env bash
set -Eeuo pipefail

# Consolidate a domain into a single Nginx vhost that serves:
# - PanelX control-plane endpoints (/panel, /v1/, /assets/, /health)
# - WordPress site for all other routes
#
# Default domain: example.com
#
# Usage:
#   sudo bash tests/fix-nginx-conflict-wptest.sh
#   sudo bash tests/fix-nginx-conflict-wptest.sh --domain example.com
#
# Optional:
#   --web-root /var/www/panelx/sites/<domain>/public_html
#   --dry-run

DOMAIN="example.com"
WEB_ROOT=""
DRY_RUN=0

SITES_AVAILABLE_DIR="/etc/nginx/sites-available"
SITES_ENABLED_DIR="/etc/nginx/sites-enabled"

usage() {
  cat <<'EOF'
Usage:
  fix-nginx-conflict-wptest.sh [options]

Options:
  --domain <fqdn>       Domain to consolidate (default: example.com)
  --web-root <path>     Explicit web root (default: /var/www/panelx/sites/<domain>/public_html)
  --dry-run             Print actions without changing files
  -h, --help            Show this help
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
  local sockets=(
    "/run/php/php8.3-fpm.sock"
    "/run/php/php8.2-fpm.sock"
    "/run/php/php8.1-fpm.sock"
    "/run/php/php-fpm.sock"
  )

  for s in "${sockets[@]}"; do
    if [[ -S "${s}" ]]; then
      echo "${s}"
      return 0
    fi
  done

  # Fallback used in some templates; nginx -t will catch if invalid.
  echo "/run/php/php8.3-fpm.sock"
}

main() {
  parse_args "$@"
  require_root
  require_cmd nginx
  require_cmd grep
  require_cmd awk
  require_cmd cp
  require_cmd mkdir
  require_cmd find
  require_cmd sed

  [[ -n "${DOMAIN}" ]] || die "Domain cannot be empty."

  if [[ -z "${WEB_ROOT}" ]]; then
    WEB_ROOT="/var/www/panelx/sites/${DOMAIN}/public_html"
  fi

  local cert_dir="/etc/letsencrypt/live/${DOMAIN}"
  local cert_file="${cert_dir}/fullchain.pem"
  local key_file="${cert_dir}/privkey.pem"
  local ssl_opts="/etc/letsencrypt/options-ssl-nginx.conf"
  local dhparam="/etc/letsencrypt/ssl-dhparams.pem"

  [[ -f "${cert_file}" ]] || die "Missing certificate: ${cert_file}"
  [[ -f "${key_file}" ]] || die "Missing certificate key: ${key_file}"
  [[ -f "${ssl_opts}" ]] || die "Missing SSL options file: ${ssl_opts}"
  [[ -f "${dhparam}" ]] || die "Missing dhparam file: ${dhparam}"
  [[ -d "${WEB_ROOT}" ]] || warn "Web root does not exist yet: ${WEB_ROOT}"

  local php_socket
  php_socket="$(detect_php_socket)"

  local ts backup_root backup_avail backup_enabled
  ts="$(date +%Y%m%d-%H%M%S)"
  backup_root="/var/backups/panelx/nginx-conflicts-${DOMAIN}-${ts}"
  backup_avail="${backup_root}/sites-available"
  backup_enabled="${backup_root}/sites-enabled"

  local target_conf="${SITES_AVAILABLE_DIR}/${DOMAIN}.conf"
  local target_link="${SITES_ENABLED_DIR}/${DOMAIN}.conf"

  log "Domain: ${DOMAIN}"
  log "Web root: ${WEB_ROOT}"
  log "PHP socket: ${php_socket}"
  log "Backup dir: ${backup_root}"

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    warn "Dry-run mode enabled. No files will be modified."
  fi

  if [[ "${DRY_RUN}" -eq 0 ]]; then
    mkdir -p "${backup_avail}" "${backup_enabled}"
    cp -a "${SITES_AVAILABLE_DIR}/." "${backup_avail}/"
    cp -a "${SITES_ENABLED_DIR}/." "${backup_enabled}/"
    ok "Backed up Nginx vhost state"
  fi

  local tmp_conf
  tmp_conf="$(mktemp)"
  cat > "${tmp_conf}" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};

    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${DOMAIN};

    ssl_certificate ${cert_file};
    ssl_certificate_key ${key_file};
    include ${ssl_opts};
    ssl_dhparam ${dhparam};

    root ${WEB_ROOT};
    index index.php index.html index.htm;

    access_log /var/log/nginx/${DOMAIN}.access.log;
    error_log /var/log/nginx/${DOMAIN}.error.log;

    client_max_body_size 128M;

    # PanelX control-plane routes
    location = /health {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /panel {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /v1/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /assets/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # WordPress application routes
    location / {
        try_files \$uri \$uri/ /index.php?\$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:${php_socket};
    }

    location ~ /\.ht {
        deny all;
    }
}
EOF

  log "Writing consolidated vhost: ${target_conf}"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    install -m 0644 "${tmp_conf}" "${target_conf}"
  fi
  rm -f "${tmp_conf}"
  ok "Consolidated vhost rendered"

  log "Disabling conflicting enabled vhosts for ${DOMAIN}"
  local conflicting=()
  while IFS= read -r conf; do
    # Keep the canonical target only.
    if [[ "${conf}" != "${target_link}" ]]; then
      conflicting+=("${conf}")
    fi
  done < <(grep -R -l --include '*.conf' -E "server_name[[:space:]]+.*\\b${DOMAIN}\\b" "${SITES_ENABLED_DIR}" 2>/dev/null || true)

  if [[ "${#conflicting[@]}" -eq 0 ]]; then
    ok "No conflicting enabled vhosts found"
  else
    printf '%s\n' "${conflicting[@]}" | sed 's/^/ - /'
    if [[ "${DRY_RUN}" -eq 0 ]]; then
      for c in "${conflicting[@]}"; do
        rm -f "${c}"
      done
    fi
    ok "Conflicting enabled vhosts removed"
  fi

  log "Ensuring canonical enabled symlink exists"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    rm -f "${target_link}"
    ln -s "${target_conf}" "${target_link}"
  fi
  ok "Canonical vhost enabled"

  log "Validating Nginx configuration"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    if ! nginx -t; then
      warn "nginx -t failed, restoring backup from ${backup_root}"
      rm -rf "${SITES_AVAILABLE_DIR}" "${SITES_ENABLED_DIR}"
      mkdir -p "${SITES_AVAILABLE_DIR}" "${SITES_ENABLED_DIR}"
      cp -a "${backup_avail}/." "${SITES_AVAILABLE_DIR}/"
      cp -a "${backup_enabled}/." "${SITES_ENABLED_DIR}/"
      nginx -t || die "Rollback failed: nginx config still invalid"
      systemctl reload nginx || die "Rollback reload failed"
      die "Consolidation failed and was rolled back"
    fi
    systemctl reload nginx
  else
    echo "DRY-RUN: nginx -t"
    echo "DRY-RUN: systemctl reload nginx"
  fi

  ok "Nginx reloaded successfully"
  ok "Conflict consolidation complete for ${DOMAIN}"
  echo "Backup saved at: ${backup_root}"
}

main "$@"
