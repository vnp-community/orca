"""Shared pieces of the acceptance runs: fresh runtimes, usage accounting, provider facts, JSON output."""
from __future__ import annotations

import copy
import json
import shutil
import subprocess
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

from ..config import Config
from ..runtime import Runtime
from ..store.base import OperationalStore, SourceDoc

DATA = Path(__file__).resolve().parent / "data"
ORACLES = Path(__file__).resolve().parent / "oracles"


@dataclass
class Check:
    id: str
    passed: bool
    detail: str = ""

    def as_dict(self) -> dict:
        return asdict(self)


def fresh_runtime(base: Config, out: Path, name: str) -> Runtime:
    """A new runtime with its own SQLite stores (real provider, nothing shared between repetitions)."""
    folder = out / name
    if folder.exists():
        shutil.rmtree(folder)
    folder.mkdir(parents=True)
    overrides = {"store": {"knowledge": f"sqlite:{folder}/knowledge.db", "operational": f"sqlite:{folder}/operational.db"},
                 "paths": {"work_dir": str(folder / "work")}, "obs": {"log_file": str(folder / "log.jsonl")},
                 "approvals": {"mode": "manual"}}  # the acceptance runs record the approvals of the registered approver, as they always did
    return Runtime.build(derive_config(base, overrides), configure_logging=True)


def derive_config(base: Config, overrides: dict) -> Config:
    """The effective configuration of the acceptance run (approvers, provider, models) with only the stores/paths/log replaced."""
    from ..config import _merge, _validate
    data = copy.deepcopy(base.data)
    source = dict(base.source)
    _merge(data, overrides, "", source, "acceptance")
    _validate(data)
    return Config(data=data, base_dir=base.base_dir, source=source)


def usage_summary(store: OperationalStore, run_id: str | None = None) -> dict:
    calls = store.list_llm_calls(run_id, limit=100000)
    totals: dict[str, float] = {}
    for c in calls:
        for k, v in (c.get("usage") or {}).items():
            totals[k] = totals.get(k, 0) + v
    return {"calls": len(calls), "blocked_calls": sum(1 for c in calls if c.get("status") != "ok"), "cost_usd": round(sum(c.get("cost_usd") or 0 for c in calls), 4),
            "latency_s": round(sum(c.get("latency_ms", 0) for c in calls) / 1000, 1), "tokens": totals, "by_skill": _by_skill(calls),
            "evidence_grades": sorted({c.get("evidence_grade") for c in calls}), "fixture_calls": sum(1 for c in calls if c.get("evidence_grade") != "real")}


def _by_skill(calls: list[dict]) -> dict:
    out: dict[str, int] = {}
    for c in calls:
        sid = (c.get("skill") or {}).get("id", "-")
        out[sid] = out.get(sid, 0) + 1
    return out


def provider_facts(rt: Runtime) -> dict:
    return {"gateway": rt.gateway.describe(), "config_hash": rt.config.fingerprint()}


def source(name: str, role: str, revision: str = "1", doc_id: str | None = None, path: Path | None = None) -> SourceDoc:
    p = path or (DATA / name)
    return SourceDoc.make(doc_id or role, revision, role, p.read_text(encoding="utf-8"))


def pbom_sources(root: Path) -> list[SourceDoc]:
    d = root / "data" / "test-pbom"
    return [SourceDoc.make("URD", "1.0", "URD", (d / "URD.md").read_text(encoding="utf-8")), SourceDoc.make("SRS", "1.0", "SRS", (d / "SRS.md").read_text(encoding="utf-8")),
            SourceDoc.make("SAD", "1.0", "SAD", (d / "SAD.md").read_text(encoding="utf-8"))]


def repo_root() -> Path:
    here = Path(__file__).resolve()
    for p in here.parents:
        if (p / "data" / "test-pbom").is_dir():
            return p
    raise FileNotFoundError("repository root with data/test-pbom not found")


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False, default=str) + "\n", encoding="utf-8")


def git_head(root: Path) -> str | None:
    try:
        return subprocess.run(["git", "-C", str(root), "rev-parse", "HEAD"], capture_output=True, text=True, timeout=10).stdout.strip() or None
    except (OSError, subprocess.SubprocessError):
        return None


class Timer:
    def __enter__(self):
        self.t0 = time.monotonic()
        return self

    def __exit__(self, *exc):
        self.seconds = round(time.monotonic() - self.t0, 1)
