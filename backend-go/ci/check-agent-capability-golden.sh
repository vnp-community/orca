#!/usr/bin/env bash
# The agent's capability fixture is the contract; the Go services keep verbatim
# copies. Fails when any copy drifts (CR-REQ-033). A copy that does not exist
# yet (request-service is added in a later batch) is reported, not an error.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source_file="$root/agent/src/relay/__fixtures__/agent-capabilities-golden.json"
copies=(
  "$root/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/testdata/agent_capabilities_v1.golden.json"
  "$root/backend-go/services/request-service/internal/adapter/grpcclient/testdata/agent_capabilities_v1.golden.json"
)

hash_of() { sha256sum "$1" | cut -d' ' -f1; }

[ -f "$source_file" ] || { echo "missing agent fixture: $source_file" >&2; exit 2; }
want="$(hash_of "$source_file")"
status=0
for copy in "${copies[@]}"; do
  if [ ! -f "$copy" ]; then
    echo "skip (not present): ${copy#"$root"/}"
    continue
  fi
  if [ "$(hash_of "$copy")" != "$want" ]; then
    echo "DRIFT: ${copy#"$root"/} differs from agent fixture" >&2
    status=1
  else
    echo "ok: ${copy#"$root"/}"
  fi
done
exit $status
