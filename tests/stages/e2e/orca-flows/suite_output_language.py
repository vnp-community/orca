"""OLG: the language the AI writes in (CR-REQ-046). The deployment default is Vietnamese (`REQUEST_DEFAULT_OUTPUT_LANGUAGE`, compose default `vi`),
and request-service adds `Output language: Vietnamese.` to the Solution, Plan and read-only analysis prompts, whatever language the Request is in.

  OLG-01  every Solution, Plan and read-only analysis prompt sent while this suite ran carries the Vietnamese directive (the fake claude records `outputLanguage`)
  OLG-02  an English change_request gets a stored Solution in Vietnamese; ids, enums and sizes are untouched
  OLG-03  a bug's Plan (native) is Vietnamese, keeps its `test:regression` label, passes the real validator and opens its approval
  OLG-04  a bug Diagnosis and a question Answer are Vietnamese documents of the right kind and schema
  OLG-05  a Request written in Vietnamese and one written in English get the same directive (no language detection)
  OLG-06  only with ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en (the stack was started with REQUEST_DEFAULT_OUTPUT_LANGUAGE=en): English directive, English output

Needs --ai-stub: the fake claude writes Vietnamese when asked and records the directive it saw. The suite cannot change the service's environment, so the
English stack case is manual (see OLG-06).
Run: python3 run_acceptance.py --ai-stub devserver output_language
"""
from __future__ import annotations

import os
import shutil

import approve_run_all_driver as ara
import base_ref_stack as stack
import base_ref_worktrees as gitwt
import ledgerlite_fixture as ledgerlite
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped
from analysis_revision_driver import Flow, new_flow
from suite_request_plan_stage import propose_and_wait

AI_PROMPTS = ("solution", "plan")  # plus every "readonly:*"


def has_vietnamese(text: str) -> bool:
    """A letter outside ASCII. The fake writes English in ASCII only, and every Vietnamese sentence it writes has accented letters."""
    return any(c.isalpha() and ord(c) > 127 for c in text)


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    expect_en = os.environ.get("ORCA_E2E_EXPECT_OUTPUT_LANGUAGE", "").strip().lower() == "en"
    want_code, want_name = ("en", "English") if expect_en else ("vi", "Vietnamese")
    held: dict = {"first_call": None}

    def stub():
        s = ctx.state.get("ai_stub")
        if s is None:
            raise Skipped("the fake claude records the directive it saw and writes Vietnamese; use --ai-stub")
        return s

    def project() -> str:
        stub()
        if "project" not in held:
            if not shutil.which("git"):
                raise Blocked("git not installed on the test runner")
            try:
                repo = ledgerlite.create(cfg.run_prefix + "-olg")
            except ledgerlite.FixtureError as exc:
                raise Blocked(f"cannot build the LedgerLite fixture: {exc}") from exc
            ctx.on_cleanup("remove the fixture repo and its worktrees", lambda: gitwt.remove_worktrees_and_repo(repo))
            held["project"] = stack.register_project(ctx, repo, "olg-project")
            held["first_call"] = len(stub().calls())
        return held["project"]

    def is_ai_prompt(call: dict) -> bool:
        kind = call.get("promptKind", "")
        return call.get("kind") == "print" and (kind in AI_PROMPTS or kind.startswith("readonly:"))

    def calls_with(marker: str) -> list[dict]:
        return [c for c in stub().calls() if is_ai_prompt(c) and marker in (c.get("prompt") or "")]

    def fresh(key: str, typ: str, title: str, size: str = "M", body: str = "") -> Flow:
        """A Request of `typ` carried to its analysis gate. The title holds a marker unique to the case so its prompts can be told apart."""
        if key not in held:
            pid = project()
            marker = f"{cfg.run_prefix} olg {key}"
            req = drv.create(ctx, f"olg {key}", body=body or "Add a function subtract(a, b) that returns a minus b, with one unit test.", project_id=pid)["request"]
            cur = drv.to_first_stage(ctx, req, typ, size)
            assert cur["status"] == "analyzing", f"after the readiness answer the Request is {cur['status']!r}"
            flow = Flow(ctx, req["id"], typ)
            flow.first_attempt()
            held[key] = (flow, marker)
        return held[key][0]

    # ── the stored Solution, Diagnosis and Answer ─────────────────────────────────────────────────────
    def solution_in_vietnamese():
        flow = fresh("cr", "change_request", "english change request")
        doc = flow.versions()[-1]["options"]
        opt = doc["options"][0]
        assert (opt["id"], doc["recommendation"]["option_id"], doc["assumptions"][0]["id"]) == ("opt-1", "opt-1", "A-1"), f"ids changed: {opt['id']!r}"
        assert opt["effort"]["size"] in ("S", "M", "L") and opt["risks"][0]["severity"] in ("low", "medium", "high"), "an enum was translated"
        if expect_en:
            return
        text = " ".join([opt["title"], opt["summary"], opt["approach"], opt["rollback"], *opt["pros"], *opt["cons"], doc["recommendation"]["reason"]])
        assert has_vietnamese(text), f"the stored Solution has no Vietnamese letter: {text[:200]!r}"
        assert has_vietnamese(opt["summary"]), f"summary is not Vietnamese: {opt['summary']!r}"
    rec.run("OLG-02", "An English change_request gets a Vietnamese Solution: prose translated, ids and enums untouched", solution_in_vietnamese)

    def bug_plan_in_vietnamese():
        flow = fresh("bug", "bug", "english bug", size="S")
        approval = flow.pending()[0]
        ws.call("approval.approve", {"id": approval["id"], "comment": "e2e", "expectedVersion": approval["version"], "expectedDigest": approval["subjectDigest"]})
        view = propose_and_wait(ctx, flow.id)  # the real validator already accepted it (a failed run would end the wait with an error)
        plan = view["proposal"]
        tasks = ara_tasks(plan)
        assert tasks, "the Plan has no task"
        assert "test:regression" in {label for t in tasks for label in t.get("labels") or []}, f"the bug Plan lost its test:regression label: {[t.get('labels') for t in tasks]}"
        if not expect_en:
            assert has_vietnamese(plan["title"]) and all(has_vietnamese(t["title"]) for t in tasks), f"the Plan is not Vietnamese: {plan['title']!r}, {[t['title'] for t in tasks]}"
        done = ws.call("request.generatePlan", {"id": flow.id, "mode": "commit", "proposal": plan, "rawAiResponse": view.get("rawAiResponse", "")}, timeout=90)
        assert done.get("planTaskId"), f"the Vietnamese Plan could not be committed: {done!r}"
        assert any(a["subjectType"] in ara.PLAN_SUBJECTS for a in ws.call("approval.list", {"requestId": flow.id, "status": "pending"}).get("approvals", [])), "no Plan approval opened"
    rec.run("OLG-03", "A bug's native Plan is Vietnamese, keeps test:regression, passes the real validator and opens its approval", bug_plan_in_vietnamese)

    def diagnosis_and_answer():
        diagnosis = fresh("bug", "bug", "english bug", size="S").versions()[-1]["options"]
        answer = fresh("question", "question", "english question", size="S", body="How does the daily transfer limit work?").versions()[-1]["options"]
        assert diagnosis.get("kind") == "diagnosis" and answer.get("kind") == "answer", f"kinds {diagnosis.get('kind')!r}, {answer.get('kind')!r}"
        assert diagnosis["root_cause"]["statement"] and diagnosis["fix_directions"][0]["id"] == "fix-1", "the Diagnosis lost its structure or ids"
        assert isinstance(answer["confidence"], float) and answer["answer_markdown"], "the Answer lost its structure"
        if not expect_en:
            assert has_vietnamese(diagnosis["summary"] + diagnosis["root_cause"]["statement"]), f"the Diagnosis is not Vietnamese: {diagnosis['summary'][:120]!r}"
            assert has_vietnamese(answer["answer_markdown"]), f"the Answer is not Vietnamese: {answer['answer_markdown'][:120]!r}"
    rec.run("OLG-04", "A bug Diagnosis and a question Answer are Vietnamese documents of the right kind, ids and keys in English", diagnosis_and_answer)

    def same_directive_for_both_languages():
        vi_flow = fresh("question-vi", "question", "vietnamese question", size="S", body="Hạn mức chuyển tiền mỗi ngày được tính như thế nào?")
        en_flow = fresh("question-en", "question", "english question two", size="S", body="How is the daily transfer limit counted?")
        for flow, key in ((vi_flow, "question-vi"), (en_flow, "question-en")):
            marker = held[key][1]
            seen = calls_with(marker)
            assert seen, f"[{key}] no analysis prompt of this Request reached the stub"
            assert {c["outputLanguage"] for c in seen} == {want_code}, f"[{key}] directive languages {sorted({c['outputLanguage'] for c in seen})!r}, expected {want_code!r}"
            if not expect_en:
                assert has_vietnamese(flow.versions()[-1]["options"]["answer_markdown"]), f"[{key}] the Answer is not Vietnamese"
    rec.run("OLG-05", "A Request written in Vietnamese and one in English both get the same directive; there is no language detection", same_directive_for_both_languages)

    def every_prompt_has_the_directive():
        first = held.get("first_call")
        if first is None:
            raise Blocked("no OLG case ran, so no prompt was sent")
        sent = [c for c in stub().calls()[first:] if is_ai_prompt(c)]
        assert sent, "no Solution, Plan or analysis prompt reached the stub"
        wrong = [c for c in sent if c.get("outputLanguage") != want_code]
        assert not wrong, f"{len(wrong)} of {len(sent)} prompts carried outputLanguage {sorted({c.get('outputLanguage') for c in wrong})!r}, expected {want_code!r} ({want_name})"
        kinds = {c["promptKind"] for c in sent}
        assert {"solution", "plan"} <= kinds and any(k.startswith("readonly:") for k in kinds), f"the suite did not cover Solution, Plan and read-only prompts: {sorted(kinds)}"
    rec.run("OLG-01", f"Every Solution, Plan and read-only analysis prompt carries the {want_name} directive", every_prompt_has_the_directive)

    def english_stack():
        if not expect_en:
            raise Skipped("the stack is started with REQUEST_DEFAULT_OUTPUT_LANGUAGE=en for this case; set ORCA_E2E_EXPECT_OUTPUT_LANGUAGE=en when it is (the other cases then expect English)")
        project()
        rows = [c for c in stub().calls()[held["first_call"]:] if is_ai_prompt(c)]
        assert rows and all(c["outputLanguage"] == "en" for c in rows), "an English stack still sent a non-English directive"
    rec.run("OLG-06", "On a stack started with REQUEST_DEFAULT_OUTPUT_LANGUAGE=en the directive is English and the fake writes English (manual stack)", english_stack)


def ara_tasks(proposal: dict) -> list[dict]:
    return list(proposal.get("tasks") or []) + [t for ph in proposal.get("phases") or [] for t in ph.get("tasks") or []]
