"""T4.1-T4.4: gate GT in change-request and the testspec workflow (TC-01..07, 09..11, 15)."""
import pytest
from harness import make
from test_agents_change_flow import approve, baseline, handler, start_change

from tdd_agent.agents.flows import default_flows
from tdd_agent.agents.testspec import gate
from tdd_agent.config import load_config


@pytest.fixture()
def repo(tmp_path):
    root = tmp_path / "repo"
    (root / "openspec" / "specs").mkdir(parents=True); (root / "openspec" / "changes").mkdir()
    (root / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    return root


@pytest.fixture()
def launches(monkeypatch):
    calls = []
    monkeypatch.setattr(gate, "LAUNCH", lambda cfg, child: calls.append(child) or 4242)
    return calls


_SERVED = {}   # runtime served by the test's MCP server (set in build)


@pytest.fixture(autouse=True)
def kg_over_mcp(monkeypatch):
    """TS1 reads the KG over MCP: point the client at a real tdd-agent MCP server built on the orchestrator's stores (in-memory transport)."""
    from mcp import Client

    from tdd_agent.agents.testspec import kgclient
    from tdd_agent.mcp import build_server
    monkeypatch.setattr(kgclient, "CLIENT_FACTORY", lambda cfg: Client(build_server(_SERVED["rt"])))
    yield
    _SERVED.clear()


def build(tmp_path, repo, **testspec):
    from tdd_agent.runtime import Runtime
    o, know, ops, provider = make(default_flows(), handler, tmp_path, overrides={"paths": {"repo_root": str(repo)}, "testspec": {"llm": "none", **testspec}})
    _SERVED["rt"] = Runtime(o.config, know, ops, o.observer, o.gateway, o)
    return o, know, ops, provider


def to_g5(o, repo):
    baseline(o, None)
    start_change(o, repo)
    o.advance("cr1")
    assert approve(o, "cr1", "G1", "alice", "BA").accepted
    assert approve(o, "cr1", "G2", "claude", "SA/Dev").accepted
    return o.get_run("cr1")


def child_of(ops, run_id="cr1"):
    return ops.get_task(run_id, "GT")["output"]["child_run_id"]


def test_gt_dispatches_child_and_g5_runs_on(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo)
    run = to_g5(o, repo)
    gt = ops.get_task("cr1", "GT")
    assert gt["status"] == "completed" and gt["output"]["launched"] == "detached" and gt["output"]["pid"] == 4242
    assert gt["output"]["preflight"] == {"ok": True, "llm": "none"}
    child = o.get_run(child_of(ops))
    assert launches == [child.run_id] and child.flow == "testspec" and child.parent_run_id == "cr1" and child.status == "running"
    p = child.request["params"]
    # GT reads no KG: it only hands over the snapshot id, the change-spec item ids and the G0 CR criteria
    assert p == {"parent_run_id": "cr1", "cr_id": "CR-1", "snapshot": ops.get_task("cr1", "G4")["output"]["snapshot"],
                 "cs_elements": ["CR-1/CS-001", "CR-1/CS-002"], "acceptance": [{"id": "AC-3", "text": None}]}
    assert ops.get_task("cr1", "G5") is not None, "G5 ran in the same advance, without waiting for the child run"
    assert run.status in ("blocked", "awaiting_approval") and run.result.get("blocked_gate", run.result.get("pending_gate")) == "G5"


def test_child_run_writes_test_specs(tmp_path, repo, monkeypatch):
    o, _know, ops, _ = build(tmp_path, repo)
    monkeypatch.setattr(gate, "LAUNCH", lambda cfg, child: o.advance(child) and 0)
    to_g5(o, repo)
    child = o.get_run(child_of(ops))
    assert child.status == "completed", child.result
    out = ops.get_task(child.run_id, "TS1")["output"]
    (f,) = out["features"]
    assert f["feature"] == "TDD-A" and f["status"] == "done" and f["tr"] >= 1 and f["tc"] >= 1 and out["llm"] == "none"
    assert out["kg"]["source"] == "mcp:stdio" and out["kg"]["mcp_calls"] >= 3 and out["kg"]["elements"] >= 4   # KG read over MCP
    assert sorted(f["files"]) == [f"docs/testspec/TDD-A/{n}" for n in ("01-test-requirements.md", "02-test-design.md", "03-test-cases.md")]
    for name in f["files"]:
        text = (repo / name).read_text(encoding="utf-8")
        assert "llm: none" in text and "status: Draft" in text
    assert f["uncovered_acceptance"] == [], "the CR criterion AC-3 has a test condition"
    run_dir = tmp_path / "work" / "runs" / child.run_id / "testspec"
    assert (run_dir / "TDD-A.tdd-kg.json").is_file() and (run_dir / "TDD-A.summary.json").is_file()
    assert (run_dir / "TDD-A" / "03-test-cases.md").is_file()
    # provenance in tsagent.db
    from tsagent.store.db import Database
    db = Database(str(tmp_path / "work" / "tsagent.db"))
    row = db.one("SELECT json_extract(params, '$.origin.snapshot') AS s, json_extract(params, '$.origin.parent_run_id') AS p FROM run")
    db.close()
    assert row["s"] == child.request["params"]["snapshot"] and row["p"] == "cr1"


def test_unset_llm_blocks_child_without_process(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo, llm="")
    to_g5(o, repo)
    gt = ops.get_task("cr1", "GT")["output"]
    assert gt["launched"] == "inline" and gt["preflight"]["code"] == "LLM_NOT_CONFIGURED" and launches == []
    child = o.get_run(child_of(ops))
    assert child.status == "blocked" and child.result["blockers"] == ["LLM_NOT_CONFIGURED"]
    assert "[testspec] llm" in ops.get_task(child.run_id, "TS1")["evidence"]["error"]


def test_api_provider_without_key(tmp_path, repo, launches, monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    o, _know, ops, _ = build(tmp_path, repo, llm="anthropic")
    to_g5(o, repo)
    child = o.get_run(child_of(ops))
    assert child.status == "blocked" and child.result["blockers"] == ["LLM_KEY_MISSING"] and launches == []


def test_manual_run_for_one_tdd(tmp_path, repo):
    o, _know, ops, _ = build(tmp_path, repo)
    baseline(o, None)
    o.start("testspec", {"params": {"tdd": "TDD-B"}}, run_id="ts-man")
    run = o.advance("ts-man")
    assert run.status == "completed", run.result
    out = ops.get_task("ts-man", "TS1")["output"]
    assert out["mode"] == "manual" and out["features"][0]["feature"] == "TDD-B" and out["features"][0]["status"] == "done"
    o.start("testspec", {"params": {}}, run_id="ts-bad")
    assert o.advance("ts-bad").result["blockers"] == ["TESTSPEC_PARAM:tdd"]


def test_disabled_branch_starts_nothing(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo, enabled=False)
    to_g5(o, repo)
    assert ops.get_task("cr1", "GT")["output"] == {"enabled": False}
    assert not [r for r in ops.list_runs(flow="testspec")] and launches == []


def test_changed_change_spec_starts_new_child_and_old_one_steps_aside(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo)
    to_g5(o, repo)
    first = child_of(ops)
    params = dict(o.get_run("cr1").request["params"])
    o.update_sources("cr1", [], keep_revisions=True, params={"code_targets": {**params["code_targets"], "FR-3": "src/notify.py::notify_all"}})
    o.advance("cr1")
    second = child_of(ops)
    assert second != first and launches == [first, second]
    assert ops.get_task("cr1", "GT")["output"]["supersedes"] == first
    assert o.advance(first).status == "completed"
    assert ops.get_task(first, "TS1")["output"] == {"superseded_by": second}


def test_testspec_settings_do_not_invalidate_change_gates(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo)
    to_g5(o, repo)
    before = {g: ops.get_task("cr1", g)["status"] for g in ("G0", "G1", "G2", "G3", "G4", "GT", "G5")}
    o.config = load_config(overrides={**_ov(o), "testspec": {"llm": "none", "group_retries": 5, "parallel": 1}}, env={})
    o.advance("cr1")
    assert {g: ops.get_task("cr1", g)["status"] for g in before} == before and len(launches) == 1
    o.config = load_config(overrides={**_ov(o), "testspec": {"llm": "claude-cli"}}, env={})
    o.advance("cr1")
    assert len(launches) == 2, "changing the LLM mode re-runs GT only"
    assert ops.get_task("cr1", "G1")["status"] == "approved" and ops.get_task("cr1", "G2")["status"] == "approved"


def _ov(o):
    return {k: o.config.data[k] for k in ("approvals", "paths")}


def test_child_resumes_after_interruption(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo)
    to_g5(o, repo)
    child = child_of(ops)
    ops.put_task(child, "TS1", {"status": "running", "input_hash": "x", "attempt": 1})   # the process died midway
    assert o.advance(child).status == "completed"
    assert ops.get_task(child, "TS1")["attempt"] == 2


# ---------------------------------------------------------------- T5.1 (TC-16)
def _status(ops):
    from types import SimpleNamespace

    from tdd_agent.project import testspec_status
    return testspec_status(SimpleNamespace(operational=ops), "cr1")


def test_change_status_line_states(tmp_path, repo, launches, monkeypatch):
    o, _know, ops, _ = build(tmp_path, repo)
    to_g5(o, repo)
    child = child_of(ops)
    from tdd_agent import project
    monkeypatch.setattr(project, "_pid_alive", lambda pid: True)
    info, line = _status(ops)
    assert info["status"] == "running" and line.startswith(f"Test specs: being generated ({child}")
    monkeypatch.setattr(project, "_pid_alive", lambda pid: False)
    info, line = _status(ops)
    assert info["status"] == "interrupted" and f"tdd-agent run advance {child}" in line
    o.advance(child)
    info, line = _status(ops)
    assert info["status"] == "completed" and "TR" in line and "docs/testspec/TDD-A/" in line
    assert info["features"][0]["feature"] == "TDD-A"


def test_change_status_line_config_error_and_off(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo, llm="")
    to_g5(o, repo)
    info, line = _status(ops)
    assert info["status"] == "blocked" and "LLM_NOT_CONFIGURED" in line and "[testspec] llm" in line


def test_change_status_line_off(tmp_path, repo, launches):
    o, _know, ops, _ = build(tmp_path, repo, enabled=False)
    to_g5(o, repo)
    assert _status(ops) == ({"enabled": False}, "Test specs: off ([testspec] enabled = false).")
