#!/usr/bin/env bash
# Tier 1 (blocking): the in-process MCP conformance suites, one command.
# No network, no Docker. Covers initialize/version negotiation/ping/tools/
# resources/prompts, resume across two replicas, cancel, auth and origin
# negatives, kill switch, metrics + trace chain, rollout flags, and a short
# fuzz of the JSON-RPC decode path. FUZZTIME=0 skips the fuzz step.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GW="$ROOT/services/api-gateway"
FUZZTIME="${FUZZTIME:-10s}"

cd "$GW"
echo "== gateway protocol conformance (mcpserver)"
go test ./internal/adapter/mcpserver/... -count=1 -race

echo "== gateway policy gate, session plumbing, metrics and tracing"
go test ./internal/adapter/mcppolicy/... ./internal/adapter/mcpmetrics/... ./internal/adapter/mcpsession/... -count=1 -race

echo "== gateway flags / rollout / OAuth discovery wiring"
go test ./cmd/server/... -count=1 -run 'MCP|Mcp' 
go test ./internal/adapter/wscompat/... -count=1 -run 'Mcp'

echo "== mcp-service decision table, rollback drill, red-team"
(cd "$ROOT/services/mcp-service" && go test ./internal/usecase/... ./internal/redteam/... ./internal/adapter/metrics/... -count=1)

if [[ "$FUZZTIME" != "0" ]]; then
  echo "== FuzzJSONRPCDecode ($FUZZTIME)"
  go test ./internal/adapter/mcpserver/ -run '^$' -fuzz FuzzJSONRPCDecode -fuzztime "$FUZZTIME"
fi
echo "OK: tier 1 MCP conformance passed"
