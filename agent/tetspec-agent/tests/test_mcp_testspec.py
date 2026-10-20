"""MCP tools of the test-spec run with extra context: testspec_start, testspec_get (SOLUTION-INTEGRATE-TDD-AGENT sections 14.11, 14.13)."""
import json

import pytest
from harness import make
from test_agents_change_flow import baseline, handler
from test_testspec_flow import _SERVED, kg_over_mcp  # noqa: F401  (autouse fixture: the KG is read over an in-memory MCP server)

from tdd_agent.agents.flows import default_flows
from tdd_agent.errors import NotFound, TddAgentError
from tdd_agent.mcp.tools import McpPolicy, McpTools
from tdd_agent.runtime import Runtime


@pytest.fixture()
def repo(tmp_path):
    root = tmp_path / "repo"
    (root / "openspec" / "specs").mkdir(parents=True)
    (root / "openspec" / "changes").mkdir()
    (root / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    return root


EXTERNAL = {"system": "orca", "executionId": "exec-1", "stepId": "testspec", "userId": "u-1"}

SOLUTION = {
    "status": "draft", "chosenOption": 0,
    "options": {
        "options": [{"id": "opt-1", "title": "Free function", "summary": "No store receiver for FR-2.", "recommended": True,
                     "approach": "Add a handler.", "affected_areas": [{"kind": "api", "name": "GET /other (FR-2)"}],
                     "risks": [{"severity": "low", "description": "A later edit adds a store dependency to FR-2."}]}],
        "recommendation": {"option_id": "opt-1", "reason": "Smallest."},
        "assumptions": [{"id": "A-1", "text": "Other methods get 405 on FR-2.", "needs_confirmation": True}],
        "open_questions": [],
    },
    "acceptance_criteria": [{"id": "AC-9", "text": "GIVEN the service WHEN GET /other THEN 200"}],
    "request": {"ref": "REQ-9"},
}

TASKS = """<!-- orca:begin plan request=REQ-9 schema=1 -->
## PH-1 Other

- [ ] T1.1 Add the handler [type=feature] [h=1] [ac=AC-9]
  Serves: FR-2
  Code: modify `main.go::handleOther`
  Verify: AC-9 via `go test ./...`
  Done when: GET /other answers 200 without reading the store.
<!-- orca:end plan -->
"""
PROPOSAL = "## Why\n\nFR-2 asks for it.\n\n## What Changes\n\n- Add `handleOther`.\n"
DESIGN = "## Decisions\n\n- The handler has no store receiver (FR-2).\n\n## Risks / Trade-offs\n\n- 405 is assumed for other methods.\n"
PLAN_FILES = {"tasks.md": TASKS, "proposal.md": PROPOSAL, "design.md": DESIGN}


def build(tmp_path, repo, *, write=True, **testspec):
    o, _know, ops, _ = make(default_flows(), handler, tmp_path,
                            overrides={"paths": {"repo_root": str(repo), "work_dir": str(repo / ".tdd-agent")},
                                       "testspec": {"llm": "none", **testspec}, "mcp": {"write": write}})
    rt = Runtime(o.config, _know, ops, o.observer, o.gateway, o)
    _SERVED["rt"] = rt
    baseline(o, None)
    return o, ops, McpTools(rt, McpPolicy.from_runtime(rt))


def context_files(repo):
    base = repo / ".tdd-agent" / "context"
    return sorted(str(p.relative_to(base)) for p in base.rglob("*")) if base.is_dir() else []


# ---------------------------------------------------------------- testspec_start (TC-67, 68)
def test_start_with_a_solution_writes_it_creates_the_run_and_the_run_uses_it(tmp_path, repo):
    o, ops, tools = build(tmp_path, repo, context_mode="solution")
    got = tools.testspec_start({"kind": "solution", "ref": "REQ-9", "document": SOLUTION}, tdd="TDD-B")
    assert got["reused"] is False and got["context"]["mode"] == "solution" and got["context"]["path"] == ".tdd-agent/context/solution-REQ-9"
    assert got["context"]["files"] == ["solution.json"] and len(got["context"]["sha256"]) == 64
    assert any("not approved" in n for n in got["context"]["notes"])                      # a draft Solution is used, with a note
    run = o.get_run(got["run_id"])
    assert run.flow == "testspec" and run.status == "running"                             # not advanced: the caller dispatches
    assert run.request["params"]["context_path"] == ".tdd-agent/context/solution-REQ-9" and run.request["params"]["context_ref"] == "REQ-9"
    assert (repo / ".tdd-agent/context/solution-REQ-9/solution.json").is_file()
    assert o.advance(got["run_id"]).status == "completed"
    (f,) = ops.get_task(got["run_id"], "TS1")["output"]["features"]
    assert f["feature"] == "TDD-B" and f["context"]["mode"] == "solution" and f["context"]["anchors"] == ["FR-2"]
    assert f["context"]["by_kind"] == {"decision": 1, "risk": 1, "assumption": 1, "acceptance": 1} and f["context"]["unanchored"] == []
    assert ops.get_task(got["run_id"], "TS1")["output"]["context_mode"] == "solution"
    cases = (repo / "docs/testspec/TDD-B/03-test-cases.md").read_text(encoding="utf-8")
    assert "AC-REQ-9-AC-9" in cases or "GET /other" in cases                              # the Solution's criterion is tested


def test_start_with_a_plan_sent_as_files(tmp_path, repo):
    o, ops, tools = build(tmp_path, repo, context_mode="plan")
    got = tools.testspec_start({"kind": "plan", "ref": "REQ-9", "files": PLAN_FILES}, tdd="TDD-B")
    assert got["context"]["files"] == ["design.md", "proposal.md", "tasks.md"] and got["context"]["notes"] == []
    assert o.advance(got["run_id"]).status == "completed"
    (f,) = ops.get_task(got["run_id"], "TS1")["output"]["features"]
    assert f["context"]["mode"] == "plan" and f["context"]["by_kind"] == {"acceptance": 1, "decision": 2, "assumption": 1}
    # the design said "assumed": that is a gap, owned by a person, not a requirement
    assert f["gaps"] >= 1


def test_start_with_a_path_already_in_the_project(tmp_path, repo):
    o, _ops, tools = build(tmp_path, repo, context_mode="solution")
    (repo / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    got = tools.testspec_start({"kind": "solution", "ref": "R", "path": "sol.json"}, tdd="TDD-B")
    assert got["context"]["path"] == "sol.json" and got["context"]["files"] == [] and context_files(repo) == []
    assert o.advance(got["run_id"]).status == "completed"
    with pytest.raises(TddAgentError) as e:
        tools.testspec_start({"kind": "solution", "ref": "R", "path": "../outside.json"}, tdd="TDD-B")
    assert e.value.code == "TESTSPEC_CONTEXT_PATH"


# ---------------------------------------------------------------- refusals (TC-69, 70, 73)
@pytest.mark.parametrize("testspec,ctx,code", [
    ({"context_mode": "solution"}, {"kind": "plan", "ref": "R", "files": PLAN_FILES}, "TESTSPEC_CONTEXT_MODE_MISMATCH"),
    ({"context_mode": "plan"}, {"kind": "solution", "ref": "R", "document": SOLUTION}, "TESTSPEC_CONTEXT_MODE_MISMATCH"),
    ({"context_mode": "none"}, {"kind": "solution", "ref": "R", "document": SOLUTION}, "TESTSPEC_CONTEXT_MODE_MISMATCH"),
    ({"context_mode": "solution"}, {"kind": "other", "ref": "R", "document": SOLUTION}, "TESTSPEC_CONTEXT_MODE_MISMATCH"),
    ({"context_mode": "solution", "enabled": False}, {"kind": "solution", "ref": "R", "document": SOLUTION}, "TESTSPEC_DISABLED"),
    ({"context_mode": "solution", "llm": ""}, {"kind": "solution", "ref": "R", "document": SOLUTION}, "LLM_NOT_CONFIGURED"),
    ({"context_mode": "solution"}, {"kind": "solution", "ref": "R"}, "TESTSPEC_CONTEXT_FILE"),
    ({"context_mode": "solution"}, {"kind": "solution", "ref": "R", "document": SOLUTION, "path": "x.json"}, "TESTSPEC_CONTEXT_FILE"),
    ({"context_mode": "solution"}, {"kind": "solution", "ref": "bad ref", "document": SOLUTION}, "TESTSPEC_CONTEXT_FILE"),
    ({"context_mode": "plan"}, {"kind": "plan", "ref": "R", "files": {"tasks.md": "# nothing"}}, "TESTSPEC_CONTEXT_INVALID"),
    ({"context_mode": "plan"}, {"kind": "plan", "ref": "R", "files": {"tasks.md": TASKS, "../x.md": "x"}}, "TESTSPEC_CONTEXT_FILE"),
    ({"context_mode": "plan"}, {"kind": "plan", "ref": "R", "files": {"tasks.md": "x" * 300_000}}, "TESTSPEC_CONTEXT_TOO_LARGE"),
])
def test_refused_calls_create_no_run_and_write_nothing(tmp_path, repo, testspec, ctx, code):
    _o, ops, tools = build(tmp_path, repo, **testspec)
    with pytest.raises(TddAgentError) as e:
        tools.testspec_start(ctx, tdd="TDD-B")
    assert e.value.code == code
    assert ops.list_runs(flow="testspec") == [] and context_files(repo) == []
    if code == "TESTSPEC_CONTEXT_MODE_MISMATCH":
        assert e.value.details == {"configured": testspec["context_mode"], "requested": ctx["kind"]}


def test_start_is_a_write_tool(tmp_path, repo):
    _o, _ops, tools = build(tmp_path, repo, write=False, context_mode="solution")
    with pytest.raises(TddAgentError) as e:
        tools.testspec_start({"kind": "solution", "ref": "R", "document": SOLUTION}, tdd="TDD-B")
    assert e.value.code == "WRITE_DISABLED"


def test_a_scope_is_needed(tmp_path, repo):
    _o, _ops, tools = build(tmp_path, repo, context_mode="solution")
    with pytest.raises(TddAgentError) as e:
        tools.testspec_start({"kind": "solution", "ref": "R", "document": SOLUTION})
    assert e.value.code == "INVALID_ARGUMENT"


# ---------------------------------------------------------------- repeatable calls (TC-71)
def test_the_same_execution_and_content_return_the_same_run(tmp_path, repo):
    o, _ops, tools = build(tmp_path, repo, context_mode="solution")
    ctx = {"kind": "solution", "ref": "REQ-9", "document": SOLUTION}
    first = tools.testspec_start(ctx, tdd="TDD-B", external=EXTERNAL)
    again = tools.testspec_start(ctx, tdd="TDD-B", external=EXTERNAL)
    assert again["reused"] is True and again["run_id"] == first["run_id"] and again["context"]["sha256"] == first["context"]["sha256"]
    changed = json.loads(json.dumps(SOLUTION))
    changed["options"]["assumptions"][0]["text"] = "Other methods get 405 on FR-2, and HEAD works."
    third = tools.testspec_start({**ctx, "document": changed}, tdd="TDD-B", external=EXTERNAL)
    assert third["reused"] is False and third["run_id"] != first["run_id"] and third["context"]["sha256"] != first["context"]["sha256"]
    other = tools.testspec_start(ctx, tdd="TDD-B", external={**EXTERNAL, "stepId": "another"})
    assert other["reused"] is False
    found = tools.run_list(flow="testspec", external_execution_id="exec-1", external_step_id="testspec").result["runs"]
    assert {r["run_id"] for r in found} == {first["run_id"], third["run_id"]}
    assert o.get_run(first["run_id"]).request["origin"]["external"] == EXTERNAL


# ---------------------------------------------------------------- testspec_get (TC-72)
def test_get_config_before_any_run(tmp_path, repo):
    _o, _ops, tools = build(tmp_path, repo, context_mode="plan", context_path="x/y")
    got = tools.testspec_get().result
    assert got == {"config": {"enabled": True, "llm": "none", "context_mode": "plan", "context_path": "x/y", "specs_dir": "docs/testspec"}, "features": []}
    with pytest.raises(NotFound) as e:
        tools.testspec_get("TDD-B")
    assert e.value.code == "TESTSPEC_NOT_FOUND"


def test_get_features_gaps_context_and_documents(tmp_path, repo):
    o, _ops, tools = build(tmp_path, repo, context_mode="solution")
    run_id = tools.testspec_start({"kind": "solution", "ref": "REQ-9", "document": SOLUTION}, tdd="TDD-B")["run_id"]
    assert o.advance(run_id).status == "completed"
    listing = tools.testspec_get().result
    (row,) = listing["features"]
    assert listing["config"]["context_mode"] == "solution" and row["feature"] == "TDD-B" and row["version"] == 1 and row["status"] == "Draft"
    assert row["files"] == [f"docs/testspec/TDD-B/{n}" for n in ("01-test-requirements.md", "02-test-design.md", "03-test-cases.md")]
    one = tools.testspec_get("TDD-B").result["feature"]
    assert one["counts"]["TestCase"] >= 1 and one["context"]["origin_context"]["ref"] == "REQ-9" and one["context"]["context"]["nodes"] == 4
    assert any("ASM-REQ-9-A-1" in g["missing"] or "A-1" in g["missing"] for g in one["gaps"])      # the unconfirmed assumption is an open gap
    assert all(set(g) == {"code", "missing", "question", "owner", "blocks"} for g in one["gaps"])
    doc = tools.testspec_get("TDD-B", "cases")
    assert doc.text == "text" and doc.result["document"] == "cases" and doc.result["length"] == len(doc.result["text"]) > 0
    assert doc.result["text"].startswith("---\nspec: TSD-TDD-B-TestCases")
    page = tools.testspec_get("TDD-B", "cases", offset=100)
    assert page.offset == 100
    with pytest.raises(TddAgentError) as e:
        tools.testspec_get("TDD-B", "slides")
    assert e.value.code == "INVALID_ARGUMENT"
    with pytest.raises(TddAgentError) as e:
        tools.testspec_get(document="cases")
    assert e.value.code == "INVALID_ARGUMENT"
    with pytest.raises(NotFound):
        tools.testspec_get("TDD-NOPE")
    assert "token" not in json.dumps(listing).lower() and "api_key" not in json.dumps(listing).lower()   # only the mode name of the LLM is shown


# ---------------------------------------------------------------- the Orca sequence (TC-74)
def test_orca_sequence_over_the_server(tmp_path, repo):
    """testspec_get -> testspec_start -> run advance -> run_get -> testspec_get, through the same MCP server a bridge would call."""
    anyio = pytest.importorskip("anyio")
    from mcp import Client

    from tdd_agent.mcp import build_server
    o, _ops, _tools = build(tmp_path, repo, context_mode="plan")
    rt = _SERVED["rt"]

    async def go():
        async with Client(build_server(rt)) as client:
            names = {t.name for t in (await client.list_tools()).tools}
            assert {"testspec_start", "testspec_get", "run_dispatch", "run_get"} <= names
            cfg = json.loads((await client.call_tool("testspec_get", {})).content[0].text)
            assert cfg["config"]["context_mode"] == "plan"
            started = json.loads((await client.call_tool("testspec_start", {"context": {"kind": "plan", "ref": "REQ-9", "files": PLAN_FILES},
                                                                             "tdd": "TDD-B", "external": EXTERNAL})).content[0].text)
            await anyio.to_thread.run_sync(o.advance, started["run_id"])
            run = json.loads((await client.call_tool("run_get", {"run_id": started["run_id"]})).content[0].text)
            assert run["run"]["status"] == "completed"
            got = json.loads((await client.call_tool("testspec_get", {"feature": "TDD-B"})).content[0].text)
            assert got["feature"]["context"]["origin_context"]["sha256"] == started["context"]["sha256"]
            bad = await client.call_tool("testspec_start", {"context": {"kind": "solution", "ref": "R", "document": SOLUTION}, "tdd": "TDD-B"})
            assert json.loads(bad.content[0].text)["error"]["code"] == "TESTSPEC_CONTEXT_MODE_MISMATCH"

    anyio.run(go)
