#!/usr/bin/env bash
set -Eeuo pipefail

# PanelX post-fix smoke test
# Verifies DNS, Nginx routing, panel APIs, WordPress, file manager, and SSL.
#
# Run on VPS:
#   sudo bash tests/vps-postfix-smoke.sh --domain example.com --admin-user admin --admin-pass 'YourPass'
#
# Or with token auth:
#   sudo bash tests/vps-postfix-smoke.sh --domain example.com --token 'PANELX_ADMIN_TOKEN'

DOMAIN=""
ADMIN_USER=""
ADMIN_PASS=""
TOKEN=""
COOKIE_JAR="/tmp/panelx-postfix-cookie.$$.txt"
TMP_JSON="/tmp/panelx-postfix.$$.json"

usage() {
  cat <<'EOF'
Usage:
  vps-postfix-smoke.sh --domain <fqdn> [auth options]

Required:
  --domain <fqdn>              Domain to validate (for example: example.com)

Auth (choose one):
  --admin-user <username> --admin-pass <password>
  --token <panelx_admin_token>

Optional:
  --help                       Show help
EOF
}

log() { printf "\n==> %s\n" "$*"; }
ok() { printf "✔ %s\n" "$*"; }
die() { printf "ERROR: %s\n" "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing command: $1"
}

cleanup() {
  rm -f "$COOKIE_JAR" "$TMP_JSON" || true
}
trap cleanup EXIT

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --domain)
        DOMAIN="${2:-}"
        shift 2
        ;;
      --admin-user)
        ADMIN_USER="${2:-}"
        shift 2
        ;;
      --admin-pass)
        ADMIN_PASS="${2:-}"
        shift 2
        ;;
      --token)
        TOKEN="${2:-}"
        shift 2
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

assert_json() {
  local payload="$1"
  local expr="$2"
  local message="$3"
  if ! echo "$payload" | jq -e "$expr" >/dev/null 2>&1; then
    echo "$payload" | jq . >&2 || true
    die "$message"
  fi
}

curl_auth() {
  local method="$1"
  local url="$2"
  local data="${3:-}"

  if [[ -n "$data" ]]; then
    if [[ -n "$TOKEN" ]]; then
      curl -fsSk -X "$method" "$url" \
        -H "Content-Type: application/json" \
        -H "X-PanelX-Token: $TOKEN" \
        --data "$data"
    else
      curl -fsSk -X "$method" "$url" \
        -H "Content-Type: application/json" \
        -b "$COOKIE_JAR" \
        --data "$data"
    fi
  else
    if [[ -n "$TOKEN" ]]; then
      curl -fsSk -X "$method" "$url" \
        -H "X-PanelX-Token: $TOKEN"
    else
      curl -fsSk -X "$method" "$url" \
        -b "$COOKIE_JAR"
    fi
  fi
}

login_if_needed() {
  if [[ -n "$TOKEN" ]]; then
    ok "Using token auth"
    return
  fi

  [[ -n "$ADMIN_USER" && -n "$ADMIN_PASS" ]] || die "Provide --token OR --admin-user + --admin-pass"

  local login_payload login_resp
  login_payload="$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p}')"
  login_resp="$(curl -fsSk -c "$COOKIE_JAR" -X POST "https://${DOMAIN}/v1/panel/auth/login" \
    -H "Content-Type: application/json" \
    --data "$login_payload")"

  assert_json "$login_resp" '.authenticated == true' "Panel login failed"
  ok "Panel login successful"
}

main() {
  parse_args "$@"

  [[ -n "$DOMAIN" ]] || die "--domain is required"

  require_cmd curl
  require_cmd jq
  require_cmd getent
  require_cmd openssl
  require_cmd nginx

  local base="https://${DOMAIN}"

  log "DNS validation"
  local dns_ips local_ips
  dns_ips="$(getent hosts "$DOMAIN" | awk '{print $1}' | tr '\n' ' ' | xargs || true)"
  [[ -n "$dns_ips" ]] || die "Domain does not resolve: $DOMAIN"
  local_ips="$(hostname -I | xargs || true)"
  [[ -n "$local_ips" ]] || die "Could not detect local IP addresses"
  echo "Domain IPs: $dns_ips"
  echo "Local IPs:  $local_ips"

  local any_match=0
  for dip in $dns_ips; do
    for lip in $local_ips; do
      if [[ "$dip" == "$lip" ]]; then
        any_match=1
      fi
    done
  done
  [[ "$any_match" -eq 1 ]] || die "DNS for $DOMAIN does not match this VPS IP set"
  ok "DNS points to this VPS"

  log "Nginx/service sanity"
  nginx -t >/dev/null
  systemctl is-active nginx >/dev/null || die "nginx not active"
  systemctl is-active panelx-control-plane >/dev/null || die "panelx-control-plane not active"
  systemctl is-active panelx-node-agent >/dev/null || die "panelx-node-agent not active"
  ok "Nginx and PanelX services active"

  log "Public route checks"
  local health panel_http wp_http
  health="$(curl -fsSk "${base}/health")"
  panel_http="$(curl -sSk -o /dev/null -w '%{http_code}' "${base}/panel")"
  wp_http="$(curl -sSk -o /dev/null -w '%{http_code}' "${base}/wp-login.php")"
  assert_json "$health" '.status == "ok"' "Health endpoint failed"
  [[ "$panel_http" == "200" || "$panel_http" == "301" || "$panel_http" == "302" ]] || die "/panel unexpected status: $panel_http"
  [[ "$wp_http" == "200" || "$wp_http" == "301" || "$wp_http" == "302" ]] || die "/wp-login.php unexpected status: $wp_http"
  ok "Panel and WordPress routes reachable"

  log "Auth + protected API checks"
  login_if_needed

  local auth_status me_resp sys_resp
  auth_status="$(curl_auth GET "${base}/v1/panel/auth/status")"
  assert_json "$auth_status" '.configured == true or .configured == false' "Invalid auth status payload"

  me_resp="$(curl_auth GET "${base}/v1/panel/me")"
  assert_json "$me_resp" '.authenticated == true' "Panel /me failed"

  sys_resp="$(curl_auth GET "${base}/v1/system/status")"
  assert_json "$sys_resp" '.status != null and .cpu.cores >= 1' "System status failed"
  ok "Protected APIs validated"

  log "Domain API checks"
  local domains_resp domain_id domain_get domain_logs
  domains_resp="$(curl_auth GET "${base}/v1/domains")"
  domain_id="$(echo "$domains_resp" | jq -r --arg d "$DOMAIN" '.items[]? | select(.hostname == $d) | .id' | head -n1 || true)"
  [[ -n "$domain_id" ]] || die "Domain not found in PanelX domain list: $DOMAIN"

  domain_get="$(curl_auth GET "${base}/v1/domains/get?id=${domain_id}")"
  assert_json "$domain_get" --arg d "$DOMAIN" '.id != null and .hostname == $d' "Domain get failed"

  domain_logs="$(curl_auth GET "${base}/v1/domains/logs?domain=${DOMAIN}&type=access&lines=20")"
  assert_json "$domain_logs" '.domain != null and .type == "access" and .count >= 0' "Domain logs failed"
  ok "Domain APIs validated"

  log "File manager write/read/delete"
  local test_path test_content write_payload write_resp read_resp delete_payload delete_resp
  test_path="/public_html/panelx-postfix-smoke.txt"
  test_content="panelx-postfix-smoke-$(date +%s)"

  write_payload="$(jq -nc --arg d "$DOMAIN" --arg p "$test_path" --arg c "$test_content" '{domain:$d,path:$p,content:$c}')"
  write_resp="$(curl_auth POST "${base}/v1/files/write" "$write_payload")"
  assert_json "$write_resp" '.message != null' "File write failed"

  read_resp="$(curl_auth GET "${base}/v1/files/read?domain=${DOMAIN}&path=${test_path}")"
  assert_json "$read_resp" --arg c "$test_content" '.content == $c' "File read content mismatch"

  delete_payload="$(jq -nc --arg d "$DOMAIN" --arg p "$test_path" '{domain:$d,path:$p}')"
  delete_resp="$(curl_auth POST "${base}/v1/files/delete" "$delete_payload")"
  assert_json "$delete_resp" '.message != null' "File delete failed"
  ok "File manager validated"

  log "SSL checks"
  local cert_path
  cert_path="/etc/letsencrypt/live/${DOMAIN}/fullchain.pem"
  [[ -f "$cert_path" ]] || die "Missing certificate file: $cert_path"
  openssl x509 -in "$cert_path" -noout -dates >/dev/null || die "Certificate parse failed"
  curl -fsS "https://${DOMAIN}/health" >/dev/null || die "HTTPS handshake/health failed"
  ok "SSL certificate and HTTPS validated"

  log "Node-agent heartbeat check"
  if journalctl -u panelx-node-agent --since "5 minutes ago" --no-pager | grep -E 'registration heartbeat failed.*404|status 404' >/dev/null; then
    die "Recent node-agent 404 heartbeat failures detected"
  fi
  ok "No recent node-agent 404 heartbeat failures"

  printf "\n✅ VPS post-fix smoke test PASSED for %s\n" "$DOMAIN"
}

main "$@"
