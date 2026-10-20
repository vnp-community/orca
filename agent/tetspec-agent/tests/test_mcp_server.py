"""MCP server tests.  They drive the real server through the SDK's in-memory client, so schemas, tool lists and results are the protocol's."""
from __future__ import annotations

import copy
import inspect
import json

import pytest
from mcp import Client, MCPError

from graphs import edge, el
from test_mcp_support import (ACTORS, READ_TOOLS, WRITE_TOOLS, anyio_backend, cr_flow, connect, events, make_runtime, trivial_flow)  # noqa: F401
from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError, TddAgentError
from tdd_agent.mcp import McpPolicy, McpTools, build_server, paginate
from tdd_agent.mcp.tools import dumps
from tdd_agent.retrieval import ClosureService
from tdd_agent.retrieval.queries import QueryService
from tdd_agent.store.base import SourceDoc
from tdd_agent.store.contract import pbom_graph

pytestmark = pytest.mark.anyio

SOURCES = [{"id": "SRS-1", "revision": "1", "role": "SRS", "text": "srs v1"}, {"id": "SAD-1", "revision": "1", "role": "SAD", "text": "sad v1"}]


def chain(n: int) -> dict:
    return {"version": "c", "elements": [el(f"N{i:03d}", "Component") for i in range(n)],
            "edges": [edge(f"N{i:03d}", "depends_on", f"N{i + 1:03d}") for i in range(n - 1)]}


def tool_spans(runtime):
    return [s for s in runtime.operational.list_spans() if s["kind"] == "tool"]


# ====================================================================== tool registration
async def test_default_flags_expose_only_read_tools(tmp_path):
    rt = make_runtime(tmp_path)
    async with connect(rt) as mcp:
        names = await mcp.tool_names()
    assert names == READ_TOOLS
    assert not {"run_start", "run_resume", "changeset_propose", "approval_submit"} & set(names)


async def test_write_flag_adds_write_tools_but_never_the_approval_tool(tmp_path):
    rt = make_runtime(tmp_path, write=True)
    async with connect(rt) as mcp:
        assert await mcp.tool_names() == READ_TOOLS + WRITE_TOOLS


async def test_approval_tool_needs_both_keys(tmp_path):
    rt = make_runtime(tmp_path, approvals=True, client_actor="claude")
    async with connect(rt) as mcp:
        assert await mcp.tool_names() == READ_TOOLS + ["approval_submit"]
    rt = make_runtime(tmp_path, write=True, approvals=True, client_actor="claude")
    async with connect(rt) as mcp:
        assert await mcp.tool_names() == READ_TOOLS + WRITE_TOOLS + ["approval_submit"]


async def test_approval_tool_absent_when_flag_off_and_calling_it_is_tool_not_found(tmp_path, events):
    rt = make_runtime(tmp_path, client_actor="claude")
    async with connect(rt) as mcp:
        assert "approval_submit" not in await mcp.tool_names()
        err = await mcp.fail("approval_submit", run_id="r", gate="G1", role="BA", decision_hash="h", rationale="x")
    assert err["code"] == "TOOL_NOT_FOUND"
    assert [e["reason"] for e in events.named("mcp_tool_denied")] == ["approvals_unavailable"]


def test_config_refuses_approvals_for_a_client_that_is_not_allowlisted():
    overrides = {"approvals": {"actors": ACTORS}}
    with pytest.raises(ConfigError):
        load_config(overrides={**overrides, "mcp": {"approvals": True, "client_actor": "mallory"}}, env={})
    with pytest.raises(ConfigError):
        load_config(overrides={**overrides, "mcp": {"approvals": True}}, env={})


async def test_client_outside_the_allowlist_gets_no_approval_tool_and_the_refusal_is_logged(tmp_path, events):
    rt = make_runtime(tmp_path, approvals=True, client_actor="claude")
    async with connect(rt, client_actor="mallory") as mcp:
        assert await mcp.tool_names() == READ_TOOLS
        err = await mcp.fail("approval_submit", run_id="r", gate="G1", role="BA", decision_hash="h", rationale="x")
    assert err["code"] == "TOOL_NOT_FOUND"
    unavailable = events.named("mcp_approvals_unavailable")
    assert len(unavailable) == 1 and unavailable[0]["actor"] == "mallory"


async def test_write_tool_call_when_disabled_is_write_disabled_and_creates_nothing(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": trivial_flow()})
    async with connect(rt) as mcp:
        for tool, args in (("run_start", {"flow": "cr", "request": {}}), ("run_resume", {"run_id": "x"}),
                           ("changeset_propose", {"changeset": {"id": "c", "ops": []}}), ("proposals_list", {})):
            err = await mcp.fail(tool, **args)
            assert err["code"] == "WRITE_DISABLED" and err["details"]["tool"] == tool
    assert rt.operational.list_runs() == [] and rt.operational.list_audit() == []


def test_write_and_approval_methods_refuse_even_when_called_without_the_protocol(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": trivial_flow()})
    tools = McpTools(rt, McpPolicy.from_runtime(rt))
    for call in (lambda: tools.run_start("cr", {}), lambda: tools.run_resume("r"), lambda: tools.changeset_propose({"id": "c"}),
                 lambda: tools.proposals_list()):
        with pytest.raises(TddAgentError) as exc:
            call()
        assert exc.value.code == "WRITE_DISABLED"
    with pytest.raises(TddAgentError) as exc:
        tools.approval_submit("r", "G1", "BA", "h", "why")
    assert exc.value.code == "APPROVALS_DISABLED" and rt.operational.list_runs() == []


def test_result_limit_below_the_minimum_is_a_config_error(tmp_path):
    rt = make_runtime(tmp_path, max_result_chars=100)
    with pytest.raises(ConfigError):
        build_server(rt)


# ====================================================================== read tools on the synthetic graph
async def test_kg_get_element_states_snapshot_and_reports_missing(tmp_path):
    rt = make_runtime(tmp_path)
    head = rt.knowledge.head()
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_get_element", id="CS-1")
        assert body["snapshot"] == head.id and body["element"] == rt.knowledge.get_element("CS-1") and body["truncated"] is False
        err = await mcp.fail("kg_get_element", id="NOPE")
        assert err["code"] == "ELEMENT_NOT_FOUND" and err["details"] == {"id": "NOPE", "snapshot": head.id}


async def test_explicit_snapshot_is_honoured_and_an_unknown_one_never_falls_back_to_head(tmp_path):
    rt = make_runtime(tmp_path)
    first = rt.knowledge.head().id
    graph = rt.knowledge.materialize()
    graph["elements"] = [dict(e, name="renamed") if e["id"] == "CS-1" else e for e in graph["elements"]]
    second = rt.knowledge.commit_graph(first, graph, actor="t", validate=False).snapshot.id
    async with connect(rt) as mcp:
        assert (await mcp.ok("kg_get_element", id="CS-1"))["element"]["name"] == "renamed"
        old = await mcp.ok("kg_get_element", id="CS-1", snapshot=first)
        assert old["snapshot"] == first and old["element"]["name"] == "CS-1"
        for tool, args in (("kg_get_element", {"id": "CS-1"}), ("kg_neighbors", {"ids": ["CS-1"]}), ("kg_resolve_scope", {"seeds": ["CS-1"]}),
                           ("kg_query", {"name": "Q4"}), ("source_get", {"id": "SRS-DOC"}), ("run_list", {})):
            is_error, body = await mcp(tool, snapshot="snap-999999", **args)
            assert is_error and body["error"]["code"] == "SNAPSHOT_NOT_FOUND" and body["snapshot"] == "snap-999999", tool
            assert "element" not in body and "closure" not in body and "rows" not in body
        err = await mcp.fail("kg_get_element", id="CS-1", snapshot="  ")
        assert err["code"] == "INVALID_ARGUMENT"
    assert second != first


async def test_empty_store_reports_no_snapshot_for_graph_tools_and_none_for_run_tools(tmp_path):
    rt = make_runtime(tmp_path, load=False)
    async with connect(rt) as mcp:
        assert (await mcp.fail("kg_get_element", id="X"))["code"] == "SNAPSHOT_NOT_FOUND"
        body = await mcp.ok("run_list")
        assert body["snapshot"] is None and body["runs"] == []
        assert (await mcp.ok("kg_snapshots"))["snapshots"] == []


async def test_kg_neighbors_filters_by_relation_and_direction(tmp_path):
    rt = make_runtime(tmp_path)
    async with connect(rt) as mcp:
        both = await mcp.ok("kg_neighbors", ids=["IF-1"])
        assert both["edges"] == rt.knowledge.neighbors(["IF-1"]) and {e["rel"] for e in both["edges"]} == {"uses", "traces_to"}
        out = await mcp.ok("kg_neighbors", ids=["OP-2"], relations=["uses"], direction="out")
        assert out["edges"] == [{"src": "OP-2", "rel": "uses", "dst": "IF-1"}] and out["direction"] == "out"
        assert (await mcp.ok("kg_neighbors", ids=["IF-1"], direction="out"))["edges"] == []
        assert (await mcp.fail("kg_neighbors", ids=["IF-1"], direction="sideways"))["code"] == "INVALID_ARGUMENT"


async def test_kg_resolve_scope_returns_the_full_closure_with_the_service_hash(tmp_path):
    rt = make_runtime(tmp_path)
    head = rt.knowledge.head().id
    expected = ClosureService(rt.knowledge).resolve(["CS-1"], profile="tdd-generation", snapshot=head).to_dict()
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_resolve_scope", seeds=["CS-1"])
        assert body["snapshot"] == head and body["resolution_hash"] == expected["resolution_hash"]
        assert body["closure"] == json.loads(dumps(expected)) and body["closure"]["snapshot"] == head
        assert body["blockers"] == ClosureService(rt.knowledge).resolve(["CS-1"], snapshot=head).blockers

        budgeted = await mcp.ok("kg_resolve_scope", seeds=["CS-1"], budget=2, mandatory_ids=["SEC-TDD-1"], profile="none", snapshot=head)
        direct = ClosureService(rt.knowledge).resolve(["CS-1"], profile="none", budget=2, mandatory_ids=["SEC-TDD-1"], snapshot=head)
        assert budgeted["resolution_hash"] == direct.resolution_hash != body["resolution_hash"]
        assert budgeted["closure"]["truncated"] is True and budgeted["closure"]["missing_ids"] == direct.missing_ids
        assert budgeted["truncated"] is False, "the closure's own budget flag is not the transport's truncation flag"

        assert (await mcp.fail("kg_resolve_scope", seeds=[]))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("kg_resolve_scope", seeds=["CS-1"], profile="nope"))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("kg_resolve_scope", seeds=["CS-1"], budget=-1))["code"] == "INVALID_ARGUMENT"


@pytest.mark.parametrize("name,args", [("Q1", {"delta_id": "DELTA-1"}), ("q2", {"tdd_id": "TDD-A"}), ("Q3", {"cs_id": "CS-1"}), ("Q4", {}), ("Q5", {}),
                                       ("Q6", {}), ("Q7", {"report_id": "IR-1"}), ("Q8", {"cr_id": "CR-1"}), ("Q9", {})])
async def test_kg_query_runs_q1_to_q9_like_the_query_service(tmp_path, name, args):
    rt = make_runtime(tmp_path)
    head = rt.knowledge.head().id
    expected = QueryService(rt.knowledge).run(name, snapshot=head, **args).to_dict()
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_query", name=name, args=args)
    assert body["snapshot"] == head and body["name"] == expected["name"] and body["kind"] == expected["kind"]
    assert body["count"] == expected["count"] == len(body["rows"]) and body["rows"] == json.loads(dumps(expected["rows"]))


async def test_kg_query_errors_are_structured(tmp_path):
    rt = make_runtime(tmp_path)
    async with connect(rt) as mcp:
        assert (await mcp.fail("kg_query", name="Q10"))["code"] == "QUERY_UNKNOWN"
        assert (await mcp.fail("kg_query", name="Q1", args={"delta_id": "NOPE"}))["code"] == "DELTA_NOT_FOUND"
        assert (await mcp.fail("kg_query", name="Q1"))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("kg_query", name="Q4", args={"bogus": 1}))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("kg_query", name="Q4", args={"snapshot": "snap-000001"}))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("kg_query", name="Q3", args={"cs_id": "CS-BADROLE"}))["code"] == "SOURCE_ROLE_ERROR"
        err = await mcp.fail("kg_query", name="Q4", args=None, snapshot="nope")
        assert err["code"] == "SNAPSHOT_NOT_FOUND"


async def test_kg_snapshots_lists_newest_first_with_limit(tmp_path):
    rt = make_runtime(tmp_path)
    first = rt.knowledge.head().id
    graph = rt.knowledge.materialize()
    graph["version"] = "wf-2"
    second = rt.knowledge.commit_graph(first, graph, actor="t", validate=False).snapshot.id
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_snapshots")
        assert body["head"] == second == body["snapshot"] and [s["id"] for s in body["snapshots"]] == [second, first]
        assert [s["id"] for s in (await mcp.ok("kg_snapshots", limit=1))["snapshots"]] == [second]
        assert (await mcp.fail("kg_snapshots", limit=0))["code"] == "INVALID_ARGUMENT"


async def test_source_get_returns_the_stored_document_and_enforces_role(tmp_path):
    rt = make_runtime(tmp_path)
    rt.knowledge.register_source(SourceDoc.make("SRS-DOC", "3", "SRS", "# SRS rev 3"))
    async with connect(rt) as mcp:
        latest = await mcp.ok("source_get", id="SRS-DOC")
        assert latest["source"]["revision"] == "3" and latest["text"] == "# SRS rev 3" and latest["length"] == 11
        stored = rt.knowledge.get_source("SRS-DOC", "2")
        old = await mcp.ok("source_get", id="SRS-DOC", revision="2", role="srs")
        assert old["text"] == stored.text and old["source"]["content_hash"] == stored.content_hash and old["source"]["role"] == "SRS"
        assert (await mcp.fail("source_get", id="SRS-DOC", role="SAD"))["code"] == "SOURCE_ROLE_ERROR"
        assert (await mcp.fail("source_get", id="NOPE"))["code"] == "SOURCE_NOT_FOUND"


# ====================================================================== truncation and pagination
async def test_large_neighbor_results_are_cut_with_a_cursor_and_nothing_is_lost(tmp_path):
    rt = make_runtime(tmp_path, load=False, max_result_chars=2000)
    rt.knowledge.commit_graph(None, chain(300), actor="t", validate=False)
    ids = [f"N{i:03d}" for i in range(300)]
    expected = rt.knowledge.neighbors(ids)
    async with connect(rt) as mcp:
        edges, offset, pages = [], 0, 0
        while True:
            result = await mcp.client.call_tool("kg_neighbors", {"ids": ids, "offset": offset})
            body = result.structured_content
            assert not result.is_error and len(result.content[0].text) <= 2000 and "oversize" not in body
            edges += body["edges"]
            pages += 1
            if not body["truncated"]:
                assert body["next"] is None
                break
            assert body["next"] == offset + len(body["edges"]) and body["page"]["offset"] == offset and body["page"]["returned"] == len(body["edges"])
            offset = body["next"]
    assert edges == expected and pages > 5


async def test_closure_pages_reassemble_to_the_service_closure(tmp_path):
    rt = make_runtime(tmp_path, load=False, max_result_chars=3000)
    rt.knowledge.commit_graph(None, chain(120), actor="t", validate=False)
    expected = ClosureService(rt.knowledge).resolve(["N000"]).to_dict()
    async with connect(rt) as mcp:
        selected, edges, reasons, offset = [], [], {}, 0
        while True:
            body = await mcp.ok("kg_resolve_scope", seeds=["N000"], offset=offset)
            assert body["resolution_hash"] == expected["resolution_hash"] == body["closure"]["resolution_hash"]
            selected += body["closure"]["selected_ids"]
            edges += body["closure"]["edges"]
            reasons.update(body["closure"]["reasons"])
            assert len(dumps(body)) <= 3000
            if not body["truncated"]:
                break
            offset = body["next"]
    assert selected == expected["selected_ids"] and edges == expected["edges"] and reasons == expected["reasons"]


async def test_source_text_is_paged_by_characters(tmp_path):
    rt = make_runtime(tmp_path, max_result_chars=1000)
    text = "line é \"quoted\"\n" * 400
    rt.knowledge.register_source(SourceDoc.make("BIG", "1", "SRS", text))
    async with connect(rt) as mcp:
        parts, offset = [], 0
        while True:
            body = await mcp.ok("source_get", id="BIG", offset=offset)
            assert len(dumps(body)) <= 1000 and body["length"] == len(text)
            parts.append(body["text"])
            if not body["truncated"]:
                break
            assert body["next"] == offset + len(body["text"]) and body["page"]["unit"] == "chars"
            offset = body["next"]
        assert "".join(parts) == text and len(parts) > 5
        assert (await mcp.fail("source_get", id="BIG", offset=len(text) + 1))["code"] == "INVALID_CURSOR"
        assert (await mcp.fail("source_get", id="BIG", offset=-1))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.ok("source_get", id="BIG", offset=len(text)))["text"] == ""


async def test_an_atomic_item_over_the_limit_is_returned_whole_and_flagged(tmp_path):
    rt = make_runtime(tmp_path, load=False, max_result_chars=500)
    graph = {"version": "v", "elements": [el("HUGE", "Component", blob="x" * 3000)], "edges": []}
    rt.knowledge.commit_graph(None, graph, actor="t", validate=False)
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_get_element", id="HUGE")
    assert body["oversize"] is True and body["truncated"] is False and body["element"]["props"]["blob"] == "x" * 3000


def test_paginate_orders_parts_pins_dict_keys_and_derives_per_page():
    result = {"head": 1, "a": list(range(10)), "tasks": {f"G{i}": {"v": "y" * 40} for i in range(6)}, "tree": ""}
    derive = lambda page: dict(page, tree=",".join(map(str, page["a"])))  # noqa: E731
    seen_a, seen_tasks, offset = [], {}, 0
    while True:
        page = paginate(result, ("a", "tasks"), limit=330, offset=offset, pinned={"tasks": {"G5"}}, derive=derive)
        seen_a += page["a"]
        seen_tasks.update(page["tasks"])
        assert page["head"] == 1 and page["tree"] == ",".join(map(str, page["a"])) and len(dumps(page)) <= 330 + 40
        assert ("G5" in page["tasks"]) == (offset == 0)
        if not page["truncated"]:
            break
        offset = page["next"]
    assert seen_a == result["a"] and seen_tasks == result["tasks"]
    with pytest.raises(TddAgentError) as exc:
        paginate(result, ("a",), limit=100, offset=11)
    assert exc.value.code == "INVALID_CURSOR"
    whole = paginate(result, ("a",), limit=10_000)
    assert whole["truncated"] is False and whole["next"] is None and "page" not in whole and whole["a"] == result["a"]


# ====================================================================== write tools
async def test_run_start_and_resume_are_attributed_to_an_agent_and_pin_sources(tmp_path, events):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, client_actor="claude")
    async with connect(rt) as mcp:
        started = await mcp.ok("run_start", flow="cr", request={"mode": "change-request", "origin": {"actor": "forged"}}, sources=SOURCES)
        run_id = started["run_id"]
        assert started["run"]["status"] == "running" and started["run"]["request"]["origin"] == {"actor": "claude", "actor_kind": "agent", "channel": "mcp"}
        assert [p["id"] for p in started["run"]["request"]["pins"]] == ["SAD-1", "SRS-1"]
        assert rt.knowledge.get_source("SRS-1", "1").text == "srs v1"
        resumed = await mcp.ok("run_resume", run_id=run_id)
        assert resumed["run"]["status"] == "awaiting_approval" and resumed["pending_gate"] == "G1"
        listed = await mcp.ok("run_list", flow="cr", status="awaiting_approval")
        assert [r["run_id"] for r in listed["runs"]] == [run_id]
    audits = rt.operational.list_audit(kind="mcp_write")
    assert [(a["payload"]["tool"], a["payload"]["actor_kind"], a["payload"]["actor"], a["run_id"]) for a in audits] == [
        ("run_start", "agent", "claude", run_id), ("run_resume", "agent", "claude", run_id)]
    started_event = [e for e in events.named("mcp_tool_call") if e["tool"] == "run_start"][0]
    assert started_event["run_id"] == run_id


async def test_run_start_errors_are_structured_and_create_no_run(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True)
    async with connect(rt) as mcp:
        assert (await mcp.fail("run_start", flow="nope", request={}))["code"] == "FLOW_UNKNOWN"
        assert (await mcp.fail("run_start", flow="cr", request={}, sources=[{"id": "X"}]))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("run_resume", run_id="run-missing"))["code"] == "RUN_NOT_FOUND"
        await mcp.ok("run_start", flow="cr", request={}, sources=[{"id": "S", "revision": 1, "role": "srs", "text": "a"}])
        err = await mcp.fail("run_start", flow="cr", request={}, sources=[{"id": "S", "revision": "1", "role": "SRS", "text": "different"}])
        assert err["code"] == "SOURCE_CONFLICT"
    assert len(rt.operational.list_runs()) == 1


@pytest.fixture
def pbom_runtime(tmp_path):
    rt = make_runtime(tmp_path, load=False, write=True, client_actor="claude")
    rt.knowledge.commit_graph(None, pbom_graph(), actor="seed")
    return rt


def modify_cs(graph: dict, target: str, after: str, *, version: str = "PBOM-TDD-v2") -> dict:
    el_ = next(e for e in graph["elements"] if e["id"] == target)
    return {"id": "CS-P1", "base_version": graph["version"], "result_version": version,
            "ops": [{"seq": 1, "op": "modify", "target": target, "path": "name", "before": el_["name"], "after": after}]}


async def test_changeset_proposal_is_validated_and_stored_but_never_committed(pbom_runtime):
    rt = pbom_runtime
    head, graph = rt.knowledge.head(), rt.knowledge.materialize()
    target = next(e["id"] for e in graph["elements"] if e["class"] == "Operation" and e.get("owner_doc") != "SAD")
    good = modify_cs(graph, target, "Renamed by agent")
    async with connect(rt) as mcp:
        body = await mcp.ok("changeset_propose", changeset=good, base_snapshot=head.id)
        assert body["ok"] is True and body["committed"] is False and body["snapshot"] == head.id and body["report"]["touched"] == [target]
        assert body["report"]["stale_base"] is False and len(body["report"]["result_hash"]) == 64
        assert rt.knowledge.head().id == head.id and rt.knowledge.get_element(target)["name"] != "Renamed by agent"
        assert rt.knowledge.materialize() == graph and len(rt.knowledge.snapshots()) == 1

        stale = dict(good, base_version="OTHER")
        bad = await mcp.ok("changeset_propose", changeset=stale)
        assert bad["ok"] is False and bad["report"]["code"] == "VERSION_CONFLICT"
        wrong_before = copy.deepcopy(good)
        wrong_before["ops"][0]["before"] = "not the current name"
        assert (await mcp.ok("changeset_propose", changeset=wrong_before))["report"]["ok"] is False
        broken = await mcp.ok("changeset_propose", changeset={"id": "x", "ops": [{"seq": 1, "op": "modify"}]})
        assert broken["ok"] is False
        garbage = await mcp.ok("changeset_propose", changeset={"id": "x", "ops": ["not an op"]})
        assert garbage["ok"] is False and garbage["report"]["code"] == "CHANGESET_MALFORMED"
        assert (await mcp.fail("changeset_propose", changeset=good, base_snapshot="snap-999"))["code"] == "SNAPSHOT_NOT_FOUND"

        listed = await mcp.ok("proposals_list")
        assert listed["total"] == 5 and all("changeset" not in p for p in listed["proposals"])
        assert [p["ok"] for p in listed["proposals"]] == [False, False, False, False, True]
        one = await mcp.ok("proposals_list", proposal_id=body["proposal_id"])
        assert one["proposals"][0]["changeset"] == good and one["proposals"][0]["actor_kind"] == "agent"
        assert (await mcp.fail("proposals_list", proposal_id="prop-none"))["code"] == "PROPOSAL_NOT_FOUND"
    audit = rt.operational.list_audit(kind="changeset_proposal")
    assert len(audit) == 5 and audit[0]["payload"]["actor"] == "claude" and audit[0]["payload"]["channel"] == "mcp"
    assert rt.knowledge.head().id == head.id and len(rt.knowledge.snapshots()) == 1


async def test_changeset_proposal_cannot_touch_sad_owned_elements(pbom_runtime):
    rt = pbom_runtime
    graph = rt.knowledge.materialize()
    sad = next(e["id"] for e in graph["elements"] if str(e.get("owner_doc")).upper() == "SAD")
    async with connect(rt) as mcp:
        body = await mcp.ok("changeset_propose", changeset=modify_cs(graph, sad, "edited SAD"))
    assert body["ok"] is False and body["report"]["code"] == "SAD_READONLY" and body["report"]["sad_violations"] == [sad]
    assert len(rt.knowledge.snapshots()) == 1


async def test_proposals_list_pages_through_the_audit_trail(tmp_path):
    rt = make_runtime(tmp_path, write=True)
    async with connect(rt) as mcp:
        for i in range(5):
            await mcp.ok("changeset_propose", changeset={"id": f"cs-{i}", "ops": []})
        first = await mcp.ok("proposals_list", limit=2)
        assert [p["changeset_id"] for p in first["proposals"]] == ["cs-4", "cs-3"] and first["next"] == 2 and first["truncated"] is False
        last = await mcp.ok("proposals_list", limit=2, offset=4)
        assert [p["changeset_id"] for p in last["proposals"]] == ["cs-0"] and last["next"] is None


# ====================================================================== runs, traces and metrics
async def test_run_tools_report_runs_traces_and_metrics(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": trivial_flow()})
    runs = [rt.orchestrator.start("cr", {}, run_id=f"run-{i}") for i in range(5)]
    done = rt.orchestrator.advance("run-0")
    head = rt.knowledge.head().id
    async with connect(rt) as mcp:
        listing = await mcp.ok("run_list")
        assert listing["snapshot"] == head and {r["run_id"] for r in listing["runs"]} == {r.run_id for r in runs}
        assert [r["run_id"] for r in (await mcp.ok("run_list", status="completed"))["runs"]] == ["run-0"]
        page = await mcp.ok("run_list", limit=2)
        assert len(page["runs"]) == 2 and page["next"] == 2 and page["truncated"] is False
        tail = await mcp.ok("run_list", limit=2, offset=4)
        assert len(tail["runs"]) == 1 and tail["next"] is None
        assert (await mcp.fail("run_list", limit=0))["code"] == "INVALID_ARGUMENT"

        got = await mcp.ok("run_get", run_id="run-0")
        view = rt.orchestrator.run_view("run-0")
        assert got["run"] == json.loads(dumps(view["run"])) and got["tasks"] == json.loads(dumps(view["tasks"]))
        assert got["transitions"] == json.loads(dumps(view["transitions"])) and got["snapshot"] == head
        assert (await mcp.fail("run_get", run_id="nope"))["code"] == "RUN_NOT_FOUND"

        trace = await mcp.ok("trace_get", run_id="run-0")
        assert trace["trace_id"] == done.trace_id and trace["run_id"] == "run-0" and "run:cr" in trace["tree"] and "gate" in trace["tree"]
        names = [s["name"] for s in trace["spans"]]
        assert names[0] == "run:cr" and "G0" in names and all(s["trace_id"] == done.trace_id for s in trace["spans"])
        by_trace = await mcp.ok("trace_get", trace_id=done.trace_id)
        assert [s["span_id"] for s in by_trace["spans"] if not s["name"].startswith("mcp:")] == \
               [s["span_id"] for s in trace["spans"] if not s["name"].startswith("mcp:")]
        assert (await mcp.fail("trace_get", trace_id="deadbeef"))["code"] == "TRACE_NOT_FOUND"
        assert (await mcp.fail("trace_get"))["code"] == "INVALID_ARGUMENT"
        assert (await mcp.fail("trace_get", run_id="run-0", trace_id="x"))["code"] == "INVALID_ARGUMENT"

        metrics = await mcp.ok("metrics_get")
        assert metrics["snapshot"] == head
        assert {"labels": {"flow": "cr", "status": "completed"}, "value": 1.0} in metrics["metrics"]["runs_total"]
        assert metrics["llm_share"] == {}


async def test_trace_pages_render_a_tree_for_each_page(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, max_result_chars=1100)
    rt.orchestrator.start("cr", {}, run_id="r")
    trace_id = rt.orchestrator.advance("r").trace_id
    expected = [s["span_id"] for s in rt.operational.list_spans(trace_id=trace_id)]
    async with connect(rt) as mcp:
        spans, offset, pages = [], 0, 0
        while True:
            body = await mcp.ok("trace_get", run_id="r", offset=offset)
            assert len(dumps(body)) <= 1100 and "oversize" not in body
            assert all(s["name"] in body["tree"] for s in body["spans"])
            spans += [s["span_id"] for s in body["spans"]]
            pages += 1
            if not body["truncated"]:
                break
            offset = body["next"]
    assert pages > 1 and spans[:len(expected)] == expected


# ====================================================================== review receipts and approvals over MCP
async def started_run(mcp, rt) -> tuple[str, str]:
    run_id = (await mcp.ok("run_start", flow="cr", request={"mode": "change-request"}, sources=SOURCES))["run_id"]
    await mcp.ok("run_resume", run_id=run_id)
    return run_id, rt.operational.get_task(run_id, "G1")["decision_hash"]


async def test_agent_client_reads_then_approves_every_gate_through_the_same_policy(tmp_path, events):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, approvals=True, client_actor="claude")
    async with connect(rt) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        submit = dict(run_id=run_id, gate="G1", role="BA", decision_hash=h1, rationale="reviewed the SRS readiness")

        err = await mcp.fail("approval_submit", **submit)
        assert err["code"] == "REVIEW_RECEIPT_MISSING" and rt.operational.get_run(run_id).status == "awaiting_approval"

        view = await mcp.ok("run_get", run_id=run_id)
        assert view["tasks"]["G1"]["decision"] == {"srs": "ready"} and view["tasks"]["G1"]["decision_hash"] == h1
        assert view["review_receipts"] == [{"gate": "G1", "decision_hash": h1, "actor_id": "claude", "source": "mcp"}]
        assert rt.operational.find_receipt(run_id, "claude", h1)["source"] == "mcp"
        assert rt.operational.find_receipt(run_id, "alice", h1) is None, "only the client identity gets a receipt"

        assert (await mcp.fail("approval_submit", **dict(submit, role="SA/Dev")))["code"] == "ROLE_NOT_PERMITTED"
        assert (await mcp.fail("approval_submit", **dict(submit, rationale="  ")))["code"] == "RATIONALE_REQUIRED"
        assert (await mcp.fail("approval_submit", **dict(submit, decision_hash="bad")))["code"] == "DECISION_HASH_MISMATCH"
        assert (await mcp.fail("approval_submit", **dict(submit, gate="G3")))["code"] == "GATE_NOT_APPROVABLE"

        accepted = await mcp.ok("approval_submit", **submit)
        assert accepted["accepted"] is True and accepted["run_status"] == "awaiting_approval"
        record = accepted["approval"]
        assert (record["actor_id"], record["actor_kind"], record["approval_grade"], record["channel"], record["role"]) == \
               ("claude", "coding_agent", "agent", "mcp", "BA")
        assert (await mcp.fail("approval_submit", **submit))["code"] == "NOT_AWAITING_APPROVAL"

        impact = await mcp.ok("impact_get", run_id=run_id)
        h2 = impact["gates"]["G2"]["decision_hash"]
        assert impact["gates"]["G2"]["decision"]["report"] == "r1" and impact["pending_gate"] == "G2" and impact["flow"] == "cr"
        assert impact["review_receipts"] == [{"gate": "G2", "decision_hash": h2, "actor_id": "claude", "source": "mcp"}]
        assert (await mcp.ok("approval_submit", run_id=run_id, gate="G2", role="SA/Dev", decision_hash=h2, rationale="impact is right"))["accepted"]

        readiness = await mcp.ok("readiness_get", run_id=run_id)
        assert readiness["gates"]["G4"]["status"] == "completed" and readiness["gates"]["G5"]["status"] == "awaiting_approval"
        h5 = readiness["gates"]["G5"]["decision_hash"]
        assert [r["gate"] for r in readiness["review_receipts"]] == ["G5"]
        done = await mcp.ok("approval_submit", run_id=run_id, gate="G5", role="SA", decision_hash=h5, rationale="spec complete")
        assert done["run_status"] == "completed"

    run = rt.operational.get_run(run_id)
    assert run.status == "completed" and run.state == "spec_authored"
    approvals = rt.operational.list_approvals(run_id)
    assert [(a["gate"], a["actor_id"], a["actor_kind"], a["approval_grade"], a["channel"]) for a in approvals] == \
           [(g, "claude", "coding_agent", "agent", "mcp") for g in ("G1", "G2", "G5")]
    assert {t["actor"] for t in rt.operational.list_transitions(run_id) if t["gate"] in ("G1", "G2", "G5")} == {"claude"}
    assert sorted(a["payload"]["code"] for a in rt.operational.list_audit(kind="approval_rejected", run_id=run_id)) == \
        ["DECISION_HASH_MISMATCH", "GATE_NOT_APPROVABLE", "NOT_AWAITING_APPROVAL", "RATIONALE_REQUIRED", "REVIEW_RECEIPT_MISSING", "ROLE_NOT_PERMITTED"]

    approvals_logged = [e for e in events.named("mcp_tool_call") if e["tool"] == "approval_submit"]
    assert len(approvals_logged) == 9 and all(e["run_id"] == run_id and e["actor"] == "claude" for e in approvals_logged)
    assert [e["status"] for e in approvals_logged].count("ok") == 3


@pytest.mark.parametrize("actor,kind,grade,second", [("alice", "human", "human", "ROLE_NOT_PERMITTED"), ("limited", "coding_agent", "agent", "ROLE_NOT_PERMITTED")])
async def test_the_approval_grade_and_roles_come_from_the_allowlist_entry_of_the_client(tmp_path, actor, kind, grade, second):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, approvals=True, client_actor=actor)
    async with connect(rt) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        await mcp.ok("run_get", run_id=run_id)
        ok = await mcp.ok("approval_submit", run_id=run_id, gate="G1", role="BA", decision_hash=h1, rationale="fine")
        assert (ok["approval"]["actor_id"], ok["approval"]["actor_kind"], ok["approval"]["approval_grade"]) == (actor, kind, grade)
        h2 = (await mcp.ok("impact_get", run_id=run_id))["gates"]["G2"]["decision_hash"]
        err = await mcp.fail("approval_submit", run_id=run_id, gate="G2", role="SA/Dev", decision_hash=h2, rationale="fine")
        assert err["code"] == second and rt.operational.get_run(run_id).status == "awaiting_approval"


def test_the_caller_cannot_name_its_own_actor(tmp_path):
    rt = make_runtime(tmp_path, approvals=True, client_actor="limited")
    method = McpTools(rt, McpPolicy.from_runtime(rt)).approval_submit
    assert list(inspect.signature(method).parameters) == ["run_id", "gate", "role", "decision_hash", "rationale", "advance"]


async def test_the_actor_argument_is_not_accepted_by_the_protocol(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, approvals=True, client_actor="limited")
    async with connect(rt) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        await mcp.ok("run_get", run_id=run_id)
        ok = await mcp.ok("approval_submit", run_id=run_id, gate="G1", role="BA", decision_hash=h1, rationale="fine", actor_id="claude")
        assert ok["approval"]["actor_id"] == "limited", "unknown arguments never choose the actor"


@pytest.mark.parametrize("build,flags", [({"client_actor": "mallory"}, {"write": True}), ({}, {"write": True}), ({"client_actor": ""}, {"write": True})])
async def test_clients_that_cannot_approve_get_no_receipts(tmp_path, build, flags):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, **flags)
    async with connect(rt, **build) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        for tool in ("run_get", "impact_get", "readiness_get"):
            assert (await mcp.ok(tool, run_id=run_id))["review_receipts"] == []
        assert rt.operational.find_receipt(run_id, "mallory", h1) is None and rt.operational.find_receipt(run_id, "claude", h1) is None
        assert rt.orchestrator.get_run(run_id).status == "awaiting_approval"


async def test_an_allowlisted_client_gets_receipts_even_when_the_approval_tool_is_off(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, client_actor="claude")
    async with connect(rt) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        await mcp.ok("run_get", run_id=run_id)
        assert "approval_submit" not in await mcp.tool_names()
    assert rt.operational.find_receipt(run_id, "claude", h1) is not None


async def test_later_pages_of_run_get_do_not_record_receipts_for_decisions_they_do_not_show(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, client_actor="claude", max_result_chars=1200)
    async with connect(rt) as mcp:
        run_id, h1 = await started_run(mcp, rt)
        late = await mcp.ok("run_get", run_id=run_id, offset=1)
        assert "G1" not in late["tasks"] and late["review_receipts"] == []
        assert rt.operational.find_receipt(run_id, "claude", h1) is None
        first = await mcp.ok("run_get", run_id=run_id)
        assert "G1" in first["tasks"] and first["tasks"]["G1"]["decision_hash"] == h1 and first["truncated"] is True
        assert rt.operational.find_receipt(run_id, "claude", h1) is not None


async def test_a_gate_that_is_not_awaiting_approval_yields_no_receipt(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, client_actor="claude")
    async with connect(rt) as mcp:
        run_id, _ = await started_run(mcp, rt)
        body = await mcp.ok("impact_get", run_id=run_id)
        assert body["gates"]["G2"] == {"status": "not_started"} and body["review_receipts"] == []
        assert (await mcp.fail("impact_get", run_id="run-missing"))["code"] == "RUN_NOT_FOUND"


# ====================================================================== observability
async def test_every_call_produces_a_tool_span_and_a_log_event_with_snapshot_status_and_duration(tmp_path, events):
    rt = make_runtime(tmp_path)
    head = rt.knowledge.head().id
    calls = [("kg_get_element", {"id": "CS-1"}), ("kg_neighbors", {"ids": ["CS-1"]}), ("kg_resolve_scope", {"seeds": ["CS-1"]}),
             ("kg_query", {"name": "Q4"}), ("kg_snapshots", {}), ("source_get", {"id": "SRS-DOC"}), ("run_list", {}), ("metrics_get", {}),
             ("kg_get_element", {"id": "MISSING"})]
    async with connect(rt) as mcp:
        for tool, args in calls:
            await mcp(tool, **args)
    spans = tool_spans(rt)
    assert [s["name"] for s in spans] == [f"mcp:{t}" for t, _ in calls]
    assert [s["status"] for s in spans] == ["ok"] * 8 + ["error"] and spans[-1]["attributes"]["error_code"] == "ELEMENT_NOT_FOUND"
    for span in spans:
        attrs = span["attributes"]
        assert attrs["actor"] == "anonymous" and attrs["transport"] == "stdio" and attrs["snapshot"] == head and span["duration_ms"] >= 0
        assert attrs["result_chars"] > 0 and "arguments" in attrs
    logged = events.named("mcp_tool_call")
    assert [(e["tool"], e["status"]) for e in logged] == [(t, "ok") for t, _ in calls[:-1]] + [("kg_get_element", "error")]
    first = logged[0]
    assert first["snapshot"] == head and first["duration_ms"] >= 0 and first["span_id"] and first["arguments"] == {"id": "CS-1"}
    assert logged[-1]["error_code"] == "ELEMENT_NOT_FOUND" and logged[-1]["level"] == "WARNING"


async def test_calls_bound_to_a_run_are_traced_under_that_run(tmp_path, events):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True, client_actor="claude")
    async with connect(rt) as mcp:
        run_id, _ = await started_run(mcp, rt)
        await mcp.ok("run_get", run_id=run_id)
        trace_id = rt.operational.get_run(run_id).trace_id
    by_name = {s["name"]: s for s in tool_spans(rt)}
    assert by_name["mcp:run_get"]["run_id"] == run_id and by_name["mcp:run_get"]["trace_id"] == trace_id
    assert by_name["mcp:run_resume"]["run_id"] == run_id and by_name["mcp:run_resume"]["trace_id"] == trace_id
    assert by_name["mcp:run_start"]["attributes"]["run_id"] == run_id
    gate_spans = [s for s in rt.operational.list_spans(trace_id=trace_id) if s["kind"] == "gate"]
    assert gate_spans and all(s["parent_id"] is not None for s in gate_spans)
    resume = by_name["mcp:run_resume"]
    run_span = next(s for s in rt.operational.list_spans(trace_id=trace_id) if s["kind"] == "run")
    assert run_span["parent_id"] == resume["span_id"], "the run's spans nest under the MCP call that advanced it"
    by_tool = {e["tool"]: e for e in events.named("mcp_tool_call")}
    assert by_tool["run_get"]["run_id"] == run_id and by_tool["run_resume"]["run_id"] == run_id and by_tool["run_start"]["run_id"] == run_id


async def test_arguments_are_abridged_and_secrets_never_reach_logs_or_spans(tmp_path, events):
    rt = make_runtime(tmp_path, flows={"cr": cr_flow()}, write=True)
    secret = "supersecretvalue123456"
    long_text = "L" * 5000
    async with connect(rt) as mcp:
        await mcp.fail("kg_query", name="Q4", args={"token": secret, "api_key": secret})
        await mcp.ok("run_start", flow="cr", request={"note": long_text, "password": secret},
                     sources=[{"id": "S", "revision": "1", "role": "SRS", "text": "document body that must not be logged"}])
    blob = json.dumps(events.items, default=str) + json.dumps(rt.operational.list_spans(), default=str)
    assert secret not in blob and "document body that must not be logged" not in blob and long_text not in blob
    assert "***" in blob


async def test_unexpected_exceptions_become_internal_errors_without_a_traceback(tmp_path, events):
    rt = make_runtime(tmp_path)
    tools = McpTools(rt, McpPolicy.from_runtime(rt))
    rt.knowledge.get_element = lambda *a, **k: (_ for _ in ()).throw(RuntimeError("disk exploded at /secret/path"))  # type: ignore[method-assign]
    from tdd_agent.mcp.server import ToolRunner
    outcome = ToolRunner(rt, tools).execute("kg_get_element", tools.kg_get_element, {"id": "CS-1"})
    assert outcome.is_error and outcome.payload["error"]["code"] == "INTERNAL_ERROR"
    assert "/secret/path" not in json.dumps(outcome.payload) and "Traceback" not in json.dumps(outcome.payload)
    assert events.named("mcp_tool_exception")[0]["error_type"] == "RuntimeError"


async def test_invalid_arguments_are_structured_and_traced(tmp_path, events):
    rt = make_runtime(tmp_path)
    async with connect(rt) as mcp:
        err = await mcp.fail("kg_neighbors", ids="not-a-list")
        assert err["code"] == "INVALID_ARGUMENTS" and err["details"]["problems"][0]["field"] == "ids"
        err = await mcp.fail("no_such_tool")
        assert err["code"] == "TOOL_NOT_FOUND"
    assert [s["status"] for s in tool_spans(rt)] == ["error", "error"]
    assert {e["reason"] for e in events.named("mcp_tool_denied")} == {"invalid_arguments", "unknown_tool"}


async def test_tool_results_never_contain_configuration(tmp_path):
    rt = make_runtime(tmp_path, write=True, token_env="MY_SECRET_ENV_NAME")
    async with connect(rt) as mcp:
        blobs = [json.dumps(await mcp.ok(t, **a)) for t, a in (("kg_snapshots", {}), ("metrics_get", {}), ("run_list", {}))]
        blobs.append(json.dumps([t.model_dump(mode="json") for t in (await mcp.client.list_tools()).tools]))
    assert not any("MY_SECRET_ENV_NAME" in b or str(tmp_path) in b for b in blobs)


# ====================================================================== resources
async def test_resources_are_readable_and_errors_are_not_silent(tmp_path):
    rt = make_runtime(tmp_path, flows={"cr": trivial_flow()})
    rt.orchestrator.start("cr", {}, run_id="run-1")
    rt.orchestrator.advance("run-1")
    head = rt.knowledge.head().id
    async with Client(build_server(rt)) as client:
        templates = {t.uri_template for t in (await client.list_resource_templates()).resource_templates}
        assert templates == {"tdd://snapshot/{id}", "tdd://source/{id}", "tdd://run/{id}"}
        snap = json.loads((await client.read_resource(f"tdd://snapshot/{head}")).contents[0].text)
        assert snap["snapshot"] == head and snap["info"]["hash"] == rt.knowledge.head().hash and snap["stats"]["elements"] == 42
        src = json.loads((await client.read_resource("tdd://source/TDD-A")).contents[0].text)
        assert src["source"]["role"] == "TDD" and src["text"] == "# TDD rev 5" and src["revisions"][0]["revision"] == "5"
        run = json.loads((await client.read_resource("tdd://run/run-1")).contents[0].text)
        assert run["run"]["status"] == "completed" and run["run_id"] == "run-1" and "G0" in run["tasks"]
        for uri in ("tdd://snapshot/snap-9", "tdd://source/NOPE", "tdd://run/nope"):
            with pytest.raises(MCPError) as exc:
                await client.read_resource(uri)
            assert "NOT_FOUND" in str(exc.value)
    names = [s["name"] for s in tool_spans(rt)]
    assert names[:3] == ["mcp:resource_snapshot", "mcp:resource_source", "mcp:resource_run"] and len(names) == 6


# ====================================================================== stdio transport
STDIO_SCRIPT = """
import sys
sys.path[:0] = [{tests!r}, {src!r}]
from pathlib import Path
from test_mcp_support import make_runtime
from tdd_agent.mcp import serve
rt = make_runtime(Path({work!r}), approvals=True, client_actor="claude")
serve(rt)
"""


async def test_stdio_is_the_default_transport_and_keeps_stdout_clean_for_the_protocol(tmp_path):
    import sys
    from pathlib import Path
    from mcp import StdioServerParameters
    here = Path(__file__).resolve().parent
    script = STDIO_SCRIPT.format(tests=str(here), src=str(here.parent / "src"), work=str(tmp_path))
    params = StdioServerParameters(command=sys.executable, args=["-c", script])
    async with Client(params) as client:
        names = [t.name for t in (await client.list_tools()).tools]
        assert names == READ_TOOLS + ["approval_submit"]
        result = await client.call_tool("kg_get_element", {"id": "CS-1"})
        assert not result.is_error and result.structured_content["element"]["id"] == "CS-1"
        assert (await client.call_tool("kg_get_element", {"id": "NOPE"})).structured_content["error"]["code"] == "ELEMENT_NOT_FOUND"


async def test_kg_search_proposes_elements_and_says_which_ones_are_verified(tmp_path):
    rt = make_runtime(tmp_path)
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_search", query="op-2")
        assert body["snapshot"] == rt.knowledge.head().id and body["hits"][0]["id"] == "OP-2" and body["hits"][0]["class"] == "Operation"
        assert all(h["verified"] is True or h.get("rejected") for h in body["hits"]) and body["truncated"] is False
        one = await mcp.ok("kg_search", query="OP", classes=["Operation"], limit=1)
        assert len(one["hits"]) == 1 and one["hits"][0]["class"] == "Operation"
        assert (await mcp.fail("kg_search", query="  "))["code"] == "INVALID_ARGUMENT"
        assert isinstance((await mcp.ok("kg_search", query='a" OR (b'))["hits"], list), "a query is text, never search syntax"
        assert (await mcp.fail("kg_search", query="OP", snapshot="snap-999999"))["code"] == "SNAPSHOT_NOT_FOUND"
        rt.knowledge.supports_search = False
        assert (await mcp.fail("kg_search", query="OP"))["code"] == "SEARCH_NOT_SUPPORTED"
