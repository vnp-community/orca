"""RAN: read-only analyses (diagnosis, findings, answer). The agent runs `claude --print` on the project's repo; request-service
validates the document, stores it as a Solution of the right kind and opens the approval gate (CR-REQ-008).

Answering the readiness Clarification starts the run by itself, so with a real Claude these tests spend AI usage and minutes:
they run with --ai-stub, or with a real Claude when ORCA_E2E_READONLY_ANALYSIS=1."""
from __future__ import annotations

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked, Skipped

# type -> (Solution kind, approval subject, what the stored document must carry)
CASES = {
    "bug": ("diagnosis", "solution"),
    "spike": ("findings", "findings"),
    "question": ("answer", "answer"),
}


def _document_checks(kind: str, doc: dict) -> None:
    assert doc.get("schema_version") == 1 and doc.get("kind") == kind, f"document says schema_version={doc.get('schema_version')!r} kind={doc.get('kind')!r}"
    if kind == "diagnosis":
        assert (doc.get("root_cause") or {}).get("statement"), "diagnosis has no root_cause.statement"
        assert (doc.get("impact") or {}).get("severity") in ("low", "medium", "high", "critical"), f"impact.severity={doc.get('impact')!r}"
    elif kind == "findings":
        assert doc.get("findings"), "findings document lists no finding"
    else:
        assert doc.get("answer_markdown"), "answer document has no answer_markdown"


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("RAN-00", "A project exists for the read-only analysis tests", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return
    timeout = 90 if ctx.state.get("ai_stub") else 600

    for index, (typ, (kind, subject)) in enumerate(CASES.items(), start=1):
        def one(typ=typ, kind=kind, subject=subject):
            if not drv.readonly_analysis_enabled(ctx):
                raise Skipped("starts a real read-only Claude agent on the dev server's repo (AI usage); set ORCA_E2E_READONLY_ANALYSIS=1, or use --ai-stub")
            if ctx.state.get("ai_available") is False:
                raise Blocked("no AI is available on the dev server")
            req = drv.create(ctx, f"analysis {typ}")["request"]
            cur = drv.to_first_stage(ctx, req, typ, "M")
            assert cur["status"] == "analyzing", f"after the readiness answer the Request is {cur['status']!r}, expected analyzing"

            # The run starts by itself once the Clarification is answered: wait for the Request to reach the analysis gate.
            def at_gate():
                r = drv.get(ctx, req["id"])
                return r if r["status"] != "analyzing" else None
            gate = wait_until(at_gate, timeout=timeout, interval=1.0, what=f"the {kind} run of a {typ} Request to finish")
            listed = ws.call("solution.list", {"requestId": req["id"]})
            runs = [r for r in listed.get("runs", []) if r.get("kind") == kind]
            assert runs, f"no {kind} run was recorded (runs: {[(r.get('kind'), r.get('status')) for r in listed.get('runs', [])]})"
            assert runs[-1]["status"] == "succeeded", f"the {kind} run ended {runs[-1]['status']!r}: {runs[-1].get('errorCode')} {str(runs[-1].get('errorMessage', ''))[:160]}"
            assert gate["status"] == "awaiting_analysis_approval", f"after the analysis the Request is {gate['status']!r}, expected awaiting_analysis_approval"

            sols = [s for s in listed.get("solutions", []) if s.get("kind") == kind]
            assert sols, f"no Solution of kind {kind!r} (kinds: {[s.get('kind') for s in listed.get('solutions', [])]})"
            assert sols[-1]["status"] in ("draft", "proposed"), f"a fresh {kind} is {sols[-1]['status']!r}"
            _document_checks(kind, sols[-1].get("options") or {})

            approvals = ws.call("approval.list", {"requestId": req["id"], "status": "pending"}).get("approvals", [])
            assert any(a["subjectType"] == subject for a in approvals), f"no pending '{subject}' approval: {[a['subjectType'] for a in approvals]}"
            ctx.state["readonly_analysis_ran"] = True
            ctx.state.setdefault("readonly_analysis_kinds", set()).add(kind)
        rec.run(f"RAN-{index:02d}", f"A '{typ}' Request is analysed by the read-only agent: a {kind} document, the Request at awaiting_analysis_approval, a pending '{subject}' approval", one)
