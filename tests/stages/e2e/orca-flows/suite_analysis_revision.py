"""ARV: changing an analysis by prompt (CR-REQ-041) on the LedgerLite finance fixture repo, with the fake claude.

  ARV-01..02  change_request: a Solution revised twice is three versions (1 current, 2 superseded), a new pending approval after each
              round, revisionFeedback on each; the activity feed has one analysis.revision_requested row per round and never the text
  ARV-03..05  bug Diagnosis, spike Findings, question Answer: one revision each, same rules (the gate subject differs)
  ARV-06      the gate's own rules: an over-long ask and a plain Regenerate at the gate are refused and change nothing
  ARV-07..08  an approver who is not the reporter can revise; a stranger cannot (needs docker for the approval policy row)
  ARV-09..10  a double click with one idempotencyKey is one run; a second change request while one runs is REQUEST_SOLUTION_WRONG_STATE
  ARV-11..12  a revision that fails leaves the Request in analyzing, nothing to approve, and Regenerate (no text) carries the ask forward
  ARV-13      hotfix has no analysis gate, so a change request is refused for that reason

The fake claude echoes a change request into the document ("[revised, round N, asked: ...]"), which proves the ask and the earlier
versions reached the model. It also fails the first answer to "[stub:fail-once]" and delays "[stub:slow]" (fake_claude_cli/claude).
With a real Claude these cases cost money and the echo does not exist, so they run only with --ai-stub or ORCA_E2E_ANALYSIS_REVISION=1
(and then without the echo, fail-once and slow cases).
Run: python3 run_acceptance.py --ai-stub devserver analysis_revision
"""
from __future__ import annotations

import json
import os
import shutil

import analysis_revision_users as people
import base_ref_stack as stack
import ledgerlite_fixture as ledgerlite
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from analysis_revision_driver import GATE, RUN_WAIT, Flow, echo, new_flow
from ws_rpc_client import RpcError

REFUSALS = ("REQUEST_SOLUTION_FORBIDDEN", "PERMISSION_DENIED", "FORBIDDEN", "NOT_FOUND")  # a stranger is refused; which layer says so may differ


def expect(code: str, call) -> RpcError:
    try:
        call()
    except RpcError as exc:
        assert exc.has_code(code), f"expected {code}, got: {exc.message[:240]}"
        return exc
    raise AssertionError(f"the call succeeded, expected {code}")


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    stub = bool(ctx.state.get("ai_stub"))
    held: dict = {}
    asks = [f"{cfg.run_prefix} keep the public API unchanged", f"{cfg.run_prefix} add a rollback step to the rollout"]

    def ready() -> str:
        """The project on the LedgerLite repo; skips unless AI analyses may run, blocks when the environment cannot."""
        if not (stub or os.environ.get("ORCA_E2E_ANALYSIS_REVISION", "").lower() in ("1", "true", "yes")):
            raise Skipped("starts AI analyses; use --ai-stub (fake claude, free) or set ORCA_E2E_ANALYSIS_REVISION=1 for a real Claude")
        if ctx.state.get("ai_available") is False:
            raise Blocked("no AI is available on the dev server")
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-arv")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo", repo.cleanup)
            held["project"] = stack.register_project(ctx, repo, "arv-project")
        return held["project"]

    def flow_of(key: str, typ: str, title: str, size: str = "M", first: bool = True) -> Flow:
        if key not in held:
            held[key] = new_flow(ctx, ready(), typ, title, size)
            if first:
                held[key].first_attempt()
        return held[key]

    def need(key: str, test_id: str) -> Flow:
        if key not in held:
            raise Blocked(f"{test_id} did not leave a Request to work on")
        return held[key]

    def needs_stub(what: str) -> None:
        if not stub:
            raise Skipped(f"{what} is a fake claude trigger; it needs --ai-stub")

    # ── change_request: revised twice ─────────────────────────────────────────────────────────────────
    def revised_twice():
        flow = flow_of("cr", "change_request", "arv solution revised twice")
        flow.assert_history(["proposed"], [""])
        flow.assert_one_pending_on_newest()
        first_id = flow.versions()[0]["id"]
        for round_no, ask in enumerate(asks, start=2):
            flow.revise_and_wait(ask)
            flow.assert_one_pending_on_newest()
            versions = flow.versions()
            assert len(versions) == round_no, f"{len(versions)} versions after round {round_no}"
            assert first_id not in {a["subjectId"] for a in flow.pending()}, "the approval of the first version is still pending"
            if stub:
                assert echo(round_no, ask) in flow.document_text(versions[-1]), f"the document of round {round_no} lacks the fake claude's echo of the ask"
        flow.assert_history(["superseded", "superseded", "proposed"], ["", *asks])
        assert flow.status() == GATE, f"after the revisions the Request is {flow.status()!r}"
    rec.run("ARV-01", "A Solution revised twice is 3 versions (2 superseded, 1 current), a new pending approval after each round, revisionFeedback set", revised_twice)

    def activity_rows():
        flow = need("cr", "ARV-01")
        rows = flow.activity("analysis.revision_requested")
        assert len(rows) == len(asks), f"{len(rows)} analysis.revision_requested rows for {len(asks)} change requests"
        assert {r["subjectId"] for r in rows} == {v["id"] for v in flow.versions()[1:]}, "the rows do not point at the versions the revisions produced"
        for r in rows:
            assert (r["summary"], r["actorKind"], r["subjectType"], r["severity"]) == ("Requested changes to the analysis", "user", "solution", "info"), f"row = {r!r}"
            assert r.get("actorId"), "the row does not name who asked"
        everything = json.dumps(ws.call("request.activity", {"id": flow.id, "pageSize": 100}), ensure_ascii=False)
        assert not any(ask in everything for ask in asks), "the text of a change request leaked into the activity feed"
    rec.run("ARV-02", "Activity has one analysis.revision_requested row per round and never the text of the ask", activity_rows)

    # ── the other analysis kinds ──────────────────────────────────────────────────────────────────────
    for test_id, typ in (("ARV-03", "bug"), ("ARV-04", "spike"), ("ARV-05", "question")):
        def one(typ=typ):
            flow = flow_of(typ, typ, f"arv {typ} revised")
            ask = f"{cfg.run_prefix} narrow the {flow.kind} to the daily limit"
            flow.revise_and_wait(ask)
            flow.assert_history(["superseded", "proposed"], ["", ask])
            flow.assert_one_pending_on_newest()
            if stub:
                assert echo(2, ask) in flow.document_text(flow.versions()[-1]), "the revised document lacks the fake claude's echo of the ask"
        rec.run(test_id, f"A '{typ}' Request's analysis can be revised once: 2 versions, a pending approval on the new one", one)

    # ── the gate's own rules ──────────────────────────────────────────────────────────────────────────
    def gate_rules():
        flow = need("cr", "ARV-01")
        versions, runs = len(flow.versions()), len(flow.runs())
        exc = None
        try:
            ws.call("solution.generate", {"requestId": flow.id, "feedback": "x" * 2001})
        except RpcError as err:
            exc = err
        assert exc and (exc.has_code("INVALID_ARGUMENT") or "2000" in exc.message), f"an ask over 2000 characters was not refused as invalid: {exc and exc.message[:200]}"
        expect("REQUEST_SOLUTION_WRONG_STATE", lambda: ws.call("solution.generate", {"requestId": flow.id}))
        assert (len(flow.versions()), len(flow.runs()), flow.status()) == (versions, runs, GATE), "a refused call changed versions, runs or the Request"
    rec.run("ARV-06", "An over-long ask is invalid and a plain Regenerate at the gate is WRONG_STATE; neither changes anything", gate_rules)

    # ── who may revise ────────────────────────────────────────────────────────────────────────────────
    def approver_revises():
        project = ready()
        held["approver"], held["stranger"] = people.create_user(ctx, "approver"), people.create_user(ctx, "stranger")
        people.name_approver(ctx, project, held["approver"])  # before the Request reaches the gate: approvers are snapshotted when it opens
        flow = flow_of("approver_flow", "change_request", "arv approver revises")
        ask = f"{cfg.run_prefix} approver asks for a smaller first step"
        reply = flow.revise(ask, ws=held["approver"].ws)
        run_ = flow.wait_run(reply["runId"])
        assert run_["status"] == "succeeded", f"the approver's revision ended {run_['status']!r}"
        flow.wait_gate()
        flow.assert_history(["superseded", "proposed"], ["", ask])
        flow.assert_one_pending_on_newest()
        row = flow.activity("analysis.revision_requested")
        assert [r.get("actorId") for r in row] == [held["approver"].id], f"the revision is not attributed to the approver: {row!r}"
        seen = held["approver"].ws.call("solution.list", {"requestId": flow.id})["solutions"]
        assert any(s.get("revisionFeedback") == ask for s in seen), "the approver cannot read the revisionFeedback"
    rec.run("ARV-07", "An approver who is not the reporter can request changes; the revision is attributed to them", approver_revises)

    def stranger_refused():
        flow = need("approver_flow", "ARV-07")
        stranger = held["stranger"]
        before = (len(flow.versions()), len(flow.runs()), flow.status(), [a["id"] for a in flow.pending()])
        exc = None
        try:
            flow.revise(f"{cfg.run_prefix} a stranger asks", ws=stranger.ws)
        except RpcError as err:
            exc = err
        assert exc and any(exc.has_code(c) for c in REFUSALS), f"a stranger's change request was not refused: {exc and exc.message[:200]}"
        assert (len(flow.versions()), len(flow.runs()), flow.status(), [a["id"] for a in flow.pending()]) == before, "a refused stranger changed the analysis"
    rec.run("ARV-08", "A user who is neither the reporter, an admin nor an approver is refused and changes nothing", stranger_refused)

    # ── double click and a second change request ──────────────────────────────────────────────────────
    def double_click():
        needs_stub("[stub:slow]")
        flow = flow_of("race", "change_request", "arv race")
        ask = f"{cfg.run_prefix} [stub:slow] widen the batch size"
        held["race_ask"], held["race_runs"] = ask, len(flow.runs())
        key = f"{cfg.run_prefix}-click"
        first = flow.revise(ask, key=key)
        again = flow.revise(ask, key=key)
        assert (again["runId"], again["solutionId"]) == (first["runId"], first["solutionId"]), f"the second click started another run: {first!r} vs {again!r}"
        held["race_run"] = first["runId"]
        assert len(flow.runs()) == held["race_runs"] + 1, f"{len(flow.runs()) - held['race_runs']} runs started by two clicks"
    rec.run("ARV-09", "A double click with one idempotencyKey starts one run", double_click)

    def second_while_running():
        flow = need("race", "ARV-09")
        if "race_run" not in held:
            raise Blocked("ARV-09 did not start the revision")
        expect("REQUEST_SOLUTION_WRONG_STATE", lambda: flow.revise(f"{cfg.run_prefix} a second ask", key=f"{cfg.run_prefix}-other"))
        quiet = ws.call("solution.generate", {"requestId": flow.id})
        assert quiet["runId"] == held["race_run"], "a plain Regenerate while a run is going must return that run"
        assert flow.wait_run(held["race_run"])["status"] == "succeeded"
        flow.wait_gate()
        assert len(flow.runs()) == held["race_runs"] + 1, "the refused change request still started a run"
        flow.assert_history(["superseded", "proposed"], ["", held["race_ask"]])
        flow.assert_one_pending_on_newest()
    rec.run("ARV-10", "A second change request while one runs is REQUEST_SOLUTION_WRONG_STATE; only the first run exists", second_while_running)

    # ── a revision that fails ─────────────────────────────────────────────────────────────────────────
    for test_id, typ in (("ARV-11", "change_request"), ("ARV-12", "question")):
        def failing(typ=typ):
            needs_stub("[stub:fail-once]")
            flow = flow_of(f"fail_{typ}", typ, f"arv {typ} failing revision")
            ask = f"{cfg.run_prefix} [stub:fail-once] drop the cache layer of the {typ} analysis"
            run_ = flow.wait_run(flow.revise(ask)["runId"])
            assert run_["status"] == "failed" and run_.get("errorCode"), f"the scripted failure did not fail the run: {run_!r}"
            assert flow.status() == "analyzing", f"after the failed revision the Request is {flow.status()!r}, expected analyzing"
            assert [v["status"] for v in flow.versions()] == ["superseded"] and not flow.pending(), "a failed revision must leave no proposal and nothing to approve"
            retry = ws.call("solution.generate", {"requestId": flow.id}, timeout=120)  # Regenerate, no text
            assert retry["runId"] != run_["id"], "Regenerate did not start a new run"
            assert flow.wait_run(retry["runId"])["status"] == "succeeded", "the retry did not succeed"
            flow.wait_gate()
            flow.assert_history(["superseded", "proposed"], ["", ask])
            flow.assert_one_pending_on_newest()
            assert echo(2, ask) in flow.document_text(flow.versions()[-1]), "the retried document lacks the echo: the ask or the superseded version did not reach the model"
        rec.run(test_id, f"A failed {typ} revision leaves the Request in analyzing; Regenerate without text retries with the same ask", failing)

    # ── hotfix: no gate ───────────────────────────────────────────────────────────────────────────────
    def hotfix_has_no_gate():
        req = drv.create(ctx, "arv hotfix", project_id=ready())["request"]
        cur = drv.to_first_stage(ctx, req, "hotfix", "S", "urgent")
        assert cur["status"] == "analyzing", f"after the readiness answer the Request is {cur['status']!r}, expected analyzing"
        flow = Flow(ctx, req["id"], "hotfix")
        try:
            after = wait_until(lambda: (lambda r: r if r["status"] != "analyzing" else None)(drv.get(ctx, req["id"])), timeout=RUN_WAIT, interval=1.0,
                               what="the hotfix analysis to end")
        except AssertionError as exc:
            last = next(iter(reversed(flow.runs())), {})
            raise Blocked(f"the hotfix analysis did not finish in this environment ({last.get('status')} {last.get('errorCode')}): {exc}") from exc
        assert after["status"] != GATE, "a hotfix stopped at the analysis gate; it has none"
        runs, versions = len(flow.runs()), len(flow.versions())
        expect("REQUEST_SOLUTION_WRONG_STATE", lambda: ws.call("solution.generate", {"requestId": req["id"], "feedback": f"{cfg.run_prefix} change the hotfix"}))
        assert (len(flow.runs()), len(flow.versions()), flow.status()) == (runs, versions, after["status"]), "the refused change request changed the hotfix"
    rec.run("ARV-13", "A hotfix has no analysis gate: a change request is refused WRONG_STATE and starts nothing", hotfix_has_no_gate)
