"""PLN: the Plan stage (CR-REQ-012) with the real AI. Proposing writes nothing; committing writes the Plan and opens the approval."""
from __future__ import annotations

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ws_rpc_client import RpcError

RUN_TIMEOUT = 240  # the AI call plus one corrective retry


def propose_and_wait(ctx: AcceptanceContext, request_id: str, feedback: str = "") -> dict:
    args = {"id": request_id}
    if feedback:
        args["feedback"] = feedback
    started = ctx.ws.call("request.generatePlan", args, timeout=60)
    run_id = started.get("runId")
    assert run_id, f"generatePlan returned no run id: {started!r}"

    def finished():
        view = ctx.ws.call("request.planProposal", {"id": request_id, "runId": run_id})
        return view if view.get("status") != "running" else None
    view = wait_until(finished, timeout=RUN_TIMEOUT, interval=2.0, what="the Plan run to finish")
    assert view["status"] == "succeeded", f"plan run ended {view['status']!r}: {view.get('errorCode')} {str(view.get('errorMessage', ''))[:200]}"
    assert view.get("proposal"), "a succeeded run returned no proposal"
    return view


def tasks_of(proposal: dict) -> list:
    return list(proposal.get("tasks") or []) + [t for ph in proposal.get("phases") or [] for t in ph.get("tasks") or []]


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("PLN-00", "A project exists for the Plan-stage tests", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return
    held: dict = {}

    def ai_ready() -> None:
        if not ctx.state.get("ai_available"):
            raise Blocked("needs an AI on the dev server (a logged-in Claude Code, or an API key for the agent)")

    def planning_request() -> dict:
        if "req" not in held:
            req = drv.create(ctx, "plan stage")["request"]
            held["req"] = drv.to_first_stage(ctx, req, "task", "S")  # a task list skips analysis and starts in planning
            assert held["req"]["status"] == "planning", f"expected planning, got {held['req']['status']!r}"
        return held["req"]

    def propose_writes_nothing():
        ai_ready()
        req = planning_request()
        held["view"] = propose_and_wait(ctx, req["id"])
        after = drv.get(ctx, req["id"])
        assert after["status"] == "planning" and not after.get("planTaskId"), "proposing must not change the Request or write a Plan"
        assert tasks_of(held["view"]["proposal"]), "the proposal has no tasks"
    rec.run("PLN-01", "Proposing a Plan runs the AI in the background and writes nothing", propose_writes_nothing)

    def proposal_is_well_formed():
        if "view" not in held:
            raise Blocked("PLN-01 did not produce a proposal")
        tasks = tasks_of(held["view"]["proposal"])
        for t in tasks:
            assert t.get("title"), f"a proposed task has no title: {t!r}"
        for i, t in enumerate(tasks):
            for d in t.get("dependsOnIndices") or []:
                assert 0 <= d < len(tasks) and d != i, f"task {i} has a bad dependency index {d}"
    rec.run("PLN-02", "The proposal has titled tasks and valid dependency indices", proposal_is_well_formed)

    def invalid_proposal_is_refused():
        req = planning_request()
        cyc = {"title": "Cycle", "tasks": [{"title": "a", "taskType": "task", "dependsOnIndices": [1]}, {"title": "b", "taskType": "task", "dependsOnIndices": [0]}]}
        try:
            ws.call("request.generatePlan", {"id": req["id"], "mode": "commit", "proposal": cyc})
        except RpcError as exc:
            assert exc.has_code("REQUEST_PLAN_INVALID_DEPENDENCY"), f"wrong error: {exc.message[:200]}"
            assert not drv.get(ctx, req["id"]).get("planTaskId"), "a refused commit wrote a Plan"
            return
        raise AssertionError("a proposal with a dependency cycle was committed")
    rec.run("PLN-03", "Committing a proposal with a dependency cycle is refused and writes nothing", invalid_proposal_is_refused)

    def commit_opens_the_approval():
        if "view" not in held:
            raise Blocked("PLN-01 did not produce a proposal")
        req = planning_request()
        done = ws.call("request.generatePlan", {"id": req["id"], "mode": "commit", "proposal": held["view"]["proposal"], "rawAiResponse": held["view"].get("rawAiResponse", "")}, timeout=90)
        assert done.get("planTaskId"), f"commit returned no plan id: {done!r}"
        held["commit"] = done
        after = drv.get(ctx, req["id"])
        assert after["status"] == "awaiting_plan_approval", f"after commit the Request is {after['status']!r}"
        assert after.get("planTaskId") == done["planTaskId"], "the Request does not point at the committed Plan"
        approvals = ws.call("approval.list", {"requestId": req["id"], "status": "pending"})
        pending = [a for a in approvals.get("approvals", []) if a["subjectType"] == "task_list"]
        assert pending and pending[0]["subjectId"] == done["planTaskId"], f"no pending task_list approval on the Plan: {approvals!r}"
        held["approval"] = pending[0]
        assert len(done.get("taskIds") or []) == len(tasks_of(held["view"]["proposal"])), "task-service created a different number of tasks than proposed"
    rec.run("PLN-04", "Committing writes the Plan, moves to awaiting_plan_approval and opens the task_list approval", commit_opens_the_approval)

    def commit_replay_is_idempotent():
        if "commit" not in held:
            raise Blocked("PLN-04 did not commit")
        req = planning_request()
        again = ws.call("request.generatePlan", {"id": req["id"], "mode": "commit", "proposal": held["view"]["proposal"]}, timeout=90)
        assert again["planTaskId"] == held["commit"]["planTaskId"] and again.get("alreadyExists"), f"replay answered {again!r}"
        pending = ws.call("approval.list", {"requestId": req["id"], "status": "pending"}).get("approvals", [])
        assert len(pending) == 1, f"a replay opened another approval: {len(pending)} pending"
    rec.run("PLN-05", "Repeating a commit returns the same Plan and opens no second approval", commit_replay_is_idempotent)

    def rejecting_returns_to_backlog():
        if "approval" not in held:
            raise Blocked("PLN-04 did not open an approval")
        a = held["approval"]
        res = ws.call("approval.reject", {"id": a["id"], "comment": "e2e: not executing this plan", "expectedVersion": a["version"], "expectedDigest": a["subjectDigest"]})
        assert res.get("requestStatus") == "request_backlog", f"after rejection the Request is {res.get('requestStatus')!r}"
    rec.run("PLN-06", "Rejecting the Plan approval returns the Request to the backlog", rejecting_returns_to_backlog)

    def commit_in_the_wrong_state():
        req = drv.create(ctx, "plan wrong state")["request"]
        try:
            ws.call("request.generatePlan", {"id": req["id"], "mode": "commit", "proposal": {"title": "x", "tasks": [{"title": "a", "taskType": "task"}]}})
        except RpcError as exc:
            assert exc.has_code("REQUEST_STATE_STALE") or exc.has_code("REQUEST_PLAN"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("a Plan was committed for a Request that is not in planning")
    rec.run("PLN-07", "A Plan cannot be committed before the Request reaches planning", commit_in_the_wrong_state)

    def phased_plan_after_an_approved_solution():
        ai_ready()
        req = drv.create(ctx, "plan phased")["request"]
        cur = drv.to_first_stage(ctx, req, "change_request", "M")
        assert cur["status"] == "analyzing", f"expected analyzing, got {cur['status']!r}"
        started = ws.call("solution.generate", {"requestId": cur["id"]}, timeout=120)

        def solved():
            listed = ws.call("solution.list", {"requestId": cur["id"]})
            run_ = next((r for r in listed.get("runs", []) if r["id"] == started["runId"]), None)
            return listed if run_ and run_["status"] != "running" else None
        listed = wait_until(solved, timeout=RUN_TIMEOUT, interval=2.0, what="the Solution run to finish")
        assert listed.get("solutions"), "the Solution run produced no Solution"
        solution = listed["solutions"][-1]
        option_id = solution["options"]["options"][0]["id"]
        chosen = ws.call("solution.choose", {"requestId": cur["id"], "solutionId": solution["id"], "optionId": option_id, "comment": "e2e"})
        assert not chosen.get("requiresConfirmation"), "the chosen option needs a Decision confirmation; the suite cannot approve it"
        approval = next(a for a in ws.call("approval.list", {"requestId": cur["id"], "status": "pending"})["approvals"] if a["subjectType"] == "solution")
        approved = ws.call("approval.approve", {"id": approval["id"], "comment": "e2e", "expectedVersion": approval["version"], "expectedDigest": chosen["approvalDigest"]})
        assert approved.get("requestStatus") == "planning", f"after the Solution approval the Request is {approved.get('requestStatus')!r}"

        view = propose_and_wait(ctx, cur["id"])
        proposal = view["proposal"]
        assert proposal.get("phases") and not proposal.get("tasks"), f"a change_request is planned in Phases: {list(proposal)}"
        done = ws.call("request.generatePlan", {"id": cur["id"], "mode": "commit", "proposal": proposal}, timeout=90)
        assert done.get("planTaskId") and len(done.get("phaseTaskIds") or []) == len(proposal["phases"]), f"commit: {done!r}"
        pending = ws.call("approval.list", {"requestId": cur["id"], "status": "pending"})["approvals"]
        plan_approval = next((a for a in pending if a["subjectType"] == "plan"), None)
        assert plan_approval and plan_approval["subjectId"] == done["planTaskId"], f"no plan approval on the Plan: {pending!r}"
        ws.call("approval.reject", {"id": plan_approval["id"], "comment": "e2e cleanup", "expectedVersion": plan_approval["version"], "expectedDigest": plan_approval["subjectDigest"]})
    rec.run("PLN-08", "A change_request is planned in Phases from the approved Solution and gets a plan approval", phased_plan_after_an_approved_solution)
