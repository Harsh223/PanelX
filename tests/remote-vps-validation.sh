#!/usr/bin/env bash
set -Eeuo pipefail

# Deterministic on-VPS validation for PanelX installation + feature smoke tests.
#
# Run this script ON the target VPS as root.
#
# Example:
#   bash tests/remote-vps-validation.sh \
#     --domain wp-test.example.com \
#     --ssl-email admin@example.com \
#     --repo /path/to/PanelX \
#     --branch main \
#     --admin-user admin \
#     --admin-email admin@example.com \
#     --admin-pass 'AdminPass!12345'
#
# Notes:
# - Script performs a clean reinstall through installer (--yes).
# - Script requires DNS for --domain to already resolve to this VPS.
# - Script fails fast on any failed assertion.

DOMAIN=""
SSL_EMAIL=""
REPO_URL="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_BRANCH="main"
ADMIN_USER="admin"
ADMIN_EMAIL="admin@panelx.local"
ADMIN_PASS="AdminPass!12345"
INSTALLER_PATH=""
SKIP_INSTALL=0
VERBOSE=0
KEEP_TMP=0

TMP_DIR=""
COOKIE_JAR=""
DOMAIN_ID=""
WP_INSTALL_PATH="/"

usage() {
  cat <<'EOF'
Usage:
  remote-vps-validation.sh --domain <fqdn> --ssl-email <email> [options]

Required:
  --domain <fqdn>            Domain pointing to this VPS
  --ssl-email <email>        Email for certbot/Let's Encrypt

Options:
  --repo <path-or-url>       Repo path/URL for installer source (default: <repo-root>)
  --branch <name>            Branch to install (default: main)
  --installer <path>         Explicit installer path (default: <repo>/deploy/install-panelx.sh)
  --admin-user <name>        Panel admin username (default: admin)
  --admin-email <email>      Panel admin email (default: admin@panelx.local)
  --admin-pass <value>       Panel admin password (default: AdminPass!12345)
  --skip-install             Skip installer run and validate current deployment only
  --verbose                  Run installer in verbose mode
  --keep-tmp                 Keep temporary files for debugging
  -h, --help                 Show this help
EOF
}

log() { printf '\n==> %s\n' "$*"; }
ok() { printf '✔ %s\n' "$*"; }
die() { printf '✖ %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

json_assert() {
  local payload="$1"
  local expr="$2"
  local msg="$3"
  if ! echo "$payload" | jq -e "$expr" >/dev/null 2>&1; then
    echo "Assertion failed: $msg" >&2
    echo "Expression: $expr" >&2
    echo "Payload:" >&2
    echo "$payload" | jq . >&2 || echo "$payload" >&2
    exit 1
  fi
}

http_json() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local use_cookie="${4:-0}"

  if [[ -n "$data" ]]; then
    if [[ "$use_cookie" -eq 1 ]]; then
      curl -fsSk -X "$method" "$url" \
        -H "Content-Type: application/json" \
        -b "$COOKIE_JAR" \
        --data "$data"
    else
      curl -fsSk -X "$method" "$url" \
        -H "Content-Type: application/json" \
        --data "$data"
    fi
  else
    if [[ "$use_cookie" -eq 1 ]]; then
      curl -fsSk -X "$method" "$url" -b "$COOKIE_JAR"
    else
      curl -fsSk -X "$method" "$url"
    fi
  fi
}

load_panel_token() {
  [[ -f /etc/panelx/control-plane.env ]] || die "/etc/panelx/control-plane.env not found"
  PANEL_TOKEN="$(grep -E '^PANELX_ADMIN_TOKEN=' /etc/panelx/control-plane.env | cut -d= -f2- || true)"
  [[ -n "${PANEL_TOKEN:-}" ]] || die "PANELX_ADMIN_TOKEN missing in /etc/panelx/control-plane.env"
}

cleanup() {
  if [[ "$KEEP_TMP" -ne 1 ]]; then
    [[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR" || true
    [[ -n "$COOKIE_JAR" ]] && rm -f "$COOKIE_JAR" || true
  fi
}
trap cleanup EXIT

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --domain) DOMAIN="${2:-}"; shift 2 ;;
      --ssl-email) SSL_EMAIL="${2:-}"; shift 2 ;;
      --repo) REPO_URL="${2:-}"; shift 2 ;;
      --branch) REPO_BRANCH="${2:-}"; shift 2 ;;
      --installer) INSTALLER_PATH="${2:-}"; shift 2 ;;
      --admin-user) ADMIN_USER="${2:-}"; shift 2 ;;
      --admin-email) ADMIN_EMAIL="${2:-}"; shift 2 ;;
      --admin-pass) ADMIN_PASS="${2:-}"; shift 2 ;;
      --skip-install) SKIP_INSTALL=1; shift ;;
      --verbose) VERBOSE=1; shift ;;
      --keep-tmp) KEEP_TMP=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "Unknown argument: $1" ;;
    esac
  done
}

preflight() {
  [[ $EUID -eq 0 ]] || die "Run as root"
  [[ -n "$DOMAIN" ]] || die "--domain is required"
  [[ -n "$SSL_EMAIL" ]] || die "--ssl-email is required"

  require_cmd bash
  require_cmd curl
  require_cmd jq
  require_cmd systemctl
  require_cmd openssl
  require_cmd nginx
}

prepare_tmp() {
  TMP_DIR="$(mktemp -d /tmp/panelx-vps-validate.XXXXXX)"
  COOKIE_JAR="$TMP_DIR/cookies.txt"
}

resolve_installer_path() {
  if [[ -n "$INSTALLER_PATH" ]]; then
    [[ -f "$INSTALLER_PATH" ]] || die "Installer not found at $INSTALLER_PATH"
    return
  fi

  INSTALLER_PATH="${REPO_URL%/}/deploy/install-panelx.sh"
  [[ -f "$INSTALLER_PATH" ]] || die "Installer not found at $INSTALLER_PATH (use --installer)"
}

run_installer() {
  [[ "$SKIP_INSTALL" -eq 1 ]] && { ok "Skipping installer run"; return; }

  log "Running PanelX installer (clean reinstall + SSL)"
  local extra=()
  [[ "$VERBOSE" -eq 1 ]] && extra+=(--verbose)

  bash "$INSTALLER_PATH" \
    --repo "$REPO_URL" \
    --branch "$REPO_BRANCH" \
    --yes \
    --enable-ssl \
    --domain "$DOMAIN" \
    --email "$SSL_EMAIL" \
    "${extra[@]}"

  ok "Installer completed"
}

assert_services() {
  log "Checking service status"
  systemctl is-active panelx-control-plane >/dev/null || die "panelx-control-plane is not active"
  systemctl is-active panelx-node-agent >/dev/null || die "panelx-node-agent is not active"
  systemctl is-active nginx >/dev/null || die "nginx is not active"
  systemctl is-active mariadb >/dev/null || die "mariadb is not active"
  systemctl is-active php8.3-fpm >/dev/null || die "php8.3-fpm is not active"
  ok "Core services active"

  nginx -t >/dev/null || die "nginx -t failed"
  ok "nginx config valid"
}

assert_health_and_routes() {
  local base_http="http://127.0.0.1:8080"
  local base_https="https://${DOMAIN}"

  log "Checking health endpoints"
  local h1 h2
  h1="$(curl -fsS "${base_http}/health")"
  h2="$(curl -fsSk "${base_https}/health")"
  json_assert "$h1" '.status == "ok"' "HTTP /health should be ok"
  json_assert "$h2" '.status == "ok"' "HTTPS /health should be ok"
  ok "Health endpoints healthy"

  log "Checking panel route"
  curl -fsSkI "${base_https}/panel" | grep -E 'HTTP/[0-9.]+ (200|301|302|307|308)' >/dev/null || die "/panel not reachable"
  local auth_status
  auth_status="$(curl -fsSk "${base_https}/v1/panel/auth/status")"
  json_assert "$auth_status" '.configured == true or .configured == false' "invalid panel auth status payload"
  ok "Panel route and auth status reachable"
}

panel_setup_login() {
  local base_https="https://${DOMAIN}"
  local auth_status configured
  auth_status="$(curl -fsSk "${base_https}/v1/panel/auth/status")"
  configured="$(echo "$auth_status" | jq -r '.configured')"

  if [[ "$configured" == "false" ]]; then
    log "Running panel setup"
    local setup_payload setup_resp
    setup_payload="$(jq -nc \
      --arg u "$ADMIN_USER" \
      --arg e "$ADMIN_EMAIL" \
      --arg p "$ADMIN_PASS" \
      '{username:$u,email:$e,password:$p}')"

    setup_resp="$(curl -fsSk -X POST "${base_https}/v1/panel/auth/setup" \
      -H "Content-Type: application/json" \
      -H "X-PanelX-Token: ${PANEL_TOKEN}" \
      --data "$setup_payload")"
    json_assert "$setup_resp" '.configured == true' "panel setup failed"
    ok "Panel setup successful"
  else
    ok "Panel already configured"
  fi

  log "Logging into panel"
  local login_payload login_resp me_resp
  login_payload="$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p}')"
  login_resp="$(curl -fsSk -c "$COOKIE_JAR" -X POST "${base_https}/v1/panel/auth/login" \
    -H "Content-Type: application/json" \
    --data "$login_payload")"
  json_assert "$login_resp" '.authenticated == true' "panel login failed"

  me_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/panel/me")"
  json_assert "$me_resp" '.authenticated == true' "panel /me failed"
  ok "Panel session works"

  local status_resp
  status_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/system/status")"
  json_assert "$status_resp" '.status != null and .cpu.cores >= 1' "system status invalid"
  ok "Protected API works"
}

assert_agent_registration() {
  log "Testing /v1/agents/register endpoint"
  local payload resp
  payload='{"agentId":"node-local","hostname":"panelx-vps-test","ipAddress":"127.0.0.1"}'
  resp="$(curl -fsS -X POST "http://127.0.0.1:8080/v1/agents/register" \
    -H "Authorization: Bearer ${PANEL_TOKEN}" \
    -H "Content-Type: application/json" \
    --data "$payload")"
  json_assert "$resp" '.accepted == true and .nodeId == "node-local"' "agent registration failed"
  ok "Agent registration endpoint works"

  log "Waiting for heartbeat log cycle"
  local since_marker
  since_marker="$(date -u '+%Y-%m-%d %H:%M:%S')"
  sleep 35
  if journalctl -u panelx-node-agent --since "$since_marker" --no-pager | grep -E 'registration heartbeat failed.*404|status 404' >/dev/null; then
    die "Node-agent logs still show registration 404 failures in recent entries"
  fi
  ok "No recent agent registration 404s detected"
}

ensure_domain() {
  local base_https="https://${DOMAIN}"
  log "Ensuring managed domain exists"

  local list_resp
  list_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/domains")"
  DOMAIN_ID="$(echo "$list_resp" | jq -r --arg h "$DOMAIN" '.items[]? | select(.hostname == $h) | .id' | head -n1 || true)"

  if [[ -z "$DOMAIN_ID" ]]; then
    local create_payload create_resp
    create_payload="$(jq -nc --arg h "$DOMAIN" \
      '{hostname:$h,phpVersion:"8.3",webRoot:"",sslMode:"none",status:"active",notes:"remote validation"}')"
    create_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/domains/create" \
      -H "Content-Type: application/json" \
      --data "$create_payload")"
    DOMAIN_ID="$(echo "$create_resp" | jq -r '.id')"
    [[ -n "$DOMAIN_ID" && "$DOMAIN_ID" != "null" ]] || die "Domain create did not return id"
    ok "Domain created: $DOMAIN_ID"
  else
    ok "Domain already exists: $DOMAIN_ID"
  fi
}

assert_domain_apis() {
  local base_https="https://${DOMAIN}"
  log "Testing domain get/redirects/health/logs"

  local get_resp redirects_payload redirects_resp health_payload health_resp logs_resp
  get_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/domains/get?id=${DOMAIN_ID}")"
  json_assert "$get_resp" ".id != null and .hostname == \"${DOMAIN}\"" "domain get failed"

  redirects_payload="$(jq -nc --arg id "$DOMAIN_ID" \
    '{id:$id,forceHttps:true,nonWwwToWww:false,wwwToNonWww:true}')"
  redirects_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/domains/redirects" \
    -H "Content-Type: application/json" \
    --data "$redirects_payload")"
  json_assert "$redirects_resp" '.id != null' "domain redirects update failed"

  health_payload="$(jq -nc --arg id "$DOMAIN_ID" '{id:$id}')"
  health_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/domains/health" \
    -H "Content-Type: application/json" \
    --data "$health_payload")"
  json_assert "$health_resp" '.id != null and .health != null' "domain health failed"

  # Hit the domain once to ensure access log has fresh lines.
  curl -fsSk "https://${DOMAIN}/health" >/dev/null || true

  logs_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/domains/logs?domain=${DOMAIN}&type=access&lines=50")"
  json_assert "$logs_resp" '.domain != null and .type == "access" and .count >= 0' "domain logs failed"

  ok "Domain APIs validated"
}

assert_wordpress_install() {
  local base_https="https://${DOMAIN}"
  log "Testing WordPress one-click install"

  local payload resp
  payload="$(jq -nc \
    --arg d "$DOMAIN" \
    --arg p "$WP_INSTALL_PATH" \
    --arg t "PanelX VPS Validation" \
    --arg e "wp-admin@${DOMAIN}" \
    '{domain:$d,installPath:$p,siteTitle:$t,adminUser:"",adminPassword:"",adminEmail:$e,dbName:"",dbUser:"",dbPassword:""}')"

  resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/wordpress/install" \
    -H "Content-Type: application/json" \
    --data "$payload")"

  json_assert "$resp" ".domain == \"${DOMAIN}\" and .installPath == \"${WP_INSTALL_PATH}\" and (.adminUrl | contains(\"/wp-admin\")) and .credentialsAutoGenerated == true" \
    "wordpress install failed"

  curl -fsSk "https://${DOMAIN}${WP_INSTALL_PATH}/wp-login.php" >/dev/null || die "wp-login page not reachable"
  ok "WordPress install validated"
}

assert_installations_and_files() {
  local base_https="https://${DOMAIN}"
  log "Testing installations API"

  local installs_resp
  installs_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/installations")"
  json_assert "$installs_resp" ".items | map(select(.domain == \"${DOMAIN}\")) | length >= 1" \
    "installations list missing expected domain"

  ok "Installations API validated"

  log "Testing file manager APIs"
  local test_file="/public_html/panelx-remote-validation.txt"
  local test_content="panelx-remote-validation-$(date +%s)"
  local list_resp write_payload write_resp read_resp delete_payload delete_resp

  list_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/files/list?domain=${DOMAIN}&path=/")"
  json_assert "$list_resp" '.entries != null' "file list failed"

  write_payload="$(jq -nc --arg d "$DOMAIN" --arg p "$test_file" --arg c "$test_content" \
    '{domain:$d,path:$p,content:$c}')"
  write_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/files/write" \
    -H "Content-Type: application/json" \
    --data "$write_payload")"
  json_assert "$write_resp" '.message != null' "file write failed"

  read_resp="$(curl -fsSk -b "$COOKIE_JAR" "${base_https}/v1/files/read?domain=${DOMAIN}&path=${test_file}")"
  json_assert "$read_resp" ".content == \"${test_content}\"" "file read content mismatch"

  delete_payload="$(jq -nc --arg d "$DOMAIN" --arg p "$test_file" '{domain:$d,path:$p}')"
  delete_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/files/delete" \
    -H "Content-Type: application/json" \
    --data "$delete_payload")"
  json_assert "$delete_resp" '.message != null' "file delete failed"

  ok "File manager APIs validated"
}

assert_ssl_apis_and_cert() {
  local base_https="https://${DOMAIN}"
  log "Testing SSL issue/renew APIs"

  local issue_payload issue_resp renew_payload renew_resp
  issue_payload="$(jq -nc --arg id "$DOMAIN_ID" --arg e "$SSL_EMAIL" \
    '{id:$id,email:$e,provider:"letsencrypt"}')"

  issue_resp="$(curl -fsSk --max-time 300 -b "$COOKIE_JAR" -X POST "${base_https}/v1/domains/ssl/issue" \
    -H "Content-Type: application/json" \
    --data "$issue_payload")"
  json_assert "$issue_resp" '.id != null' "SSL issue API failed"

  renew_payload="$(jq -nc --arg id "$DOMAIN_ID" '{id:$id}')"
  renew_resp="$(curl -fsSk --max-time 300 -b "$COOKIE_JAR" -X POST "${base_https}/v1/domains/ssl/renew" \
    -H "Content-Type: application/json" \
    --data "$renew_payload")"
  json_assert "$renew_resp" '.id != null' "SSL renew API failed"

  local cert_file="/etc/letsencrypt/live/${DOMAIN}/fullchain.pem"
  [[ -f "$cert_file" ]] || die "Expected cert file missing: $cert_file"
  openssl x509 -in "$cert_file" -noout -dates >/dev/null || die "Failed to parse certificate"
  ok "SSL APIs and certificate validated"
}

logout_and_finish() {
  local base_https="https://${DOMAIN}"
  log "Testing logout"

  local logout_resp
  logout_resp="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${base_https}/v1/panel/auth/logout")"
  json_assert "$logout_resp" '.authenticated == false' "logout failed"

  ok "Logout validated"
  printf '\n✅ ALL CHECKS PASSED\n'
  echo "Domain: ${DOMAIN}"
  echo "Repo: ${REPO_URL} (${REPO_BRANCH})"
}

main() {
  parse_args "$@"
  preflight
  prepare_tmp
  resolve_installer_path
  run_installer
  load_panel_token
  assert_services
  assert_health_and_routes
  panel_setup_login
  assert_agent_registration
  ensure_domain
  assert_domain_apis
  assert_wordpress_install
  assert_installations_and_files
  assert_ssl_apis_and_cert
  logout_and_finish
}

main "$@"
