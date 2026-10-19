#!/usr/bin/env bash
# Single relay guard (CR-REQ-042, TASK-REQ-042-08): "agent.execPrompt" and "ai.complete" are sent to the agent only through
# common/agentrelay, so wire params, timeouts and result parsing stay in one place and a copy cannot grow back.
#
# Scanned: every non-test .go file under backend-go except common/agentrelay (the relay), services/infra-fleet-service (it
# receives, times and routes these methods) and proto/ (generated). Two rules:
#   1. FAIL on `Method: "agent.execPrompt"` / `Method: "ai.complete"` (or agentrelay.MethodExecPrompt / MethodComplete used as a
#      Method:) anywhere scanned. No allowlist applies: that is exactly a hand-made Relay / RelayByDevServer call.
#   2. FAIL on any other quoted literal of the two names unless the file is in ALLOWED below with a reason. Comment lines are skipped.
#      An ALLOWED entry that no longer matches anything also fails, so the list can only shrink.
#
# Limits (a grep, not a type check): a name assembled at run time ("agent." + "execPrompt"), a literal in a non-Go file, or wire
# params built by hand without the method name are not seen; a literal in a trailing `// "agent.execPrompt"` comment on a code line is a
# false positive. Test files are exempt on purpose: they assert on the wire. Works with BSD (macOS) and GNU userland, bash 3.2+.
set -euo pipefail

root="${AGENT_RELAY_CHECK_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
cd "$root/backend-go"

# path|reason  (the files mention the names, but never relay them)
ALLOWED=(
  "services/request-service/internal/usecase/solution_artifact_recorder.go|names the generator tool in artifact provenance, no RPC"
  "services/request-service/internal/domain/agent_route_selection.go|feature names the agent advertises, compared with its capability list"
  "services/request-service/internal/adapter/contracttest/solution_artifact_contract.go|contract assertion on the recorded provenance tool name"
  "services/request-service/e2e/stubs/agent_script.go|fake agent that answers these methods; it receives them and sends nothing"
)

method_rule='Method:[[:space:]]*(["`](agent\.execPrompt|ai\.complete)["`]|(agentrelay\.)?Method(ExecPrompt|Complete)([^A-Za-z0-9_]|$))'
literal_rule='["`](agent\.execPrompt|ai\.complete)["`]'
code_lines='^[0-9]+:[[:space:]]*(//|/\*|\*)'

files="$(find . -type f -name '*.go' ! -name '*_test.go' | sed 's|^\./||' \
  | { grep -v -E '^(proto/|common/agentrelay/|services/infra-fleet-service/)' || true; } | sort)"

failed=0
used_file="$(mktemp)"
trap 'rm -f "$used_file"' EXIT

while IFS= read -r f; do
  [ -n "$f" ] || continue
  hits="$(grep -n -E "$method_rule" "$f" | { grep -v -E "$code_lines" || true; })" || true
  if [ -n "$hits" ]; then
    echo "FAIL: $f calls the agent relay by hand (use common/agentrelay):" >&2
    echo "$hits" | sed 's/^/  /' >&2
    failed=1
    continue
  fi
  hits="$(grep -n -E "$literal_rule" "$f" | { grep -v -E "$code_lines" || true; })" || true
  [ -n "$hits" ] || continue
  allowed=0
  for entry in "${ALLOWED[@]}"; do
    if [ "${entry%%|*}" = "$f" ]; then allowed=1; echo "$f" >> "$used_file"; fi
  done
  if [ "$allowed" -eq 0 ]; then
    echo "FAIL: $f names agent.execPrompt / ai.complete outside common/agentrelay:" >&2
    echo "$hits" | sed 's/^/  /' >&2
    failed=1
  fi
done <<EOF2
$files
EOF2

for entry in "${ALLOWED[@]}"; do
  path="${entry%%|*}"
  if ! grep -q -x -F "$path" "$used_file"; then
    echo "FAIL: ALLOWED entry '$path' no longer matches a literal; remove it from $(basename "$0")." >&2
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  cat >&2 <<'MSG'

Agent relay single-source check failed.
  Send agent.execPrompt / ai.complete with agentrelay.Relayer (ExecPrompt, Complete) from backend-go/common/agentrelay.
  A file that only mentions the name (provenance label, capability name) goes in ALLOWED with a reason.
MSG
  exit 1
fi
echo "ok: agent.execPrompt / ai.complete are relayed only by common/agentrelay"
