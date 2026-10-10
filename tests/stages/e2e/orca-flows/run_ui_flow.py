#!/usr/bin/env python3
"""Runs the live Request-flow UI spec (Playwright, ui-live/) against the local stack.

  python3 run_ui_flow.py                 # the dev server's own Claude (real AI, slower, spends usage)
  python3 run_ui_flow.py --ai-stub       # deterministic fake claude: swaps the agent, runs, restores it (like run_acceptance.py --ai-stub)
  python3 run_ui_flow.py --ai-stub -- --headed   # anything after `--` goes to `playwright test`

The agent switch is the same StubbedAgent run_acceptance.py uses, so the stub behaves identically in both runners.
Exit code: Playwright's, 2 bad usage / stub agent could not start, 3 the agent could not be restored.
"""
from __future__ import annotations

import argparse
import os
import subprocess
import sys
import time

from ai_stub_agent_switch import StubAgentError, StubbedAgent
from orca_stack_config import REPO_ROOT, load_config

PLAYWRIGHT_CONFIG = "tests/configs/playwright.web.config.ts"


def run_playwright(env: dict[str, str], extra: list[str]) -> int:
    cmd = ["npx", "playwright", "test", "-c", PLAYWRIGHT_CONFIG, "--project=request-live", *extra]
    return subprocess.run(cmd, cwd=str(REPO_ROOT), env=env).returncode


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--ai-stub", action="store_true", help="answer AI calls with the deterministic fake claude")
    parser.add_argument("playwright_args", nargs="*", help="extra arguments for `playwright test` (after `--`)")
    args = parser.parse_args()

    cfg = load_config(time.strftime("%m%d%H%M%S"))
    env = dict(os.environ, ORCA_E2E_LIVE="1")
    stub: StubbedAgent | None = None
    restore_failed = False
    try:
        if args.ai_stub:
            try:
                stub = StubbedAgent(cfg)
                print("starting the agent with the fake claude (logged in) ...")
                stub.start(logged_in=True)
            except StubAgentError as exc:
                print(f"cannot start the stubbed agent: {exc}", file=sys.stderr)
                return 2
            env["ORCA_E2E_DEV_SERVER_NAME"] = stub.current_cfg.dev_server_name
            env["ORCA_E2E_AI_MODE"] = "stub"
            print(f"AI mode: STUB (wiring only, not AI quality); dev server {stub.current_cfg.dev_server_name}")
        return run_playwright(env, args.playwright_args)
    finally:
        if stub:
            try:
                print(stub.restore())
            except StubAgentError as exc:
                restore_failed = True
                print(f"WARNING: the agent could not be restored: {exc}\n         run: bash deploy/dev/scripts/start-local-agent.sh", file=sys.stderr)
            if restore_failed:
                sys.exit(3)


if __name__ == "__main__":
    sys.exit(main())
