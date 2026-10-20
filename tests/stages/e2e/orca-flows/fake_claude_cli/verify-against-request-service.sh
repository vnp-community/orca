#!/usr/bin/env bash
# Runs the fake `claude` against request-service's REAL prompt builders, parsers and validators, using `go test -overlay`
# so no file is added to backend-go. Fails when a prompt or a validator changes and the stub no longer fits.
#
#   bash tests/stages/e2e/orca-flows/fake_claude_cli/verify-against-request-service.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../../../../.." && pwd)"
svc="$repo/backend-go/services/request-service"
tasksvc="$repo/backend-go/services/task-service"

command -v go >/dev/null || { echo "verify-fake-claude: go is required" >&2; exit 2; }
command -v python3 >/dev/null || { echo "verify-fake-claude: python3 is required" >&2; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
python3 - "$svc/internal/usecase/zz_fake_claude_contract_test.go" "$here/request_service_contract.overlay.go.txt" "$work/overlay.json" <<'PY'
import json, sys
target, source, out = sys.argv[1:4]
json.dump({"Replace": {target: source}}, open(out, "w"))
PY
python3 - "$tasksvc/internal/domain/zz_fake_claude_execution_contract_test.go" "$here/task_service_contract.overlay.go.txt" "$work/task-overlay.json" <<'PY'
import json, sys
target, source, out = sys.argv[1:4]
json.dump({"Replace": {target: source}}, open(out, "w"))
PY

cd "$svc"
FAKE_CLAUDE_BIN="$here/claude" go test -overlay "$work/overlay.json" -run '^TestFakeClaude_' -count=1 ./internal/usecase/ "$@"
# The execution prompt belongs to task-service (the agent runs it with write access), so its half of the contract is checked there.
cd "$tasksvc"
FAKE_CLAUDE_BIN="$here/claude" go test -overlay "$work/task-overlay.json" -run '^TestFakeClaude_' -count=1 ./internal/domain/ "$@"
