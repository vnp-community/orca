"""Acceptance harness (phases 9 and 10): real-provider TDD generation, the 10-case table and isolated agents."""
from __future__ import annotations

import json
import sys
from pathlib import Path

from ..errors import TddAgentError

EXIT_OK, EXIT_BLOCKED = 0, 1


def _print(value) -> None:
    json.dump(value, sys.stdout, indent=2, ensure_ascii=False, default=str)
    sys.stdout.write("\n")


def cli_entry(rt, args) -> int:
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    if args.action == "existing-tools":
        from .existing_tools import run_existing_tools
        report = run_existing_tools(out)
        _print({"tools": {k: {"status": v["status"], "summary": v.get("summary"), "seconds": v["seconds"]} for k, v in report["tools"].items()}})
        return EXIT_OK if all(v["status"] == "pass" for v in report["tools"].values()) else EXIT_BLOCKED
    if args.action == "providers":
        from .providers import run_providers
        report = run_providers(rt.config, out, args.agents, tuple(args.tiers))
        _print({a: {"passed": f"{r['passed']}/{r['examples']}", "seconds": r["seconds"], "calls": r["usage"]["calls"], "tokens": r["usage"]["tokens"], "models": r["usage"]["models"],
                    "failed": [f["skill"] + ": " + f["example"] for f in r["failed"]]} for a, r in report["agents"].items()})
        return EXIT_OK if all(not r["failed"] for r in report["agents"].values()) else EXIT_BLOCKED
    if args.action == "search":
        from .search import run_search_acceptance
        report = run_search_acceptance(rt.config, out)
        _print({"graded": report["graded"], "oracle_approval": report["oracle_approval"] and {k: report["oracle_approval"][k] for k in ("actor", "grade")},
                "variants": {k: {"index": v["index"], "scan": v["scan"]} for k, v in report["variants"].items()}})
        return EXIT_OK if report["graded"] else EXIT_BLOCKED
    if args.action == "approve-oracle":
        from . import oracles
        rec = oracles.approve(rt, out, args.oracle, args.actor, args.role, args.rationale)
        _print(rec)
        return EXIT_OK
    if args.action == "case-table":
        from .cases import run_case_table
        report = run_case_table(out)
        _print(report["summary"])
        return EXIT_OK if report["summary"]["fail"] == 0 else EXIT_BLOCKED
    if args.action == "generation":
        from .generation import run_generation_acceptance
        report = run_generation_acceptance(rt.config, out, args.inputs, args.repetitions or rt.config.get('acceptance.repetitions', 1))
        _print({name: [{"graded": r["graded"], "status": r["run_status"], "seconds": r["seconds"], "calls": r["usage"]["calls"]} for r in runs] for name, runs in report["inputs"].items()})
        return EXIT_OK if all(r["graded"] in ("pass", "not_graded") for runs in report["inputs"].values() for r in runs) else EXIT_BLOCKED
    if args.action == "isolated":
        from .isolated import run_isolated
        reps = args.repetitions or rt.config.get('acceptance.repetitions', 1)
        report = run_isolated(rt.config, out, args.agents, reps)
        _print({a: ({k: v["stability"] for k, v in body.items()} if a != "changespec" else body["stability"]) for a, body in report["agents"].items() if a != "generation"} |
               ({"generation": {k: v["stability"] for k, v in report["agents"]["generation"].items()}} if "generation" in report["agents"] else {}))
        return EXIT_OK
    if args.action == "real-cases-report":
        from .real_report import build_real_cases_report
        _print({"report": str(build_real_cases_report(out))})
        return EXIT_OK
    if args.action == "real-cases":
        from .real_cases import run_real_cases
        rep = run_real_cases(rt.config, out, args.only)
        _print({cid: {"verdict": r["verdict"], "safety_failed": r.get("safety_failed"), "design_unmet": r.get("design_unmet")} for cid, r in rep["results"].items()})
        return EXIT_OK
    if args.action == "e2e-second":
        from .e2e import run_second_to_publication
        rec = run_second_to_publication(rt.config, out)
        _print({"graded": rec["graded"], "final": rec["final"], "seconds": rec["seconds"], "cost_usd": rec["usage"]["cost_usd"]})
        return EXIT_OK if rec["graded"] == "pass" else EXIT_BLOCKED
    if args.action == "e2e-pbom":
        from .e2e import run_pbom_change_request
        rec = run_pbom_change_request(rt.config, out, allow_draft=bool(getattr(args, "allow_draft", False)))
        _print({k: rec[k] for k in ("final_status", "final_state", "final_blockers", "gates", "impact_actions", "change_spec_rows", "published", "seconds") if k in rec})
        return EXIT_OK if rec.get("published") else EXIT_BLOCKED
    if args.action == "report":
        from .report import build_report
        path = build_report(out)
        _print({"report": str(path)})
        return EXIT_OK
    raise TddAgentError(f"unknown acceptance action {args.action}", code="USAGE")
