#!/usr/bin/env python3
"""Runs the live Request-flow UI spec (Playwright, ui-live/) against the local stack.

  python3 run_ui_flow.py                 # the dev server's own Claude (real AI, slower, spends usage)
  python3 run_ui_flow.py --ai-stub       # deterministic fake claude: swaps the agent, runs, restores it (like run_acceptance.py --ai-stub)
  python3 run_ui_flow.py --ai-stub -- --headed   # anything after `--` goes to `playwright test`
  python3 run_ui_flow.py --ai-stub -- request-base-branch  # starting branch of a Request on a LedgerLite repo (CR-REQ-040): Create dialog, Overview row, edit, lock, unknown branch, feature off
  python3 run_ui_flow.py --ai-stub -- analysis-revision   # revising an analysis by prompt (CR-REQ-041): versions, read-only old ones, Activity line, refusals; bug, spike, question once each
  python3 run_ui_flow.py --ai-stub -- approve-run-all     # "Approve and run all steps" (CR-REQ-043): the choice and its confirmation, what it pins, the Execution tab label, the Project Settings switch, a gateway without approval.runAll
  python3 run_ui_flow.py --ai-stub -- auto-plan-generation  # the Plan written by Orca after the Analysis is approved (CR-REQ-044): no Generate button, failed run kept on screen with Retry, transient failure retried, project switch off, a gateway without request.planAuto (the dev server must run on this machine: the spec makes the fake claude fail through its state dir)
  python3 run_ui_flow.py --ai-stub -- plan-documents      # reading the plan the AI wrote (CR-REQ-045): Proposal of a native plan, Documents of an OpenSpec change (rows planted by plan_documents_seed.py), no Documents entry on the native engine; the real OpenSpec case (7 to 13 USD) is skipped unless ORCA_E2E_OPENSPEC=1
  python3 run_ui_flow.py --ai-stub -- plan-revision       # revising a plan by prompt (CR-REQ-045): three versions, read-only old ones, one Approval at a time, the History line, too long, an out-of-date screen, a failed round and Try again
  python3 run_ui_flow.py --real plan-revision # REAL Claude Code, one revision round of a native `bug` plan on a throwaway repo (a small cost)
  python3 run_ui_flow.py --real readonly # REAL Claude Code, read-only analysis on a throwaway repo (question, bug)
  python3 run_ui_flow.py --real execute  # REAL Claude Code approves and EXECUTES a tiny Plan on a throwaway repo
  python3 run_ui_flow.py --real native-execute # REAL Claude Code executes a native `bug` and a native `task` (QA 2026-10-16: engine, tabs, agent log shows no CLI diagnostic as an error)
  python3 run_ui_flow.py --real continuous # REAL Claude Code runs a two-step Plan that is approved ONCE with "Approve and run all steps" (project stays `step_by_step`); the Execution tab follows it (CR-REQ-038 / FE-REQ-037 / CR-REQ-043)
  python3 run_ui_flow.py --real monitor  # REAL Claude Code executes a one-task Plan while the spec watches the agent through the UI (TASK-FE-E2E-011)
  python3 run_ui_flow.py --ai-stub -- analysis-output-language  # the Vietnamese Solution shown whole on the Analysis tab under English labels (CR-REQ-046)
  python3 run_ui_flow.py --real language  # REAL Claude Code, one read-only question asked in English and answered in Vietnamese: valid JSON, not cut, usage printed (CR-REQ-046, a small cost)
  python3 run_ui_flow.py --real addproject # REAL Claude Code, read-only: Add Project makes its own project per repo, a duplicate folder is refused, a Request in that project is answered from the right repo (TASK-FE-E2E-027)
  python3 run_ui_flow.py --real all      # all four; never combined with --ai-stub
  python3 run_ui_flow.py --real deepseek # REAL Claude Code working on the USER's repo deepseek-harness through the UI (TASK-FE-E2E-028): Add Project, Requests, read-only analysis, review controls, execution with Phases, live agent session; `git push` is blocked for the whole run
  python3 run_ui_flow.py --real deepseek-ui|deepseek-readonly|deepseek-review|deepseek-execute|deepseek-task   # one part of it
  python3 run_ui_flow.py --real ci-cd-kit # REAL Claude Code, COST CLASS C (an OpenSpec plan plus real task runs, an order of 10 to 40 USD, up to 2 hours): a change_request taken from backlog BL-E07 of the USER's project vnp-qa-platform, started from branch claude/tms-bug-triage-report-09f310, driven from the UI through "Approve and run all steps" to the end; needs the owner's explicit consent each time; not part of `all`; the new branches and worktrees are KEPT for inspection
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

from agent_push_block import PushBlockedAgent
from ai_stub_agent_switch import StubAgentError, StubbedAgent
from orca_stack_config import REPO_ROOT, load_config

PLAYWRIGHT_CONFIG = "tests/configs/playwright.web.config.ts"
REAL_SPECS = {
    # Why: Playwright reads these as regexes on the file path; a bare "readonly-real" also matches deepseek-readonly-real.
    "readonly": ["/readonly-real"],
    "execute": ["/execute-real"],
    "continuous": ["continuous-execute-real"],
    "native-execute": ["qa-native-execute-real"],
    "monitor": ["agent-session-monitoring-real"],
    # Why: the file also holds fake-claude tests; they skip themselves when the AI mode is not the stub.
    "plan-revision": ["plan-revision.web"],
    "addproject": ["add-project-own-project-request-real"],
    # CR-REQ-046: one read-only question answered in Vietnamese by the real model (a small cost); not part of "all".
    "language": ["analysis-output-language-real"],
    "all": ["/readonly-real", "/execute-real", "agent-session-monitoring-real", "add-project-own-project-request-real"],
    # The user's repo deepseek-harness (ORCA_E2E_DEEPSEEK_REPO overrides the path); the agent runs with `git push` blocked.
    "deepseek-ui": ["deepseek-ui-real"],
    "deepseek-readonly": ["deepseek-readonly-real"],
    "deepseek-review": ["deepseek-review-real", "deepseek-hotfix-real"],
    "deepseek-execute": ["deepseek-execute-real"],
    "deepseek-task": ["deepseek-task-real"],
    "deepseek": ["deepseek-ui-real", "deepseek-readonly-real", "deepseek-review-real", "deepseek-hotfix-real", "deepseek-execute-real", "deepseek-task-real"],
    # Cost class C, never part of "all": the user's project vnp-qa-platform with an OpenSpec plan and real task runs (see the README).
    "ci-cd-kit": ["ci-cd-kit-real"],
    # Cost class C (one real context run): the Outcome Review of the Request that `ci-cd-kit` finished; needs ORCA_E2E_CICDKIT_REQUEST_ID.
    "ci-cd-kit-review": ["ci-cd-kit-review-real"],
    # No AI, reads only: the drawings of the Outcome Review of a finished Request (ORCA_E2E_REVIEW_TITLE).
    "review-diagrams": ["outcome-review-diagrams"],
}
# Why: the task and proposal worktrees of this repo sit next to it (vnp-qa-platform-task-<id>), outside the orca checkout the agent normally uses as its work dir,
# and the relay refuses writes outside AGENT_WORK_DIR; the agent runs with the parent folder for this run only.
AGENT_WORK_DIRS = {"ci-cd-kit": "/Users/datbeohbbh/Desktop/work/vnpay/agentic-app", "ci-cd-kit-review": "/Users/datbeohbbh/Desktop/work/vnpay/agentic-app"}
# Why: the whole spec, quiet-tree wait and cleanup excluded, must end within 2 hours (the owner's stop-loss); the harness reads it as the hard limit.
REAL_TIMEOUT_MIN = {"ci-cd-kit": "110", "ci-cd-kit-review": "60"}
# Why: people edit specs/ and docs/ of this checkout while a two-hour run goes on, and the exact comparison would call that a leak (README, "Bảo vệ checkout"); nothing else is ignored.
GUARD_IGNORE = {"ci-cd-kit": "specs/:docs/", "ci-cd-kit-review": "specs/:docs/"}
# Why: no AI and no Request: UI-only regression specs of reported bugs; each prints BUG REPRODUCED and fails if the bug comes back.
REPRO_SPECS = {"add-project-worktree": ["add-project-worktree-repro"]}
# Why: these approve a Plan, which runs a real agent that WRITES to the throwaway repo; read-only analysis does not need the flag
# (addproject only reads: it never approves the analysis).
EXECUTING = ("execute", "continuous", "native-execute", "monitor", "all", "deepseek-execute", "deepseek-task", "deepseek", "ci-cd-kit", "ci-cd-kit-review")


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
        if args.real in REAL_TIMEOUT_MIN:
            env.setdefault("ORCA_E2E_REAL_TIMEOUT_MIN", REAL_TIMEOUT_MIN[args.real])
        if args.real in GUARD_IGNORE:
            env.setdefault("ORCA_E2E_GUARD_IGNORE", GUARD_IGNORE[args.real])
        args.playwright_args = [*REAL_SPECS[args.real], *args.playwright_args]
        print(f"AI mode: REAL Claude Code (dev server '{env.get('ORCA_E2E_DEV_SERVER_NAME', cfg.dev_server_name)}'); specs: {', '.join(REAL_SPECS[args.real])}")
    stub: StubbedAgent | None = None
    blocked: PushBlockedAgent | None = None
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
            # Why: auto-plan-generation makes the fake claude fail by writing a marker file there; only valid when the agent runs on this machine.
            env["ORCA_E2E_FAKE_CLAUDE_STATE_DIR"] = str(stub.state_dir)
            print(f"AI mode: STUB (wiring only, not AI quality); dev server {stub.current_cfg.dev_server_name}")
        if args.real and (args.real.startswith("deepseek") or args.real in AGENT_WORK_DIRS):
            try:
                blocked = PushBlockedAgent(cfg, AGENT_WORK_DIRS.get(args.real))
                print("restarting the agent with `git push` blocked (the repo has a real GitHub remote) ...")
                blocked.start()
            except StubAgentError as exc:
                print(f"cannot start the agent with the push block: {exc}", file=sys.stderr)
                return 2
        code = run_playwright(env, args.playwright_args)
        if args.real:
            code = code or check_no_orphans()
        return code
    finally:
        if blocked:
            try:
                print(blocked.restore())
            except StubAgentError as exc:
                restore_failed = True
                print(f"WARNING: the agent could not be restored: {exc}\n         run: bash deploy/dev/scripts/start-local-agent.sh", file=sys.stderr)
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
