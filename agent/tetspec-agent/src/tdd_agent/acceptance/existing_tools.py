"""Run the pre-existing o5 quality tools (task 9.2) and compare their PBOM result with the runtime's."""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

from .common import Timer, repo_root, write_json


def _run(cmd: list[str], cwd: Path, timeout: int = 1800) -> dict:
    with Timer() as t:
        done = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=timeout)
    return {"command": " ".join(cmd[1:]), "returncode": done.returncode, "seconds": t.seconds, "stdout_tail": done.stdout[-1500:], "stderr_tail": done.stderr[-600:]}


def run_existing_tools(out: Path) -> dict:
    root = repo_root()
    o5 = root / "data" / "ontology_v0.5" / "o5"
    py = sys.executable
    report: dict = {"task": "9.2", "tools": {}}
    pilot_dir = out / "o5-quality-pilot"
    report["tools"]["run_tests"] = _run([py, str(o5 / "tools" / "run_tests.py")], o5)
    tail = report["tools"]["run_tests"]["stdout_tail"].strip().splitlines()[-1:] or [""]
    report["tools"]["run_tests"]["summary"] = tail[0]
    report["tools"]["run_quality_pilot"] = _run([py, str(o5 / "tools" / "run_quality_pilot.py"), "--run-dir", str(pilot_dir)], root)
    summary_line = next((l for l in report["tools"]["run_quality_pilot"]["stdout_tail"].splitlines() if l.startswith("{")), "{}")
    report["tools"]["run_quality_pilot"]["summary"] = json.loads(summary_line)
    fixture_out = out / "o5-settled-workflow.json"
    report["tools"]["run_settled_workflow_fixture"] = _run([py, str(o5 / "tools" / "run_settled_workflow_fixture.py"), "--output", str(fixture_out)], root)
    if fixture_out.is_file():
        data = json.loads(fixture_out.read_text())
        report["tools"]["run_settled_workflow_fixture"]["summary"] = {k: data.get(k) for k in ("fixture", "status") if k in data}
    for name, tool in report["tools"].items():
        tool["status"] = "pass" if tool["returncode"] == 0 else "fail"
    report["reference_run"] = "data/test-pbom/run-quality-verified (o5 pilot, historical)"
    write_json(out / "existing-tools.json", report)
    return report


def compare_with_runtime(existing: dict, generation_pbom: dict) -> dict:
    """Class-by-class counts: o5 pilot vs the runtime's PBOM generation (informational; the rule sets differ on purpose)."""
    o5 = existing["tools"]["run_quality_pilot"]["summary"]
    kg = generation_pbom.get("kg", {})
    return {"o5_pilot": {"nodes": o5.get("nodes"), "edges": o5.get("edges"), "oracle": o5.get("oracle"), "round_trip": (o5.get("round_trip") or {}).get("ok"), "readiness": o5.get("readiness")},
            "runtime": {"nodes": kg.get("elements"), "edges": kg.get("edges"), "by_class": kg.get("by_class"), "readiness": (generation_pbom.get("readiness") or {}).get("status")},
            "note": "The runtime keeps undefined API/algorithm references as explicit gaps instead of stub elements, so node counts differ by design; both are `draft`/`blocked` for the same reason (Appendix A/B/C missing)."}
