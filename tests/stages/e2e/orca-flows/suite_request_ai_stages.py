"""AIS: stages that call the AI through the dev server. Without an AI provider they must fail safely; with one they must produce artifacts."""
from __future__ import annotations

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ws_rpc_client import RpcError


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("AIS-00", "A project exists for the AI-stage tests", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return
    held: dict = {}

    def ai_known() -> bool:
        if "ai_available" not in ctx.state:
            raise Blocked("AI availability unknown (RQL-09 did not run)")
        return ctx.state["ai_available"]

    def solution_request() -> dict:
        if "solution" not in held:
            req = drv.create(ctx, "ai solution")["request"]
            held["solution"] = drv.to_first_stage(ctx, req, "change_request", "M")
            assert held["solution"]["status"] == "analyzing", f"expected analyzing, got {held['solution']['status']!r}"
        return held["solution"]

    def plan_request() -> dict:
        if "plan" not in held:
            req = drv.create(ctx, "ai plan")["request"]
            held["plan"] = drv.to_first_stage(ctx, req, "task", "S")
            assert held["plan"]["status"] == "planning", f"expected planning, got {held['plan']['status']!r}"
        return held["plan"]

    def solution_fails_safely_without_ai():
        if ai_known():
            raise Blocked("AI is available; the failure path cannot be exercised (see AIS-02)")
        req = solution_request()
        started = ws.call("solution.generate", {"requestId": req["id"], "idempotencyKey": ctx.cfg.run_prefix + "-sol-noai"}, timeout=45)
        assert started.get("runId"), f"generation did not return a run id: {started!r}"

        def finished():
            listed = ws.call("solution.list", {"requestId": req["id"]})
            run = next((r for r in listed.get("runs", []) if r["id"] == started["runId"]), None)
            return (run, listed) if run and run["status"] != "running" else None
        run, listed = wait_until(finished, timeout=60, what="the generation run to finish")
        assert run["status"] == "failed" and run.get("errorCode"), f"run ended as {run['status']!r} with {run.get('errorCode')!r}; it must fail with an error code"
        assert not listed.get("solutions"), "a failed run left a Solution behind"
        after = drv.get(ctx, req["id"])
        assert after["status"] == "analyzing", f"a failed generation moved the Request to {after['status']!r}"
    rec.run("AIS-01", "Solution generation without an AI ends as a failed run with an error code (Request stays in analyzing)", solution_fails_safely_without_ai)

    def solution_with_ai():
        if not ai_known():
            raise Blocked("needs an AI provider/API key reachable from the dev server (ANTHROPIC_API_KEY on the agent)")
        req = solution_request()
        started = ws.call("solution.generate", {"requestId": req["id"], "idempotencyKey": ctx.cfg.run_prefix + "-sol-ai"}, timeout=120)
        assert started.get("runId"), f"generation did not return a run id: {started!r}"

        # Generation is asynchronous: the call returns a run id and the Solution appears when the run completes.
        def finished():
            listed = ws.call("solution.list", {"requestId": req["id"]})
            run = next((r for r in listed.get("runs", []) if r["id"] == started["runId"]), None)
            return (run, listed) if run and run["status"] != "running" else None
        run, listed = wait_until(finished, timeout=240, what="the AI generation run to finish")
        assert run["status"] != "failed", f"AI generation failed with {run.get('errorCode')!r}: {str(run.get('errorMessage', ''))[:200]}"
        assert listed.get("solutions"), f"run ended as {run['status']!r} but solution.list is empty"
    rec.run("AIS-02", "With an AI available, a Solution is generated and listed", solution_with_ai)

    def plan_fails_safely_without_ai():
        if ai_known():
            raise Blocked("AI is available; the failure path cannot be exercised (see AIS-04)")
        req = plan_request()
        try:
            ws.call("request.generatePlan", {"id": req["id"]}, timeout=45)
        except RpcError as exc:
            assert exc.message, "empty error"
        else:
            raise AssertionError("request.generatePlan reported success although no AI provider is available")
        after = drv.get(ctx, req["id"])
        assert after["status"] == "planning", f"a failed plan generation moved the Request to {after['status']!r}"
    rec.run("AIS-03", "Plan generation fails safely when no AI is available (Request stays in planning)", plan_fails_safely_without_ai)

    def plan_with_ai():
        if not ai_known():
            raise Blocked("needs an AI provider/API key reachable from the dev server (ANTHROPIC_API_KEY on the agent)")
        req = plan_request()
        started = ws.call("request.generatePlan", {"id": req["id"]}, timeout=60)
        assert started.get("runId"), f"generatePlan returned no run id: {started!r}"

        def finished():
            view = ws.call("request.planProposal", {"id": req["id"], "runId": started["runId"]})
            return view if view.get("status") != "running" else None
        view = wait_until(finished, timeout=240, interval=2.0, what="the Plan run to finish")
        assert view["status"] == "succeeded", f"plan run ended {view['status']!r}: {view.get('errorCode')}"
        assert view.get("proposal"), "plan generated but request.planProposal is empty"
    rec.run("AIS-04", "With an AI available, a Plan proposal is generated", plan_with_ai)

    def approval_gate_listing():
        pending = ws.call("approval.listPending", {})
        assert isinstance(pending, (list, dict)), f"unexpected reply {pending!r}"
    rec.run("AIS-05", "The approval inbox answers (approval.listPending)", approval_gate_listing)

    def backlog_views():
        for channel in ("backlog.requests", "backlog.tasks"):
            try:
                ws.call(channel, {"projectId": ctx.state["project_id"]})
            except RpcError as exc:
                raise AssertionError(f"{channel} failed: {exc.message[:200]}")
    rec.run("AIS-06", "Backlog views answer (backlog.requests, backlog.tasks)", backlog_views)
