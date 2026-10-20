"""In-process tsagent call for gate TS1 (SOLUTION sections 7, 9.3).

Configuration comes from ``[testspec]`` on every run; results go to ``<work_dir>/tsagent.db`` (versioned test KG), the provenance
to tsagent's ``run.params.origin``, the copy of each run to ``<work_dir>/runs/<run>/testspec/``.
"""
from __future__ import annotations

import hashlib
import json
import shutil
from pathlib import Path

from ...errors import TddAgentError
from .project import dumps

FAILED_PREFIX = "TESTSPEC_FAILED"


def tsagent_config(cfg):
    """The project's ``tsagent.Config``: ``[testspec] llm`` is the only source of the LLM mode (TSAGENT_LLM is ignored)."""
    from tsagent.config import Config
    ts = cfg.section("testspec")
    c = Config()
    c.llm_provider = ts["llm"]
    c.llm_parallel = ts["parallel"]
    c.llm_group_retries = ts["group_retries"]
    c.db = str(cfg.work_dir / "tsagent.db")
    c.cache_dir = str(cfg.work_dir / "tsagent" / "cache")
    c.snapshot_dir = str(cfg.work_dir / "tsagent" / "snapshots")
    c.inbox_dir = str(cfg.work_dir / "tsagent" / "inbox")
    c.specs_root = str(cfg.repo_root / ts["specs_dir"])
    c.source_roots = (str(cfg.work_dir / "runs"),)
    return c


def preflight(cfg) -> dict:
    """Check the LLM configuration (no network). Errors -> TddAgentError carrying tsagent's code (LLM_NOT_CONFIGURED, ...)."""
    from tsagent.errors import ConfigError as TsConfigError
    from tsagent.llm import check_llm
    try:
        return check_llm(tsagent_config(cfg))
    except TsConfigError as exc:
        msg = str(exc).replace("TSAGENT_LLM (or [testspec] llm)", "[testspec] llm in .tdd-agent/config.toml")
        raise TddAgentError(msg, code=exc.code or "LLM_NOT_CONFIGURED") from None


def run_dir(cfg, run_id: str) -> Path:
    return cfg.work_dir / "runs" / run_id / "testspec"


def _failed_code(err: str) -> str:
    if "LLM_GROUPS_FAILED" in err:
        return f"{FAILED_PREFIX}:LLM_GROUPS_FAILED"
    if "LLMPermanent" in err or "LLMError" in err:
        return f"{FAILED_PREFIX}:LLM_ERROR"
    if "ValidationFailed" in err:
        return f"{FAILED_PREFIX}:VALIDATION"
    return f"{FAILED_PREFIX}:RUN_FAILED"


def _covered_codes(db, feature: str) -> set[str]:
    from tsagent.store import tso as store_tso
    if not store_tso.versions(db, feature):
        return set()
    g = store_tso.load_spec(db, feature)
    out: set[str] = set()
    for e in g.entities.values():
        out.update(e.prov.based_on)
        out.update(ev.source_code for ev in e.prov.evidence)
        for vals in e.rels.values():
            out.update(vals)
    return out


def run_packages(cfg, run_id: str, packages: list[tuple[dict, dict]], origin: dict) -> dict:
    """Run tsagent for each package (one feature per package). Returns a summary; non-empty ``blockers`` means a feature failed."""
    from tsagent.errors import TsagentError
    from tsagent.orchestrator import Orchestrator
    from tsagent.store import tso as store_tso
    from tsagent.store.db import open_db
    c = tsagent_config(cfg)
    out_dir = run_dir(cfg, run_id)
    out_dir.mkdir(parents=True, exist_ok=True)
    Path(c.specs_root).mkdir(parents=True, exist_ok=True)
    features, blockers = [], []
    db = open_db(c)
    try:
        orch = Orchestrator(db, c)
        for pkg, rep in packages:
            feature = pkg["feature"]
            path = out_dir / f"{feature}.tdd-kg.json"
            text = dumps(pkg)
            path.write_text(text, encoding="utf-8")
            org = {**origin, "package_sha256": hashlib.sha256(text.encode("utf-8")).hexdigest()}
            row = {"feature": feature, "tdd": rep.get("tdd"), "package": str(path)}
            try:
                run_id, _pkg, dedup = orch.prepare(feature, [str(path)], origin=org)
                # tsagent hands back the finished run of the same input without writing anything. When a different input was written since
                # (another context mode, an edited document that was edited back), that run is no longer what the Draft holds: run again.
                st = orch.start(feature, [str(path)], origin=org, force=dedup and run_id != _latest_finished_run(db, feature))
            except TsagentError as exc:   # package failed the input checks, feature busy...
                code = f"{FAILED_PREFIX}:INPUT_INVALID" if type(exc).__name__ == "InputError" else f"{FAILED_PREFIX}:{type(exc).__name__}"
                diags = [str(d) for d in getattr(exc, "diagnostics", [])][:20]
                row.update(status="failed", error=str(exc)[:500], diagnostics=diags)
                blockers.append(code); features.append(row)
                continue
            res = st.get("result") or {}
            counts = res.get("counts") or {}
            row.update(status=st["state"], tsagent_run=st["runId"], deduplicated=bool(st.get("deduplicated")), error=st.get("error"),
                       tr=counts.get("TestRequirement", 0), tc=counts.get("TestCase", 0), gaps=counts.get("TestGap", 0),
                       warnings=res.get("warnings") or [], notes=(st.get("notes") or [])[-10:],
                       unmapped=rep.get("unmapped", {}), unresolved_refs=rep.get("unresolved_refs", []), missing=rep.get("missing", 0),
                       code_map=rep.get("code_map", {}), context=rep.get("context_doc"))
            if st["state"] != "done":
                blockers.append(_failed_code(str(st.get("error") or "")))
                features.append(row)
                continue
            vs = store_tso.versions(db, feature)
            row["version"] = vs[-1]["version"] if vs else None
            files = [Path(f) for f in res.get("files") or []]
            snap = out_dir / feature
            snap.mkdir(exist_ok=True)
            for f in files:
                if f.is_file():
                    shutil.copy2(f, snap / f.name)
            row["files"] = sorted(_rel(cfg.repo_root, f) for f in files)
            if "acceptance" in rep:
                covered = _covered_codes(db, feature)
                row["uncovered_acceptance"] = sorted(a for a in rep["acceptance"] if a not in covered)
            features.append(row)
            (out_dir / f"{feature}.summary.json").write_text(json.dumps({"origin": org, **row, "status_detail": st}, ensure_ascii=False,
                                                                        indent=1, default=str), encoding="utf-8")
    finally:
        db.close()
    return {"llm": c.llm_provider, "features": features, "blockers": blockers, "run_dir": _rel(cfg.repo_root, out_dir)}


def _latest_finished_run(db, feature: str) -> str | None:
    """The finished (non-dry) tsagent run that wrote the Draft last."""
    for r in db.q("SELECT run_id, result FROM run WHERE feature = ? AND run_key NOT LIKE '%-dry' ORDER BY updated_at DESC, rowid DESC", (feature,)):
        if json.loads(r["result"] or "{}").get("finished"):
            return r["run_id"]
    return None


def _rel(root: Path, p: Path) -> str:
    try:
        return str(Path(p).resolve().relative_to(Path(root).resolve()))
    except ValueError:
        return str(p)
