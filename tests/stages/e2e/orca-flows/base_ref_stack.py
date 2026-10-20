"""Orca side of the base-branch suite (CR-REQ-039): the project bound to the fixture repo, Requests with a base ref, and the
walk from a created `task` Request to an approved Plan (which makes task-service create a worktree)."""
from __future__ import annotations

import json
import re

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ledgerlite_fixture import LedgerliteRepo
from suite_request_plan_stage import propose_and_wait
from ws_rpc_client import RpcError

LEAF_TASK_TYPES = ["task", "bug", "feature", "epic"]


def register_project(ctx: AcceptanceContext, repo: LedgerliteRepo, suffix: str, engine: str = "native") -> str:
    """A project on the dev server whose repo is the fixture. The engine is set explicitly: the deployment default is OpenSpec,
    and the fake claude cannot load a knowledge hub."""
    ws = ctx.ws
    if not ctx.state.get("dev_server_id") or not ctx.state.get("project_has_dev_server", True):
        raise Blocked("needs the devserver suite (an approved, connected dev server)")
    proj = ws.call("project.create", {"name": f"{ctx.cfg.run_prefix}-{suffix}", "description": "orca e2e base ref", "devServerId": ctx.state["dev_server_id"],
                                       "repoPath": repo.path, "defaultBranch": "main", "visibility": "private"})
    ctx.on_cleanup(f"delete project {proj['id']}", lambda: ws.call("project.delete", {"projectId": proj["id"]}))
    cur = ws.call("request.engineSettings", {"projectId": proj["id"]})
    ws.call("request.engineSettingsSet", {"projectId": proj["id"], "engine": engine, "openspecMinVersion": cur.get("openspecMinVersion", "") or "",
                                          "solutionAgent": False, "executionMode": cur.get("executionMode", "") or "", "expectedVersion": cur.get("version", 0) or 0,
                                          "planAutoGenerate": False})
    return proj["id"]


def repo_id_of(ctx: AcceptanceContext, project_id: str) -> str:
    """The id of the repo the project is bound to (repo.* channels take a repoId, not a projectId)."""
    repos = ctx.ws.call("repo.list", {}).get("repos", [])
    mine = [r for r in repos if r.get("projectId") == project_id]
    assert mine, f"project {project_id} has no repo in repo.list"
    return mine[0]["id"]


def cancel_when_idle(ctx: AcceptanceContext, request_id: str) -> None:
    """Cancel is refused while a task of the Request runs (agent guard); an executed Request is retried until its tasks settle."""
    def cancelled():
        try:
            drv.cancel_if_open(ctx, request_id)
            return True
        except RpcError:
            return None
    wait_until(cancelled, timeout=90, interval=3.0, what=f"Request {request_id} to be cancellable")


def create_request(ctx: AcceptanceContext, project_id: str, title: str, base_ref: str | None = None) -> dict:
    """request.create; `base_ref=None` leaves the key out (a client that does not know the field). Returns the reply."""
    args = {"projectId": project_id, "title": f"{ctx.cfg.run_prefix} {title}", "body": "Add a function subtract(a, b) that returns a minus b, with one unit test.",
            "clientRequestId": ""}
    if base_ref is not None:
        args["baseRef"] = base_ref
    reply = ctx.ws.call("request.create", args)
    rid = reply["request"]["id"]
    ctx.on_cleanup(f"cancel request {rid}", lambda: cancel_when_idle(ctx, rid))
    return reply


def error_trailer(exc: RpcError) -> dict:
    """The ` {json}` the gateway appends to a REQUEST_BASE_REF_* message (lock reason, suggested refs)."""
    m = re.search(r"(\{.*\})\s*$", exc.message)
    return json.loads(m.group(1)) if m else {}


def expect_error(code: str, call) -> RpcError:
    try:
        call()
    except RpcError as exc:
        assert exc.has_code(code), f"expected {code}, got: {exc.message[:240]}"
        return exc
    raise AssertionError(f"the call succeeded, expected {code}")


def leaf_task_ids(ctx: AcceptanceContext, project_id: str, request_id: str) -> list[str]:
    reply = ctx.ws.call("task.list", {"projectId": project_id, "requestId": request_id, "taskTypes": LEAF_TASK_TYPES, "pageSize": 100})
    tasks = reply.get("tasks", reply) if isinstance(reply, dict) else reply
    return [t["id"] for t in tasks]


def plan_and_approve(ctx: AcceptanceContext, request_id: str) -> None:
    """planning -> propose (fake or real claude) -> commit -> approve the task_list. Approving moves the Request to executing, where
    task-service creates the worktree of the first task before it hands the task to the agent."""
    ws = ctx.ws
    view = propose_and_wait(ctx, request_id)
    done = ws.call("request.generatePlan", {"id": request_id, "mode": "commit", "proposal": view["proposal"], "rawAiResponse": view.get("rawAiResponse", "")}, timeout=90)
    pending = [a for a in ws.call("approval.list", {"requestId": request_id, "status": "pending"}).get("approvals", [])
               if a["subjectType"] == "task_list" and a["subjectId"] == done["planTaskId"]]
    assert pending, f"no pending task_list approval on the Plan {done.get('planTaskId')!r}"
    res = ws.call("approval.approve", {"id": pending[0]["id"], "comment": "e2e base ref", "expectedVersion": pending[0]["version"], "expectedDigest": pending[0]["subjectDigest"]})
    assert res.get("requestStatus") == "executing", f"after approving the Plan the Request is {res.get('requestStatus')!r}"


def ai_proposed_a_type(request: dict) -> bool:
    """True when the classifier really answered; the Plan step needs the same AI."""
    return bool(request.get("type")) and request.get("classificationReason", "") not in drv.AI_FAILURE_REASONS


def approve_first_solution(ctx: AcceptanceContext, request_id: str) -> None:
    """analyzing -> Solution generated, first option chosen and its approval granted -> planning (the change_request path)."""
    ws = ctx.ws
    started = ws.call("solution.generate", {"requestId": request_id}, timeout=120)

    def solved():
        listed = ws.call("solution.list", {"requestId": request_id})
        run = next((r for r in listed.get("runs", []) if r["id"] == started["runId"]), None)
        return listed if run and run["status"] != "running" else None
    solution = wait_until(solved, timeout=300, interval=2.0, what="the Solution run to finish")["solutions"][-1]
    if any(isinstance(q, dict) and q.get("blocking") for q in solution["options"].get("open_questions") or []):
        raise Blocked("the AI's Solution has blocking open questions, which this suite does not answer")
    chosen = ws.call("solution.choose", {"requestId": request_id, "solutionId": solution["id"], "optionId": solution["options"]["options"][0]["id"], "comment": "e2e"})
    assert not chosen.get("requiresConfirmation"), "the chosen option needs a Decision confirmation; the suite cannot approve it"
    approval = next(a for a in ws.call("approval.list", {"requestId": request_id, "status": "pending"})["approvals"] if a["subjectType"] == "solution")
    ws.call("approval.approve", {"id": approval["id"], "comment": "e2e", "expectedVersion": approval["version"], "expectedDigest": chosen["approvalDigest"]})
