"""MTX: each of the 11 request types takes its own path after type confirmation (CR-REQ-003)."""
from __future__ import annotations

import request_flow_driver as drv
from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked, Skipped
from expected_request_flows import EXPECTED, PHASES


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("MTX-00", "A project exists for the type matrix", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return

    for index, (typ, want) in enumerate(EXPECTED.items(), start=1):
        def one_type(typ=typ, want=want):
            size = "L" if typ in PHASES else "M"
            # Answering the readiness Clarification starts the analysis; for a read-only analysis that is a real agent run.
            analysis_gated = typ in drv.READONLY_ANALYSIS_TYPES and not drv.readonly_analysis_enabled(ctx)
            req = drv.create(ctx, f"matrix {typ}")["request"]
            cur = drv.to_first_stage(ctx, req, typ, size, want.get("urgency", "normal"), answer_readiness=not analysis_gated)
            assert cur["type"] == typ, f"stored type is {cur['type']!r}"
            if not analysis_gated:
                assert cur["status"] == want["first"], f"after confirming {typ} the Request is in {cur['status']!r}, expected {want['first']!r}"

            flow = ws.call("request.flow", {"type": typ, "size": size})
            assert flow["statusPath"] == want["path"], f"status path {flow['statusPath']}, expected {want['path']}"
            assert flow["humanConfirmRequired"] is want["human"], f"humanConfirmRequired={flow['humanConfirmRequired']}"
            assert flow["analysisKind"] == want["analysis"], f"analysisKind={flow['analysisKind']!r}"
            assert flow["analysisGate"] == want["gate"], f"analysisGate={flow['analysisGate']!r}"
            assert flow["planKind"] == want["plan"], f"planKind={flow['planKind']!r}"
            assert flow["startGate"] == want["start"], f"startGate={flow['startGate']!r}"
            assert flow["completesAfterAnalysis"] is want["completes"], f"completesAfterAnalysis={flow['completesAfterAnalysis']}"
            if typ in PHASES:
                m_phases, l_phases = PHASES[typ]
                at_m = ws.call("request.flow", {"type": typ, "size": "M"})["hasPhases"]
                assert (at_m, flow["hasPhases"]) == (m_phases, l_phases), f"hasPhases M/L = {at_m}/{flow['hasPhases']}, expected {m_phases}/{l_phases}"
            if analysis_gated:
                raise Skipped(f"flow table for '{typ}' checked, but the first-stage check needs the readiness Clarification answered, which starts a real read-only "
                              "analysis by the agent (AI usage); set ORCA_E2E_READONLY_ANALYSIS=1, or use --ai-stub")
        rec.run(f"MTX-{index:02d}", f"Type '{typ}': first stage '{want['first']}' and the documented flow", one_type)

    def declared_kind_matches_validation():
        req = drv.create(ctx, "matrix spike kinds")["request"]
        cur = drv.confirm(ctx, drv.settle_after_classify(ctx, req["id"]) and drv.get(ctx, req["id"]), "spike", "M")
        clar = drv.open_clarification(ctx, cur["id"])
        assert clar, "spike confirmation opened no Clarification"
        bad = []
        for q in clar["questions"]:
            if q["kind"] != "text":
                continue
            value = "8" if "hours" in q["questionKey"] else "text answer"
            reply_ok = True
            try:
                ctx.ws.call("clarification.answer", {"clarificationId": clar["id"], "answers": [{"questionId": q["id"], "valueJson": '"%s"' % value, "acceptDefault": False}],
                                                     "complete": False, "expectedVersion": clar["version"]})
            except Exception as exc:  # noqa: BLE001
                reply_ok = False
                detail = str(exc)[:120]
            if not reply_ok:
                bad.append(f"{q['questionKey']} ({detail})")
            clar = drv.open_clarification(ctx, cur["id"]) or clar
        assert not bad, "questions declared kind=text reject a string answer: " + "; ".join(bad)
    rec.run("MTX-13", "Question kinds match what the service accepts (text questions accept text)", declared_kind_matches_validation)

    def type_set_matches_registry():
        shown = {t for t in EXPECTED}
        for typ in shown:
            ws.call("request.flow", {"type": typ, "size": "M"})
        try:
            ws.call("request.flow", {"type": "twelfth_type", "size": "M"})
        except Exception:
            return
        raise AssertionError("request.flow answers for a type outside the 11 documented ones")
    rec.run("MTX-12", "Exactly the 11 documented types exist", type_set_matches_registry)
