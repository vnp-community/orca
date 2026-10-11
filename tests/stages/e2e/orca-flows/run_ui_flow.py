#!/usr/bin/env python3
"""Runs the live Request-flow UI spec (Playwright, ui-live/) against the local stack.

  python3 run_ui_flow.py                 # the dev server's own Claude (real AI, slower, spends usage)
  python3 run_ui_flow.py --ai-stub       # deterministic fake claude: swaps the agent, runs, restores it (like run_acceptance.py --ai-stub)
  python3 run_ui_flow.py --ai-stub -- --headed   # anything after `--` goes to `playwright test`
  python3 run_ui_flow.py --real readonly # REAL Claude Code, read-only analysis on a throwaway repo (question, bug)
  python3 run_ui_flow.py --real execute  # REAL Claude Code approves and EXECUTES a tiny Plan on a throwaway repo
  python3 run_ui_flow.py --real monitor  # REAL Claude Code executes a one-task Plan while the spec watches the agent through the UI (TASK-FE-E2E-011)
  python3 run_ui_flow.py --real addproject # REAL Claude Code, read-only: Add Project makes its own project per repo, a duplicate folder is refused, a Request in that project is answered from the right repo (TASK-FE-E2E-027)
  python3 run_ui_flow.py --real all      # all four; never combined with --ai-stub
  python3 run_ui_flow.py --repro add-project-worktree  # no AI: Add Project (host dev-local-mac) -> worktree "test-02" through the UI on a throwaway repo (TASK-FE-E2E-015); regression spec: GREEN once the host binding of SOL-FE-E2E-004 is deployed, RED if repo/project lose their dev server again

The agent switch is the same StubbedAgent run_acceptance.py uses, so the stub behaves identically in both runners.
`--real` runs the real-AI specs (ui-live/readonly-real, execute-real) in a safety net (real-flow-harness.ts): a temp repo that
is never a repo of the user, a tool guard inside it, before/after fingerprints of the real checkout, a watchdog and a hard
time limit. It leaves the dev server's agent as it is (real Claude); there is no stub.
Exit code: Playwright's, 2 bad usage / stub agent could not start, 3 the agent could not be restored or the run left an orphan Request.
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
REAL_SPECS = {
    "readonly": ["readonly-real"],
    "execute": ["execute-real"],
    "monitor": ["agent-session-monitoring-real"],
    "addproject": ["add-project-own-project-request-real"],
    "all": ["readonly-real", "execute-real", "agent-session-monitoring-real", "add-project-own-project-request-real"],
}
# Why: no AI and no Request: UI-only regression specs of reported bugs; each prints BUG REPRODUCED and fails if the bug comes back.
REPRO_SPECS = {"add-project-worktree": ["add-project-worktree-repro"]}
# Why: these approve a Plan, which runs a real agent that WRITES to the throwaway repo; read-only analysis does not need the flag
# (addproject only reads: it never approves the analysis).
EXECUTING = ("execute", "monitor", "all")


def run_playwright(env: dict[str, str], extra: list[str]) -> int:
    # Why: one worker, always: the specs share one stack and one dev server, and two real-AI runs at once would break each other's
    # safety comparison (and would reuse the same run prefix).
    cmd = ["npx", "playwright", "test", "-c", PLAYWRIGHT_CONFIG, "--project=request-live", "--workers=1", *extra]
    return subprocess.run(cmd, cwd=str(REPO_ROOT), env=env).returncode


def check_no_orphans() -> int:
    """After a real-AI run no Request may be left behind: the specs cancel theirs, and the report must say so."""
    done = subprocess.run([sys.executable, str(REPO_ROOT / "tests/stages/e2e/orca-flows/report_orphan_requests.py")], capture_output=True, text=True)
    print(done.stdout.strip().splitlines()[-1] if done.stdout.strip() else done.stderr.strip())
    return 0 if done.returncode == 0 and " 0 orphaned" in done.stdout else 3


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--ai-stub", action="store_true", help="answer AI calls with the deterministic fake claude")
    parser.add_argument("--real", choices=sorted(REAL_SPECS), help="run the real-Claude specs: read-only analysis, plan execution, agent-session monitoring, or all")
    parser.add_argument("--repro", choices=sorted(REPRO_SPECS), help="run a UI repro of a reported bug (no AI, no Request, throwaway repo)")
    parser.add_argument("playwright_args", nargs="*", help="extra arguments for `playwright test` (after `--`)")
    args = parser.parse_args()

    if args.real and args.ai_stub:
        print("--real and --ai-stub exclude each other: the real-AI specs never run against the fake claude", file=sys.stderr)
        return 2
    if args.repro and (args.real or args.ai_stub):
        print("--repro runs alone: it uses no AI, so --real and --ai-stub do not apply", file=sys.stderr)
        return 2
    cfg = load_config(time.strftime("%m%d%H%M%S"))
    env = dict(os.environ, ORCA_E2E_LIVE="1")
    if args.repro:
        args.playwright_args = [*REPRO_SPECS[args.repro], *args.playwright_args]
        print(f"UI repro: {args.repro} (dev server '{env.get('ORCA_E2E_DEV_SERVER_NAME', cfg.dev_server_name)}', throwaway repo, no AI)")
    if args.real:
        env.pop("ORCA_E2E_AI_MODE", None)
        env["ORCA_E2E_READONLY_ANALYSIS"] = "1"
        if args.real in EXECUTING:
            env["ORCA_E2E_EXECUTE_AGENT"] = "1"
        args.playwright_args = [*REAL_SPECS[args.real], *args.playwright_args]
        print(f"AI mode: REAL Claude Code (dev server '{env.get('ORCA_E2E_DEV_SERVER_NAME', cfg.dev_server_name)}'); specs: {', '.join(REAL_SPECS[args.real])}")
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
        code = run_playwright(env, args.playwright_args)
        if args.real:
            code = code or check_no_orphans()
        return code
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
