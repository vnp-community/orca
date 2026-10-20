"""T2.1: source API: store/read round trip, immutability, implicit CR, neighbours."""
import sqlite3

import pytest

from tsagent.model import OntologyDecl, SourcePackage, SrcNode
from tsagent.store import source

pytestmark = pytest.mark.unit
BASE = ("label", "version", "status", "sourceDoc", "sourceSection", "owner", "description", "note", "testScope", "primary")


def pkg_from_json(d, feature=None):
    """Turn a tdd-kg/v1 package into a SourcePackage without checks (the real loader is in T3.1)."""
    p = SourcePackage(feature or d["feature"])
    p.ontology = OntologyDecl({c["name"]: {k: v for k, v in c.items() if k != "name"} for c in d["ontology"]["classes"]},
                              {r["name"]: {k: v for k, v in r.items() if k != "name"} for r in d["ontology"]["relations"]})
    for n in d["nodes"]:
        p.add_node(SrcNode(n["code"], n["class"], {k: n[k] for k in BASE if k in n}, dict(n.get("props", {}))))
    for s, r, t, props in d["edges"]:
        p.add_edge(s, r, t, props)
    p.supersede = list(d.get("supersede", []))
    p.meta = {"format": d["format"], "ontologyVersion": d["ontologyVersion"]}
    return p


def test_roundtrip_minipay_keeps_everything(tmp_db, minipay_pkg):
    p = pkg_from_json(minipay_pkg)
    res = source.save_baseline(tmp_db, p)
    assert res["reused"] is False and res["nodes"] == len(minipay_pkg["nodes"])
    back = source.load_source(tmp_db, p.feature)
    assert set(back.nodes) == set(p.nodes)
    for code, n in p.nodes.items():
        assert back.nodes[code].content_hash() == n.content_hash(), code
    assert back.edges == p.edges
    assert back.content_hash() == p.content_hash()
    assert back.ontology.relations["hasErrorMapping"]["range"] == ["ErrorMapping"]
    scn = back.get("SCN-ScheduledRun")
    assert isinstance(scn.props.get("responseBudgetMs"), int)


def test_same_content_is_reused(tmp_db, minipay_pkg):
    a = source.save_baseline(tmp_db, pkg_from_json(minipay_pkg))
    b = source.save_baseline(tmp_db, pkg_from_json(minipay_pkg))
    assert b["reused"] and a["baseline"] == b["baseline"]
    assert len(source.list_baselines(tmp_db, minipay_pkg["feature"])) == 1


def test_changed_node_supersedes_previous_baseline(tmp_db, minipay_pkg):
    source.save_baseline(tmp_db, pkg_from_json(minipay_pkg))
    p2 = pkg_from_json(minipay_pkg)
    p2.get("FR-046").props["text"] = "đổi nội dung"
    b2 = source.save_baseline(tmp_db, p2)["baseline"]
    rows = tmp_db.q("SELECT code, supersedes_node_id FROM src_node WHERE baseline_id = ? AND supersedes_node_id IS NOT NULL", (b2,))
    assert [r["code"] for r in rows] == ["FR-046"]


def test_source_tables_are_immutable(tmp_db, minipay_pkg):
    source.save_baseline(tmp_db, pkg_from_json(minipay_pkg))
    for sql in ("UPDATE src_node SET props = '{}'", "DELETE FROM src_edge", "UPDATE src_baseline SET ordinal = 9",
                "DELETE FROM src_onto_class"):
        with pytest.raises(sqlite3.IntegrityError, match="immutable"):
            tmp_db.x(sql)


def test_derived_cr_is_stored_even_when_baseline_exists(tmp_db, minipay_pkg):
    p = pkg_from_json(minipay_pkg)
    source.save_baseline(tmp_db, p)  # the `kg ingest` step stores the baseline first
    cr = SrcNode("CR-AUTO-x", "ChangeRequest", {"status": "Baselined"}, {"description": "CR ngầm định", "changeKind": "implicit"})
    res = source.save_baseline(tmp_db, pkg_from_json(minipay_pkg),
                               derived=[{"node": cr, "edges": [["CR-AUTO-x", "affects", "FR-046"]], "reason": "nguồn đổi không có CR"}])
    assert res["reused"] and res["derived"] == 1
    back = source.load_source(tmp_db, p.feature)
    assert back.get("CR-AUTO-x").cls == "ChangeRequest"
    assert [n.code for n in back.targets("CR-AUTO-x", "affects")] == ["FR-046"]


def test_neighbors_with_recursive_cte(tmp_db, minipay_pkg):
    p = pkg_from_json(minipay_pkg)
    bid = source.save_baseline(tmp_db, p)["baseline"]
    one = source.neighbors(tmp_db, bid, "FR-046", depth=1, direction="out")
    codes = {n["code"] for n in one["nodes"]}
    assert "MOD-Scheduler" in codes and {"src": "FR-046", "rel": "allocatedTo", "dst": "MOD-Scheduler"} in one["edges"]
    two = source.neighbors(tmp_db, bid, "FR-046", depth=2)
    assert len(two["nodes"]) > len(one["nodes"])


def test_get_node(tmp_db, minipay_pkg):
    bid = source.save_baseline(tmp_db, pkg_from_json(minipay_pkg))["baseline"]
    assert source.get_node(tmp_db, bid, "FR-046").cls == "FunctionalRequirement"
    with pytest.raises(Exception, match="no node"):
        source.get_node(tmp_db, bid, "NOPE")
