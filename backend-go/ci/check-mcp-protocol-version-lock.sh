#!/usr/bin/env bash
# ci/check-mcp-protocol-version-lock.sh - the MCP spec revision list lives in ONE
# file (mcpserver/protocol_versions.go). A revision date literal anywhere else in
# non-test, non-comment code means a version was hard-coded around the lock
# (BE-MCP-SOL-015). Static; needs no stack.
set -euo pipefail
DIR="${MCPSERVER_DIR:-$(dirname "$0")/../services/api-gateway/internal/adapter/mcpserver}"
[[ -d "$DIR" ]] || { echo "FAIL: $DIR not found" >&2; exit 1; }
hits=$(grep -rEn --include='*.go' '20[0-9]{2}-[0-9]{2}-[0-9]{2}' "$DIR" \
  | grep -vE '_test\.go:|/testdata/|/protocol_versions\.go:' \
  | grep -vE '^[^:]+:[0-9]+:[[:space:]]*//' || true)
if [[ -n "$hits" ]]; then
  echo "FAIL: protocol revision literal outside protocol_versions.go:" >&2
  echo "$hits" >&2
  exit 1
fi
echo "OK: protocol revision dates only in protocol_versions.go"
