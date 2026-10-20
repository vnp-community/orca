"""T7.1: the multi-element read tool `kg_elements` (by ids, by TDD, by class, pagination)."""
import pytest
from test_mcp_support import anyio_backend, connect, make_runtime  # noqa: F401
from testspec_graph import design_graph

pytestmark = pytest.mark.anyio


def runtime(tmp_path, graph=None, **mcp):
    rt = make_runtime(tmp_path, load=False, **mcp)
    snap = rt.knowledge.commit_graph(None, graph or design_graph(), actor="test", validate=False).snapshot
    return rt, snap


async def test_by_ids_with_missing_and_snapshot_info(tmp_path):
    rt, snap = runtime(tmp_path)
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_elements", ids=["SVC-N-01", "FR-3", "NOPE", "FR-3"], snapshot=snap.id)
    assert [e["id"] for e in body["elements"]] == ["FR-3", "SVC-N-01"] and body["missing"] == ["NOPE"]
    assert body["info"]["id"] == snap.id and body["info"]["hash"] == snap.hash and body["info"]["created_at"]
    assert body["truncated"] is False


async def test_by_tdd_and_classes(tmp_path):
    rt, _ = runtime(tmp_path)
    async with connect(rt) as mcp:
        a = await mcp.ok("kg_elements", tdd="TDD-B")
        b = await mcp.ok("kg_elements", tdd="TDD-A", classes=["Requirement", "QualityTarget"])
        err = await mcp.fail("kg_elements")
    assert [e["id"] for e in a["elements"]] == ["FR-9"] and a["membership"] == "tdd"
    assert [e["id"] for e in b["elements"]] == ["FR-3", "NFR-1"]
    assert err["code"] == "INVALID_ARGUMENT"


async def test_one_tdd_project_returns_whole_kg(tmp_path):
    g = design_graph()
    for e in g["elements"]:
        e["tdds"] = []
    rt, _ = runtime(tmp_path, g)
    async with connect(rt) as mcp:
        body = await mcp.ok("kg_elements", tdd="TDD-P")
    assert body["membership"] == "whole_kg" and len(body["elements"]) == len(g["elements"])


async def test_pages_with_next(tmp_path):
    rt, _ = runtime(tmp_path, max_result_chars=3000)
    async with connect(rt) as mcp:
        seen, offset = [], 0
        while offset is not None:
            body = await mcp.ok("kg_elements", tdd="TDD-A", offset=offset)
            seen += [e["id"] for e in body["elements"]]
            offset = body["next"]
    assert len(seen) == len(set(seen)) and len(seen) > 5
