"""Shared helpers of the split-topology suites: login, dev servers, projects, workflow runs, docker probes, report."""
from __future__ import annotations

import json
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Callable

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "orca-flows"))

from acceptance_context import AcceptanceContext  # noqa: E402
from acceptance_recorder import AcceptanceRecorder, Blocked  # noqa: E402
from orca_session import OrcaSession  # noqa: E402
from orca_stack_config import load_config  # noqa: E402
from ws_rpc_client import RpcError  # noqa: E402

TERMINAL = ("completed", "failed", "cancelled")
POSTGRES = "orca-go-postgres"


def devserver_container(name: str) -> str:
    return f"orca-e2e-{name.removeprefix('e2e-')}"


def docker(*args: str, timeout: float = 60) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", *args], capture_output=True, text=True, timeout=timeout)


def in_devserver(name: str, script: str, timeout: float = 60) -> subprocess.CompletedProcess:
    return docker("exec", devserver_container(name), "bash", "-lc", script, timeout=timeout)


def open_context(run_id: str) -> AcceptanceContext:
    cfg = load_config(run_id)
    session = OrcaSession(cfg)
    if session.login().status != 200:
        raise SystemExit(f"cannot log in to {cfg.gateway_url} as the stack admin")
    return AcceptanceContext(cfg=cfg, session=session, rec=AcceptanceRecorder())


def dev_server(ctx: AcceptanceContext, name: str) -> dict:
    found = next((s for s in ctx.ws.call("devServer.list", {}) if s.get("name") == name), None)
    if not found:
        raise Blocked(f"dev server {name!r} is not registered: run split-up.sh")
    return found


def project_on(ctx: AcceptanceContext, server_name: str, repo_path: str, label: str) -> str:
    """A project bound to repo_path on that dev server (project.create checks the path through the agent)."""
    server = dev_server(ctx, server_name)
    proj = ctx.ws.call("project.create", {"name": f"{ctx.cfg.run_prefix}-{label}", "description": "split-topology e2e",
                                          "devServerId": server["id"], "repoPath": repo_path, "defaultBranch": "main",
                                          "visibility": "private"})
    ctx.on_cleanup(f"delete project {proj['id']}", lambda: ctx.ws.call("project.delete", {"projectId": proj["id"]}))
    return proj["id"]


def create_template(ctx: AcceptanceContext, label: str, steps: list[dict]) -> str:
    tpl = ctx.ws.call("workflow.template.create", {"name": f"{ctx.cfg.run_prefix}-{label}", "dagJson": json.dumps({"steps": steps}),
                                                   "scope": "personal"})
    return tpl["id"]


def execute(ctx: AcceptanceContext, template_id: str, project_id: str, inputs: dict | None = None) -> str:
    ex = ctx.ws.call("workflow.execute", {"templateId": template_id, "projectId": project_id, "inputs": inputs or {},
                                          "requestId": f"{ctx.cfg.run_prefix}-{template_id[:8]}-{int(time.time())}"})
    return ex["id"]


def execution_status(ctx: AcceptanceContext, execution_id: str) -> str:
    return ctx.ws.call("workflow.getExecution", {"executionId": execution_id}).get("status", "")


def settle(ctx: AcceptanceContext, execution_id: str, timeout: float = 120, until: Callable[[], bool] | None = None) -> str:
    deadline = time.time() + timeout
    status = ""
    while time.time() < deadline:
        status = execution_status(ctx, execution_id)
        if status in TERMINAL or (until and until()):
            return status
        time.sleep(2)
    raise AssertionError(f"execution {execution_id[:8]} still {status!r} after {timeout:.0f}s")


def step_rows(execution_id: str) -> dict[str, dict[str, Any]]:
    """step_id -> {status, wave, output}; read from the workflow database (the WS API does not return steps)."""
    sql = (f"select step_id,status,wave,coalesce(output::text,'null'),coalesce(error_message,'') from workflow.step_executions "
           f"where execution_id='{execution_id}' order by wave,step_id")
    out = docker("exec", POSTGRES, "psql", "-U", "orca", "-d", "workflow", "-At", "-F", "\x1f", "-c", sql, timeout=20)
    if out.returncode != 0:
        raise Blocked(f"cannot query the workflow DB: {out.stderr.strip()[:160]}")
    rows = {}
    for line in out.stdout.splitlines():
        step_id, status, wave, output, error = line.split("\x1f", 4)
        rows[step_id] = {"status": status, "wave": int(wave), "output": json.loads(output), "error": error}
    return rows


def wait_step(execution_id: str, step_id: str, statuses: tuple[str, ...], timeout: float = 120) -> dict:
    deadline = time.time() + timeout
    row: dict = {}
    while time.time() < deadline:
        row = step_rows(execution_id).get(step_id, {})
        if row.get("status") in statuses:
            return row
        time.sleep(2)
    raise AssertionError(f"step {step_id} is {row.get('status')!r}, expected {statuses} within {timeout:.0f}s")


def write_report(name: str, ctx: AcceptanceContext, extra: dict | None = None) -> int:
    reports = HERE / "reports"
    reports.mkdir(exist_ok=True)
    rows = ctx.rec.as_dicts()
    counts = ctx.rec.counts()
    (reports / f"{name}.json").write_text(json.dumps({"counts": counts, "results": rows, **(extra or {})}, indent=2, ensure_ascii=False))
    lines = [f"# {name}", "", f"PASS {counts.get('PASS', 0)} · FAIL {counts.get('FAIL', 0)} · BLOCKED {counts.get('BLOCKED', 0)} · SKIP {counts.get('SKIP', 0)}", "",
             "| ID | Kết quả | Kiểm | Ghi chú |", "|---|---|---|---|"]
    for r in rows:
        lines.append(f"| {r.get('test_id')} | {r.get('status')} | {r.get('title')} | {str(r.get('detail') or '')[:300].replace('|', '/')} |")
    (reports / f"{name}.md").write_text("\n".join(lines) + "\n")
    print("\n".join(lines))
    return 1 if counts.get("FAIL") else 0


__all__ = ["Blocked", "RpcError", "AcceptanceContext", "open_context", "dev_server", "project_on", "create_template", "execute",
           "execution_status", "settle", "step_rows", "wait_step", "in_devserver", "docker", "devserver_container", "write_report"]
