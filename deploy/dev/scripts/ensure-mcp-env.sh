#!/usr/bin/env bash
# ============================================================
# ensure-mcp-env.sh — make sure .env has the secrets MCP needs
# ============================================================
# Why this exists: sync-to-server.sh pushes .env only when the server has none
# (it never overwrites a live one), so variables added later never reach an
# existing deployment. This script fills in ONLY what is missing, on whichever
# .env it targets, and never prints a secret value or rewrites a value that is
# already set.
#
# Generated here (32 random bytes, hex): shared secrets that guard internal-only
# gRPC calls between services on the flat docker network (there is no mTLS/mesh
# in this deploy, so these tokens are the only caller check):
#   MCP_INTERNAL_CALLER_TOKEN        mcp-service  <-> api-gateway
#   OAUTH_INTERNAL_CALLER_TOKEN      auth-service <-> mcp-service
#   AUTH_MCP_PRINCIPAL_CALLER_TOKEN  auth-service <-> api-gateway
#   MCP_CURSOR_KEY                   api-gateway HMAC key for pagination cursors
#                                    (empty => ephemeral key, cursors break on restart)
# Derived from PUBLIC_BASE_URL (only when it is set):
#   OAUTH_RESOURCE_URL = <PUBLIC_BASE_URL>/mcp   (must equal the gateway's MCP resource URL)
#
# Usage:
#   ./deploy/dev/scripts/ensure-mcp-env.sh            # local deploy/dev/.env
#   ./deploy/dev/scripts/ensure-mcp-env.sh --remote   # the server's .env, over SSH (secrets never leave the server)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Runs on whichever host owns the .env. $1 = path to that .env.
read -r -d '' BODY <<'EOS' || true
set -eu
ENV_FILE="$1"
[ -f "$ENV_FILE" ] || { echo "ensure-mcp-env: $ENV_FILE not found" >&2; exit 1; }
command -v openssl >/dev/null || { echo "ensure-mcp-env: openssl is required" >&2; exit 1; }

value_of() { sed -n "s/^$1=//p" "$ENV_FILE" | tail -n 1; }

set_if_missing() { # key, value
  key="$1"; val="$2"
  cur="$(value_of "$key")"
  if [ -n "$cur" ]; then return 0; fi
  if grep -q "^$key=" "$ENV_FILE"; then
    # present but empty: fill it in place (no duplicate keys)
    tmp="$(mktemp)"; sed "s|^$key=.*|$key=$val|" "$ENV_FILE" > "$tmp" && cat "$tmp" > "$ENV_FILE"; rm -f "$tmp"
  else
    [ -z "$(tail -c1 "$ENV_FILE")" ] || echo >> "$ENV_FILE"
    echo "$key=$val" >> "$ENV_FILE"
  fi
  echo "  + $key"
}

for k in MCP_INTERNAL_CALLER_TOKEN OAUTH_INTERNAL_CALLER_TOKEN AUTH_MCP_PRINCIPAL_CALLER_TOKEN MCP_CURSOR_KEY; do
  if [ -z "$(value_of "$k")" ]; then set_if_missing "$k" "$(openssl rand -hex 32)"; fi
done

base="$(value_of PUBLIC_BASE_URL)"
if [ -n "$base" ] && [ -z "$(value_of OAUTH_RESOURCE_URL)" ]; then
  set_if_missing OAUTH_RESOURCE_URL "${base%/}/mcp"
fi
chmod 600 "$ENV_FILE" 2>/dev/null || true
echo "ensure-mcp-env: $ENV_FILE is up to date"
EOS

if [ "${1:-}" = "--remote" ]; then
  if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs)
  fi
  SERVER_HOST="${SERVER_HOST:?SERVER_HOST not set}"
  SERVER_USER="${SERVER_USER:-ubuntu}"
  SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
  SERVER_PORT="${SERVER_PORT:-22}"
  SERVER_DEPLOY="${SERVER_DEPLOY:-~/orca-go-deploy}"
  SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"
  echo "==> ensure-mcp-env on ${SERVER_HOST}:${SERVER_DEPLOY}/.env"
  # The script body travels on stdin; only key NAMES come back.
  # shellcheck disable=SC2029
  printf '%s\n' "${BODY}" | ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" "sh -s -- ${SERVER_DEPLOY}/.env"
else
  echo "==> ensure-mcp-env on ${DEPLOY_DIR}/.env"
  printf '%s\n' "${BODY}" | sh -s -- "${DEPLOY_DIR}/.env"
fi
