#!/usr/bin/env python3
"""Profile-aware readiness for the three agent flows."""
from __future__ import annotations
from hashlib import sha256
import json
from ontology5 import load


def _hash(value):
    return sha256(json.dumps(value, sort_keys=True, ensure_ascii=False, default=str).encode()).hexdigest()


def execute_need_validators(profile: dict, facts: dict) -> dict:
    """Execute a scoped validator for every mandatory need.

    A need is evaluated against the pinned facts supplied to this readiness
    call. Source intake reports provide stronger applicability/evidence
    semantics; other flows still bind the result to the fact and provenance
    hashes instead of treating a global rule pass as proof for every need.
    """
    reports = {str(row.get("need_id")): row for row in facts.get("need_results", []) if isinstance(row, dict) and row.get("need_id")}
    evidence_by_need = facts.get("need_evidence", {}) if isinstance(facts.get("need_evidence", {}), dict) else {}
    provenance = facts.get("provenance") or facts.get("source_provenance")
    outcomes, blockers = {}, []
    for need_id in profile.get("mandatory_needs", []):
        report = reports.get(need_id)
        value = report if report is not None else facts.get(need_id)
        applicability = "applicable"
        matched = evidence_by_need.get(need_id, [])
        reason = None
        if report is not None:
            applicability = report.get("applicability", "applicable")
            matched = report.get("matched_evidence") or matched
            reason = report.get("reason")
            valid = report.get("status") == "found" and bool(matched)
            if applicability == "not_applicable":
                valid = bool(reason and matched)
            elif applicability == "unknown":
                valid = False
        else:
            invalid_status = isinstance(value, dict) and value.get("status") in {"unknown", "missing", "conflict", "proposed", "stale", "error"}
            valid = value not in (None, "", [], {}) and not invalid_status
            if valid and not matched and provenance:
                matched = [{"kind": "pinned-fact", "need_id": need_id, "source_hash": _hash(provenance)}]
            if not valid:
                reason = "mandatory fact is absent or unresolved"
        record = {
            "need_id": need_id,
            "rule_id": "NEED-" + str(need_id),
            "version": "1",
            "input_hash": _hash({"need": need_id, "value": value, "provenance": provenance}),
            "outcome": "pass" if valid else "fail",
            "evidence": matched or ([reason] if reason else []),
            "applicability": applicability,
            "matched_evidence": matched,
        }
        if reason:
            record["reason"] = reason
        outcomes[need_id] = record
        if not valid:
            blockers.append("NEED_VALIDATOR:" + str(need_id))
    return {"outcomes": outcomes, "blockers": sorted(set(blockers))}

def assess(run: dict, facts: dict) -> dict:
    profile = load()["profiles"].get(run.get("profile_id"))
    if not profile: return {"status": "blocked", "blockers": ["UNKNOWN_PROFILE"], "outcomes": {}}
    outcomes, blockers = {}, []
    for need in profile["mandatory_needs"]:
        val = facts.get(need)
        if val is None or val == [] or val == {} or val == "":
            outcomes[need] = {"status": "missing", "mandatory": True}; blockers.append(need)
        elif isinstance(val, dict) and val.get("status") == "conflict":
            outcomes[need] = {"status": "conflict", "mandatory": True}; blockers.append(need)
        else: outcomes[need] = {"status": "found", "mandatory": True}
    for result in facts.get("need_results", []):
        if result.get("applicability") == "not_applicable":
            if not result.get("reason") or not result.get("matched_evidence"): blockers.append("APPLICABILITY:" + result["need_id"])
        elif result.get("applicability") == "unknown":
            blockers.append("APPLICABILITY_UNKNOWN:" + result.get("need_id", "unknown"))
        elif result.get("status") != "found" or not result.get("matched_evidence"):
            blockers.append("NEED:" + result["need_id"])
    need_validation = execute_need_validators(profile, facts)
    blockers.extend(need_validation["blockers"])
    coverage = facts.get("rule_coverage", {})
    for rule in profile.get("rules", load()["readiness_rules"]):
        result = coverage.get(rule)
        if not isinstance(result, dict) or result.get("outcome") != "pass" or not all(result.get(k) for k in ("rule_id", "version", "input_hash", "evidence")):
            blockers.append(f"RULE:{rule}")
    workflow = facts.get("workflow_evaluation")
    if workflow and workflow.get("status") != "pass":
        blockers.extend(workflow.get("blockers", ["WORKFLOW_EVALUATION_BLOCKED"]))
    status = "implementation_ready" if not blockers else "draft"
    return {"status": status, "blockers": sorted(set(blockers)), "outcomes": outcomes, "need_validator_outcomes": need_validation["outcomes"], "rule_coverage": coverage}
