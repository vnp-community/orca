"""Remote-client tools (Orca workflows): background advance, cancel, run lookup by caller job, source paths, approval without advance."""
from __future__ import annotations

import os
import subprocess
import sys
import time

import pytest
from test_mcp_support import ACTORS, anyio_backend, connect, cr_flow  # noqa: F401

from tdd_agent.config import load_config
from tdd_agent.errors import NotFound, TddAgentError
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.mcp.tools import McpPolicy, McpTools
from tdd_agent.orchestrator import dispatch
from tdd_agent.runtime import Runtime

SOURCES = [{"id": "SRS-1", "revision": "1", "role": "SRS", "text": "srs v1"}, {"id": "SAD-1", "revision": "1", "role": "SAD", "text": "sad v1"}]
EXTERNAL = {"system": "orca", "executionId": "exec-1", "stepId": "start", "userId": "u-1"}


def runtime(tmp_path, **mcp):
    repo = tmp_path / "repo"
    repo.mkdir(exist_ok=True)
    config = load_config(overrides={"store": {"knowledge": "memory:", "operational": "memory:"},
                                    "paths": {"work_dir": str(tmp_path / "work"), "repo_root": str(repo)},
                                    "approvals": {"actors": ACTORS}, "mcp": dict({"write": True}, **mcp)}, env={})
    return Runtime.build(config, flows={"cr": cr_flow()}, providers={"claude_code": FakeProvider({"*": {}})}, configure_logging=False)


def tools_for(rt) -> McpTools:
    return McpTools(rt, McpPolicy.from_runtime(rt))


@pytest.fixture
def launches(monkeypatch):
    """Record launches instead of starting processes; the recorded pid is this test process (alive)."""
    calls: list[str] = []

    def fake(base_dir, work_dir, run_id):
        calls.append(run_id)
        dispatch._write(work_dir, run_id, {"run_id": run_id, "pid": os.getpid(), "dispatched_at": "t", "heartbeat_at": None, "finished_at": None})
        return os.getpid()
    monkeypatch.setattr(dispatch, "LAUNCH", fake)
    return calls


def test_dispatch_starts_once_and_reports_the_process(tmp_path, launches):
    rt = runtime(tmp_path)
    tools = tools_for(rt)
    run_id = tools.run_start("cr", {"mode": "change-request"}, SOURCES)["run_id"]
    first = tools.run_dispatch(run_id)
    assert first["dispatched"] is True and first["pid"] == os.getpid()
    again = tools.run_dispatch(run_id)
    assert again == {"run_id": run_id, "dispatched": False, "reason": "RUN_ALREADY_RUNNING", "status": "running", "pid": os.getpid()}
    assert launches == [run_id]
    process = tools.run_get(run_id).result["process"]
    assert process["alive"] is True and process["pid"] == os.getpid() and process["heartbeat_interval_s"] == dispatch.HEARTBEAT_INTERVAL_S


def test_dispatch_again_once_the_process_is_gone_and_never_for_a_finished_run(tmp_path, launches):
    rt = runtime(tmp_path)
    tools = tools_for(rt)
    run_id = tools.run_start("cr", {}, SOURCES)["run_id"]
    tools.run_dispatch(run_id)
    dispatch._update(rt.config.work_dir, run_id, finished_at="t2")  # the advance process ended (e.g. waiting for approval now)
    assert tools.run_dispatch(run_id)["dispatched"] is True
    rt.operational.update_run(run_id, status="completed")
    assert tools.run_dispatch(run_id) == {"run_id": run_id, "dispatched": False, "reason": "RUN_TERMINAL", "status": "completed"}
    assert launches == [run_id, run_id]


def test_dispatch_needs_write(tmp_path):
    rt = runtime(tmp_path, write=False)
    with pytest.raises(TddAgentError) as e:
        tools_for(rt).run_dispatch("run-x")
    assert e.value.code == "WRITE_DISABLED"


def test_heartbeat_records_beats_and_the_end(tmp_path):
    work = tmp_path / "w"
    with dispatch.Heartbeat(work, "run-h", interval=0.05):
        time.sleep(0.2)
        live = dispatch.read_proc(work, "run-h")
        assert live["pid"] == os.getpid() and live["heartbeat_at"] and live["alive"] is True
    done = dispatch.read_proc(work, "run-h")
    assert done["finished_at"] and done["exit"] == "ok" and done["alive"] is False


def test_launch_advance_runs_the_cli_with_heartbeat(tmp_path, monkeypatch):
    seen = {}

    class P:
        pid = 4321
    monkeypatch.setattr(dispatch.subprocess, "Popen", lambda cmd, **kw: seen.update(cmd=cmd, **kw) or P())
    assert dispatch.launch_advance(tmp_path / "proj", tmp_path / "w", "run-1") == 4321
    assert seen["cmd"][1:] == ["-m", "tdd_agent.cli", "-C", str(tmp_path / "proj"), "run", "advance", "run-1", "--heartbeat"]
    assert seen["start_new_session"] is True
    assert dispatch.read_proc(tmp_path / "w", "run-1")["pid"] == 4321


def test_terminate_signals_a_live_process_group(tmp_path):
    work = tmp_path / "w"
    proc = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"], start_new_session=True)
    try:
        dispatch._write(work, "run-t", {"run_id": "run-t", "pid": proc.pid, "finished_at": None})
        assert dispatch.terminate(work, "run-t") == proc.pid
        assert proc.wait(timeout=5) != 0
        assert dispatch.read_proc(work, "run-t")["exit"] == "terminated"
        assert dispatch.terminate(work, "run-t") is None
    finally:
        if proc.poll() is None:
            proc.kill()


def test_cancel_fails_the_run_and_its_children(tmp_path, launches):
    rt = runtime(tmp_path)
    tools = tools_for(rt)
    run_id = tools.run_start("cr", {}, SOURCES)["run_id"]
    child = rt.orchestrator.start("cr", {}, parent_run_id=run_id)
    out = tools.run_cancel(run_id, "user cancelled the Orca execution")
    assert set(out["cancelled"]) == {run_id, child.run_id} and out["status"] == "failed"
    for rid in (run_id, child.run_id):
        run = rt.operational.get_run(rid)
        assert run.status == "failed" and run.result["blockers"] == ["CANCELLED_BY_CLIENT"]
    assert tools.run_cancel(run_id, "again")["cancelled"] == []
    with pytest.raises(TddAgentError):
        tools.run_cancel(run_id, " ")


def test_run_found_again_by_external_job_and_children_listed(tmp_path):
    rt = runtime(tmp_path)
    tools = tools_for(rt)
    run_id = tools.run_start("cr", {}, SOURCES, external=EXTERNAL)["run_id"]
    tools.run_start("cr", {}, SOURCES, external=dict(EXTERNAL, stepId="other"))
    child = rt.orchestrator.start("cr", {}, parent_run_id=run_id)
    found = tools.run_list(external_execution_id="exec-1", external_step_id="start").result
    assert [r["run_id"] for r in found["runs"]] == [run_id]
    assert found["runs"][0]["request"]["origin"]["external"] == EXTERNAL
    assert [r["run_id"] for r in tools.run_list(parent_run_id=run_id).result["runs"]] == [child.run_id]
    assert [c["run_id"] for c in tools.run_get(run_id).result["children"]] == [child.run_id]
    with pytest.raises(TddAgentError):
        tools.run_start("cr", {}, SOURCES, external={"executionId": 1})


def test_sources_by_path_stay_inside_the_project(tmp_path):
    rt = runtime(tmp_path)
    tools = tools_for(rt)
    (tmp_path / "repo" / "docs").mkdir()
    (tmp_path / "repo" / "docs" / "cr.md").write_text("CR body", encoding="utf-8")
    (tmp_path / "secret.txt").write_text("no", encoding="utf-8")
    run_id = tools.run_start("cr", {}, [{"id": "CR-1", "revision": "1", "role": "CR", "path": "docs/cr.md"}])["run_id"]
    pin = rt.operational.get_run(run_id).request["pins"][0]
    assert pin["id"] == "CR-1" and rt.knowledge.get_source("CR-1").text == "CR body"
    with pytest.raises(TddAgentError) as e:
        tools.run_start("cr", {}, [{"id": "X", "revision": "1", "role": "CR", "path": "../secret.txt"}])
    assert e.value.code == "SOURCE_PATH_OUTSIDE_PROJECT"
    with pytest.raises(NotFound):
        tools.run_start("cr", {}, [{"id": "X", "revision": "1", "role": "CR", "path": "docs/missing.md"}])


@pytest.mark.anyio
async def test_approval_without_advance_leaves_the_run_for_dispatch(tmp_path, launches):
    rt = runtime(tmp_path, approvals=True, client_actor="claude")
    async with connect(rt) as mcp:
        run_id = (await mcp.ok("run_start", flow="cr", request={}, sources=SOURCES, external=EXTERNAL))["run_id"]
        await mcp.ok("run_resume", run_id=run_id)
        h1 = (await mcp.ok("run_get", run_id=run_id))["tasks"]["G1"]["decision_hash"]
        ok = await mcp.ok("approval_submit", run_id=run_id, gate="G1", role="BA", decision_hash=h1, rationale="orca:user=u-1 ok", advance=False)
        assert ok["run_status"] == "running"
        assert rt.operational.get_task(run_id, "G2") is None, "nothing ran after the approval"
        assert (await mcp.ok("run_dispatch", run_id=run_id))["dispatched"] is True
        assert launches == [run_id]
