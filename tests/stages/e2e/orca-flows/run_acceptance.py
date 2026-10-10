#!/usr/bin/env python3
"""Runs the ORCA acceptance e2e suites against a live local stack.

  python3 run_acceptance.py                      # every suite
  python3 run_acceptance.py request workflow     # selected suites
  python3 run_acceptance.py --list
  python3 run_acceptance.py --strict             # BLOCKED counts as failure (use for the final sign-off)
  python3 run_acceptance.py --ai-stub            # AI answers from the deterministic fake claude (CI: no Claude account)
  python3 run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02

--ai-stub swaps the dev-server agent for one that finds fake_claude_cli/claude first, runs the suites, then restarts the
agent the way it was before (see ai_stub_agent_switch.py). The report says "stub": it proves wiring, not AI quality.

Exit code: 0 = no FAIL (and no unexpected BLOCKED with --strict), 1 otherwise, 2 bad usage, 3 the agent could not be restored.
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

import suite_ai_stub_login
import suite_dev_server_agent
import suite_not_found_contract
import suite_platform_health
import suite_request_ai_stages
import suite_request_exits
import suite_request_lifecycle
import suite_request_plan_stage
import suite_request_readonly_analysis
import suite_request_type_matrix
import suite_task_and_mcp
import suite_workflow_dag
from acceptance_context import AcceptanceContext
from acceptance_phase_merge import count, merge_outcomes
from acceptance_recorder import AcceptanceRecorder
from ai_stub_agent_switch import StubAgentError, StubbedAgent, describe_stub_for_report
from orca_session import OrcaSession
from orca_stack_config import load_config

# Order matters: later suites reuse the project and the AI-availability probe produced by earlier ones.
SUITES = [
    ("platform", "Stack health, UI, auth", suite_platform_health),
    ("devserver", "Dev server agent and project binding", suite_dev_server_agent),
    ("workflow", "Workflow DAG engine", suite_workflow_dag),
    ("request", "Request intake, classification, clarification", suite_request_lifecycle),
    ("matrix", "11 request types", suite_request_type_matrix),
    ("exits", "Backlog, reopen, cancel, change type", suite_request_exits),
    ("ai", "AI-dependent stages", suite_request_ai_stages),
    ("plan", "Plan stage (propose, commit, approve)", suite_request_plan_stage),
    ("analysis", "Read-only analyses: diagnosis, findings, answer", suite_request_readonly_analysis),
    ("task", "Task CRUD, agent execution, MCP", suite_task_and_mcp),
    ("notfound", "Unknown ids are NotFound on every service", suite_not_found_contract),
    ("aistub", "Fake claude wiring and the logged-out path (--ai-stub only)", suite_ai_stub_login),
]
STUB_ONLY = {"aistub"}
AI_USING = {"request", "matrix", "exits", "ai", "plan", "analysis"}  # suites whose requests go through classification
REPORT_DIR = Path(__file__).resolve().parent / "reports"


def markdown(outcomes: list[dict], counts: dict, cfg_prefix: str, seconds: float, ai: dict) -> str:
    lines = [f"# ORCA e2e run `{cfg_prefix}`", "", f"{counts['PASS']} pass, {counts['FAIL']} fail, {counts['BLOCKED']} blocked, {counts['SKIP']} skipped, in {seconds:.0f}s.", ""]
    if ai["mode"] == "stub":
        lines += [f"> **AI mode: STUB.** {ai['note']}", ""]
    lines += ["| ID | Result | Test | Detail |", "|---|---|---|---|"]
    for o in outcomes:
        detail = o["detail"].replace("\n", " ").replace("|", "\\|")[:200]
        lines.append(f"| {o['test_id']} | {o['status']} | {o['title']} | {detail} |")
    return "\n".join(lines) + "\n"


def run_phase(cfg, chosen: list, state: dict) -> tuple[list[dict], int]:
    """Runs suites once with fresh state and a fresh login; returns the outcomes and how many writes were rate-limit retried."""
    rec = AcceptanceRecorder()
    session = OrcaSession(cfg)
    ctx = AcceptanceContext(cfg=cfg, session=session, rec=rec)
    ctx.state.update(state)
    try:
        for name, desc, module in chosen:
            rec.current_suite = name
            print(f"--- {name}: {desc}")
            if name != "platform" and not session.user:
                if session.login().status != 200:
                    print("    cannot log in; run the 'platform' suite first")
                    continue
            module.run(ctx)
    finally:
        problems = ctx.run_cleanups()
        throttled = session._ws.rate_limit_retries if session._ws else 0
        session.close()
        for p in problems:
            print(f"[cleanup] {p}")
    return rec.as_dicts(), throttled


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("suites", nargs="*", help="suite names (default: all)")
    parser.add_argument("--list", action="store_true")
    parser.add_argument("--strict", action="store_true", help="treat BLOCKED as failure")
    parser.add_argument("--allow-blocked", default="", metavar="IDS", help="with --strict: comma-separated test id prefixes that may stay BLOCKED (e.g. MCP-01,MCP-02)")
    parser.add_argument("--ai-stub", action="store_true", help="answer AI calls with the deterministic fake claude (restarts the dev-server agent, then restores it)")
    parser.add_argument("--keep-stub-agent", action="store_true", help="with --ai-stub: leave the agent running against the stub (CI throwaway agents)")
    parser.add_argument("--no-report", action="store_true", help="do not write reports/last-run.{json,md}")
    args = parser.parse_args()

    if args.list:
        for name, desc, _ in SUITES:
            print(f"{name:10s} {desc}")
        return 0
    unknown = set(args.suites) - {s[0] for s in SUITES}
    if unknown:
        print(f"unknown suite(s): {', '.join(sorted(unknown))}", file=sys.stderr)
        return 2
    if (set(args.suites) & STUB_ONLY or args.keep_stub_agent) and not args.ai_stub:
        print("the 'aistub' suite and --keep-stub-agent need --ai-stub", file=sys.stderr)
        return 2

    wanted = set(args.suites) or {s[0] for s in SUITES if s[0] not in STUB_ONLY}
    if wanted - {"platform", "devserver"}:
        wanted.add("devserver")  # every other suite binds to the project and dev server this one prepares
    if args.ai_stub and wanted & AI_USING:
        wanted.add("aistub")
    chosen = [s for s in SUITES if s[0] in wanted]
    run_id = time.strftime("%m%d%H%M%S")
    cfg = load_config(run_id)
    started = time.time()
    print(f"ORCA acceptance run {cfg.run_prefix}  gateway={cfg.gateway_url}  frontend={cfg.frontend_url}" + ("  AI=STUB (fake claude)" if args.ai_stub else "") + "\n")

    ai_info = {"mode": "real", "note": "AI answers come from the dev server's own Claude login or API key."}
    stub: StubbedAgent | None = None
    restore_failed = False
    try:
        if args.ai_stub:
            try:
                stub = StubbedAgent(cfg)
                print("starting the agent with the fake claude (logged in) ...")
                stub.start(logged_in=True)
                print(f"    agent log: {stub.log}")
            except StubAgentError as exc:
                print(f"cannot start the stubbed agent: {exc}", file=sys.stderr)
                return 2
        outcomes, throttled = run_phase(stub.current_cfg if stub else cfg, chosen, {"ai_stub": stub, "ai_stub_phase": "logged_in"} if stub else {})
        if stub and any(name == "ai" for name, _, _ in chosen):
            # AIS-01/03 need an environment with no AI; the same agent is restarted with the stub logged out.
            print("\nrestarting the agent with the fake claude logged OUT (AIS-01, AIS-03, AST-04) ...")
            stub.start(logged_in=False)
            by_name = {s[0]: s for s in SUITES}
            second = [by_name[n] for n in ("devserver", "aistub", "ai")]  # aistub first: it records that no AI is available
            more, more_throttled = run_phase(stub.current_cfg, second, {"ai_stub": stub, "ai_stub_phase": "logged_out"})
            outcomes, throttled = merge_outcomes(outcomes, more), throttled + more_throttled
        if stub:
            ai_info = describe_stub_for_report(stub)
    except StubAgentError as exc:
        print(f"the stubbed agent failed: {exc}", file=sys.stderr)
        return 2
    finally:
        if stub and not args.keep_stub_agent:
            try:
                print("\n" + stub.restore())
            except StubAgentError as exc:
                restore_failed = True
                print(f"WARNING: the agent could not be restored without the stub: {exc}\n         run: bash deploy/dev/scripts/start-local-agent.sh", file=sys.stderr)

    seconds = time.time() - started
    counts = count(outcomes)
    allowed = tuple(x.strip() for x in args.allow_blocked.split(",") if x.strip())
    unexpected_blocked = [o["test_id"] for o in outcomes if o["status"] == "BLOCKED" and not o["test_id"].startswith(allowed)]
    print(f"\n=== {counts['PASS']} pass, {counts['FAIL']} fail, {counts['BLOCKED']} blocked, {counts['SKIP']} skip  ({seconds:.0f}s)" + ("   [AI: STUB, wiring only]" if ai_info["mode"] == "stub" else ""))
    print(f"    write-rate throttling: {throttled} calls were retried after REQUEST_RATE_LIMITED")
    if args.strict and counts["BLOCKED"]:
        print(f"    blocked: {', '.join(o['test_id'] for o in outcomes if o['status'] == 'BLOCKED')}" + (f"  (not allowed: {', '.join(unexpected_blocked) or 'none'})" if allowed else ""))
    if not args.no_report:
        REPORT_DIR.mkdir(exist_ok=True)
        (REPORT_DIR / "last-run.json").write_text(json.dumps({"run": cfg.run_prefix, "ai": ai_info, "counts": counts, "seconds": round(seconds),
                                                              "throttled_retries": throttled, "results": outcomes}, indent=1))
        (REPORT_DIR / "last-run.md").write_text(markdown(outcomes, counts, cfg.run_prefix, seconds, ai_info))
    if restore_failed:
        return 3
    failed = counts["FAIL"] or (args.strict and unexpected_blocked)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
