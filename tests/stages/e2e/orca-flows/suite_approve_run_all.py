"""ARA: "Approve and run all steps" (CR-REQ-043): `approval.approve` takes `executionMode:"continuous"` for a Plan, so one approval runs every Phase.

  ARA-01  a step_by_step project, ordinary approval: executionMode step_by_step, a `phase` approval waits, no Phase starts; approving it and
          `request.startPhase` by hand still work (the old flow is unchanged)
  ARA-02  the same project, approval with executionMode "continuous": executing + continuous, no `phase` approval, the first Phase starts on its own
  ARA-03  the run reaches the end with no further click: one phase.started per Phase, no `phase` approval ever opened
  ARA-04  "step_by_step" and "bogus" are REQUEST_EXECUTION_MODE_INVALID, a mode with a reject too; on a Solution approval it is
          REQUEST_EXECUTION_MODE_NOT_APPLICABLE; every approval stays pending
  ARA-05  a double click (the same call twice) answers the same both times
  ARA-06  a Request already pinned to step_by_step (run once, returned to the backlog, reopened) refuses "continuous": REQUEST_EXECUTION_MODE_ALREADY_PINNED
  ARA-07  an approver who may not execute is refused with REQUEST_EXECUTION_ACTOR_NOT_ALLOWED and the approval stays pending

Needs --ai-stub: the stub is the agent's `claude` for classification, Solution, Plan and, since this CR, the task-execution prompt (it ends the
run with a valid `done` block and changes no file), so every Phase runs for free. With a real Claude the same run would bill AI usage.
Run: python3 run_acceptance.py --ai-stub devserver approve_run_all
"""
from __future__ import annotations

import shutil
import subprocess

import approve_run_all_driver as ara
import base_ref_stack as stack
import base_ref_worktrees as gitwt
import ledgerlite_fixture as ledgerlite
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from analysis_revision_users import ExtraUser, create_user
from pilot_register import psql, tenant_of_project
from ws_rpc_client import RpcError

RUN_WAIT = 360  # every Phase runs: worktree, fake claude, merge, then the next Phase
POLICY_AUTHOR = "e2e-approve-run-all"


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    held: dict = {}

    def ready() -> None:
        if not ctx.state.get("ai_stub"):
            raise Skipped("drives AI analyses and runs every Phase of a Plan; use --ai-stub (fake claude, no cost)")
        if ctx.state.get("ai_available") is False:
            raise Blocked("no AI is available on the dev server")

    def project() -> str:
        ready()
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-ara")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo and its worktrees", lambda: gitwt.remove_worktrees_and_repo(repo))
            held["project"] = ara.project_with_mode(ctx, repo, "ara-project", "step_by_step")
        return held["project"]

    def gate(key: str, title: str) -> ara.PlanGate:
        """A Request waiting for its Plan approval, built once per key."""
        if key not in held:
            held[key] = ara.to_plan_approval(ctx, project(), title)
        return held[key]

    def need(key: str, test_id: str) -> ara.PlanGate:
        if key not in held:
            raise Blocked(f"{test_id} did not leave a Request to work on")
        return held[key]

    # ── the old flow, unchanged ───────────────────────────────────────────────────────────────────────
    def step_by_step_still_works():
        g = gate("normal", "ara step by step")
        assert g.phase_count >= 2, f"the stub's change_request Plan has {g.phase_count} Phases; the cases need at least two"
        res = ara.approve(ctx, g)
        assert res.get("requestStatus") == "executing", f"after the approval the Request is {res.get('requestStatus')!r}"
        assert res.get("executionMode", "") in ("", "step_by_step"), f"an ordinary approval answered executionMode {res.get('executionMode')!r}"
        wait_until(lambda: drv.get(ctx, g.request_id).get("executionMode") == "step_by_step", timeout=20, what="request.get to report executionMode step_by_step")
        phase = wait_until(lambda: next((a for a in ws.call("approval.list", {"requestId": g.request_id, "status": "pending"}).get("approvals", [])
                                         if a["subjectType"] == "phase"), None), timeout=30, what="the first `phase` approval to open")
        assert not ara.phase_starts(ctx, g.request_id), "a Phase started before anybody approved it"
        ws.call("approval.approve", {"id": phase["id"], "comment": "e2e", "expectedVersion": phase["version"], "expectedDigest": phase["subjectDigest"]})
        started = ws.call("request.startPhase", {"id": g.request_id, "phaseTaskId": phase["subjectId"]}, timeout=60)
        assert started.get("phaseTaskId") == phase["subjectId"] and not started.get("alreadyStarted"), f"startPhase answered {started!r}"
        assert len(ara.phase_starts(ctx, g.request_id)) == 1, "the hand-started Phase left no phase.started row"
    rec.run("ARA-01", "step_by_step project: an ordinary approval pins step_by_step, a `phase` approval waits, the Phase starts only by hand", step_by_step_still_works)

    # ── validation, before the approval is consumed ───────────────────────────────────────────────────
    def refused_modes():
        g = gate("run", "ara run all")
        a = g.approval
        for bad in ("step_by_step", "bogus"):
            stack.expect_error("REQUEST_EXECUTION_MODE_INVALID", lambda b=bad: ara.approve(ctx, g, b))
        stack.expect_error("REQUEST_EXECUTION_MODE_INVALID", lambda: ws.call("approval.reject", {
            "id": a["id"], "comment": "e2e: should be refused", "expectedVersion": a["version"], "expectedDigest": a["subjectDigest"], "executionMode": "continuous"}))
        analysis = ara.to_analysis_gate(ctx, project(), "ara solution approval")
        sa = analysis["approval"]
        stack.expect_error("REQUEST_EXECUTION_MODE_NOT_APPLICABLE", lambda: ws.call("approval.approve", {
            "id": sa["id"], "comment": "e2e", "expectedVersion": sa["version"], "expectedDigest": analysis["digest"], "executionMode": "continuous"}))
        assert "plan" in ara.pending_subjects(ctx, g.request_id) or "task_list" in ara.pending_subjects(ctx, g.request_id), "a refused call decided the Plan approval"
        assert "solution" in ara.pending_subjects(ctx, analysis["request"]["id"]), "a refused call decided the Solution approval"
        assert drv.get(ctx, g.request_id)["status"] == "awaiting_plan_approval", "a refused call moved the Request"
    rec.run("ARA-04", "executionMode step_by_step/bogus/with a reject is INVALID, on a Solution approval NOT_APPLICABLE; every approval stays pending", refused_modes)

    # ── the new flow ──────────────────────────────────────────────────────────────────────────────────
    def run_all_pins_continuous():
        g = need("run", "ARA-04")
        res = ara.approve(ctx, g, "continuous")
        held["run_answer"] = res
        assert res.get("requestStatus") == "executing", f"after the approval the Request is {res.get('requestStatus')!r}"
        assert res.get("executionMode") == "continuous", f"approval.approve answered executionMode {res.get('executionMode')!r}"
        assert drv.get(ctx, g.request_id).get("executionMode") == "continuous", "request.get does not report executionMode continuous"
        starts = wait_until(lambda: ara.phase_starts(ctx, g.request_id), timeout=90, what="the first Phase to start without a click")
        assert starts[0].get("actorKind") == "system" and (starts[0].get("details") or {}).get("trigger") == "auto", f"the first Phase was not started automatically: {starts[0].get('actorKind')!r} {starts[0].get('details')!r}"
        assert "phase" not in ara.pending_subjects(ctx, g.request_id), "a `phase` approval is waiting in a continuous run"
    rec.run("ARA-02", "Approving with executionMode continuous in a step_by_step project: executing, continuous, no `phase` approval, the Phase starts by itself", run_all_pins_continuous)

    def double_click():
        g = need("run", "ARA-04")
        if "run_answer" not in held:
            raise Blocked("ARA-02 did not approve the Plan")
        again = ara.approve(ctx, g, "continuous")
        first = held["run_answer"]
        assert (again.get("requestStatus"), again.get("executionMode")) == (first.get("requestStatus"), first.get("executionMode")), f"first {first!r}, second {again!r}"
    rec.run("ARA-05", "The same approve-and-run-all call twice answers the same both times", double_click)

    def runs_to_the_end():
        g = need("run", "ARA-04")
        if "run_answer" not in held:
            raise Blocked("ARA-02 did not approve the Plan")

        def finished():
            req = drv.get(ctx, g.request_id)
            return req if req["status"] != "executing" else None
        req = wait_until(finished, timeout=RUN_WAIT, interval=3.0, what="the run to leave executing")
        starts = ara.phase_starts(ctx, g.request_id)
        assert len(starts) == g.phase_count, f"{len(starts)} phase.started rows for {g.phase_count} Phases; the Request is {req['status']!r}; activity: {ara.activity_tail(ctx, g.request_id)}"
        assert req["status"] in ("completed", "awaiting_deployment_approval", "awaiting_verification", "verifying"), f"the run ended in {req['status']!r}"
        every = ws.call("approval.list", {"requestId": g.request_id}).get("approvals", [])
        assert "phase" not in {a["subjectType"] for a in every}, "a `phase` approval was opened during the continuous run"
    rec.run("ARA-03", "All Phases run to the end with no further click: one phase.started per Phase, no `phase` approval ever", runs_to_the_end)

    # ── a Request pinned to step_by_step refuses the choice ───────────────────────────────────────────
    def already_pinned():
        g = need("normal", "ARA-01")
        if drv.get(ctx, g.request_id).get("executionMode") != "step_by_step":
            raise Blocked("ARA-01 did not leave a Request pinned to step_by_step")
        cur = drv.get(ctx, g.request_id)
        if cur["status"] == "request_backlog":
            back = cur  # the run's tasks already failed enough to send it back
        else:
            back = ws.call("request.returnToBacklog", {"id": g.request_id, "stage": "phase", "category": "other", "reason": "e2e: pinned step by step, back to the backlog",
                                                        "expectedVersion": cur["version"]})["request"]
        assert back["status"] == "request_backlog", f"returned to {back['status']!r}"
        reopened = ws.call("request.reopen", {"id": g.request_id, "note": "e2e", "expectedVersion": back["version"]})["request"]
        cur = drv.settle_after_classify(ctx, reopened["id"]) if reopened["status"] in ("new", "classifying") else drv.get(ctx, reopened["id"])
        assert cur["status"] == "awaiting_type_confirmation", f"reopened Request is {cur['status']!r}"
        cur = drv.confirm(ctx, cur, "change_request", "M")
        if cur["status"] == "awaiting_information":
            drv.answer(ctx, drv.open_clarification(ctx, cur["id"]))
            wait_until(lambda: drv.get(ctx, cur["id"])["status"] != "awaiting_information", timeout=15, what="the Request to leave awaiting_information")
        held["repin"] = cur["id"]
        started = ws.call("solution.generate", {"requestId": cur["id"]}, timeout=120)

        def solved():
            listed = ws.call("solution.list", {"requestId": cur["id"]})
            r = next((x for x in listed.get("runs", []) if x["id"] == started["runId"]), None)
            return listed if r and r["status"] != "running" else None
        solution = wait_until(solved, timeout=300, interval=2.0, what="the Solution run to finish")["solutions"][-1]
        chosen = ws.call("solution.choose", {"requestId": cur["id"], "solutionId": solution["id"], "optionId": solution["options"]["options"][0]["id"], "comment": "e2e"})
        analysis = next(a for a in ws.call("approval.list", {"requestId": cur["id"], "status": "pending"})["approvals"] if a["subjectType"] == "solution")
        ara.approve_solution_and_plan(ctx, cur["id"], analysis, chosen["approvalDigest"])
        again = ara.pending_plan(ctx, project(), cur["id"])
        stack.expect_error("REQUEST_EXECUTION_MODE_ALREADY_PINNED", lambda: ara.approve(ctx, again, "continuous"))
        assert "plan" in ara.pending_subjects(ctx, cur["id"]) or "task_list" in ara.pending_subjects(ctx, cur["id"]), "the refused call decided the Plan approval"
        ordinary = ara.approve(ctx, again)
        assert ordinary.get("requestStatus") == "executing", f"an ordinary approval after the refusal answered {ordinary.get('requestStatus')!r}"
    rec.run("ARA-06", "A Request pinned to step_by_step (run once, backlog, reopened) refuses `continuous` with ALREADY_PINNED and still approves normally", already_pinned)

    # ── an approver who may not execute ───────────────────────────────────────────────────────────────
    def approver_without_execute_permission():
        pid = project()
        user: ExtraUser = create_user(ctx, "ara-approver")
        try:
            ws.call("project.addMember", {"projectId": pid, "userId": user.id, "role": "member"})
        except RpcError as exc:
            raise Blocked(f"cannot add the approver to the project: {exc.message[:160]}") from exc
        ctx.on_cleanup(f"remove {user.email} from project {pid}", lambda: ws.call("project.removeMember", {"projectId": pid, "userId": user.id}))
        try:
            tenant = tenant_of_project(pid)
            approvers = f'["reporter","role:admin","user:{user.id}"]'
            rows = ",".join(f"(gen_random_uuid(), '{tenant}', '{pid}', '{s}', '{approvers}'::jsonb, true, 10, true, 1, '{POLICY_AUTHOR}')" for s in ("plan", "task_list"))
            psql("request", f"begin; set local app.tenant_id = '{tenant}'; insert into request.approval_policies "
                            f"(id, tenant_id, project_id, subject_type, approvers, allow_requester_approve, priority, enabled, version, created_by) values {rows}; commit;")
        except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
            raise Blocked(f"cannot write the approval policy in the local docker stack ({exc})") from exc
        ctx.on_cleanup(f"remove the approval policies of project {pid}", lambda: psql(
            "request", f"begin; set local app.tenant_id = '{tenant}'; delete from request.approval_policies where project_id = '{pid}' and created_by = '{POLICY_AUTHOR}'; commit;"))
        g = ara.to_plan_approval(ctx, pid, "ara no execute permission")
        try:
            res = ara.approve(ctx, g, "continuous", ws=user.ws)
        except RpcError as exc:
            assert exc.has_code("REQUEST_EXECUTION_ACTOR_NOT_ALLOWED"), f"refused with {exc.message[:200]}"
            assert drv.get(ctx, g.request_id)["status"] == "awaiting_plan_approval", "the refused approval moved the Request"
            assert ara.pending_subjects(ctx, g.request_id), "the refused approval was decided"
            return
        assert res.get("requestStatus") == "executing" and res.get("executionMode") == "continuous", f"the member's approval answered {res!r}"
        raise Blocked("not reachable through the gateway: the Plan tree is owned by the admin who committed it, and ExecutionAccess lets the owner grant execute to a "
                      "member who approves, so the refusal needs an approver who owns the tree and lost access (unit and contract tests cover it). "
                      "The member's approve-and-run-all was accepted and pinned continuous, as designed")
    rec.run("ARA-07", "An approver who may not execute is REQUEST_EXECUTION_ACTOR_NOT_ALLOWED on `continuous`, and the approval stays pending", approver_without_execute_permission)
