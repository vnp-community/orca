"""Gates GT and TS1 with extra context (TC-64, TC-65, error codes of SOLUTION section 14.9)."""
import json

import pytest
from harness import make
from test_agents_change_flow import baseline, handler
from test_testspec_flow import _SERVED, gate, kg_over_mcp, to_g5  # noqa: F401  (autouse fixture: the KG is read over an in-memory MCP server)

from tdd_agent.agents.flows import default_flows
from tdd_agent.config import load_config
from tdd_agent.runtime import Runtime


@pytest.fixture()
def repo(tmp_path):
    root = tmp_path / "repo"
    (root / "openspec" / "specs").mkdir(parents=True)
    (root / "openspec" / "changes").mkdir()
    (root / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    return root


@pytest.fixture()
def launches(monkeypatch):
    calls = []
    monkeypatch.setattr(gate, "LAUNCH", lambda cfg, child: calls.append(child) or 4242)
    return calls


SOLUTION = {"status": "approved", "chosenOption": 0, "request": {"ref": "REQ-2"},
            "options": {"options": [{"id": "opt-1", "title": "A handler", "summary": "For FR-2.", "affected_areas": [{"name": "GET /x (FR-2)"}]}],
                        "assumptions": [{"id": "A-1", "text": "Other methods get 405 on FR-2.", "needs_confirmation": True}]},
            "acceptance_criteria": [{"id": "AC-X", "text": "GIVEN a request WHEN GET /x THEN 200"}]}


def build(tmp_path, repo, **testspec):
    o, know, ops, provider = make(default_flows(), handler, tmp_path,
                                  overrides={"paths": {"repo_root": str(repo), "work_dir": str(repo / ".tdd-agent")}, "testspec": {"llm": "none", **testspec}})
    _SERVED["rt"] = Runtime(o.config, know, ops, o.observer, o.gateway, o)
    return o, know, ops, provider


def _ov(o):
    return {k: o.config.data[k] for k in ("approvals", "paths")}


def test_context_mode_without_a_document_blocks_the_child_before_any_llm_call(tmp_path, repo):
    o, _k, _ops, _ = build(tmp_path, repo, context_mode="solution")
    baseline(o, None)
    o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id="ts-1")
    run = o.advance("ts-1")
    assert run.status == "blocked" and run.result["blockers"] == ["TESTSPEC_CONTEXT_MISSING"]
    assert not (repo / ".tdd-agent" / "tsagent.db").exists() and not (repo / "docs" / "testspec").exists()      # tsagent never ran


@pytest.mark.parametrize("doc,code", [("not json", "TESTSPEC_CONTEXT_INVALID"), ('{"options": []}', "TESTSPEC_CONTEXT_INVALID")])
def test_unreadable_document_blocks_the_child(tmp_path, repo, doc, code):
    o, _k, _ops, _ = build(tmp_path, repo, context_mode="solution", context_path="sol.json")
    baseline(o, None)
    (repo / "sol.json").write_text(doc, encoding="utf-8")
    o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id="ts-1")
    assert o.advance("ts-1").result["blockers"] == [code]


def test_a_run_can_point_at_another_document_and_the_mode_stays_the_configured_one(tmp_path, repo):
    o, _k, ops, _ = build(tmp_path, repo, context_mode="solution", context_path="missing.json")
    baseline(o, None)
    (repo / "other.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    o.start("testspec", {"params": {"tdd": "TDD-B", "context_path": "other.json", "context_ref": "R7", "context_mode": "plan"}}, run_id="ts-1")
    assert o.advance("ts-1").status == "completed"
    out = ops.get_task("ts-1", "TS1")["output"]
    assert out["context_mode"] == "solution" and out["features"][0]["context"]["ref"] == "R7"


def test_change_flow_reruns_gt_when_the_document_changes_and_not_otherwise(tmp_path, repo, launches):
    o, _k, ops, _ = build(tmp_path, repo, context_mode="solution", context_path="sol.json")
    (repo / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    to_g5(o, repo)
    first = ops.get_task("cr1", "GT")["output"]["child_run_id"]
    assert len(launches) == 1
    o.advance("cr1")
    assert len(launches) == 1, "same document: GT is not run again"
    changed = json.loads(json.dumps(SOLUTION))
    changed["options"]["assumptions"][0]["needs_confirmation"] = False
    (repo / "sol.json").write_text(json.dumps(changed), encoding="utf-8")
    o.advance("cr1")
    assert len(launches) == 2 and ops.get_task("cr1", "GT")["output"]["child_run_id"] != first
    # the gates before GT do not notice the document: no approval is lost
    assert ops.get_task("cr1", "G1")["status"] == "approved" and ops.get_task("cr1", "G2")["status"] == "approved"
    o.config = load_config(overrides={**_ov(o), "testspec": {"llm": "none", "context_mode": "solution", "context_path": "sol.json", "parallel": 1}}, env={})
    o.advance("cr1")
    assert len(launches) == 2, "[testspec] settings that do not change the result do not run it again"


def test_change_run_child_uses_the_document_and_reports_it(tmp_path, repo, monkeypatch):
    o, _k, ops, _ = build(tmp_path, repo, context_mode="solution", context_path="sol.json")
    (repo / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    monkeypatch.setattr(gate, "LAUNCH", lambda cfg, child: o.advance(child) and 0)
    to_g5(o, repo)
    child = ops.get_task("cr1", "GT")["output"]["child_run_id"]
    assert o.get_run(child).status == "completed"
    (f,) = ops.get_task(child, "TS1")["output"]["features"]
    assert f["context"]["mode"] == "solution" and f["context"]["path"] == "sol.json" and f["context"]["ref"] == "REQ-2"
    run_dir = repo / ".tdd-agent" / "runs" / child / "testspec"
    summary = json.loads((run_dir / "TDD-A.summary.json").read_text(encoding="utf-8"))
    assert summary["origin"]["context"] == {"mode": "solution", "path": "sol.json", "ref": "REQ-2", "sha256": f["context"]["sha256"]}
    from types import SimpleNamespace

    from tdd_agent.project import testspec_status
    info, line = testspec_status(SimpleNamespace(operational=ops), "cr1")
    assert info["status"] == "completed" and " · context: solution REQ-2 (" in line


def test_a_snapshot_the_document_was_not_written_on_is_a_note_not_a_stop(tmp_path, repo):
    o, _k, ops, _ = build(tmp_path, repo, context_mode="solution", context_path="sol.json")
    baseline(o, None)
    doc = json.loads(json.dumps(SOLUTION))
    doc["options"]["design_references"] = {"snapshot": "snap-999999"}
    (repo / "sol.json").write_text(json.dumps(doc), encoding="utf-8")
    o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id="ts-1")
    assert o.advance("ts-1").status == "completed"
    (f,) = ops.get_task("ts-1", "TS1")["output"]["features"]
    assert any("snap-999999" in n for n in f["context"]["notes"])


def test_mode_none_leaves_the_summary_without_context(tmp_path, repo):
    o, _k, ops, _ = build(tmp_path, repo)
    baseline(o, None)
    o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id="ts-1")
    assert o.advance("ts-1").status == "completed"
    out = ops.get_task("ts-1", "TS1")["output"]
    assert out["context_mode"] == "none" and out["features"][0]["context"] is None


def test_going_back_to_an_earlier_input_runs_tsagent_again_instead_of_keeping_the_later_draft(tmp_path, repo):
    """none -> solution -> none: the third run must write the first result again, not hand back a stale 'deduplicated' run."""
    o, _k, ops, _ = build(tmp_path, repo)
    baseline(o, None)
    (repo / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")

    def run(rid, mode):
        o.config = load_config(overrides={**_ov(o), "testspec": {"llm": "none", "context_mode": mode, "context_path": "sol.json"}}, env={})
        o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id=rid)
        assert o.advance(rid).status == "completed"
        return ops.get_task(rid, "TS1")["output"]["features"][0]

    first, with_doc, third = run("r1", "none"), run("r2", "solution"), run("r3", "none")
    assert with_doc["gaps"] > first["gaps"] and with_doc["deduplicated"] is False             # the Solution's unconfirmed assumption is a gap
    assert third["deduplicated"] is False and third["gaps"] == first["gaps"] and third["tsagent_run"] not in (first["tsagent_run"], with_doc["tsagent_run"])
    again = run("r4", "none")                                                                 # the same input twice in a row is still deduplicated
    assert again["deduplicated"] is True and again["tsagent_run"] == third["tsagent_run"]
