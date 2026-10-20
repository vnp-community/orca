"""Demotion report: where a model keeps giving the same answer to the same kind of input, a rule can take over."""
from __future__ import annotations

from collections import Counter, defaultdict

from ..store.base import OperationalStore

MIN_REPEATS = 2


def demotion_report(store: OperationalStore, *, min_repeats: int = MIN_REPEATS) -> dict:
    calls = [c for c in store.list_llm_calls(limit=100000) if c.get("skill")]
    by_skill: dict[str, dict] = {}
    groups: dict[tuple[str, str], Counter] = defaultdict(Counter)
    for c in calls:
        sk = c["skill"]
        slot = by_skill.setdefault(sk["id"], {"llm_calls": 0, "tier": sk.get("tier"), "blocked": 0})
        slot["llm_calls"] += 1
        slot["blocked"] += 0 if c.get("status") == "ok" else 1
        pattern = sk.get("pattern") or f"prompt:{c.get('prompt_hash', '')[:12]}"
        groups[(sk["id"], pattern)][c.get("output_hash")] += 1
    metrics = {}
    for row in store.list_metrics("skill_resolutions_total"):
        lab = row["labels"]
        metrics.setdefault(lab["skill"], Counter())[lab["resolved_by"]] += row["value"]
    skills = {}
    for sid, counts in sorted(metrics.items()):
        total = sum(counts.values())
        skills[sid] = {"resolutions": dict(counts), "rule_resolution_rate": (counts.get("rule", 0) / total) if total else None, **by_skill.get(sid, {})}
    candidates = []
    for (sid, pattern), outputs in groups.items():
        answer, n = outputs.most_common(1)[0]
        total = sum(outputs.values())
        if total >= min_repeats and n == total:
            candidates.append({"skill": sid, "pattern": pattern, "calls": total, "identical_answers": n,
                               "proposal": f"consider a rule in {sid}: inputs of kind {pattern!r} always received the same answer ({total}/{total})"})
    candidates.sort(key=lambda c: (-c["calls"], c["skill"], c["pattern"]))
    return {"skills": skills, "candidates": candidates, "llm_share": _share(store), "calls_total": len(calls)}


def _share(store: OperationalStore) -> dict:
    from ..obs.metrics import Metrics
    return Metrics(store).llm_share()
