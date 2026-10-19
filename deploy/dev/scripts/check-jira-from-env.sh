#!/usr/bin/env bash
# ============================================================
# check-jira-from-env.sh — verify the Jira site in .env works through the running stack
# ============================================================
# Reads JR_URL, JR_EMAIL, JR_PAT_TOKEN (or JIRA_SITE_URL, JIRA_EMAIL, JIRA_PAT_TOKEN) from the environment, deploy/dev/.env or the
# repo's .env, connects through the gateway (email + PAT), lists a few issues and disconnects again. The token is never printed.
#
# Usage:
#   ./deploy/dev/scripts/check-jira-from-env.sh            # connect, list, disconnect
#   ./deploy/dev/scripts/check-jira-from-env.sh --keep     # leave the connection in place
# ============================================================
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
exec python3 "${REPO_ROOT}/tests/stages/e2e/orca-flows/jira_env_check.py" "$@"
