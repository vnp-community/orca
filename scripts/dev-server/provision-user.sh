#!/bin/bash
# scripts/dev-server/provision-user.sh
#
# Deploy to dev server:
#   sudo cp provision-user.sh /usr/local/bin/orca-provision-user.sh
#   sudo chmod +x /usr/local/bin/orca-provision-user.sh
#
# Usage:
#   sudo /usr/local/bin/orca-provision-user.sh <linux_user> <orca_public_key>
#
# Example:
#   sudo orca-provision-user.sh orca-alice-1a2b "ssh-ed25519 AAAAC3... orca-server"

set -euo pipefail

LINUX_USER="${1:?Usage: $0 <linux_user> <orca_public_key>}"
ORCA_PUBKEY="${2:?Usage: $0 <linux_user> <orca_public_key>}"

# Validate username format (must start with orca- and be safe)
if [[ ! "${LINUX_USER}" =~ ^orca-[a-z][a-z0-9-]{0,20}$ ]]; then
  echo "ERROR: Invalid linux username: ${LINUX_USER}" >&2
  echo "       Must match: orca-[a-z][a-z0-9-]{0,20}" >&2
  exit 1
fi

echo "orca-provision-user: provisioning ${LINUX_USER}"

# ── Create user (idempotent) ───────────────────────────────────────────────────
if ! id "${LINUX_USER}" &>/dev/null; then
  useradd -m -s /bin/bash "${LINUX_USER}"
  # Add to 'developers' group if it exists (ignore error if not)
  getent group developers &>/dev/null && usermod -aG developers "${LINUX_USER}" || true
  echo "✅ Created user: ${LINUX_USER}"
else
  echo "ℹ️  User exists: ${LINUX_USER}"
fi

# ── Authorize Orca Server SSH key (idempotent) ─────────────────────────────────
SSH_DIR="/home/${LINUX_USER}/.ssh"
AUTH_KEYS="${SSH_DIR}/authorized_keys"

mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"
chown "${LINUX_USER}:${LINUX_USER}" "${SSH_DIR}"

if ! grep -qF "${ORCA_PUBKEY}" "${AUTH_KEYS}" 2>/dev/null; then
  echo "${ORCA_PUBKEY}" >> "${AUTH_KEYS}"
  echo "✅ Authorized Orca key for: ${LINUX_USER}"
else
  echo "ℹ️  Key already authorized for: ${LINUX_USER}"
fi

chmod 600 "${AUTH_KEYS}"
chown "${LINUX_USER}:${LINUX_USER}" "${AUTH_KEYS}"

echo "✅ Done: ${LINUX_USER}"
