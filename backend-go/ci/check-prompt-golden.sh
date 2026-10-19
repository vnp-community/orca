#!/usr/bin/env bash
# Prompt golden gate (CR-REQ-042, TASK-REQ-042-05): every function that builds a prompt for the agent has golden files under
# <package>/testdata/prompt-golden/<function>/<case>.txt, and this script fails when any rendered prompt differs from them.
# The later move of the prompts into backend-go/prompts must render byte for byte the same text, so a difference is either a
# bug or an intended prompt change.
#
# Usage:
#   backend-go/ci/check-prompt-golden.sh            compare (what CI runs); never rewrites a golden
#   backend-go/ci/check-prompt-golden.sh --update   regenerate every golden after an INTENDED prompt change
#
# Intended change: run with --update, read `git diff -- '*/testdata/prompt-golden/*'`, and commit the golden files in the same
# commit as the prompt change so the reviewer sees both. An unintended difference is fixed in the code, never by --update.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
services="$root/backend-go/services"

mode="compare"
case "${1:-}" in
  "") ;;
  --update) mode="update" ;;
  *) echo "usage: $0 [--update]" >&2; exit 2 ;;
esac

packages=(
  "request-service ./internal/usecase"
  "request-service ./internal/adapter/grpcclient"
  "task-service ./internal/domain"
  "task-service ./internal/adapter/grpcclient"
  "task-service ./internal/usecase"
  "git-gateway-service ./internal/usecase"
)

failed=0
for entry in "${packages[@]}"; do
  service="${entry%% *}"
  pkg="${entry#* }"
  args=(-count=1 -run '^TestPromptGolden_' "$pkg")
  [ "$mode" = "update" ] && args+=(-update)
  log="$(mktemp)"
  if ! (cd "$services/$service" && go test "${args[@]}") >"$log" 2>&1; then
    echo "FAIL: $service $pkg: a prompt differs from its golden file (or a golden is missing or stale)." >&2
    cat "$log" >&2
    failed=1
  elif grep -q "no tests to run" "$log"; then
    # go test exits 0 when -run matches nothing; a renamed test would otherwise turn the gate off silently.
    echo "FAIL: $service $pkg: no TestPromptGolden_ test ran, so the gate checked nothing." >&2
    failed=1
  else
    echo "ok: $service $pkg"
  fi
  rm -f "$log"
done

if [ "$failed" -ne 0 ]; then
  cat >&2 <<'MSG'

Prompt golden check failed.
  - Unintended: undo the change to the prompt builder.
  - Intended: backend-go/ci/check-prompt-golden.sh --update, review `git diff -- '*/testdata/prompt-golden/*'`,
    and commit the updated golden files together with the prompt change.
MSG
  exit 1
fi
if [ "$mode" = "update" ]; then
  echo "Goldens regenerated. Review 'git diff -- \"*/testdata/prompt-golden/*\"' and commit them with the prompt change."
fi
