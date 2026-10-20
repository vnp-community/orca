"""DEV: the developer's machine is registered as a dev server, approved, connected, and a project can bind to it."""
from __future__ import annotations

import shutil
import subprocess
import tempfile

from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked
from ws_rpc_client import RpcError


def _make_scratch_repo(prefix: str) -> str:
    if not shutil.which("git"):
        raise Blocked("git not installed on the test runner")
    path = tempfile.mkdtemp(prefix=f"{prefix}-repo-")
    for cmd in (["git", "init", "-q", "-b", "main"], ["git", "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", "commit", "-q", "--allow-empty", "-m", "init"]):
        subprocess.run(cmd, cwd=path, check=True, capture_output=True)
    return path


def run(ctx: AcceptanceContext) -> None:
    rec, cfg, ws = ctx.rec, ctx.cfg, ctx.ws
    found: dict = {}

    def listed():
        servers = ws.call("devServer.list", {})
        match = [s for s in servers if s.get("name") == cfg.dev_server_name]
        if not match:
            raise Blocked(f"no dev server named {cfg.dev_server_name!r}; start the agent (see README: dev server setup)")
        found.update(match[0])
        ctx.state["dev_server_id"] = found["id"]
    rec.run("DEV-01", f"Dev server '{cfg.dev_server_name}' is registered", listed)

    def approved():
        if not found:
            raise Blocked("dev server not registered")
        assert found.get("approvalStatus") == "approved", f"approvalStatus={found.get('approvalStatus')!r}; an admin must approve it (devServer.approve)"
    rec.run("DEV-02", "Dev server is approved", approved)

    def connected():
        if not found:
            raise Blocked("dev server not registered")
        fresh = next((s for s in ws.call("devServer.list", {}) if s["id"] == found["id"]), None)
        assert fresh and fresh.get("status") == "connected", f"status={fresh and fresh.get('status')!r}; the agent process is not connected"
    rec.run("DEV-03", "Dev server agent is connected", connected)

    repo = {"path": cfg.repo_path}

    def project_with_dev_server():
        if not found or found.get("approvalStatus") != "approved":
            raise Blocked("needs an approved, connected dev server")
        if not repo["path"]:
            repo["path"] = _make_scratch_repo(cfg.run_prefix)  # assumes the dev server is this machine
        proj = ws.call("project.create", {"name": f"{cfg.run_prefix}-project", "description": "orca e2e acceptance",
                                           "devServerId": found["id"], "repoPath": repo["path"], "defaultBranch": "main", "visibility": "private"})
        assert proj.get("id"), f"no project id in {proj!r}"
        assert proj.get("devServerId") == found["id"], "project is not bound to the dev server"
        ctx.state["project_id"] = proj["id"]
        ctx.state["repo_path"] = repo["path"]
        ctx.on_cleanup(f"delete project {proj['id']}", lambda: ws.call("project.delete", {"projectId": proj["id"]}))
        if ctx.state.get("ai_stub"):
            _native_without_knowledge_hub(ws, proj["id"])
    rec.run("DEV-04", "Project can be created on the dev server (validates repo path via the agent)", project_with_dev_server)

    def rejects_missing_path():
        if not found or found.get("approvalStatus") != "approved":
            raise Blocked("needs an approved, connected dev server")
        try:
            ws.call("project.create", {"name": f"{cfg.run_prefix}-badpath", "devServerId": found["id"],
                                       "repoPath": "/nonexistent/" + cfg.run_prefix, "defaultBranch": "main", "visibility": "private"})
        except RpcError as exc:
            assert exc.has_code("PROJECT_REPO_PATH_NOT_FOUND"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("a project was created for a path that does not exist on the dev server")
    rec.run("DEV-05", "Nonexistent repo path is rejected (PROJECT_REPO_PATH_NOT_FOUND)", rejects_missing_path)

    def rejects_unknown_dev_server():
        try:
            ws.call("project.create", {"name": f"{cfg.run_prefix}-nods", "devServerId": "00000000-0000-0000-0000-000000000000",
                                       "repoPath": repo["path"] or "/tmp", "defaultBranch": "main", "visibility": "private"})
        except RpcError as exc:
            assert exc.has_code("PROJECT_DEV_SERVER_NOT_FOUND") or exc.has_code("NOT_FOUND"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("a project was created on a dev server that does not exist")
    rec.run("DEV-06", "Unknown dev server id is rejected", rejects_unknown_dev_server)

    def listed_in_projects():
        pid = ctx.state.get("project_id")
        if not pid:
            raise Blocked("DEV-04 did not create a project")
        assert any(p["id"] == pid for p in ws.call("project.list", {})), "created project missing from project.list"
    rec.run("DEV-07", "Created project appears in project.list", listed_in_projects)

    # Request and workflow suites need some project; fall back to one without a dev server so they can still run.
    if not ctx.state.get("project_id"):
        try:
            proj = ws.call("project.create", {"name": f"{cfg.run_prefix}-project-nods", "defaultBranch": "main", "visibility": "private"})
            ctx.state["project_id"] = proj["id"]
            ctx.state["project_has_dev_server"] = False
            ctx.on_cleanup(f"delete project {proj['id']}", lambda: ws.call("project.delete", {"projectId": proj["id"]}))
        except RpcError:
            pass


def _native_without_knowledge_hub(ws, project_id: str) -> None:
    """The fake claude cannot load a knowledge hub (MCP), and the deployment default is OpenSpec plus an agent-written Solution, which need one.
    Under --ai-stub the test project runs the native engine with the plain Solution, as the real-AI runs of a throwaway repo do."""
    cur = ws.call("request.engineSettings", {"projectId": project_id})
    ws.call("request.engineSettingsSet", {
        "projectId": project_id, "engine": "native", "openspecMinVersion": cur.get("openspecMinVersion", "") or "",
        "solutionAgent": False, "executionMode": cur.get("executionMode", "") or "", "expectedVersion": cur.get("version", 0) or 0,
        "planAutoGenerate": False,  # the suites press Generate plan themselves; automatic generation (CR-REQ-044) has its own suite
    })
