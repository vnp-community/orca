"""Operational counters kept in the ``OperationalStore``."""
from __future__ import annotations

from ..store.base import OperationalStore


class Metrics:
    def __init__(self, store: OperationalStore):
        self.store = store

    def run_status(self, flow: str, status: str) -> None:
        self.store.incr_metric("runs_total", {"flow": flow, "status": status})

    def blocked(self, gate: str, code: str) -> None:
        self.store.incr_metric("blocked_total", {"gate": gate, "code": code})

    def version_conflict(self) -> None:
        self.store.incr_metric("version_conflicts_total", {})

    def llm_call(self, provider: str, role: str, status: str, latency_ms: int, usage: dict, attempts: int) -> None:
        labels = {"provider": provider, "role": role}
        self.store.incr_metric("llm_calls_total", {**labels, "status": status})
        self.store.incr_metric("llm_latency_ms_total", labels, float(latency_ms))
        self.store.incr_metric("llm_retries_total", labels, float(max(0, attempts - 1)))
        for key in ("input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"):
            if usage.get(key):
                self.store.incr_metric("llm_tokens_total", {**labels, "kind": key}, float(usage[key]))

    def skill_resolution(self, agent: str, skill: str, resolved_by: str) -> None:
        self.store.incr_metric("skill_resolutions_total", {"agent": agent, "skill": skill, "resolved_by": resolved_by})

    def summary(self) -> dict:
        out: dict[str, list[dict]] = {}
        for row in self.store.list_metrics():
            out.setdefault(row["name"], []).append({"labels": row["labels"], "value": row["value"]})
        return out

    def llm_share(self) -> dict[str, dict]:
        """Share of skill resolutions made by an LLM, per agent (and per skill)."""
        by_agent: dict[str, dict[str, float]] = {}
        for row in self.store.list_metrics("skill_resolutions_total"):
            labels = row["labels"]
            slot = by_agent.setdefault(labels["agent"], {})
            slot[labels["resolved_by"]] = slot.get(labels["resolved_by"], 0.0) + row["value"]
        result = {}
        for agent, counts in by_agent.items():
            total = sum(counts.values())
            result[agent] = {"counts": counts, "llm_share": (counts.get("llm", 0.0) / total) if total else 0.0}
        return result
