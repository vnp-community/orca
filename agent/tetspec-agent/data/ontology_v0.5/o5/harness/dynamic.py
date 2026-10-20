from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
import json
import time
from pathlib import Path
from typing import Any
import sys

TOOLS = Path(__file__).resolve().parents[1] / "tools"
O5_ROOT = Path(__file__).resolve().parents[1]
for _path in (TOOLS, O5_ROOT):
    if str(_path) not in sys.path: sys.path.insert(0, str(_path))
from envelope5 import validate_graph
from ontology5 import load
from readiness5 import assess
from transaction5 import apply_changeset, snapshot_hash
from workflow import validate_workflow_context, pre_authoring_gate, assess_tdd_impact, approve_impact, realize_tdd_set, WorkflowError
from semantic_resolver import resolve_scope


@dataclass
class TddGenerationRequest:
    id: str
    mode: str
    scope: list[str]
    sources: list[dict]
    base_snapshot: str | None = None
    change_request: dict | None = None
    profile_id: str = "tdd-generation"
    budget: dict | None = None
    workflow_context: dict | None = None

    @classmethod
    def from_dict(cls, value: dict) -> "TddGenerationRequest":
        required = ("id", "mode", "scope", "sources")
        missing = [p for p in required if not value.get(p)]
        if missing: raise ValueError("missing request fields: " + ", ".join(missing))
        if value["mode"] not in ("initial", "change-request", "enhancement", "defect"): raise ValueError("invalid mode")
        return cls(**{k: value[k] for k in ("id", "mode", "scope", "sources")}, base_snapshot=value.get("base_snapshot"), change_request=value.get("change_request"), profile_id=value.get("profile_id", "tdd-generation"), budget=value.get("budget"), workflow_context=value.get("workflow_context"))

    def validate_source_roles(self) -> list[str]:
        """Return source-role gaps without fabricating a missing document."""
        kinds = {str(source.get("kind", source.get("role", ""))).upper() for source in self.sources}
        required = {"SRS", "SAD"} if self.mode != "initial" else {"URD", "SRS", "SAD"}
        errors = ["SOURCE_ROLE_MISSING:" + role for role in sorted(required - kinds)]
        if self.mode != "initial" and not self.base_snapshot:
            errors.append("BASE_TDD_SNAPSHOT_REQUIRED")
        if self.mode == "defect":
            change = self.change_request or {}
            if not change.get("reproduction") or not change.get("expected_behavior"):
                errors.append("DEFECT_EXPECTED_BEHAVIOR_REQUIRED")
        return errors


def source_hash(source: dict) -> str:
    return sha256(json.dumps(source, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def resolve_needs(request: TddGenerationRequest, needs: list[dict], sources: list[dict] | None = None) -> dict:
    from quality5 import verify_evidence
    available = {s["id"]: s for s in (request.sources if sources is None else sources)}
    reports, facts = [], {}
    for need in needs:
        slot = need["slot"]; matches, searched = [], []
        applicability = need.get("applicability", {"applies": True})
        if applicability.get("applies") is False and applicability.get("reason") and applicability.get("evidence"):
            reports.append({"need_id": need["id"], "scope": request.scope, "applicability": "not_applicable", "reason": applicability["reason"], "status": "not_applicable", "matched_evidence": applicability["evidence"], "searched_sources": [], "sources": [], "conflicts": []})
            continue
        for source in available.values():
            value = source.get("facts", {}).get(slot)
            evidence = source.get("fact_evidence", {}).get(slot, [])
            valid = bool(evidence) and all(not verify_evidence(ev, list(available.values())) for ev in evidence)
            if source.get("review_status") and source.get("review_status") not in {"approved", "baselined"}:
                valid = False
            if source.get("auto_approved") is True:
                valid = False
            predicate = need.get("predicate")
            if predicate and isinstance(value, dict): valid = valid and all(value.get(k) == v for k, v in predicate.items())
            found = value is not None and value != [] and value != "" and valid
            searched.append({"source_id": source["id"], "status": "matched" if found else "no_match", "reason": "evidence predicate satisfied" if found else "value/evidence/predicate absent or invalid"})
            if found: matches.append({"source_id": source["id"], "source_version": source["version"], "value": value, "evidence": evidence})
        status = "missing" if not matches else "conflict" if len({json.dumps(x["value"], sort_keys=True) for x in matches}) > 1 else "found"
        if status == "found": facts[slot] = matches[0]["value"]
        reports.append({"need_id": need["id"], "scope": request.scope, "applicability": "applicable", "reason": "required by scoped profile", "status": status, "searched_sources": searched, "sources": matches, "matched_evidence": [e for m in matches for e in m["evidence"]], "conflicts": matches if status == "conflict" else [], "question": None if status == "found" else f"Resolve {slot} for scope {request.scope}"})
    mandatory = {n["id"] for n in needs if n.get("required", True)}
    return {"request_id": request.id, "reports": reports, "facts": facts, "blockers": [r["need_id"] for r in reports if r["need_id"] in mandatory and r["status"] in ("missing", "conflict")], "source_hashes": {k: source_hash(v) for k, v in available.items()}}


def plan_ownership(elements: list[dict], scope: list[str], owners: dict[str, str] | None = None) -> dict:
    owners = owners or {}; result = []
    for x in elements:
        if x.get("id") not in scope and scope and x.get("scope") not in scope: continue
        owner = owners.get(x.get("id")) or x.get("owner_doc") or "TDD-unassigned"
        priority = x.get("priority") or x.get("props", {}).get("priority") or "P1"
        result.append({"element_id": x.get("id"), "owner": owner, "priority": priority, "consumers": x.get("consumers", [])})
    return {"assignments": result, "owners": sorted({x["owner"] for x in result})}


class KernelRegistry:
    def __init__(self, version: str, values: dict[str, Any] | None = None): self.version, self.values = version, dict(values or {})
    def get(self, key): return self.values[key]
    def put(self, key, value): self.values[key] = value
    def manifest(self): return {"version": self.version, "keys": sorted(self.values)}


def context_pack(task_id: str, graph: dict, owned_ids: list[str], budget: int | None = None) -> dict:
    resolution = resolve_scope(graph, owned_ids, budget=budget)
    ids = set(resolution["ids"])
    all_ids = {x.get("id") for x in graph.get("elements", [])}
    truncated = bool(resolution.get("truncated") or (budget and len(ids) < len(all_ids)))
    obligation_nodes = [x for x in graph.get("elements", []) if x.get("class") == "DesignObligation" and x.get("id") in ids]
    return {"task_id": task_id, "owned_ids": owned_ids, "element_ids": sorted(ids), "obligation_ids": sorted(x.get("id") for x in obligation_nodes), "unresolved_obligations": sorted(x.get("id") for x in obligation_nodes if x.get("status", x.get("props", {}).get("status", "open")) not in {"verified", "accepted", "not_applicable"}), "truncated": truncated, "missing_ids": resolution.get("missing_ids", sorted(all_ids - ids) if truncated else []), "resolution": resolution, "snapshot": snapshot_hash(graph)}


class TaskLedger:
    def __init__(self, path: str | Path): self.path = Path(path); self.state = json.loads(self.path.read_text()) if self.path.exists() else {"tasks": {}}
    def begin(self, task_id: str, input_hash: str) -> dict:
        task = self.state["tasks"].get(task_id)
        if task and task.get("input_hash") != input_hash: task.update(status="stale", input_hash=input_hash, attempt=task.get("attempt", 0) + 1)
        elif task and task.get("status") == "completed": return task
        else: task = {"status": "running", "attempt": (task or {}).get("attempt", 0) + 1, "input_hash": input_hash, "output_ids": []}
        self.state["tasks"][task_id] = task; return task
    def complete(self, task_id: str, output_ids: list[str]): self.state["tasks"][task_id].update(status="completed", output_ids=sorted(set(output_ids)))
    def save(self): self.path.parent.mkdir(parents=True, exist_ok=True); self.path.write_text(json.dumps(self.state, indent=2, ensure_ascii=False) + "\n")


def run_generation(request: TddGenerationRequest, graph: dict, needs: list[dict], proposed_changeset: dict | None = None, facts: dict | None = None) -> dict:
    started = time.perf_counter()
    def finish(payload: dict, intake: dict | None = None) -> dict:
        payload["metrics"] = {"elapsed_ms": round((time.perf_counter() - started) * 1000, 3), "element_count": len(payload.get("graph", {}).get("elements", graph.get("elements", []))), "edge_count": len(payload.get("graph", {}).get("edges", graph.get("edges", []))), "need_count": len(needs), "matched_need_count": sum(1 for row in (intake or {}).get("reports", []) if row.get("status") == "found")}
        return payload
    source_errors = request.validate_source_roles()
    if source_errors:
        return finish({"status": "blocked", "blockers": source_errors, "snapshot": snapshot_hash(graph), "graph": graph})
    if request.workflow_context is not None:
        gate = pre_authoring_gate(request.workflow_context, "tdd")
        if gate["status"] != "approved":
            return finish({"status": "blocked", "blockers": gate["errors"], "snapshot": snapshot_hash(graph), "graph": graph})
    intake = resolve_needs(request, needs)
    if intake["blockers"]: return finish({"status": "blocked", "intake": intake, "readiness": {"status": "blocked", "blockers": intake["blockers"], "outcomes": intake["reports"]}, "snapshot": snapshot_hash(graph), "graph": graph}, intake)
    if proposed_changeset:
        graph, change_report = apply_changeset(graph, proposed_changeset)
        if not change_report.get("ok"): return finish({"status": "blocked", "change": change_report, "snapshot": snapshot_hash(graph)}, intake)
    else: change_report = {"ok": True, "touched": []}
    from profiles import default_workflow_profile
    workflow_kind = request.mode if request.mode in {"initial", "change-request"} else "change-request"
    readiness_facts = {"scope": request.scope, "provenance": intake["reports"], "profile": default_workflow_profile(workflow_kind), **intake["facts"]}
    if facts: readiness_facts.update(facts)
    from quality5 import graph_checks
    readiness_facts["rule_coverage"] = graph_checks(graph, request.sources)
    readiness_facts["need_results"] = intake["reports"]
    if request.workflow_context is not None:
        from workflow_quality import evaluate_workflow
        readiness_facts["workflow_evaluation"] = evaluate_workflow({"context": request.workflow_context}, "impact")
    readiness = assess({"profile_id": request.profile_id}, readiness_facts)
    return finish({"status": readiness["status"], "intake": intake, "change": change_report, "readiness": readiness, "snapshot": snapshot_hash(graph), "graph": graph}, intake)


def assess_change_impact(request: TddGenerationRequest, tdd_inventory: list[dict], sad: dict) -> dict:
    """Stage-3 entry point for change/enhancement/defect TDD generation.

    It deliberately returns an unapproved report.  A caller must obtain the
    SA/Dev approval token before calling ``materialize_change_tdd`` or stage 5.
    """
    if request.mode == "initial":
        raise WorkflowError("IMPACT_NOT_REQUIRED_FOR_INITIAL_BASELINE")
    change = request.change_request or {}
    srs_delta = change.get("srs_delta") or change.get("srs")
    if not srs_delta:
        raise WorkflowError("SRS_DELTA_REQUIRED")
    return assess_tdd_impact(srs_delta, tdd_inventory, sad, report_id=request.id + ":impact")


def materialize_change_tdd(report, mappings: list[dict], actual_snapshot: str):
    """Stage-4 helper; rejects pending/partial/unplanned materialization."""
    return realize_tdd_set(report, mappings, actual_snapshot)


def impact_closure(graph: dict, changed_ids: list[str], max_depth: int = 3) -> dict:
    """Return affected elements and mark downstream consumers for review."""
    resolution = resolve_scope(graph, changed_ids)
    seen = set(resolution.get("ids", changed_ids))
    for x in graph.get("elements", []):
        if x.get("id") in seen and x.get("id") not in changed_ids and x.get("review_status") == "approved": x["review_status"] = "needs_review"
    return {"changed": sorted(set(changed_ids)), "affected": sorted(seen - set(changed_ids)), "depth": max_depth, "resolution": resolution}
