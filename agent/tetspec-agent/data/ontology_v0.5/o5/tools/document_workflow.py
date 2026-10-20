#!/usr/bin/env python3
"""CLI for the settled document workflow.

Examples::

  document_workflow.py impact input.json
  document_workflow.py change-spec input.json

``input.json`` contains ``srs_delta``, ``tdd_inventory`` and ``sad``.  The
change-spec command additionally needs an approved ``impact`` and a
materialized ``tdd_set``.  The CLI only emits diagnostics/table JSON; it never
silently approves a report or mutates a TDD.
"""
from __future__ import annotations

import argparse
from dataclasses import asdict
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))
from workflow import assess_tdd_impact, approve_impact, build_change_spec, ImpactEntry, ImpactReport


def _impact(value):
    if isinstance(value, ImpactReport):
        return value
    return ImpactReport(
        id=value["id"], cr_ref=value["cr_ref"], srs_ref=value["srs_ref"], sad_ref=value["sad_ref"],
        baseline_tdd_snapshot=value["baseline_tdd_snapshot"],
        entries=[ImpactEntry(**x) for x in value.get("entries", [])], findings=value.get("findings", []),
        status=value.get("status", "pending_approval"), approval=value.get("approval"),
    )


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("impact", "approve", "change-spec"))
    parser.add_argument("input", type=Path)
    parser.add_argument("--approver")
    parser.add_argument("--role", default="SA/Dev")
    args = parser.parse_args(argv)
    payload = json.loads(args.input.read_text(encoding="utf-8"))
    if args.command == "impact":
        report = assess_tdd_impact(payload["srs_delta"], payload.get("tdd_inventory", []), payload["sad"], report_id=payload.get("impact_id", "impact-1"))
        result = asdict(report)
    elif args.command == "approve":
        report = approve_impact(_impact(payload["impact"]), args.approver or "", args.role)
        result = asdict(report)
    else:
        result = build_change_spec(payload["srs_delta"], payload["tdd_set"], payload["sad"], _impact(payload["impact"]), payload.get("cr"))
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result.get("status") in {"approved", "eligible", "pending_approval"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
