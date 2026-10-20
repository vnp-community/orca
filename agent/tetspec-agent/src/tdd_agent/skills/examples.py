"""Regression examples shipped with each skill package (``examples.jsonl``)."""
from __future__ import annotations

import json
from dataclasses import dataclass

from .base import Skill, SkillRegistry, SkillRunner


def subset_match(expected, actual, path: str = "$") -> list[str]:
    """Every key/item of ``expected`` must be present and equal in ``actual`` (lists compared by position)."""
    if isinstance(expected, dict):
        if not isinstance(actual, dict):
            return [f"{path}: expected object, got {type(actual).__name__}"]
        out = []
        for k, v in expected.items():
            if k not in actual:
                out.append(f"{path}.{k}: missing")
            else:
                out += subset_match(v, actual[k], f"{path}.{k}")
        return out
    if isinstance(expected, list):
        if not isinstance(actual, list) or len(actual) < len(expected):
            return [f"{path}: expected at least {len(expected)} item(s)"]
        return [e for i, v in enumerate(expected) for e in subset_match(v, actual[i], f"{path}[{i}]")]
    return [] if expected == actual else [f"{path}: expected {expected!r}, got {actual!r}"]


@dataclass
class ExampleResult:
    skill: str
    name: str
    passed: bool
    errors: list[str]
    resolved_by: str


def load_examples(skill: Skill, root=None) -> list[dict]:
    from .base import PACKAGE_ROOT
    path = (root or PACKAGE_ROOT) / skill.meta.tier / skill.meta.id / "examples.jsonl"
    if not path.is_file():
        return []
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def run_examples(registry: SkillRegistry, runner: SkillRunner, skill_ids: list[str] | None = None, tiers: tuple[str, ...] = ("logic", "hybrid", "llm")) -> list[ExampleResult]:
    results = []
    for skill in registry.all():
        if skill_ids and skill.meta.id not in skill_ids or skill.meta.tier not in tiers:
            continue
        for ex in load_examples(skill):
            r = runner.run(skill.meta.id, ex["input"])
            actual = {"output": r.output, "resolved_by": r.resolved_by, "blockers": r.blockers, "ok": r.ok}
            errors = [] if r.ok or "blockers" in ex.get("expect", {}) else [f"skill blocked: {r.blockers}"]
            errors += subset_match(ex.get("expect", {}), actual)
            results.append(ExampleResult(skill.meta.id, ex["name"], not errors, errors, r.resolved_by))
    return results
