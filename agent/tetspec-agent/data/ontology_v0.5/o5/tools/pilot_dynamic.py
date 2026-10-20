#!/usr/bin/env python3
"""Run a small deterministic dynamic-source pilot and report measurable gates."""
from __future__ import annotations
import json, sys, time
from pathlib import Path
HERE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(HERE / "harness")); sys.path.insert(0, str(HERE / "tools"))
from dynamic import TddGenerationRequest, run_generation

def pilot() -> dict:
    request = TddGenerationRequest.from_dict({"id": "pilot-initial", "mode": "initial", "scope": ["upload"], "sources": [{"id": "srs", "kind": "SRS", "version": "1", "facts": {"purpose": "upload", "scope": "api", "requirements": ["R1"], "contracts": ["C1"], "data": ["D1"], "behavior": ["B1"], "decisions": ["ADR1"], "quality": ["Q1"], "work": ["W1"], "validation": ["T1"], "provenance": ["SRS@1"]}}]})
    needs = [{"id": "N-" + k, "slot": k} for k in ("purpose", "scope", "requirements", "contracts", "data", "behavior", "decisions", "quality", "work", "validation", "provenance")]
    started = time.perf_counter(); result = run_generation(request, {"version": "pilot", "elements": [], "edges": []}, needs); elapsed_ms = round((time.perf_counter() - started) * 1000, 3)
    intake = result["intake"]; return {"request_id": request.id, "status": result["status"], "elapsed_ms": elapsed_ms, "source_count": len(request.sources), "need_count": len(needs), "found_needs": len(needs) - len(intake["blockers"]), "blockers": intake["blockers"], "estimated_input_tokens": max(1, len(json.dumps(request.sources, ensure_ascii=False)) // 4), "snapshot": result["snapshot"]}

if __name__ == "__main__":
    print(json.dumps(pilot(), indent=2, ensure_ascii=False))
