#!/bin/bash
# usage: run_one.sh BL-XXX
id=$1
B=/private/tmp/claude-501/-Users-datbeohbbh-Desktop-work-vnpay-agentic-app-orca/ee57ae1e-77c4-42ae-9e21-4f4ddf0b8b03/scratchpad
S=$HOME/.cache/orca-pilot/spike-vnp
f=$(ls /Users/datbeohbbh/Desktop/work/vnpay/agentic-app/vnp-qa-platform/docs/backlog/$id*.md)
title=$(grep -m1 '^title:' "$f" | sed 's/^title: *"\(.*\)"$/\1/')
body=$(python3 - "$f" <<'PY'
import sys,re
t=open(sys.argv[1]).read()
t=re.sub(r"^---.*?---\n","",t,count=1,flags=re.S)
print(t[:2800])
PY
)
cat > $B/t11/prompt-$id.txt <<P
You are working an Orca Request in the Analysis step (read-only). Use the skills tdd-orca-task and tdd-knowledge. Request $id: $title

Scope (from the backlog item):
$body

Task: find in the design knowledge graph every requirement, business rule, entity, decision and constraint this Request affects or must respect. Do not write a solution. Finish with the plain text block from tdd-orca-task (Knowledge / Read / Gaps / Conflicts / Verification / Writeback).
P
cd $S && claude --print --output-format stream-json --verbose --model sonnet --permission-mode plan --tools Read,Glob,Grep --strict-mcp-config --mcp-config $B/tdd-mcp.json --allowedTools "mcp__tdd-agent__kg_search,mcp__tdd-agent__kg_get_element,mcp__tdd-agent__kg_neighbors,mcp__tdd-agent__kg_resolve_scope,mcp__tdd-agent__kg_query,mcp__tdd-agent__kg_snapshots,mcp__tdd-agent__source_get" < $B/t11/prompt-$id.txt > $B/t11/run-$id.jsonl 2> $B/t11/run-$id.err
