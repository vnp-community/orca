"""PRV: revising a Plan by prompt and reading its documents (CR-REQ-045) on the LedgerLite finance fixture repo, with the fake claude.

  PRV-01      a bug reaches awaiting_plan_approval by itself; request.planRevisions has one current version and planProposal {revision:1} reads it
  PRV-02      revised twice: 3 versions (1 current, 2 superseded), each round cancels the open Approval (revision_requested) and opens exactly one new
              one, revisionFeedback is the ask, the notes echo it; no client commit is ever sent: the server's `revision` job commits (PRV-08b)
  PRV-03      the activity feed has one plan.revision_requested row per round and never the text of an ask
  PRV-04      an ask of 2000 characters is accepted, 2001 is REQUEST_PLAN_FEEDBACK_TOO_LONG and changes nothing
  PRV-05      an approver who is a project member can revise (recorded as the actor); a member who is no approver and a stranger cannot
  PRV-06      a double click with one idempotencyKey is one run; a second ask while one runs is REQUEST_PLAN_REVISION_IN_PROGRESS
  PRV-07      a failed revision leaves the Request in planning with nothing to approve and its job gave_up; Generate without text retries with the
              same ask and succeeds; only the approver who asked (or a person who may generate) may retry
  PRV-08      a hotfix cannot be revised (REQUEST_PLAN_REVISION_NOT_APPLICABLE); PRV-08b the server commits a revision with no client commit call
  PRV-09      (not run) a dev server that is offline: no tool in this suite disconnects it
  PRV-10      (ORCA_E2E_PLAN_REVISION_LONG=1) eleven revisions in a row: 12 versions, listed in pages
  PRV-11..13  a planted OpenSpec change (plan_documents_seed.py: FAKE data, local docker stack only): the file list with the real size of a file too
              large to keep; a file read masks the planted secret and reports the omitted file; an old version keeps its text or has it pruned
  PRV-14..15  (not run) a fake claude that authors OpenSpec: see the notes of TASK-REQ-045-09

The fake claude echoes a revision into the Plan notes and honours [stub:fail-once] and [stub:slow] in the ask (fake_claude_cli/claude). With a real
Claude these cases cost money and the echo does not exist, so the suite runs only with --ai-stub.
Run: python3 run_acceptance.py --ai-stub devserver plan_revision
"""
from __future__ import annotations

import json
import os
import shutil

import base_ref_stack as stack
import ledgerlite_fixture as ledgerlite
import plan_auto_driver as apg
import plan_revision_driver as prd
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from analysis_revision_users import create_user
from ws_rpc_client import RpcError

REFUSALS = ("REQUEST_PLAN_FORBIDDEN", "REQUEST_SOLUTION_FORBIDDEN", "PERMISSION_DENIED", "FORBIDDEN", "NOT_FOUND")  # which layer refuses a stranger may differ


def expect(code: str, call) -> RpcError:
    try:
        call()
    except RpcError as exc:
        assert exc.has_code(code), f"expected {code}, got: {exc.message[:240]}"
        return exc
    raise AssertionError(f"the call succeeded, expected {code}")


def refused(call, codes=REFUSALS) -> RpcError:
    try:
        call()
    except RpcError as exc:
        assert any(exc.has_code(c) for c in codes), f"expected one of {codes}, got: {exc.message[:240]}"
        return exc
    raise AssertionError("the call succeeded, expected a refusal")


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    stub = bool(ctx.state.get("ai_stub"))
    held: dict = {}
    asks = [f"{cfg.run_prefix} split the work into smaller steps", f"{cfg.run_prefix} add a rollback task"]

    def ready() -> str:
        if not stub:
            raise Skipped("starts AI analyses and Plans; use --ai-stub (fake claude, free)")
        if ctx.state.get("ai_available") is False:
            raise Blocked("no AI is available on the dev server")
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-prv")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo", repo.cleanup)
            held["project"] = apg.configure(ctx, repo, "prv-project", plan_auto=True)
        return held["project"]

    def flow_of(key: str, title: str) -> prd.PlanFlow:
        if key not in held:
            held[key] = prd.new_plan_flow(ctx, ready(), title)
        return held[key]

    def need(key: str, test_id: str) -> prd.PlanFlow:
        if key not in held:
            raise Blocked(f"{test_id} did not leave a Request to work on")
        return held[key]

    def needs_stub(what: str) -> None:
        if not stub:
            raise Skipped(f"{what} is a fake claude trigger; it needs --ai-stub")

    # ── the first version ─────────────────────────────────────────────────────────────────────────
    def first_version():
        flow = flow_of("main", "prv bug revised twice")
        versions = flow.versions()
        assert [(v["revision"], v["status"], v.get("revisionFeedback", "")) for v in versions] == [(1, "current", "")], f"versions = {versions!r}"
        assert versions[0]["engine"] == "native", f"engine = {versions[0]['engine']!r}"
        one = flow.proposal(1)
        proposal = one.get("proposal") or {}
        assert proposal.get("summary") and (proposal.get("tasks") or proposal.get("phases")), f"planProposal {{revision:1}} = {one!r}"
        assert flow.assert_one_pending()["subjectId"], "the open Plan approval names no Plan"
        held["main_first_approval"] = flow.pending()[0]["id"]
    rec.run("PRV-01", "A bug reaches awaiting_plan_approval; planRevisions has one current version and planProposal {revision:1} reads it", first_version)

    # ── revised twice ─────────────────────────────────────────────────────────────────────────────
    def revised_twice():
        flow = need("main", "PRV-01")
        seen = [held["main_first_approval"]]
        for round_no, ask in enumerate(asks, start=2):
            new = flow.revise_and_wait(ask)
            assert new["id"] not in seen, "the new Approval is the old one"
            old = {a["id"]: a for a in flow.cancelled()}
            assert seen[-1] in old, f"the Approval of the previous version {seen[-1]} was not cancelled; cancelled: {list(old)}"
            assert "revision_requested" in json.dumps(old[seen[-1]]), f"the cancelled Approval does not say why: {old[seen[-1]]!r}"
            seen.append(new["id"])
            versions = flow.versions()
            assert len(versions) == round_no, f"{len(versions)} versions after round {round_no}"
            assert prd.echo(round_no, ask) in flow.notes(), f"the notes of version {round_no} lack the fake claude's echo of the ask: {flow.notes()!r}"
        versions = flow.versions()
        assert [(v["revision"], v["status"]) for v in versions] == [(3, "current"), (2, "superseded"), (1, "superseded")], f"versions = {versions!r}"
        assert [v.get("revisionFeedback", "") for v in versions] == [asks[1], asks[0], ""], f"revisionFeedback per version = {[v.get('revisionFeedback') for v in versions]!r}"
        assert len(set(seen)) == 3, f"approvals seen {seen}"
        assert flow.status() == "awaiting_plan_approval"
        held["main_ids"] = seen
    rec.run("PRV-02", "Revised twice: 3 versions, each round cancels the open Approval and opens one new, revisionFeedback and the echo are right", revised_twice)

    def activity_rows():
        flow = need("main", "PRV-01")
        rows = flow.activity("plan.revision_requested")
        assert len(rows) == len(asks), f"{len(rows)} plan.revision_requested rows for {len(asks)} revisions"
        for r in rows:
            assert r["actorKind"] == "user" and r.get("actorId"), f"row = {r!r}"
        assert not flow.activity("analysis.revision_requested"), "a Plan revision wrote an analysis.revision_requested row"
        everything = json.dumps(ws.call("request.activity", {"id": flow.id, "pageSize": 100}), ensure_ascii=False)
        assert not any(ask in everything for ask in asks), "the text of an ask leaked into the activity feed"
    rec.run("PRV-03", "Activity has one plan.revision_requested row per round, no analysis.revision_requested, and never the text of an ask", activity_rows)

    # ── the length of an ask ──────────────────────────────────────────────────────────────────────
    def feedback_length():
        flow = flow_of("limits", "prv bug ask length")
        before = (len(flow.versions()), len(flow.plan_runs()), flow.status(), [a["id"] for a in flow.pending()])
        exc = expect("REQUEST_PLAN_FEEDBACK_TOO_LONG", lambda: flow.revise("y" * 2001))
        assert exc is not None
        after = (len(flow.versions()), len(flow.plan_runs()), flow.status(), [a["id"] for a in flow.pending()])
        assert after == before, f"a refused ask changed the Request: {before} -> {after}"
        flow.revise_and_wait("z" * 2000)
        assert len(flow.versions()) == 2 and flow.versions()[0].get("revisionFeedback") == "z" * 2000, "an ask of 2000 characters was not kept whole"
    rec.run("PRV-04", "An ask of 2000 characters is accepted; 2001 is REQUEST_PLAN_FEEDBACK_TOO_LONG and changes nothing", feedback_length)

    # ── who may revise ────────────────────────────────────────────────────────────────────────────
    def approver_revises():
        project = ready()
        held["approver"] = prd.name_plan_approver(ctx, project, "prv-approver")  # before the Request reaches the gate: approvers are snapshotted when it opens
        held["member"] = prd.member_only(ctx, project, "prv-member")
        held["stranger"] = create_user(ctx, "prv-stranger")
        flow = flow_of("approver_flow", "prv bug approver revises")
        approver = held["approver"]
        ask = f"{cfg.run_prefix} approver asks for a smaller first step"
        before = {a["id"] for a in flow.pending()}
        flow.revise(ask, ws=approver.ws)
        flow.wait_new_gate(before)
        versions = flow.versions()
        assert (versions[0]["revision"], versions[0].get("revisionFeedback")) == (2, ask), f"versions = {versions!r}"
        assert versions[0].get("actorId") == approver.id, f"the version is not attributed to the approver: {versions[0]!r}"
        assert [r.get("actorId") for r in flow.activity("plan.revision_requested")] == [approver.id], "the activity row does not name the approver"
        seen = approver.ws.call("request.planRevisions", {"id": flow.id}).get("revisions", [])
        assert any(v.get("revisionFeedback") == ask for v in seen), "the approver cannot read the revisionFeedback"
    rec.run("PRV-05a", "An approver who is a project member (not reporter, not admin) can revise; the version and the activity row name them", approver_revises)

    def others_are_refused():
        flow = need("approver_flow", "PRV-05a")
        before = (len(flow.versions()), len(flow.plan_runs()), flow.status(), [a["id"] for a in flow.pending()])
        refused(lambda: flow.revise(f"{cfg.run_prefix} a member asks", ws=held["member"].ws), codes=("REQUEST_PLAN_FORBIDDEN",))
        refused(lambda: flow.revise(f"{cfg.run_prefix} a stranger asks", ws=held["stranger"].ws))
        after = (len(flow.versions()), len(flow.plan_runs()), flow.status(), [a["id"] for a in flow.pending()])
        assert after == before, f"a refused call changed the Plan: {before} -> {after}"
    rec.run("PRV-05b", "A member who is no approver is REQUEST_PLAN_FORBIDDEN and a stranger is refused; the Approval and the version are untouched", others_are_refused)

    # ── double click and a second ask ─────────────────────────────────────────────────────────────
    def double_click_and_second_ask():
        needs_stub("[stub:slow]")
        flow = flow_of("race", "prv bug race")
        ask = f"{cfg.run_prefix} [stub:slow] widen the batch size"
        key = f"{cfg.run_prefix}-prv-click"
        runs_before, before = len(flow.plan_runs()), {a["id"] for a in flow.pending()}
        first = flow.revise(ask, key=key)
        again = flow.revise(ask, key=key)
        assert again["runId"] == first["runId"], f"the second click started another run: {first['runId']} vs {again['runId']}"
        assert len(flow.plan_runs()) == runs_before + 1, f"{len(flow.plan_runs()) - runs_before} runs started by two clicks"
        expect("REQUEST_PLAN_REVISION_IN_PROGRESS", lambda: flow.revise(f"{cfg.run_prefix} a second ask", key=f"{cfg.run_prefix}-prv-other"))
        flow.wait_new_gate(before)
        assert len(flow.plan_runs()) == runs_before + 1, "the refused second ask still started a run"
        assert [v.get("revisionFeedback", "") for v in flow.versions()] == [ask, ""], f"versions = {flow.versions()!r}"
    rec.run("PRV-06", "A double click with one idempotencyKey is one run; a second ask while one runs is REQUEST_PLAN_REVISION_IN_PROGRESS", double_click_and_second_ask)

    # ── a revision that fails ─────────────────────────────────────────────────────────────────────
    def failed_revision_is_retried_with_the_same_ask():
        needs_stub("[stub:fail-once]")
        flow = flow_of("fail", "prv bug failing revision")
        ask = f"{cfg.run_prefix} [stub:fail-once] drop the second task"
        old_ids = {a["id"] for a in flow.pending()}
        flow.revise(ask)
        failed = wait_until(lambda: next((j for j in flow.jobs() if j["origin"] == "revision" and j["state"] in ("gave_up", "done")), None), timeout=prd.REVISION_WAIT,
                            interval=2.0, what="the revision job to end")
        assert failed["state"] == "gave_up", f"the scripted failure ended the job as {failed!r}"
        assert flow.status() == "planning", f"after the failed revision the Request is {flow.status()!r}, expected planning"
        assert not flow.pending(), "a failed revision must leave nothing to approve"
        assert len(flow.versions()) == 1, "a failed revision added a version"
        if "approver" in held:  # the person whose revision failed may retry it; one who may not generate and did not ask may not
            refused(lambda: flow.revise(ws=held["member"].ws), codes=("REQUEST_PLAN_FORBIDDEN",))
        flow.revise()  # Generate without text: the server carries the earlier ask forward
        flow.wait_new_gate(old_ids)
        versions = flow.versions()
        assert (len(versions), versions[0].get("revisionFeedback")) == (2, ask), f"versions after the retry = {versions!r}"
        assert any(j["origin"] == "revision" and j["state"] == "done" for j in flow.jobs()), f"jobs = {flow.jobs()!r}"
        assert prd.echo(2, ask) in flow.notes(), "the retried Plan lacks the echo of the carried-over ask"
    rec.run("PRV-07", "A failed revision leaves planning with nothing to approve; Generate without text retries with the same ask and succeeds", failed_revision_is_retried_with_the_same_ask)

    # ── a hotfix cannot be revised ────────────────────────────────────────────────────────────────
    def hotfix_is_not_revisable():
        pid = ready()
        apg.name_pre_deploy_approver(ctx, pid, "prv-hotfix-approver")
        req = drv.create(ctx, "prv hotfix", project_id=pid)["request"]
        cur = drv.to_first_stage(ctx, req, "hotfix", "S", "urgent")
        assert cur["status"] == "analyzing", f"after the readiness answer the hotfix is {cur['status']!r}"
        try:
            wait_until(lambda: (lambda r: r if r["status"] == "awaiting_plan_approval" else None)(drv.get(ctx, req["id"])), timeout=150, interval=1.0, what="the hotfix to reach its approval")
        except AssertionError as exc:
            raise Blocked(f"the hotfix analysis did not reach its approval in this environment: {exc}") from exc
        runs = len(apg.plan_runs(ctx, pid, req["id"]))
        expect("REQUEST_PLAN_REVISION_NOT_APPLICABLE", lambda: ws.call("request.generatePlan", {"id": req["id"], "feedback": f"{cfg.run_prefix} change the hotfix"}))
        assert (len(apg.plan_runs(ctx, pid, req["id"])), drv.get(ctx, req["id"])["status"]) == (runs, "awaiting_plan_approval"), "the refused ask changed the hotfix"
    rec.run("PRV-08", "A hotfix cannot be revised: REQUEST_PLAN_REVISION_NOT_APPLICABLE, nothing changes", hotfix_is_not_revisable)

    def server_commits_without_a_client():
        flow = need("main", "PRV-02")
        jobs = [(j["origin"], j["state"]) for j in flow.jobs()]
        assert jobs == [("event", "done"), ("revision", "done"), ("revision", "done")], f"jobs of the twice-revised Plan = {jobs!r}: the server's `revision` job must have committed both versions"
    rec.run("PRV-08b", "No client ever sent a commit: the server's `revision` job committed each version (the tab may be closed)", server_commits_without_a_client)

    def dev_server_offline():
        raise Skipped("no tool in this suite disconnects the dev server; the mapping of NO_CONNECTION to REQUEST_PLAN_AI_UNAVAILABLE and the untouched Approval are covered by "
                      "the request-service use case tests")
    rec.run("PRV-09", "A dev server that is offline: REQUEST_PLAN_AI_UNAVAILABLE and the Approval stays", dev_server_offline)

    def eleven_revisions():
        if os.environ.get("ORCA_E2E_PLAN_REVISION_LONG", "").lower() not in ("1", "true", "yes"):
            raise Skipped("ORCA_E2E_PLAN_REVISION_LONG=1 runs eleven revisions in a row (about a minute with the fake claude)")
        flow = flow_of("long", "prv bug eleven revisions")
        for n in range(11):
            flow.revise_and_wait(f"{cfg.run_prefix} round {n + 2}")
        versions = flow.versions()
        assert [v["revision"] for v in versions] == list(range(12, 0, -1)), f"versions = {[v['revision'] for v in versions]!r}"
        first_page = ws.call("request.planRevisions", {"id": flow.id, "pageSize": 5})
        assert len(first_page["revisions"]) == 5 and first_page["nextPageToken"], "the listing is not paged"
    rec.run("PRV-10", "Eleven revisions in a row: 12 versions, listed in pages (ORCA_E2E_PLAN_REVISION_LONG=1)", eleven_revisions)

    # ── a planted OpenSpec change ─────────────────────────────────────────────────────────────────
    def seeded() -> dict:
        if "seed" not in held:
            flow = need("main", "PRV-02")
            if flow.status() != "awaiting_plan_approval":
                raise Blocked("PRV-02 did not leave the Request at the Plan approval")
            held["seed"] = prd.run_seed(ctx, flow.id)
        return held["seed"]

    def planted_files():
        facts = seeded()
        flow = need("main", "PRV-02")
        view = flow.proposal()
        assert view.get("engine") == "openspec", f"engine = {view.get('engine')!r}"
        files = {f["path"]: f for f in (view.get("bundle") or {}).get("files", [])}
        assert {"proposal.md", "design.md", "tasks.md", "specs/ledger/spec.md", facts["omittedPath"]} <= set(files), f"bundle files = {sorted(files)!r}"
        big = files[facts["omittedPath"]]
        assert (big["omitted"], int(big["size"])) == (True, facts["omittedSize"]), f"the omitted file is listed as {big!r}: its size must be the real one, not the marker's length"
        assert not files["design.md"]["omitted"] and int(files["design.md"]["size"]) > 0
        assert "STRUCTURE_OK" in {c["code"] for c in view.get("checks", [])}, f"checks = {view.get('checks')!r}"
        assert (view.get("bundle") or {}).get("changeId") == facts["changeId"]
    rec.run("PRV-11", "A planted change lists its files; the file too large to keep shows its real size and omitted=true", planted_files)

    def secret_masked_on_read():
        facts = seeded()
        flow = need("main", "PRV-02")
        design = ws.call("request.planBundleFile", {"id": flow.id, "path": "design.md"})
        assert facts["secret"] not in design["content"] and "[REDACTED]" in design["content"], f"design.md as read: {design['content'][:300]!r}"
        full = ws.call("request.planBundleFile", {"id": flow.id, "path": f"openspec/changes/{facts['changeId']}/design.md"})
        assert full["content"] == design["content"], "the full path and the relative path read different texts"
        big = ws.call("request.planBundleFile", {"id": flow.id, "path": facts["omittedPath"]})
        assert big["omitted"] is True and big["content"].startswith("<omitted size="), f"the omitted file read as {big!r}"
        tasks = ws.call("request.planBundleFile", {"id": flow.id, "path": "tasks.md"})
        assert "orca:begin plan" in tasks["content"] and not tasks["omitted"]
        everything = json.dumps([design, full, big, tasks, flow.proposal()])
        assert facts["secret"] not in everything, "the planted secret is in an answer"
        expect("REQUEST_PLAN_BUNDLE_PATH_INVALID", lambda: ws.call("request.planBundleFile", {"id": flow.id, "path": "../../etc/passwd"}))
    rec.run("PRV-12", "Reading a file masks the planted secret, reports the omitted file as omitted, and refuses a path outside the change", secret_masked_on_read)

    def old_versions():
        facts = seeded()
        flow = need("main", "PRV-02")
        if facts["versions"] < 3:
            raise Blocked("the Plan has fewer than three versions, so no version can be pruned")
        versions = {v["revision"]: v for v in flow.versions()}
        assert (versions[1]["textKept"], versions[2]["textKept"], versions[3]["textKept"]) == (False, True, True), \
            f"textKept per version = {[(n, versions[n]['textKept']) for n in sorted(versions)]!r}"
        expect("REQUEST_PLAN_REVISION_TEXT_PRUNED", lambda: ws.call("request.planBundleFile", {"id": flow.id, "path": "design.md", "revision": 1}))
        older = ws.call("request.planBundleFile", {"id": flow.id, "path": "design.md", "revision": 2})
        assert facts["secret"] not in older["content"] and "[REDACTED]" in older["content"], f"design.md of version 2: {older['content'][:300]!r}"
        v1 = flow.proposal(1)
        assert {f["path"] for f in (v1.get("bundle") or {}).get("files", [])} >= {"design.md", "tasks.md"}, "a version whose text was pruned must still list its files"
        second = {f["path"]: f.get("change") for f in (flow.proposal(2).get("bundle") or {}).get("files", [])}
        assert (second.get("tasks.md"), second.get("design.md"), second.get("specs/ledger/spec.md")) == ("changed", "changed", "unchanged"), f"changes of version 2 against 1: {second!r}"
        newest = {f["path"]: f.get("change") for f in (flow.proposal(3).get("bundle") or {}).get("files", [])}
        assert (newest.get("tasks.md"), newest.get("design.md"), newest.get("specs/audit-trail/spec.md")) == ("unchanged", "changed", "unchanged"), f"changes of version 3 against 2: {newest!r}"
    rec.run("PRV-13", "An old version keeps its text or has it pruned (its file list stays); a kept one is masked too; versions compare file by file", old_versions)

    # ── what this suite does not do ───────────────────────────────────────────────────────────────
    def fake_openspec_author():
        raise Skipped("a fake claude that authors OpenSpec was judged not feasible in half a day (an `openspec` shim, a knowledge hub for --ai-stub, a proposal worktree and "
                      "the result block); the Documents are proved with planted rows (PRV-11..13). Real OpenSpec is cost class C: ORCA_E2E_OPENSPEC=1 "
                      "ui-live/plan-documents.web.e2e.ts, 7 to 13 USD per generated Plan, only with the user's consent")
    rec.run("PRV-14", "OpenSpec Plan authored by a fake claude: documents, one revision, the edit-in-place block", fake_openspec_author)
    rec.run("PRV-15", "A secret in the Request text is masked in the stored design.md of an authored change", fake_openspec_author)
