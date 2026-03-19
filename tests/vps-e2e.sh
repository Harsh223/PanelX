#!/usr/bin/env bash
set -Eeuo pipefail

# PanelX VPS end-to-end validator
#
# This script runs a full black-box validation flow against a remote VPS:
# 1) Clones source on VPS and runs installer end-to-end
# 2) Verifies services, health, routing, panel auth
# 3) Verifies agents registration endpoint
# 4) Verifies domain APIs, redirects, health, logs
# 5) Verifies WordPress provisioning
# 6) Verifies file manager read/write/delete
# 7) Verifies SSL issue/renew + certificate presence
#
# Requirements (local machine):
# - bash, ssh, scp, curl, jq
# - optional: sshpass (if using password-based auth non-interactively)

usage() {
  cat <<'EOF'
Usage:
  tests/vps-e2e.sh --host <ip-or-hostname> --domain <fqdn> --ssl-email <email> [options]

Required:
  --host <host>                 VPS host or IP
  --domain <fqdn>               Domain pointing to VPS (used for panel + WP + SSL)
  --ssl-email <email>           Email used by certbot/Let's Encrypt

Optional:
  --user <name>                 SSH user (default: root)
  --password <value>            SSH password (requires sshpass locally)
  --port <port>                 SSH port (default: 22)
  --repo <url>                  Git repo URL/path for installer source (default: https://github.com/Harsh223/PanelX.git)
  --branch <name>               Branch to test (default: main)
  --admin-user <name>           Panel admin username for setup/login (default: panelxadmin)
  --admin-email <email>         Panel admin email (default: ops@panelx.local)
  --admin-pass <value>          Panel admin password (default: PanelX!E2E!Admin!2026)
  --hostkey <fingerprint>       Explicit SSH host key fingerprint
  --strict-host-key             Enforce strict host key checking
  --no-clean                    Keep temporary working directory on VPS
  --verbose                     Print additional command output
  -h, --help                    Show help

Examples:
  tests/vps-e2e.sh \
    --host 203.0.113.10 \
    --domain wp-test.example.com \
    --ssl-email admin@example.com \
    --password 'StrongPassword!ChangeMe' \
    --hostkey 'ssh-ed25519 255 SHA256:YOUR_FINGERPRINT'

  tests/vps-e2e.sh \
    --host 203.0.113.10 \
    --user root \
    --domain wp-test.example.com \
    --ssl-email admin@example.com \
    --repo https://github.com/Harsh223/PanelX.git \
    --branch main
EOF
}

HOST=""
USER_NAME="root"
PASSWORD=""
PORT="22"
DOMAIN=""
SSL_EMAIL=""
REPO_URL="https://github.com/Harsh223/PanelX.git"
REPO_BRANCH="main"
ADMIN_USER="panelxadmin"
ADMIN_EMAIL="ops@panelx.local"
ADMIN_PASS="PanelX!E2E!Admin!2026"
HOSTKEY=""
STRICT_HOST_KEY=0
KEEP_REMOTE_TMP=0
VERBOSE=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="${2:-}"; shift 2 ;;
    --user) USER_NAME="${2:-}"; shift 2 ;;
    --password) PASSWORD="${2:-}"; shift 2 ;;
    --port) PORT="${2:-}"; shift 2 ;;
    --domain) DOMAIN="${2:-}"; shift 2 ;;
    --ssl-email) SSL_EMAIL="${2:-}"; shift 2 ;;
    --repo) REPO_URL="${2:-}"; shift 2 ;;
    --branch) REPO_BRANCH="${2:-}"; shift 2 ;;
    --admin-user) ADMIN_USER="${2:-}"; shift 2 ;;
    --admin-email) ADMIN_EMAIL="${2:-}"; shift 2 ;;
    --admin-pass) ADMIN_PASS="${2:-}"; shift 2 ;;
    --hostkey) HOSTKEY="${2:-}"; shift 2 ;;
    --strict-host-key) STRICT_HOST_KEY=1; shift ;;
    --no-clean) KEEP_REMOTE_TMP=1; shift ;;
    --verbose) VERBOSE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

[[ -n "$HOST" ]] || { echo "--host is required" >&2; exit 1; }
[[ -n "$DOMAIN" ]] || { echo "--domain is required" >&2; exit 1; }
[[ -n "$SSL_EMAIL" ]] || { echo "--ssl-email is required" >&2; exit 1; }

require_cmd ssh
require_cmd scp
require_cmd bash
require_cmd curl
require_cmd jq

if [[ -n "$PASSWORD" ]]; then
  require_cmd sshpass
fi

SSH_OPTS=("-p" "$PORT")
if [[ "$STRICT_HOST_KEY" -eq 1 ]]; then
  SSH_OPTS+=("-o" "StrictHostKeyChecking=yes")
else
  SSH_OPTS+=("-o" "StrictHostKeyChecking=no" "-o" "UserKnownHostsFile=/dev/null")
fi
if [[ -n "$HOSTKEY" ]]; then
  SSH_OPTS+=("-o" "HostKeyAlgorithms=+ssh-ed25519" "-o" "PubkeyAcceptedAlgorithms=+ssh-ed25519")
fi

if [[ "$VERBOSE" -eq 1 ]]; then
  echo "Using host: $HOST"
  echo "Using user: $USER_NAME"
  echo "Using port: $PORT"
  echo "Using domain: $DOMAIN"
  echo "Using repo: $REPO_URL ($REPO_BRANCH)"
fi

ssh_exec() {
  local remote_cmd="$1"
  if [[ -n "$PASSWORD" ]]; then
    sshpass -p "$PASSWORD" ssh "${SSH_OPTS[@]}" "$USER_NAME@$HOST" "$remote_cmd"
  else
    ssh "${SSH_OPTS[@]}" "$USER_NAME@$HOST" "$remote_cmd"
  fi
}

scp_push() {
  local src="$1"
  local dst="$2"
  if [[ -n "$PASSWORD" ]]; then
    sshpass -p "$PASSWORD" scp "${SSH_OPTS[@]}" "$src" "$USER_NAME@$HOST:$dst"
  else
    scp "${SSH_OPTS[@]}" "$src" "$USER_NAME@$HOST:$dst"
  fi
}

REMOTE_SCRIPT_LOCAL="$(mktemp)"
trap 'rm -f "$REMOTE_SCRIPT_LOCAL"' EXIT

cat >"$REMOTE_SCRIPT_LOCAL" <<'REMOTE_EOF'
#!/usr/bin/env bash
set -Eeuo pipefail

log() { printf '\n==> %s\n' "$*"; }
ok() { printf '✔ %s\n' "$*"; }
fail() { printf '✖ %s\n' "$*" >&2; exit 1; }

expect_json() {
  local payload="$1"
  local jq_expr="$2"
  local msg="$3"
  if ! echo "$payload" | jq -e "$jq_expr" >/dev/null; then
    echo "Payload:" >&2
    echo "$payload" | jq . >&2 || echo "$payload" >&2
    fail "$msg"
  fi
}

RUN_ID="$(date +%s)"
WORKDIR="/tmp/panelx-e2e-${RUN_ID}"
COOKIE_JAR="/tmp/panelx-e2e-cookie-${RUN_ID}.txt"
INSTALL_PATH="/e2e-${RUN_ID}"
TEST_FILE="/public_html/panelx-e2e-${RUN_ID}.txt"

cleanup() {
  rm -f "$COOKIE_JAR" || true
  if [[ "${KEEP_REMOTE_TMP:-0}" -ne 1 ]]; then
    rm -rf "$WORKDIR" || true
  fi
}
trap cleanup EXIT

log "Preparing VPS workspace"
mkdir -p "$WORKDIR"

log "Preflight on VPS"
command -v git >/dev/null 2>&1 || fail "git is required on VPS"
command -v curl >/dev/null 2>&1 || fail "curl is required on VPS"
command -v jq >/dev/null 2>&1 || fail "jq is required on VPS"

log "Cloning source to VPS workspace"
git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$WORKDIR/src"

INSTALLER="$WORKDIR/src/deploy/install-panelx.sh"
[[ -x "$INSTALLER" || -f "$INSTALLER" ]] || fail "installer not found at $INSTALLER"

log "Running installer (full clean install + SSL)"
bash "$INSTALLER" \
  --repo "$WORKDIR/src" \
  --branch "$REPO_BRANCH" \
  --yes \
  --enable-ssl \
  --domain "$DOMAIN" \
  --email "$SSL_EMAIL" \
  ${VERBOSE_INSTALL_FLAG:-}

log "Loading control-plane token"
[[ -f /etc/panelx/control-plane.env ]] || fail "/etc/panelx/control-plane.env not found"
PANEL_TOKEN="$(grep -E '^PANELX_ADMIN_TOKEN=' /etc/panelx/control-plane.env | cut -d= -f2- || true)"
[[ -n "$PANEL_TOKEN" ]] || fail "PANELX_ADMIN_TOKEN missing in /etc/panelx/control-plane.env"

BASE_HTTP="http://127.0.0.1:8080"
BASE_HTTPS="https://${DOMAIN}"

log "Service checks"
systemctl is-active panelx-control-plane >/dev/null || fail "panelx-control-plane not active"
systemctl is-active panelx-node-agent >/dev/null || fail "panelx-node-agent not active"
systemctl is-active nginx >/dev/null || fail "nginx not active"
systemctl is-active mariadb >/dev/null || fail "mariadb not active"
systemctl is-active php8.3-fpm >/dev/null || fail "php8.3-fpm not active"

ok "All core services active"

log "Nginx config validation"
nginx -t >/dev/null
ok "nginx -t passed"

log "Health checks (HTTP + HTTPS)"
HEALTH_HTTP="$(curl -fsS "${BASE_HTTP}/health")"
HEALTH_HTTPS="$(curl -fsSk "${BASE_HTTPS}/health")"
expect_json "$HEALTH_HTTP" '.status == "ok"' "HTTP health check failed"
expect_json "$HEALTH_HTTPS" '.status == "ok"' "HTTPS health check failed"
ok "Health endpoints are healthy"

log "Panel route sanity"
curl -fsSkI "${BASE_HTTPS}/panel" | grep -E 'HTTP/[0-9.] (200|301|302|307|308)' >/dev/null || fail "/panel not reachable"
AUTH_STATUS="$(curl -fsSk "${BASE_HTTPS}/v1/panel/auth/status")"
expect_json "$AUTH_STATUS" '.configured == true or .configured == false' "panel auth status invalid payload"

CONFIGURED="$(echo "$AUTH_STATUS" | jq -r '.configured')"
if [[ "$CONFIGURED" == "false" ]]; then
  log "Panel auth setup"
  SETUP_PAYLOAD="$(jq -nc \
    --arg u "$ADMIN_USER" \
    --arg e "$ADMIN_EMAIL" \
    --arg p "$ADMIN_PASS" \
    '{username:$u,email:$e,password:$p}')"
  SETUP_RESP="$(curl -fsSk -X POST "${BASE_HTTPS}/v1/panel/auth/setup" \
    -H "Content-Type: application/json" \
    -H "X-PanelX-Token: ${PANEL_TOKEN}" \
    --data "$SETUP_PAYLOAD")"
  expect_json "$SETUP_RESP" '.configured == true' "panel setup failed"
  ok "Panel setup completed"
else
  ok "Panel already configured; skipping setup"
fi

log "Panel login + session"
LOGIN_PAYLOAD="$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p}')"
LOGIN_RESP="$(curl -fsSk -c "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/panel/auth/login" \
  -H "Content-Type: application/json" \
  --data "$LOGIN_PAYLOAD")"
expect_json "$LOGIN_RESP" '.authenticated == true' "panel login failed"

ME_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/panel/me")"
expect_json "$ME_RESP" '.authenticated == true' "panel me failed"
ok "Panel login/session works"

log "Protected API check via session"
SYS_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/system/status")"
expect_json "$SYS_RESP" '.status != null and .cpu.cores >= 1' "system status response invalid"
ok "Protected APIs accessible with session"

log "Agent registration endpoint check"
AGENT_PAYLOAD='{"agentId":"node-local","hostname":"panelx-e2e-host","ipAddress":"127.0.0.1"}'
AGENT_RESP="$(curl -fsS -X POST "${BASE_HTTP}/v1/agents/register" \
  -H "Authorization: Bearer ${PANEL_TOKEN}" \
  -H "Content-Type: application/json" \
  --data "$AGENT_PAYLOAD")"
expect_json "$AGENT_RESP" '.accepted == true and .nodeId == "node-local"' "agent registration failed"
ok "Agent registration endpoint works"

log "Domain lookup/create"
DOMAINS_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/domains")"
DOMAIN_ID="$(echo "$DOMAINS_RESP" | jq -r --arg h "$DOMAIN" '.items[]? | select(.hostname == $h) | .id' | head -n1 || true)"

if [[ -z "$DOMAIN_ID" ]]; then
  CREATE_PAYLOAD="$(jq -nc --arg h "$DOMAIN" \
    '{hostname:$h, phpVersion:"8.3", webRoot:"", sslMode:"none", status:"active", notes:"e2e-created"}')"
  CREATE_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/domains/create" \
    -H "Content-Type: application/json" \
    --data "$CREATE_PAYLOAD")"
  DOMAIN_ID="$(echo "$CREATE_RESP" | jq -r '.id')"
  [[ -n "$DOMAIN_ID" && "$DOMAIN_ID" != "null" ]] || fail "domain create did not return id"
  ok "Domain created ($DOMAIN_ID)"
else
  ok "Domain already exists ($DOMAIN_ID)"
fi

log "Domain get/update redirects/health/logs"
GET_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/domains/get?id=${DOMAIN_ID}")"
expect_json "$GET_RESP" --arg d "$DOMAIN" '.id != null and .hostname == $d' "domain get failed"

REDIRECT_PAYLOAD="$(jq -nc --arg id "$DOMAIN_ID" \
  '{id:$id, forceHttps:true, nonWwwToWww:false, wwwToNonWww:true}')"
REDIRECT_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/domains/redirects" \
  -H "Content-Type: application/json" \
  --data "$REDIRECT_PAYLOAD")"
expect_json "$REDIRECT_RESP" '.id != null' "domain redirects update failed"

HEALTH_PAYLOAD="$(jq -nc --arg id "$DOMAIN_ID" '{id:$id}')"
HEALTH_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/domains/health" \
  -H "Content-Type: application/json" \
  --data "$HEALTH_PAYLOAD")"
expect_json "$HEALTH_RESP" '.id != null and .health != null' "domain health failed"

LOGS_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/domains/logs?domain=${DOMAIN}&type=access&lines=20")"
expect_json "$LOGS_RESP" '.domain != null and .type == "access" and .count >= 0' "domain logs failed"
ok "Domain APIs validated"

log "WordPress one-click install"
WP_PAYLOAD="$(jq -nc \
  --arg domain "$DOMAIN" \
  --arg path "$INSTALL_PATH" \
  --arg title "PanelX E2E ${RUN_ID}" \
  --arg email "wp-admin@${DOMAIN}" \
  '{domain:$domain,installPath:$path,siteTitle:$title,adminUser:"",adminPassword:"",adminEmail:$email,dbName:"",dbUser:"",dbPassword:""}')"

WP_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/wordpress/install" \
  -H "Content-Type: application/json" \
  --data "$WP_PAYLOAD")"
expect_json "$WP_RESP" --arg d "$DOMAIN" --arg p "$INSTALL_PATH" \
  '.domain == $d and .installPath == $p and (.adminUrl | contains("/wp-admin")) and .credentialsAutoGenerated == true' \
  "wordpress install failed"

curl -fsSk "${BASE_HTTPS}${INSTALL_PATH}/wp-login.php" >/dev/null || fail "wp-login not reachable"
ok "WordPress provisioning validated"

log "Installations listing"
INSTALLS_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/installations")"
expect_json "$INSTALLS_RESP" --arg d "$DOMAIN" --arg p "$INSTALL_PATH" \
  '.items | map(select(.domain == $d and .installPath == $p)) | length >= 1' \
  "installations list missing expected record"
ok "Installations API validated"

log "File manager list/write/read/delete"
FILES_LIST_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/files/list?domain=${DOMAIN}&path=/")"
expect_json "$FILES_LIST_RESP" '.entries != null' "files list failed"

WRITE_PAYLOAD="$(jq -nc --arg d "$DOMAIN" --arg p "$TEST_FILE" --arg c "panelx-e2e-${RUN_ID}" \
  '{domain:$d,path:$p,content:$c}')"
WRITE_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/files/write" \
  -H "Content-Type: application/json" --data "$WRITE_PAYLOAD")"
expect_json "$WRITE_RESP" '.message != null' "files write failed"

READ_RESP="$(curl -fsSk -b "$COOKIE_JAR" "${BASE_HTTPS}/v1/files/read?domain=${DOMAIN}&path=${TEST_FILE}")"
expect_json "$READ_RESP" --arg c "panelx-e2e-${RUN_ID}" '.content == $c' "files read content mismatch"

DELETE_PAYLOAD="$(jq -nc --arg d "$DOMAIN" --arg p "$TEST_FILE" '{domain:$d,path:$p}')"
DELETE_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/files/delete" \
  -H "Content-Type: application/json" --data "$DELETE_PAYLOAD")"
expect_json "$DELETE_RESP" '.message != null' "files delete failed"
ok "File manager APIs validated"

log "SSL issue + renew APIs"
SSL_ISSUE_PAYLOAD="$(jq -nc --arg id "$DOMAIN_ID" --arg e "$SSL_EMAIL" \
  '{id:$id,email:$e,provider:"letsencrypt"}')"
SSL_ISSUE_RESP="$(curl -fsSk --max-time 300 -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/domains/ssl/issue" \
  -H "Content-Type: application/json" \
  --data "$SSL_ISSUE_PAYLOAD")"
expect_json "$SSL_ISSUE_RESP" '.id != null' "ssl issue failed"

SSL_RENEW_PAYLOAD="$(jq -nc --arg id "$DOMAIN_ID" '{id:$id}')"
SSL_RENEW_RESP="$(curl -fsSk --max-time 300 -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/domains/ssl/renew" \
  -H "Content-Type: application/json" \
  --data "$SSL_RENEW_PAYLOAD")"
expect_json "$SSL_RENEW_RESP" '.id != null' "ssl renew failed"

[[ -f "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" ]] || fail "expected cert file not found"
openssl x509 -in "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" -noout -dates >/dev/null || fail "unable to parse certificate"
ok "SSL APIs and certificate validated"

log "Panel logout"
LOGOUT_RESP="$(curl -fsSk -b "$COOKIE_JAR" -X POST "${BASE_HTTPS}/v1/panel/auth/logout")"
expect_json "$LOGOUT_RESP" '.authenticated == false' "logout failed"
ok "Panel logout validated"

log "Node agent log sanity (no 404 on /v1/agents/register recently)"
if journalctl -u panelx-node-agent -n 200 --no-pager | grep -E '/v1/agents/register|registration heartbeat failed' >/tmp/panelx-agent-log-check.$$; then
  if grep -E '404|status 404' /tmp/panelx-agent-log-check.$$ >/dev/null; then
    cat /tmp/panelx-agent-log-check.$$ >&2
    rm -f /tmp/panelx-agent-log-check.$$
    fail "node-agent still shows registration 404s"
  fi
fi
rm -f /tmp/panelx-agent-log-check.$$ || true
ok "Node agent registration logs look healthy"

log "E2E validation completed successfully"
REMOTE_EOF

chmod +x "$REMOTE_SCRIPT_LOCAL"

REMOTE_SCRIPT_PATH="/tmp/panelx-vps-e2e-run.sh"
scp_push "$REMOTE_SCRIPT_LOCAL" "$REMOTE_SCRIPT_PATH"

VERBOSE_INSTALL_FLAG=""
if [[ "$VERBOSE" -eq 1 ]]; then
  VERBOSE_INSTALL_FLAG="--verbose"
fi

REMOTE_ENV=(
  "DOMAIN=$(printf '%q' "$DOMAIN")"
  "SSL_EMAIL=$(printf '%q' "$SSL_EMAIL")"
  "REPO_URL=$(printf '%q' "$REPO_URL")"
  "REPO_BRANCH=$(printf '%q' "$REPO_BRANCH")"
  "ADMIN_USER=$(printf '%q' "$ADMIN_USER")"
  "ADMIN_EMAIL=$(printf '%q' "$ADMIN_EMAIL")"
  "ADMIN_PASS=$(printf '%q' "$ADMIN_PASS")"
  "KEEP_REMOTE_TMP=$(printf '%q' "$KEEP_REMOTE_TMP")"
  "VERBOSE_INSTALL_FLAG=$(printf '%q' "$VERBOSE_INSTALL_FLAG")"
)

echo "Starting remote VPS E2E run..."
ssh_exec "$(printf '%s ' "${REMOTE_ENV[@]}") bash $REMOTE_SCRIPT_PATH"

echo
echo "✅ PanelX VPS E2E validation completed successfully."
echo "Host: $HOST"
echo "Domain: $DOMAIN"
echo "Repo: $REPO_URL ($REPO_BRANCH)"
