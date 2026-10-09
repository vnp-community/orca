#!/usr/bin/env bash
# Every method a backend service relays to the dev server agent must be implemented by the agent.
# `fs.checkPath` was called by project-service for weeks while the agent answered "method not found", and no test
# on either side could see it: each tested against a mock of the other.
#
# Compares (1) method-name literals in Go files that relay to the agent with (2) the methods the agent dispatches
# (case labels and onRequest registrations in agent/src/relay). codeintel.* and quality.* are covered by the agent
# capability golden check. Known gaps live in agent-methods-called-known-gaps.txt so only NEW gaps fail (ratchet).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
known="$root/backend-go/ci/agent-methods-called-known-gaps.txt"

agent_methods="$(grep -rhoE "(case '|onRequest\(')[a-zA-Z0-9_.]+'" "$root"/agent/src/relay --include='*.ts' --exclude='*.test.ts' \
  | sed -E "s/^(case '|onRequest\(')//; s/'\$//" | sort -u)"

relaying_files="$(grep -rlE 'Relay|relay\(' "$root"/backend-go/services --include='*.go' --exclude='*_test.go' \
  | grep -vE '/api-gateway/|/proto/gen/|/e2e/|/contracttest/' || true)"

called="$(echo "$relaying_files" | xargs grep -hoE '"(fs|git|shell|preflight|ai|agent|pty|worktree|host|emulator|vm|terminal)\.[A-Za-z.]+"' 2>/dev/null \
  | tr -d '"' | sort -u)"

known_names="$(grep -vE '^\s*#|^\s*$' "$known" | awk '{print $1}' | sort -u)"

missing="$(comm -23 <(echo "$called") <(echo "$agent_methods") | comm -23 - <(echo "$known_names") || true)"

if [ -n "$missing" ]; then
  echo "Backend relays methods the agent does not implement (add them to the agent, or to $known with a reason):" >&2
  echo "$missing" | sed 's/^/  /' >&2
  exit 1
fi
stale="$(comm -12 <(echo "$known_names") <(echo "$agent_methods") || true)"
if [ -n "$stale" ]; then
  echo "note: these known gaps are now implemented by the agent; remove them from the known-gaps file:" >&2
  echo "$stale" | sed 's/^/  /' >&2
fi
echo "ok: every relayed method is implemented by the agent (known gaps: $(echo "$known_names" | grep -c . || true))"
