"""Pure, policy-driven semantic closure for TDD impact planning.

The resolver is intentionally read-only.  It returns the boundary evidence a
caller needs to decide whether a TDD, consumer, contract or operational
dependency is in scope; it never changes review status or approval records.
"""
from __future__ import annotations

from dataclasses import dataclass, asdict
from hashlib import sha256
import json
from typing import Any, Iterable


DEFAULT_POLICY_VERSION = "semantic-impact.v1"


@dataclass(frozen=True)
class RelationPolicy:
    relation: str
    direction: str = "both"
    semantic: str = "dependency"
    terminal: bool = False
    external: bool = False


DEFAULT_POLICIES = tuple(
    RelationPolicy(r, "both", "dependency")
    for r in (
        "depends_on", "depends_on_work", "satisfies", "implemented_by",
        "verified_by", "has_scenario", "constrained_by", "references_entity",
        "reads", "writes", "may_fail_with", "has_attribute", "models_state_of",
        "has_state", "traces_to", "in_machine", "transition_from", "transition_to",
        "before_requirement", "after_requirement", "decided_by", "requires_role",
        "requires_scope", "uses_contract", "consumed_by", "publishes", "deployed_on",
        "configured_by", "has_control", "has_threat", "has_quality_target",
    )
)


def _fingerprint(value: Any) -> str:
    return sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def _graph_fingerprint(graph: dict[str, Any]) -> str:
    """Hash graph content independent of export ordering."""
    canonical = dict(graph)
    canonical["elements"] = sorted(graph.get("elements", []), key=lambda x: str(x.get("id", "")))
    canonical["edges"] = sorted(graph.get("edges", []), key=lambda x: (str(x.get("src", "")), str(x.get("rel", "")), str(x.get("dst", ""))))
    return _fingerprint(canonical)


def _node_id(value: Any) -> str:
    return str(value.get("id", "")) if isinstance(value, dict) else str(value)


def resolve_scope(
    graph: dict[str, Any],
    roots: Iterable[str],
    *,
    policies: Iterable[RelationPolicy] = DEFAULT_POLICIES,
    budget: int | None = None,
    external_inventory: Iterable[dict[str, Any]] = (),
    policy_version: str = DEFAULT_POLICY_VERSION,
) -> dict[str, Any]:
    """Compute a deterministic fixed point and explain every boundary.

    An unknown relation is reported as an unresolved boundary rather than
    silently ignored.  External inventory entries are included only when an
    edge explicitly points at their stable id.
    """
    policy_list = tuple(policies)
    by_id = {_node_id(n): n for n in graph.get("elements", []) if _node_id(n)}
    external = {_node_id(n): n for n in external_inventory if _node_id(n)}
    all_nodes = {**by_id, **external}
    policy_map = {p.relation: p for p in policy_list}
    adjacency: dict[str, list[tuple[str, dict[str, Any]]]] = {}
    unresolved: list[dict[str, Any]] = []
    for edge in graph.get("edges", []):
        src, dst, rel = str(edge.get("src", "")), str(edge.get("dst", "")), str(edge.get("rel", ""))
        policy = policy_map.get(rel)
        if not policy:
            unresolved.append({"type": "unsupported_relation", "src": src, "rel": rel, "dst": dst})
            continue
        if policy.direction in ("both", "out"):
            adjacency.setdefault(src, []).append((dst, edge))
        if policy.direction in ("both", "in"):
            adjacency.setdefault(dst, []).append((src, edge))

    root_ids = sorted({str(x) for x in roots if str(x)})
    ids: set[str] = set(root_ids)
    reasons: dict[str, list[dict[str, Any]]] = {}
    queue = list(root_ids)
    while queue:
        current = queue.pop(0)
        for target, edge in sorted(adjacency.get(current, []), key=lambda x: (str(x[0]), str(x[1].get("rel", "")))):
            if target not in all_nodes:
                unresolved.append({"type": "dangling_reference", "src": current, "rel": edge.get("rel"), "dst": target})
                continue
            reasons.setdefault(target, []).append({"from": current, "relation": edge.get("rel"), "direction": "traversed"})
            if target not in ids:
                ids.add(target)
                queue.append(target)

    ordered = sorted(ids)
    missing_ids: list[str] = []
    if budget is not None and budget >= 0 and len(ordered) > budget:
        missing_ids = ordered[budget:]
        ordered = ordered[:budget]
        unresolved.append({"type": "budget_truncated", "missing_ids": missing_ids})
    selected = set(ordered)
    boundary = []
    for ident in ordered:
        for target, edge in adjacency.get(ident, []):
            if target not in selected and target in all_nodes:
                boundary.append({"src": ident, "rel": edge.get("rel"), "dst": target, "reason": "budget_or_scope_boundary"})
    evidence = {
        "policy_version": policy_version,
        "policy_hash": _fingerprint([asdict(p) for p in policy_list]),
        "roots": root_ids,
        "ids": ordered,
        "reasons": {k: v for k, v in sorted(reasons.items()) if k in selected},
        "boundary": sorted(boundary, key=lambda x: (x["src"], x["rel"], x["dst"])),
        "unresolved": sorted(unresolved, key=lambda x: json.dumps(x, sort_keys=True)),
        "missing_ids": missing_ids,
        "truncated": bool(missing_ids),
        "graph_snapshot": _graph_fingerprint(graph),
        "external_inventory": sorted(external),
    }
    evidence["resolution_hash"] = _fingerprint(evidence)
    return evidence


def validate_resolution(resolution: dict[str, Any], graph: dict[str, Any], *, expected_policy_version: str = DEFAULT_POLICY_VERSION) -> list[str]:
    """Check that a planning result still belongs to the current graph/policy."""
    errors = []
    if resolution.get("policy_version") != expected_policy_version:
        errors.append("RESOLUTION_POLICY_STALE")
    if resolution.get("graph_snapshot") != _graph_fingerprint(graph):
        errors.append("RESOLUTION_GRAPH_STALE")
    if resolution.get("truncated") or resolution.get("unresolved"):
        errors.append("RESOLUTION_NOT_CLOSED")
    return sorted(set(errors))


def partition_scope(obligations: Iterable[dict[str, Any]], *, budget: int | None = None) -> dict[str, Any]:
    """Partition mandatory obligations without dropping cross-package links."""
    rows = sorted((dict(x) for x in obligations), key=lambda x: str(x.get("id", "")))
    if budget is None or budget <= 0 or len(rows) <= budget:
        return {"partitions": [rows] if rows else [], "missing": [], "truncated": False, "coverage_ids": [str(x.get("id")) for x in rows]}
    partitions = [rows[i:i + budget] for i in range(0, len(rows), budget)]
    missing = [str(x.get("id")) for part in partitions[1:] for x in part]
    mandatory_missing = [str(x.get("id")) for part in partitions[1:] for x in part if x.get("applicability", "applicable") == "applicable"]
    return {"partitions": partitions, "missing": missing, "mandatory_missing": mandatory_missing, "truncated": bool(missing), "coverage_ids": [str(x.get("id")) for x in rows]}


def field_impact(graph: dict[str, Any], changed: Iterable[dict[str, Any]], *, policies: Iterable[RelationPolicy] = DEFAULT_POLICIES) -> dict[str, Any]:
    """Resolve impact while preserving changed field paths and dimensions."""
    rows = [dict(x) for x in changed]
    roots = [str(x.get("id", "")) for x in rows if x.get("id")]
    resolution = resolve_scope(graph, roots, policies=policies)
    dimensions = {str(x.get("id")): sorted(set(str(p) for p in x.get("fields", x.get("paths", [])) or [])) for x in rows if x.get("id")}
    resolution["field_impacts"] = dimensions
    resolution["impact_dimensions"] = sorted({str(x.get("dimension")) for x in rows if x.get("dimension")})
    resolution["resolution_hash"] = _fingerprint({k: v for k, v in resolution.items() if k != "resolution_hash"})
    return resolution


def authorized_invalidation_changeset(graph: dict[str, Any], affected_ids: Iterable[str], *, actor: str, authorization: dict[str, Any], change_set_id: str = "impact-invalidation") -> dict[str, Any]:
    """Build, but do not apply, a transactional consumer invalidation set."""
    if not actor or actor.startswith("agent:") or authorization.get("actor_type") != "human":
        raise ValueError("INVALIDATION_HUMAN_AUTHORIZATION_REQUIRED")
    if "invalidate" not in set(authorization.get("operations", [])):
        raise ValueError("INVALIDATION_OPERATION_NOT_AUTHORIZED")
    scope = set(authorization.get("scope", [])); targets = sorted(set(str(x) for x in affected_ids))
    if "*" not in scope and not set(targets) <= scope:
        raise ValueError("INVALIDATION_SCOPE_FORBIDDEN")
    by_id = {str(x.get("id")): x for x in graph.get("elements", [])}
    ops = []
    for ident in targets:
        node = by_id.get(ident)
        if not node or node.get("review_status") not in {"approved", "baselined"}: continue
        ops.append({"seq": len(ops) + 1, "op": "modify", "target": ident, "path": "review_status", "before": node.get("review_status"), "after": "needs_review", "provenance": [{"kind": "derived", "source_id": change_set_id, "source_version": str(graph.get("version", "")), "inputs": targets, "derivation": "semantic impact invalidation"}]})
    return {"id": change_set_id, "base_version": graph.get("version"), "result_version": str(graph.get("version", "")) + "+invalidate", "ops": ops, "authorization": dict(authorization), "status": "planned"}
