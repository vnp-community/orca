"""T7.2: TS1 reads the design KG over MCP (in-memory client and real stdio)."""
from pathlib import Path

import pytest
from mcp import Client
from test_mcp_support import make_runtime
from testspec_graph import CR_PARAMS, design_graph

from tdd_agent.agents.testspec import kgclient
from tdd_agent.agents.testspec.mapping import load_mapping
from tdd_agent.agents.testspec.project import build_package, dumps
from tdd_agent.agents.testspec.scope import cr_scopes, tdd_scope
from tdd_agent.config import load_config
from tdd_agent.mcp import build_server


@pytest.fixture()
def served(tmp_path, monkeypatch):
    """A real tdd-agent MCP server on a KG with the design_graph snapshot; TS1 connects through an in-memory client."""
    rt = make_runtime(tmp_path, load=False)
    snap = rt.knowledge.commit_graph(None, design_graph(), actor="test", validate=False).snapshot
    monkeypatch.setattr(kgclient, "CLIENT_FACTORY", lambda cfg: Client(build_server(rt)))
    return rt, snap


def _pkg(graph, scope):
    return dumps(build_package(graph, scope, cr="CR-1", generated_at="t", source_hash="h")[0])


def test_cr_subgraph_projects_like_the_full_snapshot(served):
    rt, snap = served
    m = load_mapping()
    read = kgclient.read_graph(rt.config, "cr", {**CR_PARAMS, "snapshot": snap.id}, m)
    assert read["info"] == {"id": snap.id, "seq": snap.seq, "hash": snap.hash, "created_at": snap.created_at, "graph_version": snap.graph_version}
    assert read["calls"] >= 3 and len(read["graph"]["elements"]) < len(design_graph()["elements"])
    (sub,) = cr_scopes(read["graph"], CR_PARAMS, m)
    (full,) = cr_scopes(design_graph(), CR_PARAMS, m)
    assert sub.targets == full.targets and sub.context == full.context
    assert _pkg(read["graph"], sub) == _pkg(design_graph(), full)


def test_tdd_subgraph_projects_like_the_full_snapshot(served):
    rt, _ = served
    m = load_mapping()
    read = kgclient.read_graph(rt.config, "tdd", {"tdd": "TDD-A"}, m)          # no snapshot given: head
    sub = tdd_scope(read["graph"], "TDD-A", m, members_ids=read["members"])
    full = tdd_scope(design_graph(), "TDD-A", m)
    assert sub.targets == full.targets and sub.context == full.context
    assert _pkg(read["graph"], sub) == _pkg(design_graph(), full)


def test_unknown_snapshot_is_reported(served):
    rt, _ = served
    with pytest.raises(Exception) as e:
        kgclient.read_graph(rt.config, "cr", {**CR_PARAMS, "snapshot": "snap-999999"}, load_mapping())
    assert getattr(e.value, "code", "") == "SNAPSHOT_NOT_FOUND"


def test_connection_failure_and_missing_token(tmp_path, monkeypatch):
    def broken(cfg):
        raise ConnectionRefusedError("no server")
    monkeypatch.setattr(kgclient, "CLIENT_FACTORY", broken)
    cfg = load_config(env={})
    with pytest.raises(kgclient.KgMcpError) as e:
        kgclient.read_graph(cfg, "tdd", {"tdd": "TDD-A"}, load_mapping())
    assert e.value.code == "TESTSPEC_KG_MCP" and "ConnectionRefusedError" in str(e.value)
    monkeypatch.setattr(kgclient, "CLIENT_FACTORY", None)
    monkeypatch.delenv("TDD_AGENT_MCP_TOKEN", raising=False)
    cfg = load_config(env={}, overrides={"testspec": {"kg_mcp": "http://127.0.0.1:9/mcp"}})
    with pytest.raises(kgclient.KgMcpError) as e:
        kgclient.read_graph(cfg, "tdd", {"tdd": "TDD-A"}, load_mapping())
    assert e.value.code == "TESTSPEC_KG_MCP_TOKEN"


def test_reads_over_a_real_stdio_server(tmp_path, monkeypatch):
    """Real SQLite project: TS1 runs `tdd-agent -C <project> mcp serve --transport stdio` as a child process."""
    from tdd_agent.project import init_project
    from tdd_agent.runtime import Runtime
    monkeypatch.setattr(kgclient, "CLIENT_FACTORY", None)
    root = tmp_path / "proj"
    init_project(root, name="P")
    cfg = load_config(root / ".tdd-agent" / "config.toml", base_dir=root, env={})
    rt = Runtime.build(cfg, configure_logging=False)
    snap = rt.knowledge.commit_graph(None, design_graph(), actor="test", validate=False).snapshot
    rt.close()
    read = kgclient.read_graph(cfg, "tdd", {"tdd": "TDD-A"}, load_mapping())
    assert read["info"]["id"] == snap.id and "FR-3" in read["members"]
    assert Path(root / ".tdd-agent" / "knowledge.db").is_file()
