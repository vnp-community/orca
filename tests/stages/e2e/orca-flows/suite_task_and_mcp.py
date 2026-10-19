"""TSK / MCP: Task CRUD through the gateway, agent execution (opt-in), and MCP availability."""
from __future__ import annotations

from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked, Skipped
from orca_session import http_request
from ws_rpc_client import RpcError


def _delete_quietly(ws, task_id: str) -> None:
    try:
        ws.call("task.delete", {"id": task_id})
    except RpcError:
        pass  # already deleted by the test body


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    held: dict = {}
    pid = ctx.state.get("project_id", "")

    def create():
        if not pid:
            raise Blocked("no project (see DEV-04)")
        task = ws.call("task.create", {"title": f"{cfg.run_prefix} task", "projectId": pid})
        assert task.get("id"), f"no id in {task!r}"
        held["id"] = task["id"]
        ctx.on_cleanup(f"delete task {task['id']}", lambda: _delete_quietly(ws, task["id"]))
        assert task.get("status") in ("open", None, ""), f"new task status {task.get('status')!r}"
    rec.run("TSK-01", "A task can be created in a project", create)

    def read_back():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        got = ws.call("task.get", {"id": held["id"]})
        assert got.get("title") == f"{cfg.run_prefix} task", f"title {got.get('title')!r}"
    rec.run("TSK-02", "task.get returns what was created", read_back)

    def update_title():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        upd = ws.call("task.update", {"id": held["id"], "title": f"{cfg.run_prefix} renamed"})
        assert upd.get("title") == f"{cfg.run_prefix} renamed", f"title after update {upd.get('title')!r}"
    rec.run("TSK-03", "task.update renames a task", update_title)

    def listed():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        reply = ws.call("task.list", {"projectId": pid, "pageSize": 100})
        tasks = reply.get("tasks", reply) if isinstance(reply, dict) else reply
        assert any(t.get("id") == held["id"] for t in tasks), "created task is missing from task.list"
    rec.run("TSK-04", "task.list includes the task (camelCase fields)", listed)

    def title_required():
        if not pid:
            raise Blocked("no project (see DEV-04)")
        try:
            ws.call("task.create", {"title": "", "projectId": pid})
        except RpcError:
            return
        raise AssertionError("a task without a title was created")
    rec.run("TSK-05", "A task without a title is rejected", title_required)

    def cannot_force_in_progress():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        try:
            ws.call("task.update", {"id": held["id"], "status": "in_progress"})
        except RpcError:
            return
        raise AssertionError("status in_progress was set through update; only execute may do that")
    rec.run("TSK-06", "in_progress cannot be set through task.update", cannot_force_in_progress)

    def child_task():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        child = ws.call("task.create", {"title": f"{cfg.run_prefix} child", "projectId": pid, "parentId": held["id"]})
        ctx.on_cleanup(f"delete task {child['id']}", lambda: _delete_quietly(ws, child["id"]))
        sub = ws.call("task.getSubtree", {"rootId": held["id"]})
        assert child["id"] in str(sub), "child task missing from the parent's subtree"
    rec.run("TSK-07", "A child task appears in the parent's subtree", child_task)

    def execute_on_dev_server():
        if not cfg.allow_agent_execution:
            raise Skipped("opt-in: set ORCA_E2E_EXECUTE_AGENT=1 (runs an AI agent on the dev server and spends AI usage)")
        if "id" not in held or not ctx.state.get("repo_path"):
            raise Blocked("needs a task and a project bound to a connected dev server")
        reply = ws.call("task.execute", {"id": held["id"]}, timeout=60)
        assert reply, "task.execute returned nothing"
    rec.run("TSK-08", "task.execute hands the task to the agent on the dev server", execute_on_dev_server)

    def delete_task():
        if "id" not in held:
            raise Blocked("TSK-01 did not create a task")
        ws.call("task.delete", {"id": held["id"]})
        try:
            ws.call("task.get", {"id": held["id"]})
        except RpcError:
            return
        raise AssertionError("a deleted task can still be read")
    rec.run("TSK-09", "A deleted task is gone", delete_task)

    def delete_missing_is_not_found():
        try:
            ws.call("task.delete", {"id": "00000000-0000-0000-0000-000000000000"})
        except RpcError as exc:
            assert exc.has_code("NOT_FOUND") or "NotFound" in exc.message, f"deleting a task that does not exist returned: {exc.message[:160]}"
            return
        raise AssertionError("deleting a task that does not exist reported success")
    rec.run("TSK-10", "Deleting a task that does not exist is NOT_FOUND (not an internal error)", delete_missing_is_not_found)

    mcp_on = cfg.stack_env.get("MCP_ENABLED", "").lower() == "true"

    def mcp_discovery():
        if not mcp_on:
            raise Blocked("MCP_ENABLED is not true in deploy/dev/.env")
        r = http_request("GET", cfg.gateway_url + "/.well-known/oauth-protected-resource")
        assert r.status == 200, f"protected-resource metadata -> {r.status}"
    rec.run("MCP-01", "MCP publishes OAuth protected-resource metadata", mcp_discovery)

    def mcp_requires_auth():
        if not mcp_on:
            raise Blocked("MCP_ENABLED is not true in deploy/dev/.env")
        r = http_request("POST", cfg.gateway_url + "/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
        assert r.status == 401, f"anonymous tools/list -> {r.status}, expected 401"
    rec.run("MCP-02", "MCP rejects anonymous tool listing", mcp_requires_auth)
