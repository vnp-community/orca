"""AST: only with `--ai-stub`. Proves the agent really used the fake claude the way the production path must (tools off,
empty scratch directory), and that a logged-out claude surfaces as REQUEST_AI_NOT_CONFIGURED instead of a vague failure.

Two entry points because the runner restarts the agent between two phases: `run_after_logged_in_suites` closes the first
phase (the stub answered everything), `run_logged_out` opens the second (the stub says `loggedIn: false`).
"""
from __future__ import annotations

import os
from pathlib import Path

import request_flow_driver as drv
from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked

EXPECTED_PRINT_PREFIX = ["-p", "--output-format", "json", "--tools", ""]


def _stub(ctx: AcceptanceContext):
    stub = ctx.state.get("ai_stub")
    if stub is None:
        raise Blocked("this suite only runs with --ai-stub")
    return stub


def run(ctx: AcceptanceContext) -> None:
    if ctx.state.get("ai_stub_phase") == "logged_out":
        run_logged_out(ctx)
    else:
        run_after_logged_in_suites(ctx)


def run_after_logged_in_suites(ctx: AcceptanceContext) -> None:
    rec = ctx.rec

    def agent_found_the_stub():
        calls = _stub(ctx).calls()
        kinds = {c["kind"] for c in calls}
        # `claude --help` is ai.complete's own capability check (PATH lookup). version/auth belong to the capability probe, which
        # infra-fleet throttles per dev server (5 min) and stores for 24h, so a restart may legitimately skip it; AST-04 proves that route.
        assert "help" in kinds, f"the agent never ran `claude --help` from the stub (saw {sorted(kinds)}); the PATH lookup is broken"
        assert "print" in kinds, "no `claude -p` call reached the stub: the AI answers did not come from the CLI source"
        assert "unsupported" not in kinds, "the agent called the stub in a way the real CLI is not used by the product"
        auth = [c for c in calls if c["kind"] == "auth"]
        assert all(c["argv"] == ["auth", "status", "--json"] for c in auth), "the agent asked `claude auth` something other than `status --json`"
    rec.run("AST-01", "The agent found the stub on its PATH (claude --help, then claude -p); any capability probe used `auth status --json`", agent_found_the_stub)

    def ai_complete_ran_with_tools_off():
        prints = [c for c in _stub(ctx).calls() if c["kind"] == "print"]
        assert prints, "no ai.complete call reached the stub; the AI answers did not come from the CLI source"
        scratch_parent = Path(os.path.realpath(ctx.state.get("repo_path") or "/nonexistent"))
        for c in prints:
            assert c["argv"][:5] == EXPECTED_PRINT_PREFIX, f"unexpected claude arguments {c['argv'][:6]!r}; `--tools \"\"` must be passed so the prompt cannot use tools"
            assert "--system-prompt" in c["argv"], "no --system-prompt was passed"
            assert "--bare" not in c["argv"], "--bare ignores the OAuth login this path depends on"
            assert c["cwdEntries"] == [], f"claude started in a non-empty directory {c['cwd']!r}: {c['cwdEntries']!r}"
            assert Path(c["cwd"]).name.startswith("orca-ai-"), f"claude ran in {c['cwd']!r}, not in the agent's scratch directory"
            assert scratch_parent not in Path(os.path.realpath(c["cwd"])).parents and Path(os.path.realpath(c["cwd"])) != scratch_parent, "claude ran inside the project repository"
            assert c["stdinBytes"] > 0, "the prompt was not delivered on stdin"
    rec.run("AST-02", "Every ai.complete call ran `claude -p --output-format json --tools \"\"` in an empty scratch directory with the prompt on stdin", ai_complete_ran_with_tools_off)

    def every_prompt_was_recognised():
        unknown = [c for c in _stub(ctx).calls() if c["kind"] == "print" and c["promptKind"] == "unknown"]
        assert not unknown, f"{len(unknown)} prompt(s) matched no canned answer: the request-service prompts changed, update fake_claude_cli/claude"
        seen = {c["promptKind"] for c in _stub(ctx).calls() if c["kind"] == "print"}
        assert "classification" in seen, f"no classification prompt reached the stub (saw {sorted(seen)})"
    rec.run("AST-03", "The stub recognised every prompt the product sent (no unknown prompt)", every_prompt_was_recognised)

    def hotfix_proposal_does_not_strand_the_request():
        if not ctx.state.get("project_id"):
            raise Blocked("no project (see DEV-04)")
        req = drv.create(ctx, "stub hotfix proposal", body="Production is down for all users.")["request"]
        try:
            cur = drv.settle_after_classify(ctx, req["id"], timeout=25)
        except AssertionError:
            raise AssertionError("an AI proposal of 'hotfix' left the Request in 'classifying'; classification must always hand over to a human "
                                 "(typical cause: REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER when the reporter is the only user)") from None
        assert cur["status"] == "awaiting_type_confirmation", f"classification ended in {cur['status']!r}"
    rec.run("AST-05", "An AI proposal of a human-approved type (hotfix) still ends in awaiting_type_confirmation", hotfix_proposal_does_not_strand_the_request)


def run_logged_out(ctx: AcceptanceContext) -> None:
    rec = ctx.rec
    if not ctx.state.get("project_id"):
        rec.run("AST-04", "Logged-out claude surfaces as REQUEST_AI_NOT_CONFIGURED", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return

    def logged_out_means_not_configured():
        stub = _stub(ctx)
        ctx.state["ai_available"] = False  # this phase has no AI by construction: AIS-01/03 must exercise their failure paths
        before = len([c for c in stub.calls() if c["kind"] == "print"])
        req = drv.create(ctx, "stub logged out")["request"]
        cur = drv.settle_after_classify(ctx, req["id"])
        assert cur["status"] == "awaiting_type_confirmation", f"classification ended in {cur['status']!r}; it must always hand over to a human"
        assert not cur.get("type"), f"a logged-out claude still proposed type {cur.get('type')!r}"
        assert cur.get("classificationReason") == "REQUEST_AI_NOT_CONFIGURED", (
            f"classificationReason={cur.get('classificationReason')!r}; a logged-out claude must read as REQUEST_AI_NOT_CONFIGURED")
        after = len([c for c in stub.calls() if c["kind"] == "print"])
        assert after == before, "the product spawned claude although the capability report said it is not logged in"
    rec.run("AST-04", "A logged-out claude is REQUEST_AI_NOT_CONFIGURED: classification hands over to a human and claude is not even spawned", logged_out_means_not_configured)
