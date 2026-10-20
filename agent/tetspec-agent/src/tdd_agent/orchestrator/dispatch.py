"""Background advance of a run for remote clients (Orca workflows) that must not hold a call open while gates run.

``run_dispatch`` starts ``tdd-agent run advance <id> --heartbeat`` in its own session and returns at once; the client then
polls ``run_get``. A small JSON file per run (``<work_dir>/runs/<run_id>.proc.json``) records the process: its pid, when it
was dispatched, its last heartbeat and when it finished. It lives next to the store on the same host as the process, so a
pid is meaningful there. Orchestrator locks are per process and cannot tell whether another process is advancing a run;
this file is what does.
"""
from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import threading
from pathlib import Path
from typing import Any, Self

from ..store.base import now_iso

HEARTBEAT_INTERVAL_S = 15


def proc_file(work_dir: Path, run_id: str) -> Path:
    return Path(work_dir) / "runs" / f"{run_id}.proc.json"


def read_proc(work_dir: Path, run_id: str) -> dict[str, Any] | None:
    """The recorded process of ``run_id`` with ``alive`` checked now, or None if the run was never dispatched."""
    path = proc_file(work_dir, run_id)
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    data["alive"] = bool(data.get("pid")) and not data.get("finished_at") and pid_alive(int(data["pid"]))
    data.setdefault("heartbeat_interval_s", HEARTBEAT_INTERVAL_S)
    return data


def _write(work_dir: Path, run_id: str, data: dict[str, Any]) -> None:
    path = proc_file(work_dir, run_id)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(".tmp")
    tmp.write_text(json.dumps(data, ensure_ascii=False), encoding="utf-8")
    os.replace(tmp, path)  # readers never see a half-written file


def _update(work_dir: Path, run_id: str, **fields: Any) -> None:
    current = read_proc(work_dir, run_id) or {}
    current.pop("alive", None)
    current.update(fields)
    _write(work_dir, run_id, current)


def pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:  # exists, owned by someone else
        return True
    # a finished child that nobody waited for is a zombie: gone for our purposes
    try:
        stat = Path(f"/proc/{pid}/stat").read_text()
        return stat.rsplit(")", 1)[1].split()[0] != "Z"
    except OSError:
        return True


def launch_advance(base_dir: Path, work_dir: Path, run_id: str) -> int:
    """Start the background advance of ``run_id`` and record it; returns the pid."""
    logs = Path(work_dir) / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    log = (logs / f"run-{run_id}.log").open("ab")
    cmd = [sys.executable, "-m", "tdd_agent.cli", "-C", str(base_dir), "run", "advance", run_id, "--heartbeat"]
    proc = subprocess.Popen(cmd, stdout=log, stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, start_new_session=True,
                            close_fds=True, env=os.environ.copy())
    log.close()
    _write(work_dir, run_id, {"run_id": run_id, "pid": proc.pid, "dispatched_at": now_iso(), "heartbeat_at": None,
                              "finished_at": None, "heartbeat_interval_s": HEARTBEAT_INTERVAL_S})
    return proc.pid


LAUNCH = launch_advance  # replaced in tests


def terminate(work_dir: Path, run_id: str) -> int | None:
    """SIGTERM the process group of the run's live background advance; returns the pid that was signalled."""
    proc = read_proc(work_dir, run_id)
    if not proc or not proc["alive"]:
        return None
    pid = int(proc["pid"])
    try:
        os.killpg(pid, signal.SIGTERM)  # start_new_session made the advance its own group leader
    except ProcessLookupError:
        return None
    except PermissionError:
        os.kill(pid, signal.SIGTERM)
    _update(work_dir, run_id, finished_at=now_iso(), exit="terminated")
    return pid


class Heartbeat:
    """Context manager for the advance process: beats every ``interval`` seconds, records the end."""

    def __init__(self, work_dir: Path, run_id: str, interval: float = HEARTBEAT_INTERVAL_S):
        self.work_dir, self.run_id, self.interval = Path(work_dir), run_id, interval
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._loop, name=f"heartbeat-{run_id}", daemon=True)

    def _beat(self) -> None:
        try:
            _update(self.work_dir, self.run_id, pid=os.getpid(), heartbeat_at=now_iso(), heartbeat_interval_s=self.interval)
        except OSError:
            pass  # a missed beat only makes the run look stale sooner; it must never fail the gate

    def _loop(self) -> None:
        while not self._stop.wait(self.interval):
            self._beat()

    def __enter__(self) -> Self:
        self._beat()
        self._thread.start()
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self._stop.set()
        self._thread.join(timeout=self.interval)
        _update(self.work_dir, self.run_id, finished_at=now_iso(), exit="error" if exc_type else "ok")
