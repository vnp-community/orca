from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
import json, re, sys
from copy import deepcopy
from pathlib import Path
from typing import Any
TOOLS = Path(__file__).resolve().parents[1] / "tools"
HARNESS = Path(__file__).resolve().parents[1] / "harness"
O5_ROOT = Path(__file__).resolve().parents[1]
for p in (TOOLS, HARNESS, O5_ROOT):
    if str(p) not in sys.path: sys.path.insert(0, str(p))
from transaction5 import apply_changeset, snapshot_hash
from dynamic import impact_closure
from render import render_tdd, round_trip
from workflow import pre_authoring_gate, validate_document_freshness, validate_impact_consistency


@dataclass
class EditRequest:
    id: str
    prompt: str
    target: list[str]
    base_snapshot: str
    authorization: dict
    user_id: str = "user"
    intent: str | None = None

    def __post_init__(self):
        self.prompt_hash = sha256(self.prompt.encode()).hexdigest()

    @classmethod
    def from_dict(cls, x: dict) -> "EditRequest":
        missing = [p for p in ("id", "prompt", "target", "base_snapshot", "authorization") if not x.get(p)]
        if missing: raise ValueError("missing edit fields: " + ", ".join(missing))
        return cls(x["id"], x["prompt"], x["target"], x["base_snapshot"], x["authorization"], x.get("user_id", "user"), x.get("intent"))


def classify_intent(prompt: str) -> str:
    p = prompt.lower()
    return "presentation" if any(k in p for k in ("format", "layout", "reorder", "đổi thứ tự", "trình bày", "heading")) and not any(k in p for k in ("value", "behavior", "timeout", "limit", "sửa logic")) else "semantic"


def resolve_target(request: EditRequest, graph: dict) -> dict:
    explicit = set(request.target); elements = graph.get("elements", []); matches = [x for x in elements if x.get("id") in explicit]
    terms = [t.lower() for t in request.target if t not in {x.get("id") for x in elements}]
    for term in terms: matches.extend(x for x in elements if term in (x.get("name", "").lower()) and x not in matches)
    if "sbom" in request.prompt.lower() and matches and all("sbom" not in (x.get("name", "") + str(x.get("props", {}))).lower() for x in matches):
        candidates = [x["id"] for x in elements if "sbom" in (x.get("name", "") + str(x.get("props", {}))).lower()]
        return {"status": "target_mismatch", "candidates": candidates, "question": "Target definition does not match SBOM intent."}
    ids = sorted({x.get("id") for x in matches}); intent = request.intent or classify_intent(request.prompt)
    if not ids: return {"status": "missing", "candidates": [], "question": "Identify a TDD element or section by stable ID."}
    if len(ids) > 1 and not explicit.intersection(ids): return {"status": "ambiguous", "candidates": ids, "question": "Which target ID should be edited?"}
    return {"status": "resolved", "target_ids": ids, "intent": intent}


def text_only_intake(markdown: str, document_id: str = "TDD-text", version: str = "1") -> tuple[dict, dict]:
    elements, unresolved = [], []
    for line in markdown.splitlines():
        m = re.match(r"^- \[([^\]]+)\]\s+([^:]+):\s*(.*)$", line.strip())
        if m:
            elements.append({"id": m.group(2).strip(), "class": m.group(1).strip(), "name": m.group(3).strip(), "origin": "upstream", "maturity": "sketch", "version_introduced": version, "provenance": [{"kind": "extracted", "source_id": document_id, "source_version": version, "block": f"line:{len(elements)+1}"}], "review_status": "needs_review", "props": {}})
    if not elements: unresolved.append({"slot": "elements", "status": "unknown", "reason": "No anchored semantic element found in text", "owner": "intake", "proposed": True, "lineage": {"source_id": document_id, "source_version": version, "derivation": "text-only semantic extraction unsupported"}})
    graph = {"version": version, "elements": elements, "edges": []}
    return graph, {"status": "found" if elements else "missing", "element_count": len(elements), "gaps": unresolved, "source": {"id": document_id, "version": version}}


def authorized(request: EditRequest, operation: str, targets: list[str]) -> bool:
    scope = request.authorization.get("scope", [])
    allowed_ops = request.authorization.get("operations", [])
    return operation in allowed_ops and ("*" in scope or all(t in scope for t in targets))


def prompt_provenance(request: EditRequest) -> dict:
    return {"kind": "prompt", "source_id": request.id, "source_version": request.base_snapshot, "prompt_hash": request.prompt_hash, "source_hash": request.prompt_hash, "block": "prompt:1", "inputs": list(request.target)}


def plan_edit(request: EditRequest, graph: dict, operation: str, target: str | None = None, path: str = "name", after: Any = None, cascade: str | None = None) -> dict:
    resolved = resolve_target(request, graph); targets = resolved.get("target_ids", [])
    target = target or (targets[0] if targets else None)
    if resolved["status"] != "resolved": return {"status": resolved["status"], "resolution": resolved}
    if not authorized(request, operation, [target]): return {"status": "unauthorized", "target": target}
    current = next(x for x in graph.get("elements", []) if x.get("id") == target)
    conflict = upstream_conflict(current, after) if operation == "modify" and path.startswith("props.") else None
    op = {"seq": 1, "op": operation, "target": target, "provenance": [prompt_provenance(request)]}
    if "timeout" in request.prompt.lower() and path == "name" and operation == "modify":
        return {"status": "behavior_property_required", "reason": "Timeout must update a contract property, not name"}
    if target not in targets: return {"status": "target_mismatch", "target": target}
    if operation == "modify":
        value = current
        for key in path.split("."): value = value.get(key) if isinstance(value, dict) else None
        op.update(path=path, before=value, after=after)
    elif operation == "rename": op.update(path="name", before=current.get("name"), after=after)
    elif operation == "remove": op.update(before=current.get("name"), cascade=cascade or "restrict")
    elif operation == "add": op.update(after=after)
    else: return {"status": "unsupported_operation"}
    return {"status": "planned", "change_set": {"id": "CS-" + request.id, "request": request.id, "base_version": request.base_snapshot, "result_version": request.base_snapshot + "+1", "ops": [op], "prompt_provenance": prompt_provenance(request)}, "targets": targets, "conflicts": [conflict] if conflict else []}


def upstream_conflict(current: dict, requested: Any) -> dict | None:
    upstream = current.get("props", {}).get("upstream_value")
    if upstream is not None and upstream != requested:
        return {"type": "Deviation", "status": "proposed", "element_id": current.get("id"), "upstream": upstream, "requested": requested, "resolution": "pending"}
    return None


def presentation_overlay(request: EditRequest, document: dict, changes: dict) -> dict:
    semantic = json.dumps(document.get("semantic_graph", {}), sort_keys=True, ensure_ascii=False).encode()
    return {"document_id": document.get("id"), "revision": document.get("revision", 0) + 1, "overlay": changes, "semantic_hash_before": sha256(semantic).hexdigest(), "semantic_hash_after": sha256(semantic).hexdigest(), "prompt": prompt_provenance(request)}


def validate_presentation_overlay(overlay: dict, source_refs: list[dict], sources: list[dict]) -> list[str]:
    """Presentation edits keep stable semantic identities and fresh source refs."""
    errors = []
    if overlay.get("semantic_hash_before") != overlay.get("semantic_hash_after"):
        errors.append("PRESENTATION_SEMANTIC_HASH_CHANGED")
    errors.extend(validate_document_freshness(source_refs, sources))
    for ref in source_refs:
        if ref.get("role") not in {"TDD", "SRS", "SAD", "CR", "URD"}:
            errors.append("PRESENTATION_SOURCE_ROLE_INVALID")
    return sorted(set(errors))


def commit_edit(request: EditRequest, graph: dict, plan: dict, render_fn=None, artifacts: list[dict] | None = None) -> dict:
    if plan.get("status") != "planned": return plan
    if request.authorization.get("enforce_workflow") and not request.authorization.get("workflow_context"):
        return {"status": "blocked", "reason": "WORKFLOW_CONTEXT_REQUIRED", "plan": plan, "graph": graph}
    if request.authorization.get("enforce_workflow"):
        gate = pre_authoring_gate(request.authorization.get("workflow_context", {}), "prompt")
        if gate["status"] != "approved":
            return {"status": "blocked", "reason": "WORKFLOW_GATE_BLOCKED", "blockers": gate["errors"], "plan": plan, "graph": graph}
    if request.intent == "presentation" or classify_intent(request.prompt) == "presentation":
        overlay = presentation_overlay(request, {"id": request.target[0], "semantic_graph": graph}, {"prompt": request.prompt})
        return {"status": "presentation_committed", "overlay": overlay, "graph": graph, "audit": {"request": asdict(request), "change_set": plan["change_set"]}}
    resolution = request.authorization.get("deviation_resolution")
    if plan.get("conflicts"):
        # A prompt is provenance and intent.  It cannot turn a value that
        # conflicts with the authoritative SRS into an effective TDD by local
        # approval.  The SRS/CR clarification must be processed through the
        # stage-3 impact gate first.
        workflow_context = request.authorization.get("workflow_context")
        if not workflow_context or workflow_context.get("impact", {}).get("status") != "approved" or workflow_context.get("srs_clarification") is not True:
            return {"status": "blocked", "reason": "UPSTREAM_SRS_CLARIFICATION_REQUIRED", "conflicts": plan["conflicts"], "graph": graph, "plan": plan}
        valid = resolution and resolution.get("status") == "approved" and resolution.get("decision_id") and resolution.get("approved_by") and resolution.get("prompt_hash") == request.prompt_hash and resolution.get("base_hash") == snapshot_hash(graph)
        if not valid: return {"status": "blocked", "conflicts": plan["conflicts"], "graph": graph, "plan": plan}
        for conflict in plan["conflicts"]:
            conflict.update(status="approved_deviation", resolution=deepcopy(resolution))
        for op in plan["change_set"]["ops"]: op["policy_resolution"] = deepcopy(resolution)
    candidate, result = apply_changeset(graph, plan["change_set"])
    if not result.get("ok"): return {"status": "rejected", "result": result}
    audit = {"request": asdict(request), "change_set": plan["change_set"], "before_snapshot": snapshot_hash(graph), "after_snapshot": snapshot_hash(candidate), "impact": impact_closure(candidate, plan.get("targets", [])), "validation": result, "deviations": deepcopy(plan.get("conflicts", []))}
    try:
        content = render_fn(candidate) if render_fn else render_tdd(candidate)
        check = round_trip(candidate, content)
        if not check["ok"]: raise ValueError("ROUND_TRIP_FAILED")
    except Exception as exc:
        audit["render_error"] = str(exc)
        return {"status": "committed_pending_render", "graph": candidate, "audit": audit}
    from quality5 import graph_checks
    from readiness5 import assess
    checks = graph_checks(candidate)
    consistency_errors = validate_impact_consistency(candidate, audit["impact"]["affected"] + plan.get("targets", []))
    if consistency_errors:
        audit["impact"]["consistency_errors"] = consistency_errors
    readiness_facts = {"prompt": request.prompt, "target": request.target, "base_snapshot": request.base_snapshot, "authorization": request.authorization, "impact": audit["impact"], "validation": check, "provenance": [prompt_provenance(request)], "rule_coverage": checks}
    workflow_context = request.authorization.get("workflow_context")
    if workflow_context:
        from workflow_quality import evaluate_workflow
        from profiles import default_workflow_profile
        readiness_facts["profile"] = workflow_context.get("profile") or default_workflow_profile("prompt-edit", context={"target": request.target})
        readiness_facts["workflow_evaluation"] = evaluate_workflow({"context": workflow_context, "profile": readiness_facts["profile"]}, "impact")
    readiness_report = assess({"profile_id": "prompt-tdd-edit"}, readiness_facts)
    if consistency_errors:
        readiness_report.setdefault("blockers", []).extend(consistency_errors)
        readiness_report["blockers"] = sorted(set(readiness_report["blockers"]))
        readiness_report["status"] = "draft"
    for artifact in artifacts or []:
        if set(artifact.get("element_ids", [])) & set(audit["impact"]["affected"] + plan.get("targets", [])):
            artifact["status"] = "stale"
    audit["readiness"] = readiness_report
    audit["render"] = {"round_trip": check}; return {"status": "committed", "graph": candidate, "audit": audit, "rendered": content}


def undo(audit: dict, current_graph: dict) -> dict:
    if snapshot_hash(current_graph) != audit.get("after_snapshot"): return {"status": "conflict", "reason": "CURRENT_VERSION_CHANGED"}
    cs = audit["change_set"]; inverse = {"base_version": current_graph.get("version"), "result_version": current_graph.get("version", "") + "+undo", "ops": []}
    for op in reversed(cs.get("ops", [])):
        if op["op"] in ("modify", "rename"): inverse["ops"].append({"seq": len(inverse["ops"])+1, "op": "modify", "target": op["target"], "path": op["path"], "before": op["after"], "after": op["before"]})
    graph, report = apply_changeset(current_graph, inverse); return {"status": "undone" if report.get("ok") else "conflict", "graph": graph, "report": report, "inverse": inverse}


def plan_prompt(request: EditRequest, graph: dict) -> dict:
    """Resolve supported typed edits from prompt; ambiguous intent never mutates.

    Broader edits must supply an agent-authored ChangeSet through plan_edit and
    the same validation/authorization gate; unsupported prose is a knowledge gap.
    """
    resolved = resolve_target(request, graph)
    if resolved['status'] != 'resolved': return {'status': resolved['status'], 'resolution': resolved}
    if len(resolved['target_ids']) != 1: return {'status': 'ambiguous', 'resolution': resolved}
    target = resolved['target_ids'][0]; node = next(n for n in graph['elements'] if n['id'] == target)
    if 'timeout' in request.prompt.lower():
        value = re.search(r'\b(\d+(?:\.\d+)?)\s*(?:seconds?|giây|s)\b', request.prompt, re.I)
        properties = [key for key in node.get('props', {}) if 'timeout' in key]
        if not value or len(properties) != 1:
            return {'status': 'ambiguous', 'candidates': ['props.'+k for k in properties], 'reason': 'Specify one timeout property and a value with units'}
        after = float(value[1]); after = int(after) if after.is_integer() else after
        if after <= 0: return {'status': 'invalid_value', 'reason': 'Timeout must be positive'}
        return plan_edit(request, graph, 'modify', target=target, path='props.'+properties[0], after=after)
    rename = re.search(r'(?:rename|đổi tên).+?(?:to|thành)\s+["“](.+?)["”]\s*$', request.prompt, re.I)
    if rename: return plan_edit(request, graph, 'rename', target=target, after=rename[1])
    return {'status': 'planning_required', 'target_ids': resolved['target_ids'], 'reason': 'An agent-authored semantic ChangeSet is required for this prompt'}
