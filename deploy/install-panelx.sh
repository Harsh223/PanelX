#!/usr/bin/env bash
set -Eeuo pipefail

PANELX_REPO_URL="https://github.com/Harsh223/PanelX.git"
PANELX_REPO_BRANCH="main"
PANELX_INSTALL_DIR="/opt/panelx"
PANELX_SRC_DIR="${PANELX_INSTALL_DIR}/src"
PANELX_BIN_DIR="${PANELX_INSTALL_DIR}/bin"
PANELX_WEB_DIR="${PANELX_INSTALL_DIR}/web"
PANELX_ETC_DIR="/etc/panelx"
PANELX_SITES_ROOT="/var/www/panelx/sites"
PANELX_GO_VERSION="1.23.7"

PANELX_ADMIN_TOKEN=""
PANELX_DB_ADMIN_USER="panelx_admin"
PANELX_DB_ADMIN_PASSWORD=""
PANELX_PHP_SOCKET=""

PANELX_ENABLE_SSL=0
PANELX_DOMAIN=""
PANELX_SSL_EMAIL=""
PANELX_ENABLE_UFW=1
PANELX_ENABLE_FAIL2BAN=1
PANELX_ENABLE_BACKUPS=1
PANELX_BACKUP_RETENTION_DAYS=7
PANELX_BACKUP_CRON_SCHEDULE="0 3 * * *"
PANELX_BACKUP_DIR="/var/backups/panelx"

PANELX_COLOR_RESET='\033[0m'
PANELX_COLOR_BLUE='\033[1;34m'
PANELX_COLOR_GREEN='\033[1;32m'
PANELX_COLOR_YELLOW='\033[1;33m'
PANELX_COLOR_RED='\033[1;31m'
PANELX_COLOR_CYAN='\033[1;36m'
PANELX_START_TS="$(date +%s)"
PANELX_VERBOSE=0
PANELX_ASSUME_YES=0
PANELX_LAST_LOG="/tmp/panelx-installer-last.log"

trap 'on_error $? $LINENO "$BASH_COMMAND"' ERR

print_banner() {
  printf '\n%b%s%b\n' "${PANELX_COLOR_CYAN}" "██████╗  █████╗ ███╗   ██╗███████╗██╗     ██╗  ██╗"
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     ╚██╗██╔╝"
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "██████╔╝███████║██╔██╗ ██║█████╗  ██║      ╚███╔╝ "
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║      ██╔██╗ "
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "██║     ██║  ██║██║ ╚████║███████╗███████╗██╔╝ ██╗"
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝╚═╝  ╚═╝" "${PANELX_COLOR_RESET}"
  printf '%b%s%b\n' "${PANELX_COLOR_BLUE}" "────────────────────────────────────────────────────" "${PANELX_COLOR_RESET}"
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "    Multi-WordPress Control Plane • One-Click Ops   " "${PANELX_COLOR_RESET}"
  printf '%b%s%b\n\n' "${PANELX_COLOR_BLUE}" "────────────────────────────────────────────────────" "${PANELX_COLOR_RESET}"
}

installer_intro_prompt() {
  printf '%b%s%b\n' "${PANELX_COLOR_GREEN}" "What this installer will do:" "${PANELX_COLOR_RESET}"
  printf '  • Install and configure Nginx, MariaDB, PHP-FPM, Node.js, Go, WP-CLI\n'
  printf '  • Build and deploy PanelX control-plane, node-agent, and web console\n'
  printf '  • Configure reverse proxy and optional SSL/firewall/fail2ban/backups\n'
  printf '  • Perform a clean reinstall (existing PanelX state will be wiped)\n\n'

  if [[ "${PANELX_ASSUME_YES}" -eq 1 ]]; then
    success "Auto-confirm enabled (--yes). Continuing installation."
    return
  fi

  printf '%b%s%b' "${PANELX_COLOR_YELLOW}" "Continue with PanelX installation? [y/N]: " "${PANELX_COLOR_RESET}"
  local answer
  read -r answer
  case "${answer}" in
    y|Y|yes|YES)
      success "Starting installation."
      ;;
    *)
      die "Installation cancelled by user."
      ;;
  esac
}

print_usage() {
  cat <<'EOF'
Usage:
  install-panelx.sh [options]

Core options:
  --repo <url>                  Git repository URL (default: Harsh223/PanelX)
  --branch <name>               Git branch to install (default: main)
  --install-dir <path>          Install directory (default: /opt/panelx)

Safety and output:
  --yes, -y                     Skip destructive wipe confirmation prompt
  --verbose, -v                 Show detailed command output
  --help, -h                    Show this help and exit

Production options:
  --enable-ssl                  Enable Let's Encrypt SSL provisioning
  --domain <fqdn>               Domain used for reverse proxy/SSL
  --email <address>             Email for Let's Encrypt registration
  --no-ufw                      Skip UFW firewall setup
  --no-fail2ban                 Skip fail2ban setup
  --no-backups                  Skip scheduled backups
  --backup-retention-days <n>   Backup retention in days (default: 7)
  --backup-cron "<expr>"        Backup cron schedule (default: "0 3 * * *")

Examples:
  install-panelx.sh --yes
  install-panelx.sh --enable-ssl --domain panel.example.com --email ops@example.com --yes
  install-panelx.sh --verbose --backup-retention-days 14 --yes
EOF
}

log() {
  printf '\n%b==>%b %s\n' "${PANELX_COLOR_BLUE}" "${PANELX_COLOR_RESET}" "$1"
}

success() {
  printf '%b✔%b %s\n' "${PANELX_COLOR_GREEN}" "${PANELX_COLOR_RESET}" "$1"
}

warn() {
  printf '%b!%b %s\n' "${PANELX_COLOR_YELLOW}" "${PANELX_COLOR_RESET}" "$1"
}

die() {
  printf '\n%bERROR:%b %s\n' "${PANELX_COLOR_RED}" "${PANELX_COLOR_RESET}" "$1" >&2
  if [[ -f "${PANELX_LAST_LOG}" ]]; then
    printf '%bINFO:%b Detailed output saved at %s\n' "${PANELX_COLOR_YELLOW}" "${PANELX_COLOR_RESET}" "${PANELX_LAST_LOG}" >&2
  fi
  exit 1
}

on_error() {
  local exit_code="$1"
  local line_no="$2"
  local cmd="$3"

  if [[ "${PANELX_VERBOSE}" -eq 1 ]]; then
    printf '\n%bERROR:%b installer failed at line %s\n' "${PANELX_COLOR_RED}" "${PANELX_COLOR_RESET}" "${line_no}" >&2
    printf '%bCOMMAND:%b %s\n' "${PANELX_COLOR_RED}" "${PANELX_COLOR_RESET}" "${cmd}" >&2
    printf '%bEXIT CODE:%b %s\n' "${PANELX_COLOR_RED}" "${PANELX_COLOR_RESET}" "${exit_code}" >&2
  else
    printf '\n%bERROR:%b Installation stopped unexpectedly.\n' "${PANELX_COLOR_RED}" "${PANELX_COLOR_RESET}" >&2
    printf '%bTIP:%b Re-run with --verbose for detailed troubleshooting output.\n' "${PANELX_COLOR_YELLOW}" "${PANELX_COLOR_RESET}" >&2
  fi

  print_runtime_summary >&2
  exit "${exit_code}"
}

run_quiet() {
  if [[ "${PANELX_VERBOSE}" -eq 1 ]]; then
    "$@"
  else
    "$@" >>"${PANELX_LAST_LOG}" 2>&1
  fi
}

retry() {
  local attempts="$1"
  shift
  local n=1
  until run_quiet "$@"; do
    if [[ "${n}" -ge "${attempts}" ]]; then
      return 1
    fi
    warn "Temporary issue detected, retrying (${n}/${attempts})..."
    sleep $((n * 2))
    n=$((n + 1))
  done
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

confirm_destructive_action() {
  if [[ "${PANELX_ASSUME_YES}" -eq 1 ]]; then
    return 0
  fi

  printf '\n%b%s%b\n' "${PANELX_COLOR_YELLOW}" "This install will DELETE existing PanelX sites, configs, and data." "${PANELX_COLOR_RESET}"
  printf '%s' "Type WIPE to continue: "
  local answer
  read -r answer
  [[ "${answer}" == "WIPE" ]] || die "Confirmation not provided. Aborting."
}

print_runtime_summary() {
  local now_ts
  now_ts="$(date +%s)"
  local duration
  duration="$((now_ts - PANELX_START_TS))"

  printf '\n%b%s%b\n' "${PANELX_COLOR_BLUE}" "---------------- Installer Summary ----------------" "${PANELX_COLOR_RESET}"
  printf '%b%-24s%b %ss\n' "${PANELX_COLOR_BLUE}" "Elapsed time:" "${PANELX_COLOR_RESET}" "${duration}"
  printf '%b%-24s%b %s\n' "${PANELX_COLOR_BLUE}" "Install dir:" "${PANELX_COLOR_RESET}" "${PANELX_INSTALL_DIR}"
  printf '%b%-24s%b %s\n' "${PANELX_COLOR_BLUE}" "Sites root:" "${PANELX_COLOR_RESET}" "${PANELX_SITES_ROOT}"
  printf '%b%s%b\n' "${PANELX_COLOR_BLUE}" "---------------------------------------------------" "${PANELX_COLOR_RESET}"
}

require_arg_value() {
  local flag="$1"
  local value="${2:-}"
  if [[ -z "${value}" || "${value}" == --* ]]; then
    die "Missing value for ${flag}. Use --help for usage."
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --repo)
        require_arg_value "$1" "${2:-}"
        PANELX_REPO_URL="$2"
        shift 2
        ;;
      --branch)
        require_arg_value "$1" "${2:-}"
        PANELX_REPO_BRANCH="$2"
        shift 2
        ;;
      --install-dir)
        require_arg_value "$1" "${2:-}"
        PANELX_INSTALL_DIR="$2"
        PANELX_SRC_DIR="${PANELX_INSTALL_DIR}/src"
        PANELX_BIN_DIR="${PANELX_INSTALL_DIR}/bin"
        PANELX_WEB_DIR="${PANELX_INSTALL_DIR}/web"
        shift 2
        ;;
      --yes|-y)
        PANELX_ASSUME_YES=1
        shift
        ;;
      --verbose|-v)
        PANELX_VERBOSE=1
        shift
        ;;
      --help|-h)
        print_usage
        exit 0
        ;;
      --enable-ssl)
        PANELX_ENABLE_SSL=1
        shift
        ;;
      --domain)
        require_arg_value "$1" "${2:-}"
        PANELX_DOMAIN="$2"
        shift 2
        ;;
      --email)
        require_arg_value "$1" "${2:-}"
        PANELX_SSL_EMAIL="$2"
        shift 2
        ;;
      --no-ufw)
        PANELX_ENABLE_UFW=0
        shift
        ;;
      --no-fail2ban)
        PANELX_ENABLE_FAIL2BAN=0
        shift
        ;;
      --no-backups)
        PANELX_ENABLE_BACKUPS=0
        shift
        ;;
      --backup-retention-days)
        require_arg_value "$1" "${2:-}"
        PANELX_BACKUP_RETENTION_DAYS="$2"
        shift 2
        ;;
      --backup-cron)
        require_arg_value "$1" "${2:-}"
        PANELX_BACKUP_CRON_SCHEDULE="$2"
        shift 2
        ;;
      *)
        die "Unknown argument: $1 (use --help for available options)"
        ;;
    esac
  done
}

validate_args() {
  if ! [[ "${PANELX_BACKUP_RETENTION_DAYS}" =~ ^[0-9]+$ ]] || [[ "${PANELX_BACKUP_RETENTION_DAYS}" -lt 1 ]]; then
    die "--backup-retention-days must be a positive integer"
  fi

  if [[ -z "${PANELX_BACKUP_CRON_SCHEDULE}" ]]; then
    die "--backup-cron cannot be empty"
  fi

  if [[ "${PANELX_ENABLE_SSL}" -eq 1 ]]; then
    if [[ -z "${PANELX_DOMAIN}" ]]; then
      die "--enable-ssl requires --domain"
    fi
    if [[ -z "${PANELX_SSL_EMAIL}" ]]; then
      die "--enable-ssl requires --email"
    fi
  fi
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    die "Run installer as root (or with sudo)."
  fi
}

preflight_checks() {
  log "Running preflight checks"

  require_cmd curl
  require_cmd git
  require_cmd tar
  require_cmd systemctl

  if ! retry 3 curl -fsSLI https://github.com >/dev/null; then
    die "Network preflight failed: unable to reach GitHub"
  fi

  local avail_kb
  avail_kb="$(df -Pk / | awk 'NR==2 {print $4}')"
  if [[ -z "${avail_kb}" || "${avail_kb}" -lt 2097152 ]]; then
    die "At least 2GB free disk space on / is required for install"
  fi

  success "Preflight checks passed"
}

detect_os() {
  if [[ ! -f /etc/os-release ]]; then
    die "Cannot detect OS: /etc/os-release is missing."
  fi

  local arch
  arch="$(dpkg --print-architecture)"
  if [[ "${arch}" != "amd64" ]]; then
    die "Unsupported architecture ${arch}. Current installer supports x86_64/amd64 only."
  fi

  # shellcheck disable=SC1091
  source /etc/os-release
  case "${ID}:${VERSION_ID}" in
    ubuntu:24.04|debian:12)
      log "Detected fully supported OS: ${PRETTY_NAME} (${arch})"
      ;;
    ubuntu:22.04|ubuntu:20.04)
      log "Detected compatible legacy Ubuntu: ${PRETTY_NAME} (${arch})"
      log "Legacy compatibility mode enabled. Some package/runtime combinations may require minor manual adjustments."
      ;;
    *)
      die "Unsupported OS ${PRETTY_NAME}. Supported targets: Ubuntu 24.04, Ubuntu 22.04, Ubuntu 20.04, Debian 12."
      ;;
  esac
}

apt_install_base() {
  log "Installing base dependencies"
  export DEBIAN_FRONTEND=noninteractive
  retry 3 apt-get update -y
  retry 3 apt-get install -y ca-certificates curl git gnupg lsb-release build-essential tar xz-utils jq unzip
  success "Base dependencies installed"
}

install_runtime_packages() {
  log "Installing runtime stack (Nginx, MariaDB, PHP-FPM, PHP extensions)"
  retry 3 apt-get install -y nginx mariadb-server mariadb-client php-fpm php-mysql php-cli php-curl php-xml php-mbstring php-zip php-gd certbot python3-certbot-nginx rsync
  systemctl enable --now nginx
  systemctl enable --now mariadb

  if [[ "${PANELX_ENABLE_FAIL2BAN}" -eq 1 ]]; then
    retry 3 apt-get install -y fail2ban
  fi

  if [[ "${PANELX_ENABLE_UFW}" -eq 1 ]]; then
    retry 3 apt-get install -y ufw
  fi

  ensure_php_fpm_socket
  success "Runtime stack installed and services started"
}

ensure_php_fpm_socket() {
  local service
  for service in php8.3-fpm php8.2-fpm php8.1-fpm php-fpm; do
    systemctl enable --now "${service}" >/dev/null 2>&1 || true
  done

  # Fallback when service manager is limited (containers/chroots)
  php-fpm8.3 -D >/dev/null 2>&1 || true
  php-fpm8.2 -D >/dev/null 2>&1 || true
  php-fpm8.1 -D >/dev/null 2>&1 || true
  php-fpm -D >/dev/null 2>&1 || true

  PANELX_PHP_SOCKET="$(ls /run/php/php*-fpm.sock /var/run/php/php*-fpm.sock 2>/dev/null | head -n 1 || true)"
  if [[ -z "${PANELX_PHP_SOCKET}" ]]; then
    die "PHP-FPM socket not found in /run/php or /var/run/php"
  fi
}

install_wp_cli() {
  log "Installing WP-CLI"
  retry 3 curl -fsSL https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar -o /usr/local/bin/wp
  chmod +x /usr/local/bin/wp
  wp --info >/dev/null
  success "WP-CLI installed"
}

install_nodejs_22() {
  log "Installing Node.js 22"
  retry 3 bash -c "curl -fsSL https://deb.nodesource.com/setup_22.x | bash -"
  retry 3 apt-get install -y nodejs
  node --version
  npm --version
  success "Node.js runtime ready"
}

install_go() {
  log "Installing Go ${PANELX_GO_VERSION}"
  local go_tar="go${PANELX_GO_VERSION}.linux-amd64.tar.gz"
  retry 3 curl -fsSL "https://go.dev/dl/${go_tar}" -o "/tmp/${go_tar}"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "/tmp/${go_tar}"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  go version
  success "Go toolchain installed"
}

prepare_directories() {
  log "Preparing directories"
  mkdir -p "${PANELX_INSTALL_DIR}" "${PANELX_BIN_DIR}" "${PANELX_WEB_DIR}" "${PANELX_ETC_DIR}" "${PANELX_SITES_ROOT}"
}

create_panelx_user() {
  if ! id -u panelx >/dev/null 2>&1; then
    log "Creating system user: panelx"
    useradd --system --create-home --home-dir /var/lib/panelx --shell /usr/sbin/nologin panelx
  fi
  chown -R panelx:panelx "${PANELX_INSTALL_DIR}"
}

wipe_previous_state() {
  log "Preparing clean install"
  confirm_destructive_action

  run_quiet systemctl stop panelx-control-plane.service || true
  run_quiet systemctl stop panelx-node-agent.service || true

  # Deterministic Nginx reset: backup current vhost state, then clear enabled/available custom vhosts.
  local nginx_backup_dir
  nginx_backup_dir="/var/backups/panelx/nginx-vhosts-$(date +%Y%m%d-%H%M%S)"
  mkdir -p "${nginx_backup_dir}/sites-available" "${nginx_backup_dir}/sites-enabled"

  cp -a /etc/nginx/sites-available/. "${nginx_backup_dir}/sites-available/" 2>/dev/null || true
  cp -a /etc/nginx/sites-enabled/. "${nginx_backup_dir}/sites-enabled/" 2>/dev/null || true

  find /etc/nginx/sites-enabled -mindepth 1 -maxdepth 1 -exec rm -f {} +
  find /etc/nginx/sites-available -maxdepth 1 -type f -name '*.conf' -exec rm -f {} +

  success "Nginx vhost state reset (backup: ${nginx_backup_dir})"

  rm -rf "${PANELX_INSTALL_DIR}"
  rm -rf "${PANELX_ETC_DIR}"
  rm -rf "${PANELX_SITES_ROOT}"

  success "Old PanelX data removed"
}

fetch_source() {
  log "Fetching PanelX source from ${PANELX_REPO_URL} (${PANELX_REPO_BRANCH})"
  rm -rf "${PANELX_SRC_DIR}"

  if ! retry 3 env GIT_TERMINAL_PROMPT=0 git clone --depth 1 --branch "${PANELX_REPO_BRANCH}" "${PANELX_REPO_URL}" "${PANELX_SRC_DIR}"; then
    die "Failed to clone ${PANELX_REPO_URL}. Ensure the repository is publicly accessible or provide explicit overrides: --repo https://github.com/Harsh223/PanelX.git --branch main"
  fi
  success "Source cloned into ${PANELX_SRC_DIR}"
}

build_control_plane() {
  log "Building control-plane"
  cd "${PANELX_SRC_DIR}/apps/control-plane"
  /usr/local/bin/go mod tidy
  /usr/local/bin/go build -o "${PANELX_BIN_DIR}/panelx-control-plane" ./cmd/server
  success "Control-plane binary built"
}

build_node_agent() {
  log "Building node-agent"
  cd "${PANELX_SRC_DIR}/apps/node-agent"
  /usr/local/bin/go mod tidy
  /usr/local/bin/go build -o "${PANELX_BIN_DIR}/panelx-node-agent" ./cmd/agent
  success "Node-agent binary built"
}

build_web_console() {
  log "Building web-console"
  cd "${PANELX_SRC_DIR}/apps/web-console"
  npm install
  npm run build
  rm -rf "${PANELX_WEB_DIR}"/*
  cp -r dist/* "${PANELX_WEB_DIR}/"
  success "Web console assets built"
}

generate_secrets() {
  PANELX_ADMIN_TOKEN="$(head -c 36 /dev/urandom | base64 | tr -d '\n' | tr '/+' 'ab' | cut -c1-48)"
  PANELX_DB_ADMIN_PASSWORD="$(head -c 24 /dev/urandom | base64 | tr -d '\n' | tr '/+' 'cd' | cut -c1-32)"
  success "Generated fresh admin token and DB admin password"
}

configure_mariadb() {
  log "Configuring MariaDB admin user for PanelX"
  mysql -u root <<SQL
CREATE USER IF NOT EXISTS '${PANELX_DB_ADMIN_USER}'@'127.0.0.1' IDENTIFIED BY '${PANELX_DB_ADMIN_PASSWORD}';
CREATE USER IF NOT EXISTS '${PANELX_DB_ADMIN_USER}'@'localhost' IDENTIFIED BY '${PANELX_DB_ADMIN_PASSWORD}';
ALTER USER '${PANELX_DB_ADMIN_USER}'@'127.0.0.1' IDENTIFIED BY '${PANELX_DB_ADMIN_PASSWORD}';
ALTER USER '${PANELX_DB_ADMIN_USER}'@'localhost' IDENTIFIED BY '${PANELX_DB_ADMIN_PASSWORD}';
GRANT ALL PRIVILEGES ON *.* TO '${PANELX_DB_ADMIN_USER}'@'127.0.0.1' WITH GRANT OPTION;
GRANT ALL PRIVILEGES ON *.* TO '${PANELX_DB_ADMIN_USER}'@'localhost' WITH GRANT OPTION;
FLUSH PRIVILEGES;
SQL
  success "MariaDB admin grants configured"
}

verify_mariadb_admin_login() {
  log "Verifying MariaDB admin login for control-plane"
  local login_ok=0
  if mysql -h 127.0.0.1 -u "${PANELX_DB_ADMIN_USER}" -p"${PANELX_DB_ADMIN_PASSWORD}" -e "SELECT 1;" >/dev/null 2>&1; then
    login_ok=1
  elif mysql -u "${PANELX_DB_ADMIN_USER}" -p"${PANELX_DB_ADMIN_PASSWORD}" -e "SELECT 1;" >/dev/null 2>&1; then
    login_ok=1
  fi

  if [[ "${login_ok}" -ne 1 ]]; then
    die "MariaDB admin login verification failed for user ${PANELX_DB_ADMIN_USER}"
  fi

  success "MariaDB admin login verified"
}

create_env_files() {
  log "Writing environment files"

  cat > "${PANELX_ETC_DIR}/control-plane.env" <<EOF
PANELX_HTTP_HOST=0.0.0.0
PANELX_HTTP_PORT=8080
PANELX_HTTP_READ_TIMEOUT=15s
PANELX_HTTP_WRITE_TIMEOUT=15s
PANELX_HTTP_IDLE_TIMEOUT=60s
PANELX_ADMIN_TOKEN=${PANELX_ADMIN_TOKEN}
PANELX_WEB_ROOT=${PANELX_WEB_DIR}
PANELX_SITES_ROOT=${PANELX_SITES_ROOT}
PANELX_PHP_FPM_SOCKET=${PANELX_PHP_SOCKET}
PANELX_DB_ADMIN_HOST=127.0.0.1
PANELX_DB_ADMIN_PORT=3306
PANELX_DB_ADMIN_USER=${PANELX_DB_ADMIN_USER}
PANELX_DB_ADMIN_PASSWORD=${PANELX_DB_ADMIN_PASSWORD}
EOF

  cat > "${PANELX_ETC_DIR}/node-agent.env" <<EOF
PANELX_AGENT_ID=node-local
PANELX_CONTROL_PLANE_URL=http://127.0.0.1:8080
PANELX_REGISTRATION_TOKEN=${PANELX_ADMIN_TOKEN}
PANELX_HEARTBEAT_INTERVAL=30s
PANELX_AGENT_HTTP_ADDR=0.0.0.0:8090
PANELX_INSECURE_SKIP_TLS_VERIFY=false
EOF

  chmod 640 "${PANELX_ETC_DIR}/control-plane.env" "${PANELX_ETC_DIR}/node-agent.env"
  chown root:panelx "${PANELX_ETC_DIR}/control-plane.env" "${PANELX_ETC_DIR}/node-agent.env"
}

install_systemd_units() {
  log "Installing systemd units"

  cat > /etc/systemd/system/panelx-control-plane.service <<EOF
[Unit]
Description=PanelX Control Plane
After=network-online.target mariadb.service nginx.service
Wants=network-online.target

[Service]
Type=simple
User=root
Group=panelx
EnvironmentFile=${PANELX_ETC_DIR}/control-plane.env
WorkingDirectory=${PANELX_INSTALL_DIR}
ExecStart=${PANELX_BIN_DIR}/panelx-control-plane
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

  cat > /etc/systemd/system/panelx-node-agent.service <<EOF
[Unit]
Description=PanelX Node Agent
After=network-online.target panelx-control-plane.service
Wants=network-online.target

[Service]
Type=simple
User=panelx
Group=panelx
EnvironmentFile=${PANELX_ETC_DIR}/node-agent.env
WorkingDirectory=${PANELX_INSTALL_DIR}
ExecStart=${PANELX_BIN_DIR}/panelx-node-agent
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now panelx-control-plane.service
  systemctl enable --now panelx-node-agent.service
}

configure_reverse_proxy() {
  log "Configuring Nginx reverse proxy"
  local server_name="_"
  if [[ -n "${PANELX_DOMAIN}" ]]; then
    server_name="${PANELX_DOMAIN}"
  fi

  cat > /etc/nginx/sites-available/panelx-control-plane.conf <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${server_name};

    client_max_body_size 128M;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF

  ln -sf /etc/nginx/sites-available/panelx-control-plane.conf /etc/nginx/sites-enabled/panelx-control-plane.conf
  rm -f /etc/nginx/sites-enabled/default
  run_quiet nginx -t || die "Nginx config test failed after reverse proxy setup"
  run_quiet systemctl reload nginx || die "Failed to reload Nginx"
  success "Reverse proxy configured"
}

configure_ssl() {
  if [[ "${PANELX_ENABLE_SSL}" -ne 1 ]]; then
    return
  fi

  log "Configuring SSL (Let's Encrypt)"
  [[ -n "${PANELX_DOMAIN}" ]] || die "--enable-ssl requires --domain"
  [[ -n "${PANELX_SSL_EMAIL}" ]] || die "--enable-ssl requires --email"

  run_quiet certbot --nginx --non-interactive --agree-tos -m "${PANELX_SSL_EMAIL}" -d "${PANELX_DOMAIN}" --redirect || die "SSL setup failed"
  success "SSL certificate installed for ${PANELX_DOMAIN}"
}

configure_firewall() {
  if [[ "${PANELX_ENABLE_UFW}" -ne 1 ]]; then
    warn "Skipping firewall setup (--no-ufw)"
    return
  fi

  log "Configuring firewall (UFW)"

  local ssh_port
  ssh_port="$(awk '/^[[:space:]]*Port[[:space:]]+[0-9]+/{print $2}' /etc/ssh/sshd_config /etc/ssh/sshd_config.d/*.conf 2>/dev/null | tail -n 1 || true)"
  if [[ -z "${ssh_port}" || ! "${ssh_port}" =~ ^[0-9]+$ ]]; then
    ssh_port="22"
  fi

  run_quiet ufw --force reset
  run_quiet ufw default deny incoming
  run_quiet ufw default allow outgoing
  run_quiet ufw allow "${ssh_port}/tcp"
  run_quiet ufw allow 80/tcp
  run_quiet ufw allow 443/tcp
  run_quiet ufw --force enable
  success "Firewall enabled (ports: ${ssh_port}, 80, 443)"
}

configure_fail2ban() {
  if [[ "${PANELX_ENABLE_FAIL2BAN}" -ne 1 ]]; then
    warn "Skipping fail2ban setup (--no-fail2ban)"
    return
  fi

  log "Configuring fail2ban"
  cat > /etc/fail2ban/jail.d/panelx.local <<EOF
[sshd]
enabled = true
maxretry = 5
findtime = 10m
bantime = 1h
EOF

  run_quiet systemctl enable --now fail2ban || die "Failed to start fail2ban"
  success "Fail2ban enabled"
}

configure_backups() {
  if [[ "${PANELX_ENABLE_BACKUPS}" -ne 1 ]]; then
    warn "Skipping automated backups (--no-backups)"
    return
  fi

  log "Configuring automated backups"
  mkdir -p "${PANELX_BACKUP_DIR}"

  cat > /usr/local/bin/panelx-backup.sh <<EOF
#!/usr/bin/env bash
set -euo pipefail
TS="\$(date +%Y%m%d-%H%M%S)"
TARGET_DIR="${PANELX_BACKUP_DIR}"
mkdir -p "\${TARGET_DIR}"

MYSQL_PWD="${PANELX_DB_ADMIN_PASSWORD}" mysqldump -h 127.0.0.1 -u "${PANELX_DB_ADMIN_USER}" --all-databases > "\${TARGET_DIR}/mysql-\${TS}.sql"
tar -czf "\${TARGET_DIR}/panelx-files-\${TS}.tar.gz" "${PANELX_ETC_DIR}" "${PANELX_SITES_ROOT}" "${PANELX_INSTALL_DIR}"
find "\${TARGET_DIR}" -type f -mtime +${PANELX_BACKUP_RETENTION_DAYS} -delete
EOF

  chmod 750 /usr/local/bin/panelx-backup.sh

  local cron_file="/etc/cron.d/panelx-backups"
  cat > "${cron_file}" <<EOF
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
${PANELX_BACKUP_CRON_SCHEDULE} root /usr/local/bin/panelx-backup.sh >/var/log/panelx-backup.log 2>&1
EOF

  chmod 644 "${cron_file}"
  success "Automated backups configured (retention: ${PANELX_BACKUP_RETENTION_DAYS} days)"
}

post_install_checks() {
  log "Final checks"
  sleep 2

  run_quiet systemctl --no-pager --full status panelx-control-plane.service || die "Control service did not start properly"
  run_quiet systemctl --no-pager --full status panelx-node-agent.service || die "Agent service did not start properly"

  run_quiet curl -fsS http://127.0.0.1:8080/health || die "Control-plane health check failed"
  run_quiet curl -fsS http://127.0.0.1:8090/health || die "Node-agent health check failed"

  log "Verifying agent registration endpoint contract"
  local agent_payload agent_resp
  agent_payload='{"agentId":"node-local","hostname":"panelx-installer-check","ipAddress":"127.0.0.1"}'
  agent_resp="$(curl -fsS -X POST "http://127.0.0.1:8080/v1/agents/register" \
    -H "Authorization: Bearer ${PANELX_ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    --data "${agent_payload}")" || die "Agent registration endpoint check failed"

  echo "${agent_resp}" | jq -e '.accepted == true and .nodeId == "node-local"' >/dev/null \
    || die "Agent registration contract validation failed"

  local host_ip
  host_ip="$(hostname -I | awk '{print $1}')"

  success "PanelX is ready!"
  printf '\n%b%s%b\n' "${PANELX_COLOR_CYAN}" "════════════════════════════════════════════════════" "${PANELX_COLOR_RESET}"
  printf '%b%s%b\n' "${PANELX_COLOR_GREEN}" "Open this URL in your browser:" "${PANELX_COLOR_RESET}"
  if [[ "${PANELX_ENABLE_SSL}" -eq 1 && -n "${PANELX_DOMAIN}" ]]; then
    printf '  https://%s/panel\n' "${PANELX_DOMAIN}"
  elif [[ -n "${PANELX_DOMAIN}" ]]; then
    printf '  http://%s/panel\n' "${PANELX_DOMAIN}"
  else
    printf '  http://%s/panel\n' "${host_ip}"
  fi
  printf '\n%b%s%b\n' "${PANELX_COLOR_GREEN}" "Use this one-time setup token:" "${PANELX_COLOR_RESET}"
  printf '  %s\n' "${PANELX_ADMIN_TOKEN}"
  printf '\n%b%s%b\n' "${PANELX_COLOR_BLUE}" "Need details later? Check:" "${PANELX_COLOR_RESET}"
  printf '  %s/control-plane.env\n' "${PANELX_ETC_DIR}"
  if [[ "${PANELX_ENABLE_BACKUPS}" -eq 1 ]]; then
    printf '%b%-24s%b %s\n' "${PANELX_COLOR_BLUE}" "Backup directory:" "${PANELX_COLOR_RESET}" "${PANELX_BACKUP_DIR}"
  fi
  printf '%b%s%b\n' "${PANELX_COLOR_CYAN}" "════════════════════════════════════════════════════" "${PANELX_COLOR_RESET}"
}

main() {
  : > "${PANELX_LAST_LOG}"
  parse_args "$@"
  validate_args
  print_banner
  installer_intro_prompt
  log "Tip: Use --help to view all installer options."
  require_root
  detect_os
  preflight_checks
  apt_install_base
  install_runtime_packages
  install_wp_cli
  install_nodejs_22
  install_go
  wipe_previous_state
  generate_secrets
  prepare_directories
  create_panelx_user
  fetch_source
  build_control_plane
  build_node_agent
  build_web_console
  configure_mariadb
  verify_mariadb_admin_login
  create_env_files
  install_systemd_units
  configure_reverse_proxy
  configure_ssl
  configure_firewall
  configure_fail2ban
  configure_backups
  post_install_checks
  print_runtime_summary
}

main "$@"
