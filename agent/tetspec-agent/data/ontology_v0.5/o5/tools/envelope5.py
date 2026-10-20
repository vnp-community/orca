#!/usr/bin/env python3
"""Normalize v0.4 graph envelopes and validate v0.5 mandatory fields."""
from __future__ import annotations
from pathlib import Path
import json
from typing import Any
from ontology5 import ROOT, load, legacy_relation_names, legacy_class_names, legacy_relation_specs, canonical_properties, manifest_errors, resolved_required, conditional_required

def normalize_element(raw: dict[str, Any]) -> dict[str, Any]:
    out = dict(raw)
    out["class"] = out.get("class") or out.get("type")
    out["props"], alias_conflicts = canonical_properties(dict(out.get("props") or {}))
    if alias_conflicts:
        out.setdefault("validation", {})["property_alias_conflicts"] = alias_conflicts
    for key in ("owner_doc", "origin", "maturity", "version_introduced", "review_status"):
        if key in out and key not in out["props"]:
            out["props"][key] = out[key]
    out["provenance"] = list(out.get("provenance") or out.get("sources") or out.get("evidence") or [])
    return out

def validate_element(raw: dict[str, Any]) -> list[str]:
    x = normalize_element(raw); errors = []
    if x.get("validation", {}).get("property_alias_conflicts"):
        errors.append("conflicting property aliases")
    if not x.get("id"): errors.append("id is required")
    if not x.get("class"): errors.append("class/type is required")
    m = load()["classes"].get(x.get("class"))
    if x.get("class") and x["class"] not in load()["classes"] and x["class"] not in legacy_class_names(): errors.append("class is not declared")
    if m or x.get("class") in legacy_class_names():
        for p in resolved_required(x.get("class")):
            if x.get(p) is None and x["props"].get(p) is None:
                errors.append(f"{x['class']}.{p} is required")
    maturity = x.get("maturity") or x["props"].get("maturity")
    maturity_policy = load().get("maturity_policy", {}).get("requirements", {})
    if maturity in maturity_policy:
        for p in maturity_policy[maturity]:
            if not x.get(p) and not x["props"].get(p): errors.append(f"{p} is required at {maturity}")
    if x.get("origin") in ("own", "upstream", "external") and not x.get("provenance"):
        errors.append("provenance is required")
    def prop(name):
        return x.get(name, x["props"].get(name))
    if x.get("class") == "SectionRef" and prop("role") not in {"CR", "URD", "SRS", "SAD", "TDD", "repo"}:
        errors.append("SectionRef.role is invalid")
    if x.get("class") == "ImpactReport":
        entries = prop("entries")
        if not isinstance(entries, list) or not entries:
            errors.append("ImpactReport.entries must be non-empty")
        if prop("status") == "approved" and not prop("approval"):
            errors.append("approved ImpactReport requires approval")
    if x.get("class") == "ChangeSpecItem":
        if not isinstance(prop("srs_refs"), list) or not prop("srs_refs"):
            errors.append("ChangeSpecItem.srs_refs is required")
        if not isinstance(prop("tdd_refs"), list) or not prop("tdd_refs"):
            errors.append("ChangeSpecItem.tdd_refs is required")
        if not isinstance(prop("cr_acceptance_refs"), list) or not prop("cr_acceptance_refs"):
            errors.append("ChangeSpecItem.cr_acceptance_refs is required")
    for field in conditional_required(x):
        if prop(field) in (None, "", [], {}): errors.append(f"{x.get('class')}.{field} is conditionally required")
    typed = {
        "Quantity": {"value": (int, float), "unit": (str,), "stage": (str,)},
        "MigrationStage": {"action": (str,), "depends_on": (list, tuple), "preconditions": (list, tuple, dict), "verification": (str, dict)},
        "Measurement": {"metric": (str,), "stage": (str,), "observed_at": (str,), "source_ref": (str, dict)},
        "PermissionDecision": {"subject": (str, dict), "resource": (str, dict), "action": (str,), "effect": (str,), "evidence": (list, tuple, dict)},
        "LoadProfile": {"average": (int, float, dict), "peak": (int, float, dict), "payload": (int, float, dict), "growth": (int, float, dict)},
        "HighAvailabilityTopology": {"replicas": (int,), "failure_domains": (list, tuple, dict), "failover": (str, dict)},
        "RecoveryObjective": {"rto": (int, float, dict), "rpo": (int, float, dict)},
        "LedgerInvariant": {"currency": (str, dict), "scale": (int, dict), "invariants": (list, tuple)},
    }.get(x.get("class"), {})
    for field, expected in typed.items():
        value = prop(field)
        if value is not None and not isinstance(value, expected): errors.append(f"{x.get('class')}.{field} has invalid type")
    if x.get("class") in {"LoadProfile", "CapacityModel", "RecoveryObjective"}:
        for field in ("average", "peak", "payload", "growth", "headroom", "rto", "rpo"):
            value = prop(field)
            if isinstance(value, dict) and value.get("value") is not None and not value.get("unit"):
                errors.append(f"{x.get('class')}.{field} quantity unit is required")
    return errors

def validate_graph(graph: dict[str, Any]) -> dict[str, Any]:
    errors = [{"message": e} for e in manifest_errors()]
    elements = graph.get("elements", [])
    ids = {x.get("id") for x in elements}
    if len(ids) != len(elements): errors.append({"message": "duplicate element IDs"})
    for x in elements:
        errors.extend({"id": x.get("id"), "message": e} for e in validate_element(x))
    rels = {**legacy_relation_specs(), **load()["relations"]}
    by_id = {x.get("id"): x.get("class") or x.get("type") for x in elements}
    def compatible(actual, allowed):
        if allowed == "ANY" or not allowed: return True
        current=actual;seen=set()
        while current and current not in seen:
            if current in allowed:return True
            seen.add(current);current=load()["classes"].get(current,{}).get("extends")
        return False
    for e in graph.get("edges", []):
        if e.get("src") not in ids or e.get("dst") not in ids: errors.append({"edge": e, "message": "edge endpoint does not exist"})
        if e.get("rel") not in rels: errors.append({"edge": e, "message": "relation is not declared"})
        if e.get("rel") in rels and e.get("src") in ids and e.get("dst") in ids:
            spec=rels[e["rel"]]
            if not compatible(by_id[e['src']],spec.get('domain')):errors.append({'edge':e,'message':'relation domain violation'})
            if not compatible(by_id[e['dst']],spec.get('range')):errors.append({'edge':e,'message':'relation range violation'})
    # Enforce only explicit 1-1 cardinality; legacy n-ary relationships remain
    # compatible with the v0.4 graph representation.
    for rel, spec in rels.items():
        if spec.get("cardinality") != "1-1": continue
        grouped = {}
        for edge in graph.get("edges", []):
            if edge.get("rel") == rel: grouped.setdefault(edge.get("src"), 0); grouped[edge.get("src")] += 1
        for src, count in grouped.items():
            if count > 1: errors.append({"edge": {"src": src, "rel": rel}, "message": "relation cardinality 1-1 violated"})
    return {"version": graph.get("version", load()["version"]), "ok": not errors, "errors": errors, "element_count": len(elements), "edge_count": len(graph.get("edges", []))}
