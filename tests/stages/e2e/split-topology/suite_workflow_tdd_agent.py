"""TWA-*: workflow steps drive tdd-agent on a separate dev server (TDD-WF-AGENT-001 §11, TASKS appendix).

Needs the split topology with tdd-agent baked in:
  TDD_AGENT_SRC=/path/to/tdd-agentic-ai bash split-up.sh --build
The fixture project (/srv/projects/tdd-project) runs without a language model (no prose, rule-only test specs), so
every case here is deterministic. ORCA_E2E_REAL_AI=1 additionally runs the change-request cases whose gates need one.

Usage: python3 suite_workflow_tdd_agent.py [TWA-01 TWA-02 ...]
"""
from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path

from split_harness import (Blocked, create_template, docker, execute, execution_status, in_devserver, open_context, project_on,
                           settle, step_rows, wait_step, write_report)

DEV = "e2e-devserver-a"
PROJECT_DIR = "/srv/projects/tdd-project"
TEMPLATES = Path(__file__).resolve().parents[4] / "docs" / "integrate" / "templates"
REAL_AI = os.environ.get("ORCA_E2E_REAL_AI", "").lower() in ("1", "true", "yes")
PY = "/opt/tdd-agent/bin/python"


def template_steps(name: str) -> list[dict]:
    return json.loads((TEMPLATES / f"{name}.json").read_text(encoding="utf-8"))["steps"]


def tdd_runs() -> list[dict]:
    """tdd-agent runs of the fixture project, read on the dev server (newest first)."""
    code = ("import json,sys; from tdd_agent.runtime import Runtime; from tdd_agent.config import load_config; "
            "from tdd_agent import project; root=project.find_project_root(__import__('pathlib').Path('.')); "
            "rt=Runtime.build(load_config(root/'.tdd-agent'/'config.toml', base_dir=root)); "
            "print(json.dumps([dict(run_id=r.run_id, flow=r.flow, status=r.status, parent=r.parent_run_id, "
            "external=((r.request or {}).get('origin') or {}).get('external')) for r in rt.operational.list_runs(limit=200)]))")
    out = in_devserver(DEV, f"cd {PROJECT_DIR} && {PY} -c \"{code}\"")
    if out.returncode != 0:
        raise Blocked(f"cannot read tdd-agent runs on the dev server: {out.stderr.strip()[-300:]}")
    return json.loads(out.stdout.strip().splitlines()[-1])


def main(selected: set[str]) -> int:
    ctx = open_context(f"twa-{int(time.time())}")
    rec = ctx.rec
    rec.current_suite = "workflow-tdd-agent"
    state: dict = {}

    def run(test_id: str, title: str, fn) -> None:
        if not selected or test_id in selected:
            rec.run(test_id, title, fn)

    def prerequisites():
        if in_devserver(DEV, "command -v tdd-agent").returncode != 0:
            raise Blocked("tdd-agent is not installed on the dev server: split-up.sh with TDD_AGENT_SRC")
        if in_devserver(DEV, f"test -f {PROJECT_DIR}/.tdd-agent/config.toml").returncode != 0:
            raise Blocked(f"{PROJECT_DIR} is not a tdd-agent project")
        state["project"] = project_on(ctx, DEV, PROJECT_DIR, "tdd-project")
        for name in ("tdd-generation", "testspec", "change-request"):
            state[f"tpl:{name}"] = create_template(ctx, name, template_steps(name))
    rec.run("TWA-00", "Fixture project on the dev server, project bound in Orca, templates created", prerequisites)
    if "project" not in state:
        ctx.run_cleanups()
        return write_report("workflow-tdd-agent", ctx)
    pid = state["project"]

    def generation():
        eid = execute(ctx, state["tpl:tdd-generation"], pid)
        state["gen_exec"] = eid
        status = settle(ctx, eid, timeout=300)
        rows = step_rows(eid)
        assert status == "completed", f"execution {status}: {json.dumps(rows)[:500]}"
        start = rows["start"]["output"]
        assert start["mode"] == "generate" and start["runId"].startswith("gen-"), start
        listing = in_devserver(DEV, f"ls {PROJECT_DIR}/docs/tdd/")
        assert listing.returncode == 0 and listing.stdout.strip().endswith(".md"), f"no TDD file on the dev server: {listing.stdout}{listing.stderr}"
        state["tdd_id"] = listing.stdout.strip().split()[0].removesuffix(".md")
        on_orca = docker("exec", "orca-go-api-gateway", "ls", f"{PROJECT_DIR}")
        assert on_orca.returncode != 0, "the project directory exists on the Orca host"
    run("TWA-01", "TDD generation template writes the TDD on the dev server", generation)

    def testspecs():
        if "tdd_id" not in state:
            raise Blocked("TWA-01 produced no TDD")
        eid = execute(ctx, state["tpl:testspec"], pid, {"request": json.dumps({"params": {"tdd": state["tdd_id"]}})})
        status = settle(ctx, eid, timeout=300)
        rows = step_rows(eid)
        assert status == "completed", f"execution {status}: {json.dumps(rows)[:600]}"
        files = in_devserver(DEV, f"ls {PROJECT_DIR}/docs/testspec/{state['tdd_id']}/")
        names = files.stdout.split()
        assert {"01-test-requirements.md", "02-test-design.md", "03-test-cases.md"} <= set(names), f"test specs: {names} {files.stderr}"
        state["ts_exec"], state["ts_run"] = eid, rows["start"]["output"]["runId"]
    run("TWA-02", "Test specs template writes the three documents on the dev server", testspecs)

    def linked():
        if "gen_exec" not in state:
            raise Blocked("TWA-01 did not run")
        runs = [r for r in tdd_runs() if (r.get("external") or {}).get("executionId") == state["gen_exec"]]
        assert len(runs) == 1, f"tdd-agent runs of execution {state['gen_exec']}: {runs}"
        ext = runs[0]["external"]
        assert ext["system"] == "orca" and ext["stepId"] == "start" and ext["userId"], ext
    run("TWA-03", "The tdd-agent run records the Orca execution and step that started it (one run per step)", linked)

    def blocked():
        eid = execute(ctx, state["tpl:testspec"], pid, {"request": json.dumps({"params": {"tdd": "TDD-DOES-NOT-EXIST"}})})
        status = settle(ctx, eid, timeout=180)
        row = step_rows(eid)["testspecs"]
        out = row["output"] or {}
        assert status == "failed" and out.get("error") in ("TDD_AGENT_BLOCKED", "TDD_AGENT_RUN_FAILED"), f"{status} {row}"
        assert out.get("blockers"), f"no blockers in the step output: {out}"
    run("TWA-04", "A blocked gate fails the step with machine-readable blockers", blocked)

    def unexpected_gate():
        steps = template_steps("testspec")
        steps[1]["config"]["params"]["expectGate"] = "G9"
        steps[1]["config"]["params"].pop("stopAt", None)
        tid = create_template(ctx, "unexpected-gate", steps)
        eid = execute(ctx, tid, pid, {"request": json.dumps({"params": {"tdd": state.get("tdd_id", "TDD")}})})
        settle(ctx, eid, timeout=180)
        out = step_rows(eid)["testspecs"]["output"] or {}
        assert out.get("error") == "TDD_AGENT_UNEXPECTED_GATE", out
    run("TWA-05", "A run stopping elsewhere than the template expects fails with TDD_AGENT_UNEXPECTED_GATE", unexpected_gate)

    def outside_dir():
        steps = template_steps("tdd-generation")
        steps[0]["config"]["params"]["projectDir"] = "/etc"
        tid = create_template(ctx, "outside-dir", steps)
        eid = execute(ctx, tid, pid)
        assert settle(ctx, eid, timeout=60) == "failed"
        out = step_rows(eid)["start"]["output"] or {}
        assert out.get("error") == "TDD_AGENT_PROJECT_DIR_OUTSIDE_PROJECT", out
    run("TWA-06", "A projectDir outside the project's repo is refused", outside_dir)

    # ── approvals: the change-request flow needs a language model at some gates (G0/G2 skills) ──
    def change_request_to_g1():
        if not REAL_AI:
            raise Blocked("the change-request gates call a language model: set ORCA_E2E_REAL_AI=1 with AI_MODE=real")
        eid = execute(ctx, state["tpl:change-request"], pid, {"changeName": f"e2e-{int(time.time())}", "crPath": "docs/CR-1.md",
                                                              "srsAfterPath": "docs/SRS-v2.md"})
        state["cr_exec"] = eid
        row = wait_step(eid, "approve_g1", ("running", "failed"), timeout=900)
        assert row["status"] == "running", f"approve_g1 {row}"
        deadline = time.time() + 120
        pending = []
        while time.time() < deadline and not pending:
            pending = ctx.ws.call("workflow.stepApproval.list", {"executionId": eid})
            time.sleep(2)
        assert pending and pending[0]["subject"]["gate"] == "G1" and pending[0]["decisionHash"], pending
        state["g1"] = pending[0]
    run("TWA-07", "A change request stops at G1 and its decision waits in Orca", change_request_to_g1)

    def stale_hash():
        if "g1" not in state:
            raise Blocked("TWA-07 did not reach G1")
        try:
            ctx.ws.call("workflow.stepApproval.resolve", {"id": state["g1"]["id"], "decision": "approved", "decisionHash": "stale", "rationale": "x"})
        except Exception as exc:  # noqa: BLE001
            assert "STEP_APPROVAL_STALE" in str(exc), exc
            return
        raise AssertionError("an approval with a stale hash was accepted")
    run("TWA-09", "Approving with a hash that was not shown is refused", stale_hash)

    def approve_g1():
        if "g1" not in state:
            raise Blocked("TWA-07 did not reach G1")
        got = ctx.ws.call("workflow.stepApproval.get", {"id": state["g1"]["id"]})
        assert got["canDecide"] is True, got
        ctx.ws.call("workflow.stepApproval.resolve", {"id": got["id"], "decision": "approved", "decisionHash": got["decisionHash"],
                                                      "rationale": "SRS delta reviewed in the e2e"})
        row = wait_step(state["cr_exec"], "approve_g1", ("completed", "failed"), timeout=180)
        assert row["status"] == "completed" and row["output"]["approvedBy"], row
    run("TWA-08", "Approving in Orca delivers the decision to tdd-agent and the run goes on", approve_g1)

    def cancel_cancels_run():
        if "cr_exec" not in state:
            raise Blocked("needs the change-request execution of TWA-07")
        ctx.ws.call("workflow.cancel", {"executionId": state["cr_exec"]})
        time.sleep(30)
        runs = [r for r in tdd_runs() if (r.get("external") or {}).get("executionId") == state["cr_exec"]]
        assert runs and runs[0]["status"] == "failed", f"tdd-agent run after cancel: {runs}"
    run("TWA-16", "Cancelling the execution cancels the tdd-agent run", cancel_cancels_run)

    def cancel_stops_waiting():
        # an advance that can never reach its stop (a finished run, waiting for evidence) keeps polling until cancelled
        tid = create_template(ctx, "cancel-wait", [
            {"id": "start", "type": "action", "config": {"actionName": "tdd_agent.start", "params": {"mode": "run", "flow": "testspec",
                                                                                                   "request": json.dumps({"params": {"tdd": state.get("tdd_id", "TDD")}})}}},
            {"id": "wait", "type": "action", "dependsOn": ["start"], "config": {"actionName": "tdd_agent.advance", "params": {
                "runId": "{{outputs.start.runId}}", "stopAt": ["awaiting_evidence"], "pollIntervalSec": 2}}}])
        eid = execute(ctx, tid, pid)
        wait_step(eid, "wait", ("running",), timeout=60)
        time.sleep(15)
        ctx.ws.call("workflow.cancel", {"executionId": eid})
        row = wait_step(eid, "wait", ("failed", "completed"), timeout=60)
        assert (row["output"] or {}).get("error") == "TDD_AGENT_CANCELLED", row
        assert execution_status(ctx, eid) == "cancelled"
    run("TWA-16b", "A waiting step notices the cancelled execution and stops", cancel_stops_waiting)

    def restart_mid_advance():
        eid = execute(ctx, state["tpl:testspec"], pid, {"request": json.dumps({"params": {"tdd": state.get("tdd_id", "TDD")}})})
        wait_step(eid, "testspecs", ("running", "completed"), timeout=60)
        restarted = docker("restart", "orca-go-workflow", timeout=120)
        assert restarted.returncode == 0, restarted.stderr
        status = settle(ctx, eid, timeout=300)
        assert status == "completed", f"after a workflow-service restart: {status} {json.dumps(step_rows(eid))[:500]}"
        runs = [r for r in tdd_runs() if (r.get("external") or {}).get("executionId") == eid]
        assert len(runs) == 1, f"the restart started a second tdd-agent run: {runs}"
    run("TWA-13", "workflow-service restarting during an advance step: the execution completes with one tdd-agent run", restart_mid_advance)

    def outage():
        net = "orca-e2e-wan"
        eid = None
        try:
            tid = create_template(ctx, "outage", [
                {"id": "start", "type": "action", "config": {"actionName": "tdd_agent.start", "params": {"mode": "run", "flow": "testspec",
                                                                                                       "request": json.dumps({"params": {"tdd": state.get("tdd_id", "TDD")}})}}},
                {"id": "wait", "type": "action", "dependsOn": ["start"], "config": {"actionName": "tdd_agent.advance", "params": {
                    "runId": "{{outputs.start.runId}}", "stopAt": ["awaiting_evidence"], "pollIntervalSec": 2}}}])
            eid = execute(ctx, tid, pid)
            wait_step(eid, "wait", ("running",), timeout=60)
            docker("network", "disconnect", net, "orca-e2e-devserver-a")
            row = wait_step(eid, "wait", ("failed",), timeout=240)
            assert (row["output"] or {}).get("error") == "TDD_AGENT_UNREACHABLE", row
        finally:
            docker("network", "connect", "--alias", "devserver-a", net, "orca-e2e-devserver-a")
            if eid:
                try:
                    ctx.ws.call("workflow.cancel", {"executionId": eid})
                except Exception:  # noqa: BLE001 - already finished
                    pass
    run("TWA-15", "A dev server unreachable for 5 polls in a row fails the step with TDD_AGENT_UNREACHABLE", outage)

    problems = ctx.run_cleanups()
    ctx.session.close()
    return write_report("workflow-tdd-agent", ctx, {"cleanup_problems": problems, "real_ai": REAL_AI})


if __name__ == "__main__":
    sys.exit(main(set(sys.argv[1:])))
