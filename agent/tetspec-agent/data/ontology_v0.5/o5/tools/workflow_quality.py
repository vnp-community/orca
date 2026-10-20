"""Stage-aware evaluator shared by the dynamic, edit and spec agents."""
from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable, Iterable
import json
from hashlib import sha256

from quality5 import execute, fingerprint, known
from workflow import validate_document_freshness, validate_realization_freshness, validate_change_spec_coverage, validate_workflow_context


@dataclass(frozen=True)
class Rule:
    id: str
    stage: str
    check: Callable[[dict[str, Any]], list[str]]
    applicability: Callable[[dict[str, Any]], bool] = lambda _: True


class RuleRegistry:
    """Deterministic registry; unsupported applicable rules remain unknown."""

    def __init__(self, rules: Iterable[Rule] = ()):
        self._rules = {rule.id: rule for rule in rules}

    def register(self, rule: Rule) -> None:
        if rule.id in self._rules:
            raise ValueError("DUPLICATE_RULE:" + rule.id)
        self._rules[rule.id] = rule

    def evaluate(self, facts: dict[str, Any], stage: str | None = None) -> dict[str, Any]:
        result = {}
        for ident in sorted(self._rules):
            rule = self._rules[ident]
            if stage and rule.stage != stage:
                continue
            if not rule.applicability(facts):
                result[ident] = {"rule_id": ident, "stage": rule.stage, "outcome": "not_applicable", "input_hash": fingerprint(facts), "evidence": ["predicate false"]}
                continue
            result[ident] = execute(ident, facts, rule.check)
            result[ident]["stage"] = rule.stage
        return result


def default_registry() -> RuleRegistry:
    registry = RuleRegistry()
    registry.register(Rule("WF-01", "impact", lambda f: validate_workflow_context(f.get("context", {}))))
    registry.register(Rule("WF-02", "impact", lambda f: validate_document_freshness(f.get("documents", []), f.get("source_documents", [])), lambda f: bool(f.get("documents"))))
    registry.register(Rule("WF-03", "spec", lambda f: list(f.get("change_spec", {}).get("errors", []))))
    registry.register(Rule("WF-04", "acceptance", lambda f: list(validate_change_spec_coverage(f.get("change_spec", {}), f.get("test_receipts", [])).get("errors", [])), lambda f: bool(f.get("change_spec"))))
    registry.register(Rule("WF-05", "acceptance", lambda f: ["BA_ACCEPTANCE_MISSING"] if f.get("acceptance", {}).get("status") not in {"accepted", "complete"} else [] , lambda f: bool(f.get("acceptance"))))
    registry.register(Rule("WF-06", "impact", lambda f: validate_realization_freshness(f.get("context", {}).get("tdd_set", {}), f.get("tdd_sources", [])), lambda f: bool(f.get("tdd_sources"))))
    registry.register(Rule("WF-07", "spec", lambda f: ["PROFILE_PIN_MISSING"] if f.get("profile", {}).get("hash") is None else [], lambda f: bool(f.get("profile"))))
    return registry


def evaluate_workflow(facts: dict[str, Any], stage: str) -> dict[str, Any]:
    """Run applicable checks and keep unknown/missing evidence in denominator."""
    results = default_registry().evaluate(facts, stage)
    blockers = []
    for ident, record in results.items():
        if record.get("outcome") not in {"pass", "not_applicable"}:
            blockers.append(f"RULE:{ident}")
    return {"stage": stage, "status": "pass" if not blockers else "blocked", "rules": results, "blockers": sorted(blockers), "input_hash": fingerprint(facts)}


def coverage_denominator(obligations: Iterable[dict[str, Any]], resolutions: Iterable[dict[str, Any]]) -> dict[str, Any]:
    resolution_by_id = {str(x.get("obligation_id")): x for x in resolutions}
    rows = []
    for obligation in obligations:
        ident = str(obligation.get("id", "")); resolution = resolution_by_id.get(ident)
        rows.append({"obligation_id": ident, "applicable": obligation.get("applicability", "applicable"), "resolved": bool(resolution and resolution.get("status") in {"verified", "accepted"}), "resolution": resolution})
    unresolved = [row["obligation_id"] for row in rows if row["applicable"] == "applicable" and not row["resolved"]]
    invalid_na = [row["obligation_id"] for row in rows if row["applicable"] == "not_applicable" and not (row["resolution"] and row["resolution"].get("evidence"))]
    resolved = len(rows) - len(unresolved) - len(invalid_na)
    return {"total": len(rows), "resolved": resolved, "unresolved": unresolved, "invalid_not_applicable": invalid_na, "coverage": resolved / len(rows) if rows else 0.0, "rows": rows, "status": "blocked" if unresolved or invalid_na or not rows else "complete"}
