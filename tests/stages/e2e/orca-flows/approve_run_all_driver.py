"""Orca side of the ARA suite (CR-REQ-043): a project of a chosen execution mode, and a `change_request` driven through the public WebSocket
channels to the pending Plan approval, where the cases decide how to approve it. The AI is the fake claude (--ai-stub): it answers
classification, Solution and Plan, and (since CR-REQ-043) the task-execution prompt with a valid `done` block."""
from __future__ import annotations

from dataclasses import dataclass

import base_ref_stack as stack
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from ledgerlite_fixture import LedgerliteRepo
from suite_request_plan_stage import propose_and_wait

PLAN_SUBJECTS = ("plan", "task_list")


@dataclass
class PlanGate:
    """A Request waiting for its Plan approval, and the approval itself."""
    request_id: str
    approval: dict
    plan_task_id: str
    phase_count: int


def project_with_mode(ctx: AcceptanceContext, repo: LedgerliteRepo, suffix: str, execution_mode: str) -> str:
    """A native-engine project on the fixture repo whose execution mode is the given one. The engine and `solutionAgent` stay as
    register_project set them (native, off): the deployment default is OpenSpec, which a fake claude cannot author and a real one bills."""
    pid = stack.register_project(ctx, repo, suffix)
    cur = ctx.ws.call("request.engineSettings", {"projectId": pid})
    ctx.ws.call("request.engineSettingsSet", {"projectId": pid, "engine": cur.get("engine") or "native", "openspecMinVersion": cur.get("openspecMinVersion", "") or "",
                                              "solutionAgent": False, "executionMode": execution_mode, "expectedVersion": cur.get("version", 0) or 0,
                                              "planAutoGenerate": False})
    return pid


def to_analysis_gate(ctx: AcceptanceContext, project_id: str, title: str) -> dict:
    """create -> classify -> confirm change_request/M -> readiness answered -> Solution generated and chosen: the Request waits for the
    Solution approval. Returns {request, solution approval, digest}."""
    ws = ctx.ws
    req = stack.create_request(ctx, project_id, title)["request"]
    cur = drv.to_first_stage(ctx, req, "change_request", "M")
    assert cur["status"] == "analyzing", f"expected analyzing, got {cur['status']!r}"
    started = ws.call("solution.generate", {"requestId": req["id"]}, timeout=120)

    def solved():
        listed = ws.call("solution.list", {"requestId": req["id"]})
        run = next((r for r in listed.get("runs", []) if r["id"] == started["runId"]), None)
        return listed if run and run["status"] != "running" else None
    solution = wait_until(solved, timeout=300, interval=2.0, what="the Solution run to finish")["solutions"][-1]
    chosen = ws.call("solution.choose", {"requestId": req["id"], "solutionId": solution["id"], "optionId": solution["options"]["options"][0]["id"], "comment": "e2e"})
    assert not chosen.get("requiresConfirmation"), "the chosen option needs a Decision confirmation; the suite cannot approve it"
    approval = next(a for a in ws.call("approval.list", {"requestId": req["id"], "status": "pending"})["approvals"] if a["subjectType"] == "solution")
    return {"request": req, "approval": approval, "digest": chosen["approvalDigest"]}


def to_plan_approval(ctx: AcceptanceContext, project_id: str, title: str) -> PlanGate:
    """Drives a new Request to `awaiting_plan_approval` with a Plan in Phases and returns the pending approval, undecided."""
    gate = to_analysis_gate(ctx, project_id, title)
    approve_solution_and_plan(ctx, gate["request"]["id"], gate["approval"], gate["digest"])
    return pending_plan(ctx, project_id, gate["request"]["id"])


def approve_solution_and_plan(ctx: AcceptanceContext, request_id: str, solution_approval: dict, digest: str) -> None:
    ws = ctx.ws
    ws.call("approval.approve", {"id": solution_approval["id"], "comment": "e2e", "expectedVersion": solution_approval["version"], "expectedDigest": digest})
    view = propose_and_wait(ctx, request_id)
    done = ws.call("request.generatePlan", {"id": request_id, "mode": "commit", "proposal": view["proposal"], "rawAiResponse": view.get("rawAiResponse", "")}, timeout=90)
    assert done.get("planTaskId"), f"the Plan commit returned no plan id: {done!r}"


def pending_plan(ctx: AcceptanceContext, project_id: str, request_id: str) -> PlanGate:
    pending = ctx.ws.call("approval.list", {"requestId": request_id, "status": "pending"}).get("approvals", [])
    plan = next((a for a in pending if a["subjectType"] in PLAN_SUBJECTS), None)
    assert plan, f"no pending Plan approval on the Request: {[a['subjectType'] for a in pending]}"
    phases = ctx.ws.call("task.list", {"projectId": project_id, "requestId": request_id, "taskTypes": ["phase"], "pageSize": 100})
    listed = phases.get("tasks", phases) if isinstance(phases, dict) else phases
    return PlanGate(request_id=request_id, approval=plan, plan_task_id=plan["subjectId"], phase_count=len(listed))


def approve(ctx: AcceptanceContext, gate: PlanGate, execution_mode: str | None = None, ws=None) -> dict:
    """approval.approve of the Plan; `execution_mode=None` leaves the key out, like a client that does not know it."""
    a = gate.approval
    args = {"id": a["id"], "comment": "e2e", "expectedVersion": a["version"], "expectedDigest": a["subjectDigest"]}
    if execution_mode is not None:
        args["executionMode"] = execution_mode
    return (ws or ctx.ws).call("approval.approve", args)


def pending_subjects(ctx: AcceptanceContext, request_id: str) -> list[str]:
    return [a["subjectType"] for a in ctx.ws.call("approval.list", {"requestId": request_id, "status": "pending"}).get("approvals", [])]


def phase_starts(ctx: AcceptanceContext, request_id: str) -> list[dict]:
    return ctx.ws.call("request.activity", {"id": request_id, "kinds": ["phase.started"], "pageSize": 100}).get("entries", [])


def activity_tail(ctx: AcceptanceContext, request_id: str, count: int = 10) -> str:
    """The latest Activity rows as `kind: summary`, for a failure message (the rows carry no Request content)."""
    rows = ctx.ws.call("request.activity", {"id": request_id, "pageSize": 100}).get("entries", [])
    return " | ".join(f"{r.get('kind')}: {r.get('summary')}" for r in rows[:count])
