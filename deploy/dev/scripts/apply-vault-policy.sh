#!/usr/bin/env bash
# ============================================================
# apply-vault-policy.sh — apply deploy/dev/orca-policy.hcl to the shared Vault
# ============================================================
# `vault policy write` REPLACES the whole policy, so this script refuses to write
# when the live policy contains rules that the repo file lacks (someone changed
# Vault by hand) unless you pass --force. It never needs the Vault CLI (curl +
# python3 only), backs the live policy up first, and verifies afterwards using
# the orca service token from .env (read-only capability checks).
#
# Needs an ADMIN token able to read/write sys/policies/acl/<name>. The orca
# service token cannot. Put it in the environment WITHOUT echoing it:
#   read -rs VAULT_ADMIN_TOKEN && export VAULT_ADMIN_TOKEN
# (the root token was revoked after the shared-Vault migration; a temporary one
#  comes from `vault operator generate-root` — see README "MCP rollout").
#
# Usage:
#   scripts/apply-vault-policy.sh            # dry run: show state + diff, change nothing
#   scripts/apply-vault-policy.sh --apply    # write it (prompts unless --yes)
# Options: --yes (no prompt)  --force (write even if live has extra rules)
# Env: VAULT_ADDR (default http://172.20.2.21:8200)  VAULT_POLICY_NAME (default orca)
#      VAULT_POLICY_BACKUP_DIR (default $TMPDIR or /tmp)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
POLICY_FILE="${DEPLOY_DIR}/orca-policy.hcl"
VAULT_ADDR="${VAULT_ADDR:-http://172.20.2.21:8200}"
NAME="${VAULT_POLICY_NAME:-orca}" # verified live: lookup-self on the orca token reports policies [default, orca]
APPLY=0; YES=0; FORCE=0
for a in "$@"; do
  case "$a" in
    --apply) APPLY=1 ;; --yes) YES=1 ;; --force) FORCE=1 ;;
    *) echo "unknown option: $a" >&2; exit 2 ;;
  esac
done

if [ -z "${VAULT_ADMIN_TOKEN:-}" ]; then
  cat >&2 <<'MSG'
VAULT_ADMIN_TOKEN is not set in this shell.

This script needs a Vault ADMIN token (the orca service token in .env cannot write policies).
Set it in the SAME shell you run the script from, without echoing it, then re-run:

    read -rs VAULT_ADMIN_TOKEN     # paste the token, press Enter (nothing is displayed)
    export VAULT_ADMIN_TOKEN
    deploy/dev/scripts/apply-vault-policy.sh

Notes: `sudo` and `ssh host cmd` do not carry the variable over; VAULT_TOKEN is deliberately not
used (it holds the orca token). No admin token yet? A temporary root token comes from
`vault operator generate-root` (needs 3 of 5 unseal-key holders) - see README "MCP rollout".
MSG
  exit 2
fi
[ -f "$POLICY_FILE" ] || { echo "missing $POLICY_FILE" >&2; exit 1; }
for bin in curl python3; do command -v "$bin" >/dev/null || { echo "$bin is required" >&2; exit 1; }; done

# method path [json-file] -> sets RESP (body) and STATUS (HTTP code). Globals on purpose: calling it
# inside $(...) would run it in a subshell and lose STATUS.
RESP=""; STATUS=""
api() {
  local out; out="$(mktemp)"
  STATUS="$(curl -sS -m 15 -o "$out" -w '%{http_code}' -X "$1" -H "X-Vault-Token: ${TOKEN_FOR_CALL}" \
    ${3:+-H 'Content-Type: application/json' --data-binary "@$3"} "${VAULT_ADDR}/v1/$2" 2>/dev/null || echo 000)"
  RESP="$(cat "$out")"; rm -f "$out"
}
TOKEN_FOR_CALL="$VAULT_ADMIN_TOKEN"

echo "==> Vault ${VAULT_ADDR}, policy '${NAME}'"
H="$(curl -sS -m 8 "${VAULT_ADDR}/v1/sys/health" 2>/dev/null || true)"
python3 -c 'import sys,json; d=json.loads(sys.argv[1]); print("    vault %s, initialized=%s sealed=%s"%(d.get("version"),d.get("initialized"),d.get("sealed")))' "$H" 2>/dev/null \
  || { echo "    cannot reach Vault" >&2; exit 1; }

api GET "sys/policies/acl/${NAME}"; LIVE="$RESP"
case "$STATUS" in
  200) ;;
  403) echo "    admin token cannot read sys/policies/acl/${NAME} (HTTP 403) — it is not an admin token" >&2; exit 1 ;;
  404) echo "    policy '${NAME}' does not exist on this Vault — check VAULT_POLICY_NAME (vault token lookup shows the orca token's policies)" >&2; exit 1 ;;
  *)   echo "    reading the live policy failed (HTTP ${STATUS})" >&2; exit 1 ;;
esac

BACKUP_DIR="${VAULT_POLICY_BACKUP_DIR:-${TMPDIR:-/tmp}}"
BACKUP="${BACKUP_DIR}/vault-policy-${NAME}-$(date +%Y%m%d-%H%M%S).hcl"
( umask 077; printf '%s' "$LIVE" | python3 -c 'import sys,json; sys.stdout.write(json.load(sys.stdin)["data"]["policy"])' > "$BACKUP" )
echo "    live policy backed up to ${BACKUP}"

# Rules present live but absent from the repo file would be silently dropped by the write.
REPORT="$(python3 - "$BACKUP" "$POLICY_FILE" <<'EOF'
import sys,re
def rules(p):
    out=[]
    for l in open(p).read().splitlines():
        l=l.split('#',1)[0].strip()
        if l: out.append(re.sub(r'\s+',' ',l))
    return out
live,repo=rules(sys.argv[1]),rules(sys.argv[2])
extra=[l for l in live if l not in repo]
new=[l for l in repo if l not in live]
print("EXTRA_LIVE=%d"%len(extra)); print("NEW_IN_REPO=%d"%len(new))
for l in extra: print("  live-only : "+l)
for l in new:   print("  repo-only : "+l)
EOF
)"
echo "$REPORT" | sed 's/^/    /'
EXTRA="$(printf '%s\n' "$REPORT" | sed -n 's/^EXTRA_LIVE=//p')"
NEWN="$(printf '%s\n' "$REPORT" | sed -n 's/^NEW_IN_REPO=//p')"

if [ "$NEWN" = "0" ] && [ "$EXTRA" = "0" ]; then
  echo "==> live policy already matches the repo file — nothing to do"; exit 0
fi
if [ "$EXTRA" != "0" ] && [ "$FORCE" -ne 1 ]; then
  echo "==> STOP: the live policy has ${EXTRA} rule(s) the repo file lacks (listed above as 'live-only')." >&2
  echo "    Applying would DELETE them. Merge them into deploy/dev/orca-policy.hcl first, or re-run with --force." >&2
  exit 3
fi
if [ "$APPLY" -ne 1 ]; then
  echo "==> dry run only. Re-run with --apply to write policy '${NAME}'."; exit 0
fi
if [ "$YES" -ne 1 ]; then
  read -r -p "Write policy '${NAME}' to ${VAULT_ADDR}? [y/N] " ans
  [ "$ans" = "y" ] || [ "$ans" = "Y" ] || { echo "aborted"; exit 1; }
fi

BODY="$(mktemp)"; trap 'rm -f "$BODY"' EXIT
python3 -c 'import sys,json; print(json.dumps({"policy": open(sys.argv[1]).read()}))' "$POLICY_FILE" > "$BODY"
api PUT "sys/policies/acl/${NAME}" "$BODY"
[ "$STATUS" = "204" ] || [ "$STATUS" = "200" ] || { echo "write failed (HTTP ${STATUS}); live policy untouched? restore from ${BACKUP}" >&2; exit 1; }
echo "==> policy '${NAME}' written (previous version saved at ${BACKUP})"

# Verify with the orca service token (read-only capability check; the token is never printed).
ORCA_TOKEN="${VAULT_ORCA_TOKEN:-$(grep -m1 '^VAULT_TOKEN=' "${DEPLOY_DIR}/.env" 2>/dev/null | cut -d= -f2- || true)}"
if [ -n "$ORCA_TOKEN" ]; then
  TOKEN_FOR_CALL="$ORCA_TOKEN"
  CAPS="$(mktemp)"; echo '{"paths":["transit/keys/vapid-signing-probe","transit/keys/some-other-key","transit/encrypt/probe","transit/sign/vapid-signing-probe","credential-secrets/data/probe"]}' > "$CAPS"
  api POST sys/capabilities-self "$CAPS"; RES="$RESP"; rm -f "$CAPS"
  echo "==> capabilities of the orca token after the change:"
  printf '%s' "$RES" | python3 -c '
import sys,json
d=json.load(sys.stdin)
want={"transit/keys/vapid-signing-probe":"create","transit/keys/some-other-key":None}
for k,v in d.items():
    if isinstance(v,list): print("    %-40s %s"%(k,", ".join(v)))
ok = "create" in d.get("transit/keys/vapid-signing-probe",[]) and "create" not in d.get("transit/keys/some-other-key",[])
print("    => " + ("OK: create is granted only on vapid-signing-*" if ok else "UNEXPECTED — review the policy"))
sys.exit(0 if ok else 1)'
else
  echo "==> (skipped verification: no orca token in .env / VAULT_ORCA_TOKEN)"
fi
