"""generate_start / change_start: an Orca workflow starts the project flows over MCP and advances them in the background.

Same fixtures as test_project_flows (real stores, skills and gates; scripted model)."""
from __future__ import annotations

import json

import pytest
from test_agents_change_flow import SAD, SRS_V1, URD
from test_project_flows import APPROVERS, CR_TEXT, answers, generated, runtime

from tdd_agent import project
from tdd_agent.errors import TddAgentError
from tdd_agent.mcp.tools import McpPolicy, McpTools

EXTERNAL = {"system": "orca", "executionId": "exec-1", "stepId": "start", "userId": "u-1"}


@pytest.fixture()
def proj(tmp_path):
    root = tmp_path / "P"
    (root / "docs").mkdir(parents=True)
    for name, text in (("URD", URD), ("SRS", SRS_V1), ("SAD", SAD)):
        (root / "docs" / f"{name}.md").write_text(text, encoding="utf-8")
    project.init_project(root, name="P", sources={r: str(root / "docs" / f"{r.upper()}.md") for r in ("urd", "srs", "sad")}, approvers=APPROVERS)
    return root


def tools(rt) -> McpTools:
    return McpTools(rt, McpPolicy(write=True, approvals_requested=False, actor="claude", actor_allowlisted=True, transport="stdio", max_chars=100000))


def test_generate_start_creates_the_run_and_the_background_advance_writes_the_tdd(proj):
    rt = runtime(proj, answers())
    t = tools(rt)
    out = t.generate_start(external=EXTERNAL)
    assert out["status"] == "started"
    run_id = out["run_id"]
    run = rt.operational.get_run(run_id)
    assert run.status == "running" and run.request["origin"]["external"] == EXTERNAL
    assert rt.operational.get_task(run_id, "G3") is None, "generate_start must not run the gate"
    document = proj / run.request["publish"]["document"]
    assert not document.exists(), "nothing is published before the run completes"

    rt.orchestrator.advance(run_id)                    # what `run advance --heartbeat` does in the background
    project.after_background_advance(rt, run_id)
    assert rt.operational.get_run(run_id).status == "completed" and document.is_file()
    assert json.loads(project._state_path(proj).read_text(encoding="utf-8"))["run_id"] == run_id
    assert t.generate_start()["status"] == "up_to_date"


def test_change_start_creates_the_run_without_running_a_gate(proj):
    rt = generated(proj)
    (proj / "docs" / "CR-1.md").write_text(CR_TEXT, encoding="utf-8")
    t = tools(rt)
    out = t.change_start("keep-evidence", cr_path="docs/CR-1.md", external=dict(EXTERNAL, executionId="exec-2"),
                         code_targets=["FR-1=src/evidence.py::retain"],
                         verifications=["FR-1=GIVEN stored evidence WHEN 5 years have passed THEN it may be purged"])
    run_id = out["run_id"]
    assert run_id == "change-keep-evidence" and rt.operational.get_run(run_id).status == "running"
    assert rt.operational.get_task(run_id, "G0") is None
    found = t.run_list(external_execution_id="exec-2").result["runs"]
    assert [r["run_id"] for r in found] == [run_id]
    rt.orchestrator.advance(run_id)
    assert rt.operational.get_run(run_id).status == "awaiting_approval"


def test_change_start_refuses_paths_outside_the_project(proj):
    rt = generated(proj)
    with pytest.raises(TddAgentError) as e:
        tools(rt).change_start("x-change", cr_path="../outside.md")
    assert e.value.code == "SOURCE_PATH_OUTSIDE_PROJECT"


def test_generate_after_the_split_still_writes_the_tdd_in_one_call(proj):
    rt = runtime(proj, answers())
    result, lines = project.generate(rt, {}, tdd_id=None, out=None, prose=False, prose_limit=None, force=False)
    assert result["status"] == "completed" and (proj / result["document"]).is_file()
    assert lines[0].startswith("TDD written:")
