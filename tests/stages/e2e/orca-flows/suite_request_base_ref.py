"""BRF: the base branch of a Request's worktrees (CR-REQ-039), on the LedgerLite fixture repo with real git.

  BRF-01..05  request.create / request.setBaseRef / request.get / request.flowStatus (default, stored, invalid, unknown, version)
  BRF-10      repo.listBranches (CONTRACT 2.12) lists the fixture's branches for the "Start from" picker: no HEAD, no duplicates
  BRF-06..07  THE PROOF: a `task` Request is planned and approved; task-service creates the task's worktree, and `git rev-parse HEAD` in
              it equals the tip of the chosen branch, or of the default branch (never the third branch the primary checkout is left on)
  BRF-08      once a worktree exists the ref is locked (REQUEST_BASE_REF_LOCKED, reason task or execution)
  BRF-09      OpenSpec proposal worktree starts from the chosen ref (opt-in: needs the real Claude, ORCA_E2E_OPENSPEC=1)

Approving a Plan starts an agent on the dev server. With --ai-stub that agent is the fake claude, which refuses to write, so the
task fails right after its worktree exists: exactly what these cases need, at no cost. With a real Claude it needs ORCA_E2E_EXECUTE_AGENT=1.
Run: python3 run_acceptance.py --ai-stub devserver request_base_ref
"""
from __future__ import annotations

import os
import shutil

import base_ref_stack as stack
import base_ref_worktrees as gitwt
import ledgerlite_fixture as ledgerlite
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from base_ref_worktrees import assert_started_from

THIRD_BRANCH = "feature/other"
RELEASE = ledgerlite.RELEASE_BRANCH
DEFAULT = ledgerlite.DEFAULT_BRANCH
WORKTREE_WAIT = 150  # classification, Plan, approval, dispatch, then git-gateway adds the worktree


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    held: dict = {"executed": {}}

    def fixture():
        if "fx" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-brf", release_branch=True, third_branch=THIRD_BRANCH)
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo and its worktrees", lambda: gitwt.remove_worktrees_and_repo(repo))
            held["fx"] = (repo, stack.register_project(ctx, repo, "brf-project"))
        return held["fx"]

    def new_request(title: str, base_ref: str | None = None) -> dict:
        return stack.create_request(ctx, fixture()[1], title, base_ref)["request"]

    def settled_request(title: str) -> dict:
        """A new Request starts classifying and its version moves when that ends; the version-sensitive cases wait for it first."""
        req = new_request(title)
        drv.settle_after_classify(ctx, req["id"])
        return drv.get(ctx, req["id"])

    # ── request.create / request.get / request.flowStatus ──────────────────────────────────────────────────
    def default_ref():
        req = new_request("base ref default")
        assert req["baseRef"] == "", f"a Request created without baseRef reports {req['baseRef']!r}, expected the empty string (repository default)"
        got = drv.get(ctx, req["id"])
        assert got["baseRef"] == "" and got.get("baseRefLocked") is False, f"request.get: baseRef={got['baseRef']!r} baseRefLocked={got.get('baseRefLocked')!r}"
        assert not got.get("baseRefLockReason"), f"an unlocked Request has a lock reason: {got.get('baseRefLockReason')!r}"
        features = ws.call("request.flowStatus", {}).get("features") or []
        assert "request.baseRef" in features, f"request.flowStatus features {features!r} lack request.baseRef, so the UI would hide the field"
    rec.run("BRF-01", "Without baseRef the Request keeps the repository default, is unlocked, and the gateway announces request.baseRef", default_ref)

    def stored_ref():
        reply = stack.create_request(ctx, fixture()[1], "base ref release", RELEASE)
        assert reply["request"]["baseRef"] == RELEASE, f"create answered baseRef {reply['request']['baseRef']!r}"
        assert not reply["request"].get("baseRefUnverified"), "an existing branch on a connected dev server was left unverified"
        assert drv.get(ctx, reply["request"]["id"])["baseRef"] == RELEASE, "request.get does not return the stored ref"
    rec.run("BRF-02", f"request.create with baseRef '{RELEASE}' stores it, verified against the repository", stored_ref)

    def invalid_refs():
        for bad in ("-x", "a..b"):
            stack.expect_error("REQUEST_BASE_REF_INVALID", lambda b=bad: stack.create_request(ctx, fixture()[1], "base ref invalid", b))
        req = settled_request("base ref invalid set")
        stack.expect_error("REQUEST_BASE_REF_INVALID", lambda: ws.call("request.setBaseRef", {"id": req["id"], "baseRef": "-x", "expectedVersion": req["version"]}))
        assert drv.get(ctx, req["id"])["version"] == req["version"], "a refused setBaseRef bumped the version"
    rec.run("BRF-03", "'-x' and 'a..b' are REQUEST_BASE_REF_INVALID on create and setBaseRef, and change nothing", invalid_refs)

    def unknown_ref():
        repo, _ = fixture()
        exc = stack.expect_error("REQUEST_BASE_REF_NOT_FOUND", lambda: stack.create_request(ctx, fixture()[1], "base ref unknown", "release/9.x"))
        suggestions = stack.error_trailer(exc).get("suggestions") or []
        assert suggestions and set(suggestions) <= set(repo.branches), f"suggested refs {suggestions!r} are not branches of the repo {repo.branches!r}"
        req = settled_request("base ref unknown set")
        stack.expect_error("REQUEST_BASE_REF_NOT_FOUND", lambda: ws.call("request.setBaseRef", {"id": req["id"], "baseRef": "release/9.x", "expectedVersion": req["version"]}))
        assert drv.get(ctx, req["id"])["baseRef"] == "", "a refused setBaseRef changed the stored ref"
    rec.run("BRF-04", "A ref git does not know is REQUEST_BASE_REF_NOT_FOUND with suggested branches in the message trailer", unknown_ref)

    def set_ref_and_version():
        req = settled_request("base ref set")
        done = ws.call("request.setBaseRef", {"id": req["id"], "baseRef": RELEASE, "expectedVersion": req["version"]})["request"]
        assert done["baseRef"] == RELEASE and done["version"] > req["version"], f"setBaseRef -> baseRef={done['baseRef']!r} version {req['version']} -> {done['version']}"
        stack.expect_error("REQUEST_VERSION_CONFLICT", lambda: ws.call("request.setBaseRef", {"id": req["id"], "baseRef": DEFAULT, "expectedVersion": req["version"]}))
        assert drv.get(ctx, req["id"])["baseRef"] == RELEASE, "a version conflict changed the ref"
        reset = ws.call("request.setBaseRef", {"id": req["id"], "baseRef": "", "expectedVersion": done["version"]})["request"]
        assert reset["baseRef"] == "", f"setBaseRef with '' left {reset['baseRef']!r}; it must go back to the repository default"
    rec.run("BRF-05", "setBaseRef on an unlocked Request changes the ref and bumps the version; a stale version conflicts; '' resets", set_ref_and_version)

    def list_branches():
        repo, pid = fixture()
        reply = ws.call("repo.listBranches", {"repoId": stack.repo_id_of(ctx, pid)})
        branches = reply.get("branches") or []
        names = [b["name"] for b in branches]
        for want in (DEFAULT, THIRD_BRANCH, RELEASE):
            assert want in names, f"repo.listBranches {names!r} lacks {want!r} (fixture branches {repo.branches!r})"
        assert not [n for n in names if n == "HEAD" or n.endswith("/HEAD")], f"a HEAD entry is listed: {names!r}"
        assert len(names) == len(set(names)), f"duplicate names in {names!r}"
        remote_flagged = [b["name"] for b in branches if b["name"] in (DEFAULT, THIRD_BRANCH, RELEASE) and b.get("remote")]
        assert not remote_flagged, f"local branches flagged remote: {remote_flagged!r}"
        assert not reply.get("truncated"), "a three-branch repo reported truncated"
    rec.run("BRF-10", "repo.listBranches lists main, feature/other and release/1.x as local branches, without HEAD or duplicates", list_branches)

    # ── the proof: where the task's worktree really starts ────────────────────────────────────────────────
    def executed(key: str, base_ref: str | None) -> dict:
        """A `task` Request driven to the first worktree: classify, confirm, Plan, approve, then wait for git to show the worktree."""
        if key in held["executed"]:
            return held["executed"][key]
        if not (ctx.state.get("ai_stub") or cfg.allow_agent_execution):
            raise Skipped("opt-in: approving a Plan runs an AI agent on the dev server; set ORCA_E2E_EXECUTE_AGENT=1, or use --ai-stub (fake claude, no cost)")
        repo, pid = fixture()
        req = new_request(f"base ref worktree {key}", base_ref)
        cur = drv.settle_after_classify(ctx, req["id"])
        if not stack.ai_proposed_a_type(cur):
            raise Blocked(f"the AI classifier produced no proposal (reason={cur.get('classificationReason')!r}); the Plan step needs an AI on the dev server")
        cur = drv.confirm(ctx, cur, "task", "S")
        if cur["status"] == "awaiting_information":
            drv.answer(ctx, drv.open_clarification(ctx, cur["id"]))
            cur = wait_until(lambda: (lambda r: r if r["status"] != "awaiting_information" else None)(drv.get(ctx, cur["id"])), timeout=15, what="the Request to leave awaiting_information")
        assert cur["status"] == "planning", f"a task list goes straight to planning, got {cur['status']!r}"
        assert cur["baseRef"] == (base_ref or ""), f"classification or confirmation changed the base ref to {cur['baseRef']!r}"
        stack.plan_and_approve(ctx, req["id"])
        ids = stack.leaf_task_ids(ctx, pid, req["id"])
        assert ids, "the committed Plan has no working task"
        wt = gitwt.wait_for_worktree(repo, {f"task/{i}" for i in ids}, WORKTREE_WAIT, "the task's worktree to appear in the repo (git worktree list)")
        held["executed"][key] = {"request": req["id"], "worktree": wt}
        return held["executed"][key]

    def tips() -> dict[str, str]:
        repo, _ = fixture()
        return {RELEASE: repo.rev_parse(RELEASE), DEFAULT: repo.rev_parse(DEFAULT), THIRD_BRANCH: repo.rev_parse(THIRD_BRANCH)}

    def check_start(key: str, base_ref: str | None, want: str, label: str) -> None:
        repo, _ = fixture()
        t = tips()
        assert repo.current_branch() == THIRD_BRANCH and repo.rev_parse("HEAD") == t[THIRD_BRANCH], "precondition: the primary checkout must be on the third branch"
        wt = executed(key, base_ref)["worktree"]
        assert gitwt.branch_of(wt.path) == wt.branch and wt.branch.startswith("task/"), f"the worktree is on {gitwt.branch_of(wt.path)!r}, expected its own task/<id> branch"
        assert_started_from(wt, label, t[want], {f"the third branch {THIRD_BRANCH}": t[THIRD_BRANCH], f"{RELEASE}": t[RELEASE]} if want == DEFAULT
                            else {f"the third branch {THIRD_BRANCH}": t[THIRD_BRANCH]})
        assert repo.current_branch() == THIRD_BRANCH, "creating the task worktree moved the primary checkout off the third branch"
    rec.run("BRF-06", f"A task worktree starts from '{RELEASE}' when the Request chose it, not from the checkout's branch (git rev-parse)",
            lambda: check_start("release", RELEASE, RELEASE, RELEASE))
    rec.run("BRF-07", f"Without a baseRef the task worktree starts from the default branch '{DEFAULT}', not from the third branch the checkout is on",
            lambda: check_start("default", None, DEFAULT, f"the default branch {DEFAULT}"))

    def locked_after_worktree():
        if not held["executed"]:
            raise Blocked("BRF-06/07 produced no worktree")
        for key, run_ in held["executed"].items():
            got = drv.get(ctx, run_["request"])
            exc = stack.expect_error("REQUEST_BASE_REF_LOCKED", lambda g=got: ws.call("request.setBaseRef", {"id": g["id"], "baseRef": DEFAULT, "expectedVersion": g["version"]}))
            reason = stack.error_trailer(exc).get("reason")
            # Why: the fake claude now finishes the task (CR-REQ-043), so by the time of this check the Request may be done, which locks it as `terminal`.
            assert reason in ("task", "execution", "terminal"), f"[{key}] lock reason {reason!r}, expected task, execution or terminal"
            assert got.get("baseRefLocked") is True and got.get("baseRefLockReason") in ("task", "execution", "terminal"), (
                f"[{key}] request.get says baseRefLocked={got.get('baseRefLocked')!r} reason={got.get('baseRefLockReason')!r} though a worktree exists")
            assert drv.get(ctx, run_["request"])["baseRef"] == got["baseRef"], f"[{key}] a refused setBaseRef changed the ref"
    rec.run("BRF-08", "After the first task worktree exists, setBaseRef is REQUEST_BASE_REF_LOCKED (reason task or execution)", locked_after_worktree)

    # ── OpenSpec proposal worktree (real Claude, costs money) ─────────────────────────────────────────────
    def openspec_proposal_worktree():
        if os.environ.get("ORCA_E2E_OPENSPEC", "") not in ("1", "true", "yes"):
            raise Skipped("opt-in: set ORCA_E2E_OPENSPEC=1 (real Claude authors an OpenSpec change on the dev server; costs several USD). Not run by default")
        if ctx.state.get("ai_stub"):
            raise Skipped("the fake claude cannot author an OpenSpec change; run without --ai-stub and with a logged-in Claude on the dev server")
        if not ledgerlite.tdd_agent_available():
            raise Blocked("tdd-agent is not on PATH: the OpenSpec engine needs the fixture's knowledge hub")
        repo = ledgerlite.create(cfg.run_prefix + "-brfos", knowledge=True, release_branch=True, third_branch=THIRD_BRANCH)
        ctx.on_cleanup("remove the OpenSpec fixture repo and its worktrees", lambda: gitwt.remove_worktrees_and_repo(repo))
        pid = stack.register_project(ctx, repo, "brf-openspec", engine="openspec")
        req = stack.create_request(ctx, pid, "base ref openspec", RELEASE)["request"]
        cur = drv.to_first_stage(ctx, req, "change_request", "M")
        assert cur["status"] == "analyzing", f"expected analyzing, got {cur['status']!r}"
        stack.approve_first_solution(ctx, req["id"])
        ws.call("request.generatePlan", {"id": req["id"]}, timeout=60)  # starts the OpenSpec run, which prepares the proposal worktree
        wt = gitwt.wait_for_worktree(repo, {f"request/{cur['number']}-proposal"}, 600, "the proposal worktree to appear")
        assert_started_from(wt, RELEASE, repo.rev_parse(RELEASE), {f"the third branch {THIRD_BRANCH}": repo.rev_parse(THIRD_BRANCH)})
        now = drv.get(ctx, req["id"])
        exc = stack.expect_error("REQUEST_BASE_REF_LOCKED", lambda: ws.call("request.setBaseRef", {"id": req["id"], "baseRef": DEFAULT, "expectedVersion": now["version"]}))
        assert stack.error_trailer(exc).get("reason") == "proposal", f"lock reason trailer: {exc.message[:200]}"
    rec.run("BRF-09", f"The OpenSpec proposal worktree starts from '{RELEASE}' and locks the ref (opt-in, real Claude)", openspec_proposal_worktree)
