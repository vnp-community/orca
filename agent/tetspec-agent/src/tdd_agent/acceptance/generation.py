"""Acceptance of TDD generation with the real provider (task 9.3).

Three inputs: PBOM (curated oracle), free prose (no tables) and a second synthetic
project.  Grading is deterministic and uses only approved oracles; prose quality is
out of scope here (D5 L2).
"""
from __future__ import annotations

import json
from pathlib import Path

from ..config import Config
from ..core import o5_path
from ..store.base import SourceDoc
from . import oracles
from .common import DATA, Check, Timer, fresh_runtime, pbom_sources, provider_facts, repo_root, usage_summary, write_json

INPUTS = {
    "pbom": {"oracle": "pbom-generation", "tdd_id": "TDD-PBOM", "synthetic": False, "prose_limit": 40},
    "prose": {"oracle": "prose-generation", "tdd_id": "TDD-PROSE", "synthetic": True, "prose_limit": 100},
    "second": {"oracle": "second-generation", "tdd_id": "TDD-PAY", "synthetic": True, "prose_limit": 100},
}


def sources_for(name: str) -> list[SourceDoc]:
    if name == "pbom":
        return pbom_sources(repo_root())
    if name == "prose":
        return [SourceDoc.make("URD", "1", "URD", "# Notification service - user requirements\n| ID | Statement |\n|---|---|\n| REQ-1 | Owners must be told before an entitlement expires. |\n"),
                SourceDoc.make("SRS", "1", "SRS", (DATA / "prose-spec.md").read_text(encoding="utf-8")),
                SourceDoc.make("SAD", "1", "SAD", "# Notification service - architecture\n| ID | Decision | Rationale | Alternatives | Tradeoffs |\n|---|---|---|---|---|\n| ADR-1 | Daily batch job | simple scheduling | event driven | latency |\n")]
    if name == "second":
        return [SourceDoc.make(r, "1", r, (DATA / "second" / f"{r}.md").read_text(encoding="utf-8")) for r in ("URD", "SRS", "SAD")]
    raise KeyError(name)


def run_one(base: Config, out: Path, name: str, repetition: int, approval: dict | None) -> dict:
    spec = INPUTS[name]
    rt = fresh_runtime(base, out, f"generation-{name}-r{repetition}")
    try:
        docs = sources_for(name)
        request = {"mode": "initial", "params": {"tdd_id": spec["tdd_id"], "prose": True, "prose_limit": spec["prose_limit"]}}
        with Timer() as t:
            rt.orchestrator.start("tdd-generation", request, run_id=f"gen-{name}-{repetition}", sources=docs)
            run = rt.orchestrator.advance(f"gen-{name}-{repetition}")
        task = rt.operational.get_task(run.run_id, "G3") or {}
        output = task.get("output", {})
        record = {"input": name, "repetition": repetition, "run_status": run.status, "blockers": run.result.get("blockers", []), "seconds": t.seconds, "synthetic": spec["synthetic"],
                  "usage": usage_summary(rt.operational, run.run_id), "provider": provider_facts(rt)}
        if run.status != "completed":
            record["graded"] = "blocked"
            record["checks"] = [Check("run-completed", False, f"run ended {run.status}: {run.result.get('blockers')}").as_dict()]
            return record
        graph = rt.knowledge.materialize()
        oracle = oracles.load(spec["oracle"])
        record["kg"] = rt.knowledge.stats()
        record["readiness"] = {"status": output.get("status"), "blockers": output.get("blockers")}
        record["gaps"] = len(graph.get("gaps", []))
        record["questions"] = len(output.get("questions", []))
        record["roundtrip"] = output.get("roundtrip")
        record["tdd_document"] = output.get("artifact")
        if approval is None:
            record["graded"] = "not_graded"
            record["checks"] = []
            record["reason"] = f"oracle {spec['oracle']} has no approval from an allowlisted actor"
        else:
            checks = oracles.evaluate_generation(oracle, graph, output, docs)
            record["graded"] = "pass" if all(c.passed for c in checks) else "fail"
            record["checks"] = [c.as_dict() for c in checks]
            record["oracle_approval"] = {k: approval[k] for k in ("actor", "kind", "role", "grade", "oracle_hash")}
        return record
    finally:
        rt.close()


def run_generation_acceptance(base: Config, out: Path, inputs: list[str], repetitions: int = 1) -> dict:
    o5_path.o5_root()
    report = {"task": "9.3", "inputs": {}}
    for name in inputs:
        approval = oracles.approval_for(out, INPUTS[name]["oracle"])
        runs = [run_one(base, out, name, r, approval) for r in range(1, repetitions + 1)]
        report["inputs"][name] = runs
        write_json(out / f"generation-{name}.json", runs)
    existing = out / "existing-tools.json"
    if existing.is_file() and "pbom" in report["inputs"] and report["inputs"]["pbom"]:
        from .existing_tools import compare_with_runtime
        report["comparison_with_existing_tools"] = compare_with_runtime(json.loads(existing.read_text()), report["inputs"]["pbom"][0])
    report["limits"] = {"synthetic_inputs": sorted(n for n in report["inputs"] if INPUTS[n]["synthetic"]), "prose_quality": "not graded here (D5 L2 requires an independent reviewer)",
                        "oracle_grades": {n: (oracles.approval_for(out, INPUTS[n]["oracle"]) or {}).get("grade", "unapproved") for n in report["inputs"]}}
    write_json(out / "acceptance-report-tdd.json", report)
    return report
