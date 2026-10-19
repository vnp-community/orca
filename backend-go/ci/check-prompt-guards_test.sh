#!/usr/bin/env bash
# Self-test of check-agent-relay-single-source.sh and check-prompt-outside-catalog.sh: both pass on the real tree, and each fails
# on a deliberate violation planted in a temp copy (never in the real tree). Works on GNU and BSD userland, bash 3.2+.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
real_root="$(cd "$here/../.." && pwd)"
relay_script="$here/check-agent-relay-single-source.sh"
catalog_script="$here/check-prompt-outside-catalog.sh"

# Real files each script's allow/exclude list points at, so the temp tree has no stale entry.
relay_allowed=(
  backend-go/services/request-service/internal/usecase/solution_artifact_recorder.go
  backend-go/services/request-service/internal/domain/agent_route_selection.go
  backend-go/services/request-service/internal/adapter/contracttest/solution_artifact_contract.go
  backend-go/services/request-service/e2e/stubs/agent_script.go
)
catalog_excluded=(
  backend-go/services/request-service/internal/adapter/contracttest/solution_revision_feedback_contract.go
)

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
failures=0

fresh_tree() { # fresh_tree FILE...: an empty backend-go tree holding copies of the given real files
  rm -rf "$tmp/tree"
  mkdir -p "$tmp/tree/backend-go"
  local f
  for f in "$@"; do
    mkdir -p "$tmp/tree/$(dirname "$f")"
    cp "$real_root/$f" "$tmp/tree/$f"
  done
}

plant() { # plant PATH: write stdin to backend-go/PATH in the temp tree
  mkdir -p "$tmp/tree/backend-go/$(dirname "$1")"
  cat > "$tmp/tree/backend-go/$1"
}

run_relay() { AGENT_RELAY_CHECK_ROOT="$tmp/tree" bash "$relay_script" > "$tmp/out" 2> "$tmp/err" && echo 0 || echo $?; }
run_catalog() { PROMPT_CATALOG_CHECK_ROOT="$tmp/tree" bash "$catalog_script" > "$tmp/out" 2> "$tmp/err" && echo 0 || echo $?; }

expect_pass() { # expect_pass NAME RUNNER
  if [ "$("$2")" != "0" ]; then
    echo "FAIL: '$1' must pass" >&2; cat "$tmp/err" >&2; failures=$((failures + 1))
  else
    echo "ok   $1"
  fi
}

expect_fail() { # expect_fail NAME RUNNER FRAGMENT
  if [ "$("$2")" = "0" ]; then
    echo "FAIL: '$1' passed although it violates the rule" >&2; failures=$((failures + 1))
  elif ! grep -Fq "$3" "$tmp/err"; then
    echo "FAIL: '$1' failed but did not say '$3':" >&2; cat "$tmp/err" >&2; failures=$((failures + 1))
  else
    echo "ok   $1"
  fi
}

# The real tree is the clean case for both scripts.
for s in "$relay_script" "$catalog_script"; do
  if ! bash "$s" > "$tmp/out" 2> "$tmp/err"; then
    echo "FAIL: $(basename "$s") must pass on the real tree" >&2; cat "$tmp/err" >&2; failures=$((failures + 1))
  else
    echo "ok   $(basename "$s") passes on the real tree"
  fi
done

# --- check-agent-relay-single-source.sh ---
fresh_tree "${relay_allowed[@]}"
expect_pass "relay: allowed files only" run_relay

fresh_tree "${relay_allowed[@]}"
printf 'package x\n\nvar _ = Req{Method: "agent.execPrompt"}\n' | plant services/foo/internal/adapter/relay.go
expect_fail "relay: Method agent.execPrompt in a service" run_relay "calls the agent relay by hand"

fresh_tree "${relay_allowed[@]}"
printf 'package x\n\nvar _ = Req{ConnectionId: c, Method:   "ai.complete", ParamsJson: p}\n' | plant services/foo/internal/adapter/relay.go
expect_fail "relay: Method ai.complete in a service" run_relay "calls the agent relay by hand"

fresh_tree "${relay_allowed[@]}"
printf 'package x\n\nvar _ = Req{Method: agentrelay.MethodExecPrompt}\n' | plant services/foo/internal/adapter/relay.go
expect_fail "relay: agentrelay.MethodExecPrompt used as a Method" run_relay "calls the agent relay by hand"

fresh_tree "${relay_allowed[@]}"
printf 'package x\n\nconst method = `agent.execPrompt`\n' | plant services/foo/internal/adapter/method.go
expect_fail "relay: raw-string literal outside the allowlist" run_relay "names agent.execPrompt / ai.complete outside common/agentrelay"

fresh_tree "${relay_allowed[@]}"
printf '\nvar _ = Req{Method: "ai.complete"}\n' >> "$tmp/tree/${relay_allowed[0]}"
expect_fail "relay: Method call inside an allowlisted file" run_relay "calls the agent relay by hand"

fresh_tree "${relay_allowed[@]:1}"
expect_fail "relay: allowlist entry whose file no longer has a literal" run_relay "no longer matches a literal"

fresh_tree "${relay_allowed[@]}"
printf 'package x\n\nvar _ = Req{Method: "agent.execPrompt"}\n' | plant services/foo/internal/adapter/relay_test.go
printf 'package x\n\nvar _ = Req{Method: "agent.execPrompt"}\n' | plant services/infra-fleet-service/internal/adapter/dispatch.go
printf 'package x\n\nvar _ = Req{Method: "agent.execPrompt"}\n' | plant common/agentrelay/relayer.go
printf 'package x\n\n// relayed as "agent.execPrompt" and "ai.complete" by agentrelay\nfunc F() {}\n' | plant services/foo/internal/usecase/port.go
expect_pass "relay: test files, infra-fleet-service, common/agentrelay and comments are exempt" run_relay

# --- check-prompt-outside-catalog.sh ---
fresh_tree "${catalog_excluded[@]}"
expect_pass "catalog: excluded contract harness only" run_catalog

fresh_tree "${catalog_excluded[@]}"
printf 'package x\n\nfunc build(b string) string { return "<feedback>\\n" + b + "\\n</feedback>" }\n' | plant services/foo/internal/usecase/prompt.go
expect_fail "catalog: <feedback> fence built in a service" run_catalog "builds text with a prompt fence tag"

fresh_tree "${catalog_excluded[@]}"
printf 'package x\n\nvar tmpl = `<request>\n{{.Body}}\n</request>`\n' | plant services/foo/internal/adapter/prompt.go
expect_fail "catalog: <request> fence in a raw string" run_catalog "builds text with a prompt fence tag"

fresh_tree "${catalog_excluded[@]}"
printf 'package x\n\nvar a = "<prior_artifacts>"\nvar b = "<clarifications>"\n' | plant services/foo/internal/adapter/prompt.go
expect_fail "catalog: <prior_artifacts> / <clarifications>" run_catalog "builds text with a prompt fence tag"

fresh_tree
expect_fail "catalog: EXCLUDED entry that matches nothing" run_catalog "no longer matches a file"

fresh_tree "${catalog_excluded[@]}"
printf 'package x\n\nvar a = "<request>"\n' | plant prompts/analysis/analysis.go
printf 'package x\n\nvar a = "<request>"\n' | plant services/foo/internal/usecase/prompt_test.go
printf 'package x\n\n// wraps the body in <request> ... </request>\nfunc F() {}\n' | plant services/foo/internal/usecase/port.go
expect_pass "catalog: backend-go/prompts, test files and comments are exempt" run_catalog

if [ "$failures" -ne 0 ]; then
  echo "$failures prompt-guard self-test(s) failed" >&2
  exit 1
fi
echo "all prompt-guard self-tests passed"
