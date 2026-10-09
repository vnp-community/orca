#!/usr/bin/env python3
"""Runs the ORCA acceptance e2e suites against a live local stack.

  python3 run_acceptance.py                      # every suite
  python3 run_acceptance.py request workflow     # selected suites
  python3 run_acceptance.py --list
  python3 run_acceptance.py --strict             # BLOCKED counts as failure (use for the final sign-off)

Exit code: 0 = no FAIL (and no BLOCKED with --strict), 1 otherwise.
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

import suite_dev_server_agent
import suite_not_found_contract
import suite_platform_health
import suite_request_ai_stages
import suite_request_exits
import suite_request_lifecycle
import suite_request_plan_stage
import suite_request_type_matrix
import suite_task_and_mcp
import suite_workflow_dag
from acceptance_context import AcceptanceContext
from acceptance_recorder import AcceptanceRecorder
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
    ("task", "Task CRUD, agent execution, MCP", suite_task_and_mcp),
    ("notfound", "Unknown ids are NotFound on every service", suite_not_found_contract),
]
REPORT_DIR = Path(__file__).resolve().parent / "reports"


def markdown(outcomes: list[dict], counts: dict, cfg_prefix: str, seconds: float) -> str:
    lines = [f"# ORCA e2e run `{cfg_prefix}`", "", f"{counts['PASS']} pass, {counts['FAIL']} fail, {counts['BLOCKED']} blocked, {counts['SKIP']} skipped, in {seconds:.0f}s.", "",
             "| ID | Result | Test | Detail |", "|---|---|---|---|"]
    for o in outcomes:
        detail = o["detail"].replace("\n", " ").replace("|", "\\|")[:200]
        lines.append(f"| {o['test_id']} | {o['status']} | {o['title']} | {detail} |")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("suites", nargs="*", help="suite names (default: all)")
    parser.add_argument("--list", action="store_true")
    parser.add_argument("--strict", action="store_true", help="treat BLOCKED as failure")
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

    wanted = set(args.suites) or {s[0] for s in SUITES}
    if wanted - {"platform", "devserver"}:
        wanted.add("devserver")  # every other suite binds to the project and dev server this one prepares
    chosen = [s for s in SUITES if s[0] in wanted]

    run_id = time.strftime("%m%d%H%M%S")
    cfg = load_config(run_id)
    rec = AcceptanceRecorder()
    session = OrcaSession(cfg)
    ctx = AcceptanceContext(cfg=cfg, session=session, rec=rec)
    started = time.time()
    print(f"ORCA acceptance run {cfg.run_prefix}  gateway={cfg.gateway_url}  frontend={cfg.frontend_url}\n")

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

    seconds = time.time() - started
    counts = rec.counts()
    print(f"\n=== {counts['PASS']} pass, {counts['FAIL']} fail, {counts['BLOCKED']} blocked, {counts['SKIP']} skip  ({seconds:.0f}s)")
    print(f"    write-rate throttling: {throttled} calls were retried after REQUEST_RATE_LIMITED")
    if not args.no_report:
        REPORT_DIR.mkdir(exist_ok=True)
        (REPORT_DIR / "last-run.json").write_text(json.dumps({"run": cfg.run_prefix, "counts": counts, "seconds": round(seconds), "throttled_retries": throttled, "results": rec.as_dicts()}, indent=1))
        (REPORT_DIR / "last-run.md").write_text(markdown(rec.as_dicts(), counts, cfg.run_prefix, seconds))
    failed = counts["FAIL"] or (args.strict and counts["BLOCKED"])
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
