"""RQL: intake, classification, type confirmation and the readiness Clarification, plus the feature flag."""
from __future__ import annotations

import threading

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from expected_request_flows import EXPECTED
from ws_rpc_client import RpcError


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("RQL-00", "A project exists for the Request suites", lambda: (_ for _ in ()).throw(Blocked("no project could be created (see DEV-04)")))
        return
    flow: dict = {}

    def flag_on():
        status = ws.call("request.flowStatus")
        if not status["enabled"]:
            ws.call("request.flowSet", {"enabled": True})  # tenant switch; REQUEST_FLOW_ENABLED must also be true on the service
            status = ws.call("request.flowStatus")
        assert status["enabled"], "Request flow is off; set REQUEST_FLOW_ENABLED=true on request-service and request.flowSet {enabled:true}"
    rec.run("RQL-01", "Request flow is enabled for the tenant", flag_on)

    def flag_off_blocks_forward_steps():
        ws.call("request.flowSet", {"enabled": False})
        try:
            try:
                drv.create(ctx, "flag-off probe")
            except RpcError as exc:
                assert exc.has_code("REQUEST_FLOW_DISABLED"), f"wrong error: {exc.message[:200]}"
                return
            raise AssertionError("a Request was created while the flow was switched off")
        finally:
            ws.call("request.flowSet", {"enabled": True})
    rec.run("RQL-02", "With the flow off, creating a Request is refused (REQUEST_FLOW_DISABLED)", flag_off_blocks_forward_steps)

    def safe_exit_while_off():
        req = drv.create(ctx, "safe-exit while off")["request"]
        ws.call("request.flowSet", {"enabled": False})
        try:
            cancelled = ws.call("request.cancel", {"id": req["id"], "reason": "e2e safe exit", "expectedVersion": drv.get(ctx, req["id"])["version"]})["request"]
        finally:
            ws.call("request.flowSet", {"enabled": True})
        assert cancelled["status"] == "cancelled", f"cancel while off left status {cancelled['status']!r}"
    rec.run("RQL-03", "With the flow off, cancelling an existing Request still works", safe_exit_while_off)

    def title_required():
        try:
            ws.call("request.create", {"projectId": ctx.state["project_id"], "title": "", "body": "x"})
        except RpcError as exc:
            assert "INVALID" in exc.message or "REQUIRED" in exc.message, f"unexpected error: {exc.message[:200]}"
            return
        raise AssertionError("a Request without a title was created")
    rec.run("RQL-04", "Creating a Request without a title is rejected", title_required)

    def unknown_project():
        try:
            ws.call("request.create", {"projectId": "00000000-0000-0000-0000-000000000000", "title": "x", "body": "x"})
        except RpcError:
            return
        raise AssertionError("a Request was created in a project that does not exist")
    rec.run("RQL-05", "Creating a Request in an unknown project is rejected", unknown_project)

    def create_ok():
        created = drv.create(ctx, "lifecycle main", client_request_id=f"{ctx.cfg.run_prefix}-main")
        req = created["request"]
        flow["id"], flow["client_id"] = req["id"], f"{ctx.cfg.run_prefix}-main"
        assert created["created"] is True, "created flag is not true for a new Request"
        assert req["status"] == "classifying", f"new Request is {req['status']!r}, expected classifying"
        assert req["number"] >= 1 and req["projectId"] == ctx.state["project_id"]
        assert req["reporterId"] == ctx.session.user["id"], "reporter is not the logged-in user"
        assert req["sourceProvider"] == "manual"
    rec.run("RQL-06", "Manual Request is created in 'classifying' with reporter and number", create_ok)

    def idempotent():
        if "id" not in flow:
            raise Blocked("RQL-06 did not create a Request")
        again = ws.call("request.create", {"projectId": ctx.state["project_id"], "title": f"{ctx.cfg.run_prefix} lifecycle main",
                                           "body": "dup", "clientRequestId": flow["client_id"]})
        assert again["created"] is False and again["request"]["id"] == flow["id"], "same clientRequestId created a second Request"
    rec.run("RQL-07", "Re-sending the same clientRequestId returns the same Request", idempotent)

    def classify_settles():
        if "id" not in flow:
            raise Blocked("RQL-06 did not create a Request")
        cur = drv.settle_after_classify(ctx, flow["id"])
        flow["classified"] = cur
        assert cur["status"] == "awaiting_type_confirmation", f"classification ended in {cur['status']!r}; it must always hand over to a human"
    rec.run("RQL-08", "Classification always ends in awaiting_type_confirmation, even if the AI fails", classify_settles)

    def ai_proposes_type():
        cur = flow.get("classified")
        if not cur:
            raise Blocked("RQL-08 did not complete")
        reason = cur.get("classificationReason", "")
        if reason in drv.AI_FAILURE_REASONS or not cur.get("type"):
            ctx.state["ai_available"] = False
            raise Blocked(f"the AI classifier produced no proposal (reason={reason!r}); needs an AI provider/API key on the dev server")
        ctx.state["ai_available"] = True
        # The proposal parser already rejects values outside the enums; checking again keeps the oracle independent of it.
        assert cur["type"] in EXPECTED, f"AI proposed type {cur['type']!r}, which is not one of the 11 request types"
        assert cur.get("size") in ("S", "M", "L"), f"AI proposed size {cur.get('size')!r}"
        assert cur.get("urgency") in ("normal", "urgent"), f"AI proposed urgency {cur.get('urgency')!r}"
        if ctx.state.get("ai_stub"):
            assert "fake-claude" in reason, f"with --ai-stub the proposal must come from the fake claude, got reason {reason!r}"
    rec.run("RQL-09", "AI classifier proposes a type, size and urgency", ai_proposes_type)

    def invalid_type():
        cur = flow.get("classified")
        if not cur:
            raise Blocked("RQL-08 did not complete")
        try:
            drv.confirm(ctx, cur, "not_a_type")
        except RpcError:
            return
        raise AssertionError("an unknown request type was accepted")
    rec.run("RQL-10", "Confirming an unknown type is rejected", invalid_type)

    def stale_version():
        cur = flow.get("classified")
        if not cur:
            raise Blocked("RQL-08 did not complete")
        try:
            ws.call("request.confirmType", {"id": cur["id"], "type": "task", "size": "S", "urgency": "normal", "expectedVersion": cur["version"] + 50})
        except RpcError as exc:
            assert exc.has_code("STALE") or exc.has_code("VERSION") or exc.has_code("CONFLICT"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("confirmType accepted a stale expectedVersion")
    rec.run("RQL-11", "A stale expectedVersion is rejected (optimistic concurrency)", stale_version)

    def concurrent_confirm():
        req = drv.create(ctx, "concurrent confirm")["request"]
        cur = drv.settle_after_classify(ctx, req["id"])
        cur = drv.get(ctx, req["id"])
        outcomes: list[str] = []

        def attempt():
            try:
                outcomes.append(drv.confirm(ctx, cur, "task", "S")["status"])
            except RpcError:
                outcomes.append("rejected")
        threads = [threading.Thread(target=attempt) for _ in range(5)]
        [t.start() for t in threads]
        [t.join() for t in threads]
        # Repeated identical commands may be answered idempotently (CR-REQ-003 L4); what must never happen is applying the transition twice.
        opened = ctx.ws.call("clarification.list", {"requestId": req["id"]})["clarifications"]
        assert len(opened) <= 1, f"{len(opened)} Clarifications were opened by 5 concurrent confirmations; the transition was applied more than once"
        assert any(o != "rejected" for o in outcomes), "no confirmation succeeded"
    rec.run("RQL-12", "Five concurrent confirmations apply the transition once (no duplicate Clarification)", concurrent_confirm)

    def confirm_opens_clarification():
        cur = flow.get("classified")
        if not cur:
            raise Blocked("RQL-08 did not complete")
        after = drv.confirm(ctx, drv.get(ctx, cur["id"]), "task", "S")
        flow["confirmed"] = after
        assert after["type"] == "task" and after["size"] == "S", "confirmed type/size not stored"
        assert after["status"] in ("awaiting_information", "planning"), f"after confirming a task the status is {after['status']!r}"
    rec.run("RQL-13", "Confirming 'task' stores the type and leaves awaiting_type_confirmation", confirm_opens_clarification)

    def clarification_is_listed():
        after = flow.get("confirmed")
        if not after:
            raise Blocked("RQL-13 did not complete")
        if after["status"] != "awaiting_information":
            raise Blocked("no Clarification was opened for this Request")
        clar = drv.open_clarification(ctx, after["id"])
        assert clar, "awaiting_information but clarification.list returns no open Clarification (the gateway channel is missing or empty)"
        keys = {q["questionKey"]: q for q in clar["questions"]}
        assert "acceptance_criteria" in keys and keys["acceptance_criteria"]["required"], f"questions: {list(keys)}"
        assert clar["resumeStatus"] in ("analyzing", "planning", "executing"), f"resumeStatus={clar['resumeStatus']!r}"
        flow["clarification"] = clar
    rec.run("RQL-14", "The readiness Clarification is visible via clarification.list", clarification_is_listed)

    def draft_keeps_open():
        clar = flow.get("clarification")
        if not clar:
            raise Blocked("RQL-14 did not complete")
        reply = drv.answer(ctx, clar, complete=False, text="draft only")
        assert reply["clarification"]["status"] == "open", f"a draft closed the Clarification ({reply['clarification']['status']!r})"
        flow["clarification"] = reply["clarification"]
    rec.run("RQL-15", "Saving a draft answer keeps the Clarification open", draft_keeps_open)

    def complete_answer_resumes():
        clar = flow.get("clarification")
        if not clar:
            raise Blocked("RQL-14 did not complete")
        reply = drv.answer(ctx, clar, complete=True)
        assert reply["clarification"]["status"] == "answered", f"status {reply['clarification']['status']!r}"
        assert reply["stillMissing"] is False
        assert reply["requestStatus"] == clar["resumeStatus"], f"Request resumed in {reply['requestStatus']!r}, Clarification said {clar['resumeStatus']!r}"
        now = wait_until(lambda: (lambda r: r if r["status"] == clar["resumeStatus"] else None)(drv.get(ctx, flow["id"])), timeout=15, what="Request to resume")
        assert now["status"] == "planning", f"a task Request must resume in planning, got {now['status']!r}"
    rec.run("RQL-16", "Completing the answer moves the Request to the Clarification's resume status", complete_answer_resumes)

    def answered_is_final():
        clar = flow.get("clarification")
        if not clar:
            raise Blocked("RQL-14 did not complete")
        before = ws.call("clarification.get", {"clarificationId": clar["id"]})["clarification"]
        if before["status"] != "answered":
            raise Blocked("RQL-16 did not complete the Clarification")
        stored = next(q for q in before["questions"] if q["questionKey"] == "acceptance_criteria")["answer"]
        try:
            drv.answer(ctx, before, text="A different answer sent after the round was completed.")
        except RpcError:
            return  # rejected: best outcome
        after = ws.call("clarification.get", {"clarificationId": clar["id"]})["clarification"]
        kept = next(q for q in after["questions"] if q["questionKey"] == "acceptance_criteria")["answer"]
        assert kept == stored, f"a completed answer was overwritten: {stored!r} -> {kept!r}"
    rec.run("RQL-17", "A completed Clarification cannot be overwritten by a later answer", answered_is_final)

    def listed_by_status():
        if "id" not in flow:
            raise Blocked("RQL-06 did not create a Request")
        reply = ws.call("request.list", {"projectId": ctx.state["project_id"], "status": ["planning"], "pageSize": 50})
        assert any(r["id"] == flow["id"] for r in reply["requests"]), "request.list(status=planning) does not return the Request"
    rec.run("RQL-19", "request.list filters by status", listed_by_status)

    def write_rate_limit_applies():
        # CR-REQ-035: writes are throttled (5/s refill, burst 10). A burst of 40 unthrottled writes must trip the limiter.
        hits = 0
        for _ in range(40):
            try:  # idempotent creates are cheap, valid writes that pass authorization and reach the limiter
                ws.call("request.create", {"projectId": ctx.state["project_id"], "title": f"{ctx.cfg.run_prefix} rate probe", "body": "x",
                                           "clientRequestId": f"{ctx.cfg.run_prefix}-rate-probe"}, retry_rate_limit=False)
            except RpcError as exc:
                hits += exc.has_code("REQUEST_RATE_LIMITED")
        assert hits > 0, "40 rapid write calls were never throttled; the write rate limiter is not active"
        import time
        time.sleep(3)  # let the bucket refill so later suites are not throttled by this probe
        # The probe's idempotent create left one Request behind; register it so the run cancels it.
        probe = ws.call("request.create", {"projectId": ctx.state["project_id"], "title": f"{ctx.cfg.run_prefix} rate probe", "body": "x",
                                           "clientRequestId": f"{ctx.cfg.run_prefix}-rate-probe"})
        ctx.on_cleanup(f"cancel rate-probe request {probe['request']['id']}", lambda rid=probe["request"]["id"]: drv.cancel_if_open(ctx, rid))
    rec.run("RQL-20", "Rapid write bursts are throttled (REQUEST_RATE_LIMITED)", write_rate_limit_applies)
