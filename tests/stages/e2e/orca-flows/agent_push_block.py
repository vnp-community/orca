"""Runs the dev-server agent with `git push` made impossible, for specs that let the real Claude work in a repo of the user.

The user's repo (deepseek-harness) has a real GitHub remote and the agent runs `claude --dangerously-skip-permissions`, so a stray
`git push` would leave the machine. The block lives only in the environment of the agent process (and so of every git it and its
claude spawn); the user's repo and ~/.gitconfig are not touched:

  * `remote.origin.pushurl` points at a path that does not exist, so `git push origin` fails locally;
  * `url.<nowhere>.pushInsteadOf` rewrites the GitHub URL forms (scp-style ssh, ssh:// and https) to the same dead path, so pushing to the URL typed out fails too.

Fetching is untouched (worktree creation fetches). The values must match PUSH_BLOCK_ENV in ui-live/deepseek-repo-fixture.ts, which
refuses to start unless every agent process carries them and a dry-run push proves they work.
"""
from __future__ import annotations

import os
import subprocess

from ai_stub_agent_switch import START_SCRIPT, StubAgentError, capture_running_agent, wait_until_connected
from orca_stack_config import REPO_ROOT, StackConfig

DEAD_REMOTE = "/nonexistent/orca-e2e-push-blocked"
PUSH_BLOCK_ENV = {
    "GIT_CONFIG_COUNT": "4",
    "GIT_CONFIG_KEY_0": "remote.origin.pushurl",
    "GIT_CONFIG_VALUE_0": DEAD_REMOTE,
    "GIT_CONFIG_KEY_1": f"url.{DEAD_REMOTE}/.pushInsteadOf",
    "GIT_CONFIG_VALUE_1": "git@github.com:",
    "GIT_CONFIG_KEY_2": f"url.{DEAD_REMOTE}/.pushInsteadOf",
    "GIT_CONFIG_VALUE_2": "https://github.com/",
    "GIT_CONFIG_KEY_3": f"url.{DEAD_REMOTE}/.pushInsteadOf",
    "GIT_CONFIG_VALUE_3": "ssh://git@github.com/",
}


class PushBlockedAgent:
    """Restarts the agent (same dev server, same work dir, same log, real HOME and PATH) with the push block, and back."""

    def __init__(self, cfg: StackConfig) -> None:
        self.cfg = cfg
        self.original = capture_running_agent()

    def _restart(self, env: dict[str, str]) -> None:
        for key, value in self.original.items():
            env.setdefault(key, value)
        done = subprocess.run(["bash", str(START_SCRIPT)], env=env, cwd=str(REPO_ROOT), stdin=subprocess.DEVNULL,
                              capture_output=True, text=True, timeout=60)
        if done.returncode != 0:
            raise StubAgentError(f"start-local-agent.sh failed ({done.returncode}): {done.stderr.strip()[:300]}")
        wait_until_connected(self.cfg, 90.0, approve=False)

    def start(self) -> None:
        env = dict(os.environ)
        env.update(PUSH_BLOCK_ENV)
        self._restart(env)

    def restore(self) -> str:
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_CONFIG_")}
        self._restart(env)
        return "agent restored without the push block"
