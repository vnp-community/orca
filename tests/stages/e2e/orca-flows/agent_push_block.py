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

import argparse
import os
import re
import subprocess
import sys

from ai_stub_agent_switch import START_SCRIPT, StubAgentError, _pids_of_agent, capture_running_agent, wait_until_connected
from orca_stack_config import REPO_ROOT, StackConfig, load_config

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

    def __init__(self, cfg: StackConfig, work_dir: str | None = None) -> None:
        self.cfg = cfg
        # Why: a repo whose task worktrees sit outside the agent's AGENT_WORK_DIR (vnp-qa-platform-task-<id> next to the repo) cannot be written by the
        # relay; `work_dir` widens it for the run only, and restore() puts back the captured original.
        self.work_dir = work_dir
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
        if self.work_dir:
            env["AGENT_WORK_DIR"] = self.work_dir
        self._restart(env)

    def restore(self) -> str:
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_CONFIG_")}
        if self.work_dir:
            env.pop("AGENT_WORK_DIR", None)  # the captured original comes back through setdefault in _restart
        self._restart(env)
        return "agent restored without the push block"


def agent_block_state() -> dict[int, bool]:
    """For each running agent process: does its environment carry the whole push block (read with `ps eww`; git inherits it)."""
    state = {}
    for pid in _pids_of_agent():
        out = subprocess.run(["ps", "eww", "-p", str(pid)], capture_output=True, text=True).stdout
        found = dict(re.findall(r"(?<=\s)(GIT_CONFIG_[A-Z0-9_]+)=(\S*)", out))
        state[pid] = all(found.get(k) == v for k, v in PUSH_BLOCK_ENV.items())
    return state


def probe_push(repo: str) -> list[str]:
    """Pushes that must fail locally, run with the block's environment. Returns the targets that did not fail the way they must."""
    env = {**os.environ, **PUSH_BLOCK_ENV, "GIT_TERMINAL_PROMPT": "0"}
    bad = []
    for target in ("origin", "git@github.com:example/example.git", "ssh://git@github.com/example/example.git", "https://github.com/example/example.git"):
        done = subprocess.run(["git", "-C", repo, "push", "--dry-run", target, "HEAD:refs/heads/orca-push-probe"], env=env, capture_output=True, text=True, timeout=30)
        if done.returncode == 0 or "orca-e2e-push-blocked" not in done.stdout + done.stderr:
            bad.append(target)
    return bad


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="start|status|restore the dev-server agent with `git push` blocked (the pilot's safety step)")
    ap.add_argument("action", choices=["start", "status", "restore", "probe"])
    ap.add_argument("--repo", help="probe: a repo whose pushes must fail locally")
    args = ap.parse_args(argv)
    if args.action == "probe":
        if not args.repo:
            ap.error("probe needs --repo")
        bad = probe_push(args.repo)
        print("every push form fails locally" if not bad else f"NOT blocked: {', '.join(bad)}")
        return 1 if bad else 0
    if args.action in ("start", "restore"):
        agent = PushBlockedAgent(load_config("push-block"))
        print(agent.restore() if args.action == "restore" else (agent.start() or "agent restarted with the push block"))
    state = agent_block_state()
    for pid, blocked in state.items():
        print(f"agent pid {pid}: push block {'ON' if blocked else 'off'}")
    if not state:
        print("no agent process found", file=sys.stderr)
        return 1
    if args.action == "start":
        return 0 if all(state.values()) else 1
    if args.action == "restore":
        return 0 if not any(state.values()) else 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
