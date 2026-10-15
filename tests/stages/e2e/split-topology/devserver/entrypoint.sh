#!/usr/bin/env bash
# Starts the Dev Server Agent the way deploy/agent does on a real machine: direct-websocket to the public gateway,
# with a token the operator issued (the server-side ORCA_AGENT_API_SECRET never reaches this box).
set -euo pipefail

: "${ORCA_URL:?ORCA_URL is required (ws://orca-gateway:8081/agent)}"
: "${AGENT_TOKEN:?AGENT_TOKEN is required: issued by split-up.sh}"
: "${DEV_SERVER_ID:?DEV_SERVER_ID is required}"
AGENT_JS="${AGENT_JS:-/opt/orca-agent/agent.js}"
[ -f "${AGENT_JS}" ] || { echo "agent.js not mounted at ${AGENT_JS}: run 'node agent/build.mjs' on the host" >&2; exit 1; }

git config --global user.name "${GIT_AUTHOR_NAME:-orca-e2e}"
git config --global user.email "${GIT_AUTHOR_EMAIL:-orca-e2e@example.test}"
git config --global init.defaultBranch main

# Fixture repos: each directory under /fixtures becomes a committed git repo under /srv/projects, once (the volume keeps it).
seed_repo() { # $1 = fixture dir, $2 = destination
  mkdir -p "$2"
  cp -a "$1." "$2/"
  git -C "$2" init -q
  git -C "$2" add -A
  git -C "$2" commit -q -m "fixture: $(basename "$2")"
  echo "seeded $2"
}
if [ -d /fixtures ]; then
  for src in /fixtures/*/; do
    [ -d "${src}" ] || continue
    dst="/srv/projects/$(basename "${src}")"
    [ -d "${dst}/.git" ] || seed_repo "${src}" "${dst}"
  done
fi

# The tdd-agent fixture becomes a tdd-agent project without a language model: no prose reading, rule-only test specs,
# MCP writes and approvals for the Orca client (docs/integrate/TDD-WF-AGENT-001 §7.2). The TDD itself is generated
# by the workflow under test, not here (--no-generate).
TDD_PROJECT=/srv/projects/tdd-project
if command -v tdd-agent >/dev/null && [ -d "${TDD_PROJECT}" ] && [ ! -f "${TDD_PROJECT}/.tdd-agent/config.toml" ]; then
  (
    cd "${TDD_PROJECT}"
    tdd-agent init --no-prose --no-generate --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md \
      --approver orca-workflow:human:BA,SA/Dev,SA,BA/owner >/dev/null
    # Test specs use the same language-model source as the agent: rules only unless AI_MODE=real (then Claude Code).
    TS_LLM=none; [ "${AI_MODE:-stub}" = "real" ] && TS_LLM=claude-cli
    sed -i "s/^llm = \"\"  /llm = \"${TS_LLM}\"/" .tdd-agent/config.toml
    tdd-agent mcp enable --approvals --client-actor orca-workflow >/dev/null
    git add -A && git commit -q -m "tdd-agent project" || true
  )
  echo "tdd-agent project ready in ${TDD_PROJECT}"
fi

# AI source of the agent. stub: the deterministic fake CLI of tests/stages/e2e/orca-flows (calls logged for assertions);
# real: Claude Code installed in the image, logged in through a mounted ~/.claude. API keys are never used, as in start-local-agent.sh.
case "${AI_MODE:-stub}" in
  stub)
    mkdir -p "${HOME}/.local/bin" "${HOME}/.fake-claude"
    ln -sf /opt/fake-claude/claude "${HOME}/.local/bin/claude"
    export FAKE_CLAUDE_STATE_DIR="${HOME}/.fake-claude"
    export FAKE_CLAUDE_LOGGED_IN="${FAKE_CLAUDE_LOGGED_IN:-1}"
    echo "${FAKE_CLAUDE_LOGGED_IN}" > "${FAKE_CLAUDE_STATE_DIR}/logged_in"
    ;;
  real)
    command -v claude >/dev/null || { echo "AI_MODE=real needs an image built with INSTALL_CLAUDE_CLI=1" >&2; exit 1; }
    ;;
  none) ;;
  *) echo "unknown AI_MODE=${AI_MODE}" >&2; exit 1 ;;
esac
export PATH="${HOME}/.local/bin:${PATH}"
unset ANTHROPIC_API_KEY OPENAI_API_KEY GOOGLE_API_KEY

echo "dev server ${DEV_SERVER_ID} ($(hostname)) -> ${ORCA_URL}, AI_MODE=${AI_MODE:-stub}, tdd-agent: $(command -v tdd-agent || echo absent)"
export MODE=direct-websocket AGENT_WORK_DIR="${AGENT_WORK_DIR:-/srv/projects}" LOG_LEVEL="${LOG_LEVEL:-info}"
exec node "${AGENT_JS}"
