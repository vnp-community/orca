#!/usr/bin/env bash
# Lists repos with no dev server (dry-run by default); --apply --dev-server <id|name> binds them.
# See rebind_unbound_repos.py for the safety rules. Never run against data you do not intend to change.
set -euo pipefail
exec python3 "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/rebind_unbound_repos.py" "$@"
