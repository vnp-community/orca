"""Compare the agents that can run the skills: the regression examples of every skill that uses a model, on each agent, with what it cost.

The examples run in a scratch in-memory runtime (nothing of the project is touched).  The report names the CLI version and the model the agent
actually reported, the number of model calls, tokens, time, and every example that failed with the reason.
"""
from __future__ import annotations

import copy
import dataclasses
import json
import time
from pathlib import Path

from ..runtime import Runtime
from ..skills import SkillRunner
from ..skills.examples import run_examples

AGENTS = {"claude": "claude_code", "codex": "codex_cli", "claude_code": "claude_code", "codex_cli": "codex_cli"}


def run_providers(config, out: Path, agents: list[str], tiers: tuple[str, ...] = ("hybrid", "llm")) -> dict:
    from ..agents.common import registry
    report: dict = {"tiers": list(tiers), "agents": {}}
    for name in agents:
        provider = AGENTS[name]
        data = copy.deepcopy(config.data)
        data["llm"]["default"] = provider
        data["llm"]["roles"] = {"author": "", "critic": "", "extractor": ""}
        data["store"] = {"knowledge": "memory:", "operational": "memory:"}
        rt = Runtime.build(dataclasses.replace(config, data=data), configure_logging=False)
        try:
            started = time.monotonic()
            results = run_examples(registry(), SkillRunner(registry(), rt.observer, rt.gateway), None, tiers)
            seconds = round(time.monotonic() - started, 1)
            calls = rt.operational.list_llm_calls(limit=100000)
            usage = {"calls": len(calls), "failed_calls": sum(1 for c in calls if c.get("status") != "ok"),
                     "tokens": sum(int((c.get("usage") or {}).get("input_tokens") or 0) + int((c.get("usage") or {}).get("output_tokens") or 0) + int((c.get("usage") or {}).get("total_tokens") or 0) for c in calls),
                     "cost_usd": round(sum(float(c.get("cost_usd") or 0) for c in calls), 4), "models": sorted({str(c.get("model")) for c in calls})}
            report["agents"][provider] = {"describe": rt.gateway.describe()["providers"], "examples": len(results), "passed": sum(r.passed for r in results), "seconds": seconds, "usage": usage,
                                          "failed": [{"skill": r.skill, "example": r.name, "errors": r.errors[:3]} for r in results if not r.passed]}
        finally:
            rt.close()
    (out / "providers.json").write_text(json.dumps(report, indent=2, ensure_ascii=False), encoding="utf-8")
    return report
