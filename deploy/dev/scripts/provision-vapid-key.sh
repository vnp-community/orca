#!/usr/bin/env bash
# ============================================================
# provision-vapid-key.sh — one VAPID signing key per tenant (Web Push)
# ============================================================
# OPTIONAL PRE-WARM. Provisioning is automatic: on a tenant's first use
# (browser asks for the VAPID public key, or the first push is delivered)
# notification-service calls credential-broker-service EnsureVapidSigningKey,
# which creates the same ecdsa-p256 Transit key, and stores the metadata row.
# Run this script only to create keys ahead of that first use. The Vault policy
# (orca-policy.hcl) is still required either way; without it Vault answers 403
# and the broker reports CREDBROKER_VAULT_FORBIDDEN.
#
# notification-service signs Web Push with the Vault Transit key
# "vapid-signing-<tenant_id>" (through credential-broker-service) and serves the
# matching PUBLIC key to browsers from notification.vapid_key_metadata. Nothing
# creates either before first use, and the key must be ecdsa-p256 — a Transit key
# auto-created by an encrypt call is aes256-gcm96 and cannot sign.
#
# What this does, per tenant, idempotently:
#   1. creates Transit key vapid-signing-<tenant_id> (type ecdsa-p256) if absent;
#      refuses (never deletes) if a key of that name exists with another type
#   2. stores its public half (base64url, uncompressed P-256 point) in
#      notification.vapid_key_metadata unless an active row already exists
# The private key never leaves Vault.
#
# Needs: the orca Vault token to be allowed `create` on transit/keys/vapid-signing-*
# (see orca-policy.hcl — a Vault admin must apply it to the shared Vault), plus
# curl, openssl and python3 on the host that runs this.
#
# Usage (run ON the deploy server, from the deploy dir):
#   scripts/provision-vapid-key.sh --all                 # every tenant in the tenant DB
#   scripts/provision-vapid-key.sh <tenant-uuid> [...]   # specific tenants
# From your machine:
#   deploy/dev/scripts/provision-vapid-key.sh --remote --all
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [ "${1:-}" = "--remote" ]; then
  shift
  if [ -f "${DEPLOY_DIR}/.env" ]; then
    export $(grep -v '^#' "${DEPLOY_DIR}/.env" | xargs)
  fi
  SERVER_HOST="${SERVER_HOST:?SERVER_HOST not set}"
  SERVER_USER="${SERVER_USER:-ubuntu}"
  SERVER_KEY="${SERVER_KEY:-${HOME}/.ssh/id_ed25519}"
  SERVER_PORT="${SERVER_PORT:-22}"
  SERVER_DEPLOY="${SERVER_DEPLOY:-~/orca-go-deploy}"
  SSH_OPTS="-i ${SERVER_KEY} -p ${SERVER_PORT} -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"
  # shellcheck disable=SC2029
  exec ssh ${SSH_OPTS} "${SERVER_USER}@${SERVER_HOST}" "cd ${SERVER_DEPLOY} && bash scripts/provision-vapid-key.sh $*"
fi

cd "${DEPLOY_DIR}"
[ -f .env ] || { echo "provision-vapid-key: .env not found in ${DEPLOY_DIR}" >&2; exit 1; }
set -a
# shellcheck disable=SC1091
. ./.env
set +a
VAULT_ADDR="${VAULT_ADDR:-http://172.20.2.21:8200}" # same address docker-compose.yml's x-go-common-env uses
: "${VAULT_TOKEN:?VAULT_TOKEN not set in .env}"
for bin in curl openssl python3 docker; do
  command -v "$bin" >/dev/null || { echo "provision-vapid-key: $bin is required" >&2; exit 1; }
done

UUID_RE='^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
# VAPID_PSQL overrides how psql is reached (tests point it at a throwaway container).
psql_in() { ${VAPID_PSQL:-docker compose exec -T postgres psql -U orca} -v ON_ERROR_STOP=1 "$@"; }

VAULT_BODY=""; VAULT_STATUS=""
vault() { # method path [json-body] -> sets VAULT_STATUS / VAULT_BODY
  local out; out="$(mktemp)"
  VAULT_STATUS="$(curl -sS -o "$out" -w '%{http_code}' -X "$1" -H "X-Vault-Token: ${VAULT_TOKEN}" \
    ${3:+-H 'Content-Type: application/json' -d "$3"} "${VAULT_ADDR}/v1/$2" 2>/dev/null || echo 000)"
  VAULT_BODY="$(cat "$out")"; rm -f "$out"
}

provision() {
  local tenant="$1" key="vapid-signing-$1" type pem pub
  [[ "$tenant" =~ $UUID_RE ]] || { echo "  ! skip '$tenant': not a UUID" >&2; return 1; }

  vault GET "transit/keys/${key}"
  if [ "$VAULT_STATUS" = "404" ]; then
    vault POST "transit/keys/${key}" '{"type":"ecdsa-p256"}'
    case "$VAULT_STATUS" in
      2??) echo "  + created Transit key ${key}" ;;
      403) echo "  ! ${key}: Vault denied create — apply the updated deploy/dev/orca-policy.hcl on the shared Vault (see README 'MCP rollout')" >&2; return 1 ;;
      *)   echo "  ! ${key}: create failed (HTTP ${VAULT_STATUS})" >&2; return 1 ;;
    esac
    vault GET "transit/keys/${key}"
  fi
  [ "$VAULT_STATUS" = "200" ] || { echo "  ! ${key}: read failed (HTTP ${VAULT_STATUS})" >&2; return 1; }

  type="$(printf '%s' "$VAULT_BODY" | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["type"])')"
  if [ "$type" != "ecdsa-p256" ]; then
    echo "  ! ${key} exists with type '${type}', not ecdsa-p256 — a Vault admin must remove it (we never delete keys); skipped" >&2
    return 1
  fi
  pem="$(printf '%s' "$VAULT_BODY" | python3 -c 'import sys,json; d=json.load(sys.stdin)["data"]; print(d["keys"][str(d["latest_version"])]["public_key"])')"
  # SubjectPublicKeyInfo DER ends with the 65-byte uncompressed point (0x04||X||Y)
  pub="$(printf '%s\n' "$pem" | openssl ec -pubin -inform PEM -outform DER 2>/dev/null | tail -c 65 | base64 -w0 | tr '+/' '-_' | tr -d '=')"
  [ "${#pub}" -eq 87 ] || { echo "  ! ${key}: unexpected public key length (${#pub}, want 87)" >&2; return 1; }

  psql_in -d notification <<EOSQL >/dev/null
SET app.tenant_id = '${tenant}';
INSERT INTO notification.vapid_key_metadata (tenant_id, public_key, vault_key_ref, status)
SELECT '${tenant}', '${pub}', '${key}', 'active'
WHERE NOT EXISTS (
  SELECT 1 FROM notification.vapid_key_metadata WHERE tenant_id = '${tenant}' AND status = 'active'
);
EOSQL
  echo "  ✓ ${tenant}: VAPID key ready"
}

TENANTS=()
if [ "${1:-}" = "--all" ]; then
  while IFS= read -r t; do [ -n "$t" ] && TENANTS+=("$t"); done < <(psql_in -d tenant -Atc "SELECT id FROM tenant.companies")
  [ "${#TENANTS[@]}" -gt 0 ] || { echo "provision-vapid-key: no tenants yet (bootstrap not run?) — nothing to do"; exit 0; }
elif [ "$#" -gt 0 ]; then
  TENANTS=("$@")
else
  echo "usage: provision-vapid-key.sh [--remote] (--all | <tenant-uuid>...)" >&2; exit 2
fi

fail=0
echo "==> VAPID keys for ${#TENANTS[@]} tenant(s)"
for t in "${TENANTS[@]}"; do provision "$t" || fail=1; done
exit $fail
