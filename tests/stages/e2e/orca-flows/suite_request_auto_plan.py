"""APG: automatic Plan generation after the Analysis is approved (CR-REQ-044) on the LedgerLite finance fixture repo, with the fake claude.

  APG-01  a bug Analysis approved -> within 90 s the Request waits at awaiting_plan_approval with a Plan and one open Plan approval, from one run the
          server started itself (key plan-auto-...), and autoPlan says done; no client call asked for it
  APG-02  the Plan comes back as text that is not JSON twice (fail_plan_next 2:invalid): the job gives up with REQUEST_PLAN_AI_INVALID_JSON after one
          run, the Request stays in planning, and request.retryPlan produces the Plan
  APG-03  one failed call (fail_plan_next 1:error): the failed run is retried by the job as a second run and the Plan appears
  APG-04  a project without a dev server: the job is skipped, there is no run, and Generate by hand is the way (needs --ai-stub)
  APG-05  spike and question complete after their Analysis: neither gets a job or a Plan run; APG-05b a hotfix goes from its diagnosis to the approval
          of its fix task and gets none either (a second user approves pre_deploy; needs docker for the policy row)
  APG-06  the project switch off (request.engineSettingsSet): the Request stays in planning, no run, autoPlan.enabled is false, and the manual
          request.generatePlan still works
  APG-07  a person pressing Generate plan right after the approval, while that run is going, is joined by the job: one run, one Plan, one approval;
          APG-07b if the person's run ends before the job starts, there may be two runs (fast stub), still one Plan, one approval and no error
  APG-08  asking for changes to a Plan (feedback) and rejecting a Plan start no automatic job of their own
  APG-09  a `task` Request that is ready when its type is confirmed (an acceptance criterion is written to the row, because no gateway channel sets one)
          gets its Plan by itself; APG-09b one that had to answer the readiness Clarification does not (information_provided never plans, D3)
  APG-10  (cost class C, ORCA_E2E_OPENSPEC=1, never with --ai-stub) an OpenSpec change_request: the server commits the authored bundle
  APG-11  Requests that waited in planning from before the feature (every one on the stack, up to five, such as #1241) get no job, and a status_changed event
          dated three hours back, written to the outbox for each of them, is ignored with the log line of the start window

The failure switches are fake claude files (`fail_plan_next`, see fake_claude_cli/claude); the runs and jobs are counted in the request database of
the local docker stack (BLOCKED when it cannot be reached). Run: python3 run_acceptance.py --ai-stub devserver request_auto_plan
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import time
import uuid

import base_ref_stack as stack
import ledgerlite_fixture as ledgerlite
import plan_auto_driver as apg
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from pilot_register import psql
from suite_request_plan_stage import propose_and_wait
from ws_rpc_client import RpcError

RETRY_WAIT = 150  # the retry waits REQUEST_PLAN_AUTO_RETRY_DELAY (30 s by default) before its second run
PLAN_FAIL_WAIT = 90
QUIET = 20  # seconds with nothing happening before "the server did not start a Plan" is believed (the job poll is 5 s)


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    stub = bool(ctx.state.get("ai_stub"))
    held: dict = {}

    def ready() -> str:
        if not stub:
            raise Skipped("starts AI analyses and Plans; use --ai-stub (fake claude, free)")
        if ctx.state.get("ai_available") is False:
            raise Blocked("no AI is available on the dev server")
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-apg")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo", repo.cleanup)
            held["project"] = apg.configure(ctx, repo, "apg-project", plan_auto=True)
            ctx.on_cleanup("disarm the Plan failures of the fake claude", lambda: apg.disarm_plan_failure(ctx))
        return held["project"]

    def gate_of(key: str, typ: str, title: str, size: str = "S"):
        if key not in held:
            held[key] = apg.to_analysis_gate(ctx, ready(), typ, title, size)
        return held[key]

    def need(key: str, test_id: str):
        if key not in held:
            raise Blocked(f"{test_id} did not leave a Request to work on")
        return held[key]

    def quiet_for(request_id: str, seconds: float) -> dict:
        """The Request after `seconds` in which it must not have left planning."""
        deadline = time.time() + seconds
        cur = drv.get(ctx, request_id)
        while time.time() < deadline:
            cur = drv.get(ctx, request_id)
            assert cur["status"] == "planning" and not cur.get("planTaskId"), f"the Request left planning by itself: {cur['status']!r}"
            time.sleep(2)
        return cur

    # ── the happy path ────────────────────────────────────────────────────────────────────────────
    def plan_appears_by_itself():
        flow = gate_of("bug", "bug", "apg bug plan by itself")
        pid = held["project"]
        assert not apg.jobs(ctx, pid, flow.id), "a job exists before the Analysis was approved"
        started = time.time()
        approved = apg.approve_analysis(ctx, flow)
        assert approved.get("requestStatus") == "planning", f"after the Analysis approval the Request is {approved.get('requestStatus')!r}"
        req = apg.wait_plan(ctx, flow.id)
        held["bug_seconds"] = round(time.time() - started, 1)
        pending = apg.pending_plan_approvals(ctx, flow.id)
        assert len(pending) == 1 and pending[0]["subjectId"] == req["planTaskId"], f"open Plan approvals: {[(a['subjectType'], a['subjectId']) for a in pending]!r}, Plan {req['planTaskId']}"
        runs = apg.plan_runs(ctx, pid, flow.id)
        assert len(runs) == 1 and runs[0].status == "succeeded", f"Plan runs: {runs!r}"
        assert runs[0].key.startswith("plan-auto-"), f"the Plan run was not started by the server: key {runs[0].key!r} (a client call leaves it empty)"
        auto = apg.auto_plan(ctx, flow.id)
        assert auto.get("enabled") is True, f"autoPlan = {auto!r}"
        assert auto.get("state") == "done", f"autoPlan.state is {auto.get('state')!r} after the Plan was committed, expected done: {auto!r}"
        jobs = apg.jobs(ctx, pid, flow.id)
        assert [(j["origin"], j["state"]) for j in jobs] == [("event", "done")], f"jobs = {jobs!r}"
        assert "request.planAuto" in ws.call("request.flowStatus", {}).get("features", []), "request.flowStatus does not publish request.planAuto"
    rec.run("APG-01", "Approving a bug Analysis produces the Plan by itself (one server-started run, one open approval, autoPlan done)", plan_appears_by_itself)

    # ── a failure that is not temporary ───────────────────────────────────────────────────────────
    def invalid_json_gives_up_then_retry():
        flow = gate_of("invalid", "bug", "apg bug invalid plan")
        pid = held["project"]
        apg.arm_plan_failure(ctx, 2, "invalid")
        apg.approve_analysis(ctx, flow)
        auto = apg.wait_state(ctx, flow.id, ("gave_up",), timeout=PLAN_FAIL_WAIT, what="the job to give up on the invalid Plan")
        assert auto.get("reasonCode") == "REQUEST_PLAN_AI_INVALID_JSON", f"autoPlan = {auto!r}"
        runs = apg.plan_runs(ctx, pid, flow.id)
        assert len(runs) == 1 and (runs[0].status, runs[0].error_code) == ("failed", "REQUEST_PLAN_AI_INVALID_JSON"), f"after the failure the Plan runs are {runs!r}; a bad answer is not retried"
        cur = quiet_for(flow.id, QUIET)
        assert cur["status"] == "planning" and len(apg.plan_runs(ctx, pid, flow.id)) == 1, "the job kept going after it gave up"
        assert apg.failures_left(ctx) == 0, f"the fake claude has {apg.failures_left(ctx)} failures left: the Plan call was not made twice"
        reply = ws.call("request.retryPlan", {"id": flow.id})
        assert (reply.get("autoPlan") or {}).get("state") in ("pending", "running", "committing", "done"), f"request.retryPlan answered {reply!r}"
        apg.wait_plan(ctx, flow.id)
        runs = apg.plan_runs(ctx, pid, flow.id)
        assert [r.status for r in runs] == ["failed", "succeeded"], f"Plan runs after the retry: {runs!r}"
        jobs = apg.jobs(ctx, pid, flow.id)
        assert [(j["origin"], j["state"]) for j in jobs] == [("event", "gave_up"), ("retry", "done")], f"jobs = {jobs!r}"
    rec.run("APG-02", "Invalid Plan output twice: gave_up with REQUEST_PLAN_AI_INVALID_JSON after one run, Request in planning; retryPlan then succeeds", invalid_json_gives_up_then_retry)

    # ── a failure that is temporary ───────────────────────────────────────────────────────────────
    def error_is_retried_once():
        flow = gate_of("error", "bug", "apg bug transient plan")
        pid = held["project"]
        apg.arm_plan_failure(ctx, 1, "error")
        apg.approve_analysis(ctx, flow)
        apg.wait_plan(ctx, flow.id, timeout=RETRY_WAIT)
        runs = apg.plan_runs(ctx, pid, flow.id)
        assert len(runs) == 2, f"Plan runs = {runs!r}, expected the failed one and its retry"
        assert runs[0].status == "failed" and runs[0].error_code, f"the first run is {runs[0]!r}"
        assert runs[1].status == "succeeded", f"the second run is {runs[1]!r}"
        assert runs[0].key.startswith("plan-auto-") and runs[1].key == runs[0].key + "-2", f"run keys {runs[0].key!r}, {runs[1].key!r}: the retry must use the next attempt's key"
        jobs = apg.jobs(ctx, pid, flow.id)
        assert [(j["origin"], j["state"], j["attempt"], j["maxAttempts"]) for j in jobs] == [("event", "done", 2, 2)], f"jobs = {jobs!r}"
        held["error_code"] = runs[0].error_code
    rec.run("APG-03", "One failed Plan call: the job retries it as a second run (key -2) and the Plan appears", error_is_retried_once)

    # ── no dev server ─────────────────────────────────────────────────────────────────────────────
    def no_dev_server_is_skipped():
        ready()
        try:
            proj = ws.call("project.create", {"name": f"{cfg.run_prefix}-apg-nods", "defaultBranch": "main", "visibility": "private"})
        except RpcError as exc:
            raise Blocked(f"a project without a dev server cannot be created on this stack: {exc.message[:160]}") from exc
        ctx.on_cleanup(f"delete project {proj['id']}", lambda: ws.call("project.delete", {"projectId": proj["id"]}))
        apg.set_switch(ctx, proj["id"], True)
        req = drv.create(ctx, "apg no dev server", project_id=proj["id"])["request"]
        cur = drv.settle_after_classify(ctx, req["id"])
        assert cur["status"] == "awaiting_type_confirmation", f"classification without a dev server ended in {cur['status']!r}"
        apg.make_ready(ctx, proj["id"], req["id"])
        cur = drv.confirm(ctx, drv.get(ctx, req["id"]), "task", "S")
        assert cur["status"] == "planning", f"the Request is {cur['status']!r}, expected planning"
        auto = apg.wait_state(ctx, req["id"], ("skipped", "gave_up"), timeout=60, what="the job to stop for lack of a dev server")
        assert auto.get("state") == "skipped" and auto.get("reasonCode"), f"autoPlan = {auto!r}: a missing dev server is a reason to skip, not to fail"
        assert not apg.plan_runs(ctx, proj["id"], req["id"]), "a Plan run was recorded for a project without a dev server"
        after = drv.get(ctx, req["id"])
        assert after["status"] == "planning" and not after.get("planTaskId"), "the skipped job moved the Request"
        held["nods"] = {"reason": auto.get("reasonCode")}
    rec.run("APG-04", "A project without a dev server: the job is skipped with a reason, no run exists, the Request stays in planning", no_dev_server_is_skipped)

    # ── types that never plan automatically ───────────────────────────────────────────────────────
    def no_job_after_a_completing_analysis():
        pid = ready()
        seen = {}
        for typ in ("spike", "question"):
            flow = apg.to_analysis_gate(ctx, pid, typ, f"apg {typ} no plan")
            res = apg.approve_analysis(ctx, flow)
            time.sleep(QUIET / 2)
            assert drv.get(ctx, flow.id)["status"] == "completed", f"a {typ} should complete after its Analysis, not {res.get('requestStatus')!r}"
            assert not apg.jobs(ctx, pid, flow.id) and not apg.plan_runs(ctx, pid, flow.id), f"a {typ} got a Plan job or run"
            seen[typ] = "completed"
        held["other_outcomes"] = seen
    rec.run("APG-05", "spike and question complete after their Analysis: neither gets a Plan job or run", no_job_after_a_completing_analysis)

    def hotfix_has_no_plan_job():
        pid = ready()
        apg.name_pre_deploy_approver(ctx, pid)  # the hotfix opens a pre_deploy approval over its fix task, which the reporter cannot decide alone
        req = drv.create(ctx, "apg hotfix no plan", project_id=pid)["request"]
        cur = drv.to_first_stage(ctx, req, "hotfix", "S", "urgent")
        assert cur["status"] == "analyzing", f"after the readiness answer the hotfix is {cur['status']!r}"
        try:
            after = wait_until(lambda: (lambda r: r if r["status"] != "analyzing" else None)(drv.get(ctx, req["id"])), timeout=150, interval=1.0, what="the hotfix analysis to end")
        except AssertionError as exc:
            raise Blocked(f"the hotfix analysis did not finish in this environment: {exc}") from exc
        time.sleep(QUIET / 2)
        assert after["status"] == "awaiting_plan_approval", f"a hotfix goes straight to its approval, got {after['status']!r}"
        assert not apg.jobs(ctx, pid, req["id"]), f"a hotfix got a Plan job: {apg.jobs(ctx, pid, req['id'])!r}"
        assert not apg.plan_runs(ctx, pid, req["id"]), "a hotfix got a Plan run"
    rec.run("APG-05b", "A hotfix goes from its diagnosis straight to the approval of its fix task: no Plan job or run", hotfix_has_no_plan_job)

    # ── the project switch ────────────────────────────────────────────────────────────────────────
    def project_switch_off():
        pid = ready()
        flow = apg.to_analysis_gate(ctx, pid, "bug", "apg bug switch off")
        before = apg.set_switch(ctx, pid, False)
        ctx.on_cleanup("switch automatic Plan generation back on", lambda: apg.set_switch(ctx, pid, True))
        assert before.get("planAutoGenerate") is False, f"the switch did not turn off: {before!r}"
        apg.approve_analysis(ctx, flow)
        cur = quiet_for(flow.id, QUIET)
        assert not apg.plan_runs(ctx, pid, flow.id) and not apg.jobs(ctx, pid, flow.id), "a Plan run or job exists with the switch off"
        auto = apg.auto_plan(ctx, flow.id)
        assert auto.get("enabled") is False, f"autoPlan.enabled = {auto!r}, expected false"
        view = propose_and_wait(ctx, flow.id)
        done = ws.call("request.generatePlan", {"id": flow.id, "mode": "commit", "proposal": view["proposal"], "rawAiResponse": view.get("rawAiResponse", "")}, timeout=90)
        assert done.get("planTaskId"), f"the manual commit returned no Plan: {done!r}"
        assert drv.get(ctx, flow.id)["status"] == "awaiting_plan_approval"
        assert len(apg.pending_plan_approvals(ctx, flow.id)) == 1
        apg.set_switch(ctx, pid, True)
        assert cur["status"] == "planning"
    rec.run("APG-06", "Switch off: the Request stays in planning, autoPlan.enabled is false, and Generate plan by hand still works", project_switch_off)

    # ── a person and the server at the same time ──────────────────────────────────────────────────
    def click_then_wait(key: str, title: str) -> tuple:
        flow = gate_of(key, "bug", title)
        pid = held["project"]
        apg.approve_analysis(ctx, flow)
        try:
            click = ws.call("request.generatePlan", {"id": flow.id}, timeout=60)
        except RpcError as exc:
            raise AssertionError(f"pressing Generate plan right after the approval failed: {exc.message[:240]}") from exc
        assert click.get("runId"), f"the manual propose returned no run: {click!r}"
        req = apg.wait_plan(ctx, flow.id, timeout=apg.PLAN_WAIT + 30)
        pending = apg.pending_plan_approvals(ctx, flow.id)
        assert len(pending) == 1 and pending[0]["subjectId"] == req["planTaskId"], f"open Plan approvals: {[(a['subjectType'], a['subjectId']) for a in pending]!r}"
        return flow, apg.plan_runs(ctx, pid, flow.id), click

    def manual_click_while_the_run_is_in_flight():
        apg.arm_plan_failure(ctx, 1, "slow")  # the person's run takes a few seconds, as a real one does, so the job meets it running
        flow, runs, click = click_then_wait("race", "apg bug manual click slow")
        assert [r.status for r in runs] == ["succeeded"] and runs[0].id == click["runId"], f"Plan runs {runs!r}: the job must join the person's run, not start another"
        jobs = apg.jobs(ctx, held["project"], flow.id)
        assert [(j["origin"], j["state"]) for j in jobs] == [("event", "done")], f"jobs = {jobs!r}"
    rec.run("APG-07", "Generate plan pressed right after the approval, while its run is going: the job joins it (one run, one Plan, one open approval)", manual_click_while_the_run_is_in_flight)

    def manual_click_that_finishes_first():
        flow, runs, click = click_then_wait("race_fast", "apg bug manual click fast")
        assert all(r.status == "succeeded" for r in runs) and click["runId"] in {r.id for r in runs}, f"Plan runs {runs!r}"
        held["fast_click_runs"] = len(runs)
        assert len(runs) <= 2, f"{len(runs)} Plan runs for one click: {runs!r}"
    rec.run("APG-07b", "Generate plan pressed in the moment before the job starts: no error, one Plan and one open approval (at most two runs)", manual_click_that_finishes_first)

    # ── replan and reject create no job of the automatic kind ─────────────────────────────────────
    def replan_and_reject_start_no_job():
        flow = need("bug", "APG-01")
        pid = held["project"]
        if drv.get(ctx, flow.id)["status"] != "awaiting_plan_approval":
            raise Blocked("APG-01 did not leave the Request at the Plan approval")
        before_jobs, before_runs = apg.jobs(ctx, pid, flow.id), apg.plan_runs(ctx, pid, flow.id)
        first = apg.pending_plan_approvals(ctx, flow.id)[0]
        ws.call("request.generatePlan", {"id": flow.id, "feedback": f"{cfg.run_prefix} split the work into smaller steps", "idempotencyKey": f"{cfg.run_prefix}-replan"}, timeout=60)

        def replanned():
            pending = apg.pending_plan_approvals(ctx, flow.id)
            return pending[0] if len(pending) == 1 and pending[0]["id"] != first["id"] and drv.get(ctx, flow.id)["status"] == "awaiting_plan_approval" else None
        second = wait_until(replanned, timeout=RETRY_WAIT, interval=2.0, what="the revised Plan to wait for approval")
        new_jobs = apg.jobs(ctx, pid, flow.id)[len(before_jobs):]
        automatic = [j for j in new_jobs if j["origin"] in ("event", "retry")]
        assert not automatic, f"asking for changes started an automatic job: {new_jobs!r}"
        assert len(apg.plan_runs(ctx, pid, flow.id)) == len(before_runs) + 1, "the revision started other than one Plan run"
        counts = len(apg.jobs(ctx, pid, flow.id)), len(apg.plan_runs(ctx, pid, flow.id))
        res = ws.call("approval.reject", {"id": second["id"], "comment": "e2e: rejecting the revised Plan", "expectedVersion": second["version"], "expectedDigest": second["subjectDigest"]})
        assert res.get("requestStatus") == "request_backlog", f"after the rejection the Request is {res.get('requestStatus')!r}"
        time.sleep(QUIET / 2)
        assert (len(apg.jobs(ctx, pid, flow.id)), len(apg.plan_runs(ctx, pid, flow.id))) == counts, "rejecting the Plan started a job or a Plan run"
        assert drv.get(ctx, flow.id)["status"] == "request_backlog", "the rejected Request left the backlog by itself"
    rec.run("APG-08", "Asking for changes to a Plan starts no event/retry job (only the revision), and rejecting it starts nothing", replan_and_reject_start_no_job)

    # ── a type that goes straight to planning ─────────────────────────────────────────────────────
    def task_type_plans_by_itself():
        pid = ready()
        req = drv.create(ctx, "apg task plan by itself", project_id=pid)["request"]
        cur = drv.settle_after_classify(ctx, req["id"])
        assert cur["status"] == "awaiting_type_confirmation", f"classification ended in {cur['status']!r}"
        apg.make_ready(ctx, pid, req["id"])
        cur = drv.confirm(ctx, drv.get(ctx, req["id"]), "task", "S")
        assert cur["status"] in ("planning", "awaiting_plan_approval"), f"after the type confirmation the Request is {cur['status']!r}, expected planning"
        done = apg.wait_plan(ctx, req["id"])
        runs = apg.plan_runs(ctx, pid, req["id"])
        assert len(runs) == 1 and runs[0].key.startswith("plan-auto-ty-"), f"Plan runs {runs!r}: a type-confirmation job keys its run plan-auto-ty-<event>"
        jobs = apg.jobs(ctx, pid, req["id"])
        assert [(j["origin"], j["state"]) for j in jobs] == [("event", "done")], f"jobs = {jobs!r}"
        assert len(apg.pending_plan_approvals(ctx, req["id"])) == 1 and done["planTaskId"]
    rec.run("APG-09", "A `task` Request that is ready on type confirmation gets its Plan by itself", task_type_plans_by_itself)

    def answering_the_clarification_does_not_plan():
        pid = ready()
        req = drv.create(ctx, "apg task clarification", project_id=pid)["request"]
        cur = drv.to_first_stage(ctx, req, "task", "S")  # the readiness Clarification is answered: planning is resumed with information_provided
        assert cur["status"] == "planning", f"the Request is {cur['status']!r}, expected planning"
        quiet_for(req["id"], QUIET)
        assert not apg.jobs(ctx, pid, req["id"]) and not apg.plan_runs(ctx, pid, req["id"]), "answering the Clarification started a Plan job or run, which the design forbids (D3)"
        auto = apg.auto_plan(ctx, req["id"])
        assert auto.get("enabled") is True and not auto.get("state"), f"autoPlan = {auto!r}: the card must offer Generate plan"
    rec.run("APG-09b", "A task whose readiness Clarification was answered stays in planning for the person to press Generate plan (no job)", answering_the_clarification_does_not_plan)

    # ── OpenSpec for real (cost class C) ──────────────────────────────────────────────────────────
    def openspec_commit_on_the_server():
        if os.environ.get("ORCA_E2E_OPENSPEC", "").lower() not in ("1", "true", "yes") or stub:
            raise Skipped("cost class C: a real OpenSpec Plan costs 7 to 13 USD per run and needs the user's explicit consent; ORCA_E2E_OPENSPEC=1 without --ai-stub, "
                          "scenario 3 of the LedgerLite fixture. Not run by this suite on its own")
        raise Skipped("not implemented in this suite: run the OpenSpec scenario through ui-live/plan-documents.web.e2e.ts (ORCA_E2E_OPENSPEC=1), which approves the Analysis and "
                      "reads request.planProposal (engine, bundle, satisfies)")
    rec.run("APG-10", "OpenSpec change_request: the server commits the authored bundle (cost class C, opt-in)", openspec_commit_on_the_server)

    # ── what the feature must not touch ───────────────────────────────────────────────────────────
    def old_requests_are_left_alone():
        ready()
        pid = held["project"]
        tenant = apg.tenant(pid)
        found = apg.rows(tenant, "select r.id, r.project_id, r.number, r.version from request.requests r "
                                 "where r.status = 'planning' and r.plan_task_id is null and r.updated_at < now() - interval '1 day' "
                                 "and not exists (select 1 from request.plan_auto_jobs j where j.request_id = r.id) order by r.updated_at limit 5")
        olds = [r for r in found if len(r) == 4]
        if not olds:
            raise Blocked("no old Request waits in planning on this stack; nothing to prove the window with")
        events = []
        for rid, project_id, number, version in olds:  # every old Request that waits in planning, such as #1241 on the dev stack
            event_id = str(uuid.uuid4())
            payload = json.dumps({"at": "2026-10-19T00:00:00Z", "to": "planning", "from": "awaiting_analysis_approval", "type": "bug", "trigger": "analysis_approved",
                                  "version": int(version), "request_id": rid, "project_id": project_id, "number": int(number), "actor_kind": "user", "reporter_id": "", "source_provider": "manual"})
            psql("request", f"begin; set local app.tenant_id = '{tenant}'; insert into request.outbox_events (id, tenant_id, subject, occurred_at, version, payload) "
                            f"values ('{event_id}', '{tenant}', 'orca.request.request.status_changed', now() - interval '3 hours', 1, $j${payload}$j$::jsonb); commit;")
            events.append((event_id, rid, number))
        time.sleep(15)
        for event_id, rid, number in events:
            assert not apg.rows(tenant, f"select 1 from request.plan_auto_jobs where request_id = '{rid}'"), f"an event from 3 hours ago started a job for the old Request #{number}"
            published = apg.rows(tenant, f"select published_at is not null from request.outbox_events where id = '{event_id}'")
            assert published == [["t"]], f"the test event of #{number} was not published ({published!r}), so the case proves nothing"
            cur = ws.call("request.get", {"id": rid})["request"]
            assert cur["status"] == "planning" and not cur.get("planTaskId"), f"the old Request #{number} changed: {cur['status']!r}"
        logs = subprocess.run(["docker", "logs", "orca-go-request", "--since", "2m"], capture_output=True, text=True, timeout=60)
        text = logs.stdout + logs.stderr
        for _, rid, number in events:
            assert f"older than the start window request_id={rid}" in text, f"the consumer did not log that it ignored the old event of #{number}"
        held["old_requests"] = [number for _, _, number in events]
    rec.run("APG-11", "An old Request waiting in planning gets no job, and a status_changed event from 3 hours ago is ignored (start window)", old_requests_are_left_alone)


