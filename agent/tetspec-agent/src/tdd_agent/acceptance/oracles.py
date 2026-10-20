"""Acceptance oracles: expected outcomes written before the run, graded by deterministic predicates.

An oracle is only usable once an allowlisted actor approved its exact content hash
(``acceptance approve-oracle``); an unapproved oracle yields ``not_graded``.  The
oracle for a run is never produced by the run being graded.
"""
from __future__ import annotations

import json
from hashlib import sha256
from pathlib import Path
from typing import Any

from ..core import o5_path
from ..errors import TddAgentError
from ..store.base import SourceDoc
from .common import ORACLES, Check


def load(name: str) -> dict:
    return json.loads((ORACLES / f"{name}.json").read_text(encoding="utf-8"))


def oracle_hash(oracle: dict) -> str:
    return sha256(json.dumps(oracle, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def approvals_path(out: Path) -> Path:
    return out / "oracle-approvals.json"


def approve(runtime, out: Path, name: str, actor: str, role: str, rationale: str) -> dict:
    """Record an allowlisted actor's approval of an oracle; the grade is ``human`` or ``agent`` per the allowlist kind."""
    cfg = runtime.config
    who = next((a for a in cfg.actors() if a.id == actor), None)
    if who is None or role not in who.roles or not rationale.strip():
        raise TddAgentError(f"{actor} may not approve oracles as {role}", code="ORACLE_APPROVAL_REJECTED")
    from ..obs import context
    if context.get("agent"):
        raise TddAgentError("pipeline agents cannot approve oracles", code="PIPELINE_AGENT_CANNOT_APPROVE")
    record = {"oracle": name, "oracle_hash": oracle_hash(load(name)), "actor": actor, "kind": who.kind, "role": role, "grade": "human" if who.kind == "human" else "agent", "rationale": rationale}
    existing = json.loads(approvals_path(out).read_text()) if approvals_path(out).is_file() else {}
    existing[name] = record
    out.mkdir(parents=True, exist_ok=True)
    approvals_path(out).write_text(json.dumps(existing, indent=2, ensure_ascii=False))
    return record


def approval_for(out: Path, name: str) -> dict | None:
    if not approvals_path(out).is_file():
        return None
    rec = json.loads(approvals_path(out).read_text()).get(name)
    return rec if rec and rec["oracle_hash"] == oracle_hash(load(name)) else None


def _subset(expected: Any, actual: Any) -> bool:
    if isinstance(expected, dict):
        return isinstance(actual, dict) and all(k in actual and _subset(v, actual[k]) for k, v in expected.items())
    if isinstance(expected, list):
        return isinstance(actual, list) and all(any(_subset(e, a) for a in actual) for e in expected)
    return expected == actual


def evaluate_generation(oracle: dict, graph: dict, output: dict, sources: list[SourceDoc]) -> list[Check]:
    """Deterministic grading of a TDD-generation result against a generation oracle."""
    o5 = o5_path.load()
    by_id = {e["id"]: e for e in graph["elements"]}
    checks: list[Check] = []
    src = [{"id": d.id, "version": d.revision, "content": d.text} for d in sources]
    for spec in oracle["checks"]:
        kind, cid = spec["kind"], spec["id"]
        if kind == "element":
            el = by_id.get(spec["element"])
            ok = bool(el) and el.get("class") == spec["class"] and _subset(spec.get("props", {}), el.get("props", {}))
            checks.append(Check(cid, ok, f"{spec['element']} expected {spec['class']}; got {el.get('class') if el else 'absent'}"))
        elif kind == "relation":
            ok = any(e["src"] == spec["src"] and e["rel"] == spec["rel"] and e["dst"] == spec["dst"] for e in graph["edges"])
            checks.append(Check(cid, ok, f"{spec['src']} -{spec['rel']}-> {spec['dst']}"))
        elif kind == "class_count_ge":
            n = sum(1 for e in graph["elements"] if e.get("class") == spec["class"])
            checks.append(Check(cid, n >= spec["n"], f"{spec['class']}: {n} (need >= {spec['n']})"))
        elif kind == "fact":
            hits = [e for e in graph["elements"] if (spec["class"] == "*" or e.get("class") == spec["class"]) and _subset(spec.get("props", {}), e.get("props", {})) and spec.get("quote_contains", "") in (e.get("provenance") or [{}])[0].get("quote", "")]
            checks.append(Check(cid, bool(hits), f"{spec['class']} with {spec.get('props')} anchored to a line containing {spec.get('quote_contains')!r}: {len(hits)} found"))
        elif kind == "gap":
            hits = [g for g in graph.get("gaps", []) if g.get("need") == spec["need"] and spec.get("target_prefix", "") in str(g.get("target", ""))]
            checks.append(Check(cid, len(hits) >= spec.get("n", 1), f"gaps need={spec['need']}: {len(hits)}"))
        elif kind == "not_ready":
            checks.append(Check(cid, output.get("status") != "implementation_ready", f"readiness status {output.get('status')}"))
        elif kind == "blocker":
            missing = [b for b in spec["blockers"] if b not in output.get("blockers", [])]
            checks.append(Check(cid, not missing, f"missing blockers: {missing}"))
        elif kind == "evidence_verified":
            bad = [e["id"] for e in graph["elements"] for ev in e.get("provenance", []) if ev.get("kind") == "extracted" and o5.verify_evidence(ev, src)]
            checks.append(Check(cid, not bad, f"{len(bad)} element(s) with unverifiable evidence {bad[:3]}"))
        elif kind == "no_invented_facts":
            bad = [e["id"] for e in graph["elements"] if e.get("origin") in ("upstream", "external") and not e.get("provenance")]
            checks.append(Check(cid, not bad, f"{len(bad)} sourced element(s) without provenance"))
        elif kind == "roundtrip":
            checks.append(Check(cid, bool(output.get("roundtrip", {}).get("ok")), json.dumps(output.get("roundtrip", {}))[:200]))
        elif kind == "question_for_gap":
            checks.append(Check(cid, bool(output.get("questions")), f"{len(output.get('questions', []))} question(s)"))
        else:
            raise TddAgentError(f"unknown oracle check kind {kind}", code="ORACLE_CHECK_UNKNOWN")
    return checks
