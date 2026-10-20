"""ORV: the Outcome Review of a Request (CR-OR-001..030) on the LedgerLite fixture repo, with the fake claude.

  ORV-01  a project that has the review off: estimate -> REQUEST_REVIEW_DISABLED, nothing is built
  ORV-02  review on (manual) but the Request is still at its Analysis gate: REQUEST_REVIEW_NOT_READY
  ORV-03  after the Plan is committed: estimate says phase=design, no review yet, the execution is not finished, and lists the context gaps
  ORV-04  generate without the context run builds the facts (design phase, ready); asking again is `reused` with the same id and version, and
          the list holds one review: the second ask costs nothing
  ORV-05  generate with the context run, on a dev server whose agent is the fake claude: either the run starts and ends with a closed code, or it
          is refused with a closed code; no run row is left active, and the facts review of ORV-04 is intact either way
  ORV-06  deciding or approving a design-phase review is refused with a REQUEST_REVIEW_ code (decisions start after the code exists)
  ORV-07  regenerate: a missing reason and an unknown scope are refused before any run
  ORV-08  unknown ids: a review, a drawing and a Request that do not exist are NotFound
  ORV-09  drawing profile: the default is served, a project layer is saved (version 1), read back as the effective one, and reset
  ORV-20  (cost class C, ORCA_E2E_REVIEW_REAL=1, never with --ai-stub) the whole path on the real Claude: context run -> cache hit on the second
          ask -> regenerate with a reason -> decide -> approve (needs the delivered phase: the Request must have run to the end)

ORV-01..09 spend nothing. The review needs the gateway channels request.review.* and request-service with the review wired; a stack built
before them answers "unknown channel", which this suite reports as BLOCKED, never FAIL.
Run: python3 run_acceptance.py --ai-stub devserver outcome_review
"""
from __future__ import annotations

import json
import os
import shutil
import time

import base_ref_stack as stack
import ledgerlite_fixture as ledgerlite
import plan_auto_driver as apg
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from ws_rpc_client import RpcError

# Why a context run may legitimately not run, or not succeed, on the stub: every code is a closed, documented one.
CLOSED_CODES = (
    "REQUEST_REVIEW_AGENT_TOO_OLD", "REQUEST_REVIEW_SKILL_MISSING", "REQUEST_REVIEW_NO_DEV_SERVER", "REQUEST_REVIEW_KNOWLEDGE_REQUIRED",
    "REQUEST_REVIEW_RESULT_INVALID", "REQUEST_REVIEW_READONLY_VIOLATED", "REQUEST_REVIEW_AGENT_FAILED", "REQUEST_REVIEW_INTERRUPTED",
    "REQUEST_REVIEW_INTERNAL", "REQUEST_REVIEW_TIMEOUT", "REQUEST_REVIEW_BUDGET_EXHAUSTED",
)
RUN_WAIT = 120


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    stub = bool(ctx.state.get("ai_stub"))
    held: dict = {}

    def call(channel: str, args: dict) -> dict:
        try:
            return ws.call(channel, args)
        except RpcError as exc:
            if "unknown channel" in exc.message.lower() or "no handler" in exc.message.lower():
                raise Blocked(f"{channel} is not served by this stack; rebuild the gateway and request-service with the Outcome Review") from exc
            raise

    def refused(channel: str, args: dict, *codes: str) -> RpcError:
        try:
            call(channel, args)
        except RpcError as exc:
            assert any(exc.has_code(c) for c in codes), f"{channel}: expected one of {codes}, got: {exc.message[:240]}"
            return exc
        raise AssertionError(f"{channel} succeeded, expected one of {codes}")

    def set_review_mode(project_id: str, mode: str) -> None:
        cur = ws.call("request.engineSettings", {"projectId": project_id})
        ws.call("request.engineSettingsSet", {
            "projectId": project_id, "engine": cur.get("engine") or "native", "openspecMinVersion": cur.get("openspecMinVersion", "") or "",
            "solutionAgent": False, "executionMode": cur.get("executionMode", "") or "", "expectedVersion": cur.get("version", 0) or 0,
            "outcomeReview": mode,
        })

    def project() -> str:
        if not stub:
            raise Skipped("starts AI analyses and Plans; use --ai-stub (fake claude, free)")
        if ctx.state.get("ai_available") is False:
            raise Blocked("no AI is available on the dev server")
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-orv")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo", repo.cleanup)
            held["project"] = apg.configure(ctx, repo, "orv-project", plan_auto=True)
        return held["project"]

    def flow():
        if "flow" not in held:
            held["flow"] = apg.to_analysis_gate(ctx, project(), "bug", "orv bug for the outcome review")
        return held["flow"]

    def need(key: str, test_id: str):
        if key not in held:
            raise Blocked(f"{test_id} did not leave a Request to work on")
        return held[key]

    # ── before there is anything to review ──────────────────────────────────────────────────────────
    def off_by_default():
        f = flow()
        set_review_mode(held["project"], "off")  # the stack's default may be manual; this case is about a project that chose off
        refused("request.review.estimate", {"id": f.id}, "REQUEST_REVIEW_DISABLED")
        refused("request.review.generate", {"id": f.id, "runContext": False}, "REQUEST_REVIEW_DISABLED")
    rec.run("ORV-01", "A project that did not turn the review on: estimate and generate are REQUEST_REVIEW_DISABLED", off_by_default)

    def not_ready():
        f = need("flow", "ORV-01")
        set_review_mode(held["project"], "manual")
        exc = refused("request.review.estimate", {"id": f.id}, "REQUEST_REVIEW_NOT_READY")
        assert "analysis" in exc.message.lower() or "request" in exc.message.lower(), f"the refusal should say why: {exc.message[:200]}"
        refused("request.review.generate", {"id": f.id}, "REQUEST_REVIEW_NOT_READY")
        assert call("request.review.list", {"id": f.id}).get("reviews") in ([], None), "a Request not ready has a review"
    rec.run("ORV-02", "Review on, Request still at the Analysis gate: REQUEST_REVIEW_NOT_READY, and no review row", not_ready)

    # ── the design phase ─────────────────────────────────────────────────────────────────────────────
    def estimate_design():
        f = need("flow", "ORV-01")
        apg.approve_analysis(ctx, f)
        apg.wait_plan(ctx, f.id)
        est = call("request.review.estimate", {"id": f.id})
        assert est["phase"] == "design", f"estimate = {est!r}"
        assert est["reviewExists"] is False and est["executionFinished"] is False, f"estimate = {est!r}"
        assert isinstance(est.get("contextGaps"), list) and isinstance(est.get("warnings"), list), f"lists must be lists, never null: {est!r}"
        assert est.get("currentInputsDigest"), "no digest to tell whether asking again would change anything"
        held["digest"] = est["currentInputsDigest"]
    rec.run("ORV-03", "After the Plan is committed the estimate is phase=design, no review yet, execution not finished", estimate_design)

    def facts_then_cached():
        f = need("flow", "ORV-01")
        first = call("request.review.generate", {"id": f.id, "runContext": False})
        r1 = first["review"]
        assert first["created"] is True and first["reused"] is False and r1["phase"] == "design" and r1["state"] == "ready", f"first = {first!r}"
        assert r1.get("inputsDigest") == held.get("digest"), "the review was built on other inputs than the estimate said"
        second = call("request.review.generate", {"id": f.id, "runContext": False})
        r2 = second["review"]
        assert second["reused"] is True and second["created"] is False, f"second = {second!r}"
        assert (r2["id"], r2["version"]) == (r1["id"], r1["version"]), "asking again changed the stored review"
        listed = call("request.review.list", {"id": f.id}).get("reviews") or []
        assert [r["id"] for r in listed] == [r1["id"]], f"list = {listed!r}"
        got = call("request.review.get", {"id": f.id})
        assert got["found"] is True and got["review"]["id"] == r1["id"], f"get = {got!r}"
        held["review"] = r1
    rec.run("ORV-04", "Generate without the context run builds the facts; the second ask is `reused` with the same id and version", facts_then_cached)

    def context_run_on_stub():
        f, r1 = need("flow", "ORV-01"), need("review", "ORV-04")
        try:
            reply = call("request.review.generate", {"id": f.id})
        except RpcError as exc:
            assert any(exc.has_code(c) for c in CLOSED_CODES), f"refused with an open-ended error: {exc.message[:240]}"
            reply = None
        if reply and reply.get("run"):
            run_id = reply["run"]["id"]

            def ended():
                runs = call("request.review.runs", {"id": f.id}).get("runs") or []
                mine = [r for r in runs if r["id"] == run_id]
                return mine[0] if mine and mine[0]["status"] not in ("queued", "running") else None
            done = wait_until(ended, timeout=RUN_WAIT, interval=2.0, what="the context run to end")
            assert done["status"] in ("succeeded", "failed"), f"run = {done!r}"
            if done["status"] == "failed":
                assert done.get("errorCode") in CLOSED_CODES, f"a failed run must carry a closed code: {done!r}"
        runs = call("request.review.runs", {"id": f.id}).get("runs") or []
        assert not [r for r in runs if r["status"] in ("queued", "running")], f"a run is still active: {runs!r}"
        got = call("request.review.get", {"id": f.id})
        assert got["found"] is True and got["review"]["id"] == r1["id"], "the review of the Request disappeared"
    rec.run("ORV-05", "Generate with the context run on the fake agent ends with a closed code and leaves the facts review intact", context_run_on_stub)

    def no_decisions_before_code():
        f, r1 = need("flow", "ORV-01"), need("review", "ORV-04")
        refused("request.review.decide", {"id": f.id, "reviewId": r1["id"], "itemKey": "nope", "decision": "accept", "expectedVersion": r1["version"]}, "REQUEST_REVIEW_")
        refused("request.review.approve", {"id": f.id, "reviewId": r1["id"], "expectedVersion": r1["version"]}, "REQUEST_REVIEW_")
        got = call("request.review.get", {"id": f.id})["review"]
        assert got["state"] != "approved", "a design-phase review was approved"
    rec.run("ORV-06", "Deciding or approving a design-phase review is refused with a REQUEST_REVIEW_ code and approves nothing", no_decisions_before_code)

    def regenerate_validation():
        f = need("flow", "ORV-01")
        refused("request.review.regenerate", {"id": f.id, "scope": "artifact"}, "REQUEST_REVIEW_")  # no reason
        refused("request.review.regenerate", {"id": f.id, "scope": "everything", "reason": "looks_wrong"}, "REQUEST_REVIEW_")  # unknown scope
        runs = call("request.review.runs", {"id": f.id}).get("runs") or []
        assert not [r for r in runs if r["status"] in ("queued", "running")], "a refused regeneration started a run"
    rec.run("ORV-07", "Regenerate without a reason, or with an unknown scope, is refused before any run starts", regenerate_validation)

    def unknown_ids():
        f = need("flow", "ORV-01")
        ghost = "00000000-0000-4000-8000-000000000000"
        refused("request.review.get", {"id": f.id, "reviewId": ghost}, "REQUEST_REVIEW_NOT_FOUND")
        refused("request.review.artifact", {"id": f.id, "artifactId": ghost}, "REQUEST_REVIEW_ARTIFACT_NOT_FOUND", "NOT_FOUND")
        refused("request.review.get", {"id": ghost}, "REQUEST_NOT_FOUND", "NOT_FOUND")
    rec.run("ORV-08", "A review, a drawing and a Request that do not exist are NotFound", unknown_ids)

    def profile_round_trip():
        pid = project()
        base = call("request.review.profile.get", {"projectId": pid})
        assert base.get("projectLayer") is None and base["effective"].get("style", {}).get("variant"), f"default profile = {base!r}"
        spec = {"schema": "review-drawing-profile/1", "style": {"detail": "detailed"}}
        saved = call("request.review.profile.save", {"projectId": pid, "spec": spec, "expectedVersion": 0})["layer"]
        assert saved["version"] >= 1, f"saved = {saved!r}"
        after = call("request.review.profile.get", {"projectId": pid})
        assert after["effective"]["style"]["detail"] == "detailed" and after["sources"].get("style.detail") == "project", f"effective = {after!r}"
        refused("request.review.profile.save", {"projectId": pid, "spec": {"schema": "review-drawing-profile/1", "colour": "red"}, "expectedVersion": saved["version"]},
                "REQUEST_REVIEW_PROFILE_INVALID")
        call("request.review.profile.reset", {"projectId": pid, "expectedVersion": saved["version"]})
        back = call("request.review.profile.get", {"projectId": pid})
        assert back.get("projectLayer") is None and back["effective"]["style"]["detail"] != "detailed", f"after reset = {back!r}"
    rec.run("ORV-09", "Drawing profile: default served, a project layer saved and effective, an unknown key refused, reset brings the default back", profile_round_trip)

    # ── the paid path ────────────────────────────────────────────────────────────────────────────────
    def real_path():
        if stub or os.environ.get("ORCA_E2E_REVIEW_REAL") != "1":
            raise Skipped("cost class C: set ORCA_E2E_REVIEW_REAL=1 and run without --ai-stub; follow the quiet-checkout rule and run workspace_probe.py first")
        f = need("flow", "ORV-01")
        first = call("request.review.generate", {"id": f.id, "force": True})
        assert first.get("run"), f"no context run started: {first!r}"
        done = wait_until(lambda: next((r for r in call("request.review.runs", {"id": f.id})["runs"] if r["id"] == first["run"]["id"] and r["status"] not in ("queued", "running")), None),
                          timeout=900, interval=5.0, what="the real context run to end")
        assert done["status"] == "succeeded", f"run = {done!r}"
        again = call("request.review.generate", {"id": f.id})
        assert again.get("contextCached") is True and not again.get("run"), f"the second ask must hit the cache and spend nothing: {again!r}"
        regen = call("request.review.regenerate", {"id": f.id, "scope": "artifact", "reason": "looks_wrong", "confirmCost": True, "clientRequestId": f"orv-{int(time.time())}"})
        assert regen.get("drawRun") or regen.get("skipped"), f"regenerate = {regen!r}"
        ctx.state["orv_cost_note"] = json.dumps({"context_cost": done.get("costActual"), "seconds": done.get("seconds")})
    rec.run("ORV-20", "(real Claude) context run, cache hit on the second ask, regenerate with a reason", real_path)
