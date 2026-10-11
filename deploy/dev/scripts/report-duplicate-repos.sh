#!/usr/bin/env bash
# Lists repos registered more than once on the same dev server (read-only: it never changes or deletes anything).
# See report_duplicate_repos.py for what counts as a duplicate and what it can and cannot see.
set -euo pipefail
exec python3 "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/report_duplicate_repos.py" "$@"
