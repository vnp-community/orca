"""Background jobs for the MCP server: one JSON file per job under <projectDir>/.git/tsa/jobs, run by a detached `tsa run-job` process.

A job is keyed by the caller's `external` id plus feature and spec version, so the same start sent twice (a duplicate event, a reconcile
pass) returns the same job. A finished job in error or cancelled is started again on the next start with the same key.
"""
from __future__ import annotations

import hashlib
import json
import os
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Any

from .errors import TsaError

STATES = ("queued", "running", "done", "error", "cancelled")
RESTARTABLE = ("error", "cancelled")
REPORT_MAX_CHARS = 20000


class JobError(TsaError):
    code = "TSA_JOB"


def jobs_dir(project_dir: Path) -> Path:
    """<git common dir>/tsa/jobs: inside .git so job files never show up as untracked files of the project; .tsa/jobs otherwise."""
    project_dir = Path(project_dir)
    proc = subprocess.run(["git", "rev-parse", "--git-common-dir"], cwd=project_dir, capture_output=True, text=True)
    if proc.returncode == 0 and proc.stdout.strip():
        common = Path(proc.stdout.strip())
        return (common if common.is_absolute() else project_dir / common) / "tsa" / "jobs"
    return project_dir / ".tsa" / "jobs"


def job_key(external: dict, feature: str, spec_version: int | None) -> str:
    raw = json.dumps({"external": external or {}, "feature": feature, "specVersion": spec_version}, sort_keys=True)
    return hashlib.sha256(raw.encode()).hexdigest()


def job_id_for(key: str) -> str:
    return "tsa-" + key[:16]


def _now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _path(project_dir: Path, job_id: str) -> Path:
    if not job_id.startswith("tsa-") or not job_id[4:].isalnum():
        raise JobError(f"invalid job id {job_id!r}", code="TSA_JOB_NOT_FOUND")
    return jobs_dir(project_dir) / f"{job_id}.json"


def read(project_dir: Path, job_id: str) -> dict:
    p = _path(project_dir, job_id)
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except FileNotFoundError:
        raise JobError(f"no job {job_id}", code="TSA_JOB_NOT_FOUND") from None


def write(project_dir: Path, job: dict) -> None:
    job["updated_at"] = _now()
    d = jobs_dir(project_dir)
    d.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=d, suffix=".tmp")
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        json.dump(job, fh, ensure_ascii=False, indent=1)
    os.replace(tmp, _path(project_dir, job["job_id"]))


def alive(pid: int | None) -> bool:
    if not pid:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    try:   # a zombie child of this process counts as finished
        return os.waitpid(pid, os.WNOHANG) == (0, 0)
    except ChildProcessError:
        return True


def start(project_dir: Path, params: dict, spawn: bool = True) -> tuple[dict, bool]:
    """Create (or find) the job for `params` and start its process. Returns (job, reused)."""
    project_dir = Path(project_dir)
    key = job_key(params.get("external") or {}, params["feature"], params.get("spec_version"))
    jid = job_id_for(key)
    try:
        job = read(project_dir, jid)
        if job["state"] not in RESTARTABLE:
            return job, True
        attempt = int(job.get("attempt", 1)) + 1
    except JobError:
        attempt = 1
    job = {"job_id": jid, "key": key, "attempt": attempt, "state": "queued", "params": params, "project_dir": str(project_dir),
           "created_at": _now(), "pid": None, "result": None, "error": None}
    write(project_dir, job)
    if spawn:
        log = jobs_dir(project_dir) / f"{jid}.log"
        with open(log, "ab") as fh:
            proc = subprocess.Popen([sys.executable, "-m", "tsa.cli", "-q", "run-job", "--project-dir", str(project_dir), "--job", jid],
                                    stdout=fh, stderr=fh, stdin=subprocess.DEVNULL, start_new_session=True, cwd=project_dir)
        job["pid"] = proc.pid
        write(project_dir, job)
    return job, False


def view(job: dict) -> dict:
    """What tsa_get returns: the state, with a lost process turned into an error."""
    out = {k: job.get(k) for k in ("job_id", "attempt", "state", "created_at", "updated_at", "error")}
    if job["state"] in ("queued", "running") and job.get("pid") and not alive(job["pid"]):
        out["state"] = "error"
        out["error"] = {"code": "TSA_JOB_LOST", "message": "the job process ended without writing a result"}
    res = job.get("result") or {}
    out.update({"feature": job["params"].get("feature"), "spec_version": job["params"].get("spec_version"),
                "branch": res.get("branch") or job["params"].get("branch"), "worktree": res.get("worktree"), "commit": res.get("commit"),
                "counts": res.get("counts"), "files": res.get("files"), "skipped": res.get("skipped"), "report": res.get("report"),
                "manifest": res.get("manifest"), "generator": res.get("generator"), "cost_usd": res.get("costUsd"),
                "seconds": res.get("seconds")})
    return {k: v for k, v in out.items() if v is not None}


def cancel(project_dir: Path, job_id: str, reason: str = "") -> dict:
    job = read(project_dir, job_id)
    if job["state"] in ("queued", "running"):
        pid = job.get("pid")
        if pid and alive(pid):
            try:
                os.killpg(pid, signal.SIGTERM)
            except (ProcessLookupError, PermissionError):
                pass
        job["state"] = "cancelled"
        job["error"] = {"code": "TSA_CANCELLED", "message": reason or "cancelled"}
        write(project_dir, job)
    return job


def report(project_dir: Path, job_id: str, offset: int = 0, max_chars: int = REPORT_MAX_CHARS) -> dict[str, Any]:
    job = read(project_dir, job_id)
    res = job.get("result") or {}
    if not res.get("report") or not res.get("worktree"):
        raise JobError(f"job {job_id} has no report yet (state {job['state']})", code="TSA_REPORT_NOT_READY")
    text = (Path(res["worktree"]) / res["report"]).read_text(encoding="utf-8")
    part = text[offset:offset + max_chars]
    nxt = offset + max_chars if offset + max_chars < len(text) else None
    return {"job_id": job_id, "path": res["report"], "offset": offset, "next": nxt, "total": len(text), "text": part}


def outputs(job: dict) -> list[str]:
    """Files a job wrote and a caller may read back: the generated scripts, the manifest and the report (nothing else of the worktree)."""
    res = job.get("result") or {}
    out = [f["path"] for f in res.get("files") or [] if f.get("path")]
    return out + [p for p in (res.get("manifest"), res.get("report")) if p]


def read_output(project_dir: Path, job_id: str, path: str, offset: int = 0, max_chars: int = REPORT_MAX_CHARS) -> dict[str, Any]:
    job = read(project_dir, job_id)
    res = job.get("result") or {}
    if not res.get("worktree"):
        raise JobError(f"job {job_id} has no output yet (state {job['state']})", code="TSA_REPORT_NOT_READY")
    if path not in outputs(job):
        raise JobError(f"{path} is not an output of job {job_id}", code="TSA_FILE_NOT_ALLOWED")
    try:
        text = (Path(res["worktree"]) / path).read_text(encoding="utf-8")
    except FileNotFoundError:
        raise JobError(f"{path} is no longer in the worktree", code="TSA_FILE_NOT_FOUND") from None
    part = text[offset:offset + max_chars]
    nxt = offset + max_chars if offset + max_chars < len(text) else None
    return {"job_id": job_id, "path": path, "offset": offset, "next": nxt, "total": len(text), "text": part}
