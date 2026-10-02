#!/usr/bin/env bash
# ci/check-nginx-mcp-routing.sh — static guard for the MCP routes in the dev
# nginx (BE-MCP-SOL-002 section D.5). Static on purpose: it must pass while
# MCP_ENABLED=false and needs no running stack. Sibling of
# check-nginx-admin-api-routing.sh (which probes a live stack).
#
# Optional live probe: set MCP_CHECK_URL=http://localhost:8080 against a stack
# with MCP_ENABLED=true to also assert Origin 403 and the 401 challenge.
set -euo pipefail

CONF="${NGINX_CONF:-$(dirname "$0")/../../deploy/dev/docker/nginx/orca.conf}"
fail() { echo "FAIL: $*" >&2; exit 1; }
[[ -f "$CONF" ]] || fail "nginx config not found: $CONF"

# block <location-header-regex>: prints the body of the first matching location.
block() {
  PAT="$1" awk '
    !found && $0 ~ ENVIRON["PAT"] { found = 1; depth = 0 }
    found {
      print
      n = gsub(/\{/, "{"); m = gsub(/\}/, "}")
      depth += n - m
      if (depth <= 0 && (n > 0 || m > 0)) exit
    }' "$CONF"
}

mcp=$(block '^[[:space:]]*location \^~ /mcp[[:space:]]*\{')
[[ -n "$mcp" ]] || fail "no 'location ^~ /mcp' block"
grep -Eq 'proxy_pass[[:space:]]+\$api_gateway' <<<"$mcp" || fail "/mcp is not proxied to api-gateway"
grep -Eq 'proxy_buffering[[:space:]]+off' <<<"$mcp" || fail "/mcp needs proxy_buffering off (SSE)"
timeout=$(grep -Eo 'proxy_read_timeout[[:space:]]+[0-9]+s' <<<"$mcp" | grep -Eo '[0-9]+' | head -1 || true)
[[ -n "$timeout" && "$timeout" -ge 600 ]] || fail "/mcp proxy_read_timeout must be >= 600s (got '${timeout:-unset}')"
grep -q 'proxy_hide_header' <<<"$mcp" && fail "/mcp must not use proxy_hide_header (Mcp-Session-Id must reach the client)"
# MCP depends on these request headers reaching api-gateway untouched.
for h in Authorization Origin Mcp-Session-Id MCP-Protocol-Version Last-Event-ID; do
  grep -Eiq "proxy_set_header[[:space:]]+$h\b" <<<"$mcp" && fail "/mcp must not override request header $h"
done

wk=$(block '^[[:space:]]*location \^~ /\.well-known/oauth-[[:space:]]*\{')
[[ -n "$wk" ]] && grep -q 'proxy_pass' <<<"$wk" || fail "/.well-known/oauth-* is not proxied"

oauth=$(block '^[[:space:]]*location \^~ /oauth/[[:space:]]*\{')
[[ -n "$oauth" ]] && grep -q 'proxy_pass' <<<"$oauth" || fail "/oauth/* is not proxied"

for loc in '/oauth/consent' '/oauth/consent/'; do
  consent=$(block "^[[:space:]]*location = ${loc//\//\\/}[[:space:]]*\\{")
  [[ -n "$consent" ]] || fail "no exact location for $loc (it must fall to the SPA)"
  grep -q 'proxy_pass' <<<"$consent" && fail "$loc must NOT be proxied to api-gateway"
  grep -Eq 'add_header[[:space:]]+Cache-Control[[:space:]]+"no-store"' <<<"$consent" || fail "$loc missing Cache-Control: no-store"
  grep -Eq 'add_header[[:space:]]+X-Frame-Options[[:space:]]+"DENY"' <<<"$consent" || fail "$loc missing X-Frame-Options: DENY"
  grep -Eq 'add_header[[:space:]]+Referrer-Policy[[:space:]]+"no-referrer"' <<<"$consent" || fail "$loc missing Referrer-Policy: no-referrer"
  grep -Eq 'try_files[[:space:]]+/web-index\.html' <<<"$consent" || fail "$loc must serve the SPA (web-index.html)"
done

echo "OK: nginx MCP routing is SSE-safe, header-transparent and keeps /oauth/consent in the SPA"

if [[ -n "${MCP_CHECK_URL:-}" ]]; then
  base="${MCP_CHECK_URL%/}"
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Origin: https://evil.example' \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d '{}' "$base/mcp")
  [[ "$code" == 403 ]] || fail "hostile Origin on /mcp returned $code (want 403)"
  hdrs=$(curl -s -D - -o /dev/null -X POST -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -d '{}' "$base/mcp")
  grep -qi '^HTTP/[0-9.]* 401' <<<"$hdrs" && grep -qi '^www-authenticate:.*resource_metadata=' <<<"$hdrs" \
    || fail "unauthenticated /mcp must be 401 with WWW-Authenticate resource_metadata"
  hdrs=$(curl -s -D - -o /dev/null "$base/oauth/consent")
  grep -qi '^x-frame-options: *DENY' <<<"$hdrs" || fail "live /oauth/consent lacks X-Frame-Options: DENY"
  echo "OK: live probe against $base"
fi
