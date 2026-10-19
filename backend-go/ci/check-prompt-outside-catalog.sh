#!/usr/bin/env bash
# Prompt-outside-catalog guard (CR-REQ-042, TASK-REQ-042-08): wording sent to the agent lives in backend-go/prompts (.tmpl files),
# not in strings a service builds. A prompt that wraps outside text in a fence tag is the tell, so this fails on a non-test .go file
# outside backend-go/prompts that holds one of the fence tags below on a non-comment line.
#
# Heuristic and its limits: it is a grep for `<feedback>`, `<request>`, `<prior_artifacts>`, `<clarifications>`, `<project>`,
# `<approved_decision>` (opening or closing form). It does NOT see a prompt that uses none of these tags, a tag assembled at run time
# ("<" + name + ">"), a prompt in a file type other than .go, or a prompt text pasted into a test. It can flag a non-prompt string that
# happens to contain a tag (e.g. a `req:<request>:<task>` format string); give such a file an EXCLUDED entry with the reason.
# An EXCLUDED entry that no longer matches anything fails too, so the list can only shrink. BSD (macOS) and GNU userland, bash 3.2+.
set -euo pipefail

root="${PROMPT_CATALOG_CHECK_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
cd "$root/backend-go"

# path-prefix|reason
EXCLUDED=(
  "services/request-service/internal/adapter/contracttest/|contract harness: asserts where text sits in a rendered prompt, builds no prompt"
)

tag_rule='</?(feedback|request|prior_artifacts|clarifications|project|approved_decision)>'
code_lines='^[0-9]+:[[:space:]]*(//|/\*|\*)'

files="$(find . -type f -name '*.go' ! -name '*_test.go' | sed 's|^\./||' \
  | { grep -v -E '^(proto/|prompts/)' || true; } | sort)"

failed=0
used_file="$(mktemp)"
trap 'rm -f "$used_file"' EXIT

while IFS= read -r f; do
  [ -n "$f" ] || continue
  hits="$(grep -n -E "$tag_rule" "$f" | { grep -v -E "$code_lines" || true; })" || true
  [ -n "$hits" ] || continue
  excluded=0
  for entry in "${EXCLUDED[@]}"; do
    prefix="${entry%%|*}"
    case "$f" in "$prefix"*) excluded=1; echo "$prefix" >> "$used_file" ;; esac
  done
  if [ "$excluded" -eq 0 ]; then
    echo "FAIL: $f builds text with a prompt fence tag outside backend-go/prompts:" >&2
    echo "$hits" | sed 's/^/  /' >&2
    failed=1
  fi
done <<EOF2
$files
EOF2

for entry in "${EXCLUDED[@]}"; do
  prefix="${entry%%|*}"
  if ! grep -q -x -F "$prefix" "$used_file"; then
    echo "FAIL: EXCLUDED entry '$prefix' no longer matches a file; remove it from $(basename "$0")." >&2
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  cat >&2 <<'MSG'

Prompt-outside-catalog check failed.
  Move the wording into a template under backend-go/prompts/<flow>/ (see backend-go/prompts/README.md) and call its Render.
  If the file only mentions a tag without building a prompt, add it to EXCLUDED with the reason.
MSG
  exit 1
fi
echo "ok: no prompt fence tags are built outside backend-go/prompts"
