"""Runs one MCP job (the detached `tsa run-job` process): project config + job params -> pipeline.run -> result in the job file."""
from __future__ import annotations

import tomllib
from pathlib import Path

from . import jobs, tcir
from .errors import TsaError
from .generators import make
from .pipeline import Options, run

CONFIG_FILE = Path(".tsa") / "config.toml"
DEFAULTS = {"generator": "claude", "model": None, "max_fix": 3, "tests_paths": ["tests"], "test_repo": None, "only_approved": False,
            "tcir_command": None, "tcir_tool": None, "tcir_file": None, "base": "HEAD"}


def project_config(project_dir: Path) -> dict:
    """[tsa] of <projectDir>/.tsa/config.toml (or the top level): defaults a caller's parameters override."""
    p = Path(project_dir) / CONFIG_FILE
    if not p.is_file():
        return {}
    try:
        data = tomllib.loads(p.read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise TsaError(f"{p}: {exc}", code="CONFIG_INVALID") from None
    data = data.get("tsa", data)
    return {k: v for k, v in data.items() if k in DEFAULTS}


def settings(project_dir: Path, params: dict) -> dict:
    return {**DEFAULTS, **project_config(project_dir), **{k: v for k, v in params.items() if v is not None}}


def source_for(project_dir: Path, s: dict):
    if s.get("tcir_file"):
        return tcir.load_file(Path(project_dir) / s["tcir_file"])
    command = s.get("tcir_command") or f"tdd-agent -C {project_dir} mcp serve --transport stdio"
    return tcir.McpSource.from_command_line(command, s.get("tcir_tool"), str(project_dir))


def options_for(project_dir: Path, s: dict) -> Options:
    project_dir = Path(project_dir).resolve()
    repo = Path(s["test_repo"]).resolve() if s.get("test_repo") else project_dir
    # The test repository is the application repository: check out only the test folders so the generator cannot read the app.
    sparse = list(s.get("tests_paths") or ["tests"]) if repo == project_dir else None
    return Options(repo=repo, base=s.get("base") or "HEAD", feature=s["feature"], branch=s.get("branch"), reuse=True,
                   max_fix=int(s.get("max_fix", 3)), only_approved=bool(s.get("only_approved")), sparse_paths=sparse,
                   spec_version=s.get("spec_version"))


def run_job(project_dir: Path, job_id: str) -> int:
    project_dir = Path(project_dir).resolve()
    job = jobs.read(project_dir, job_id)
    if job["state"] == "cancelled":
        return 0
    job["state"] = "running"
    jobs.write(project_dir, job)
    try:
        s = settings(project_dir, job["params"])
        out = run(options_for(project_dir, s), source_for(project_dir, s), make(s["generator"], s.get("model")))
        job["result"], job["state"], job["error"] = out.as_dict(), "done", None
    except TsaError as exc:
        job["state"], job["error"] = "error", {"code": exc.code, "message": str(exc)[:500]}
    except Exception as exc:   # the job file must always end in a final state
        job["state"], job["error"] = "error", {"code": "TSA_INTERNAL", "message": f"{type(exc).__name__}: {exc}"[:500]}
    current = jobs.read(project_dir, job_id)
    if current["state"] == "cancelled":   # cancelled while running: keep the cancellation
        return 0
    jobs.write(project_dir, job)
    return 0 if job["state"] == "done" else 1


