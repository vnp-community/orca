"""T2.2, T2.3: staging, versions, lifecycle, baseline, immutability (behaviours B1-B4, B6 of BASELINE-V1)."""
import sqlite3

import pytest

from tsagent.errors import ImmutableError, NotFound, TsagentError
from tsagent.model import Evidence, Provenance, SourcePackage, SrcNode, TsoEntity, TsoGraph
from tsagent.store import lifecycle, source, tso

pytestmark = pytest.mark.integration
F = "PAY"


def src(tmp_db, text="Ghi nhận thanh toán"):
    p = SourcePackage(F)
    p.add_node(SrcNode("FR-001", "FunctionalRequirement", {"status": "Baselined"}, {"text": text}))
    p.add_node(SrcNode("FR-002", "FunctionalRequirement", {"status": "Baselined"}, {"text": "Xem lịch sử"}))
    p.add_node(SrcNode("DOC-TDD", "Document", {"status": "Baselined"}, {"docType": "TDD"}))
    return source.save_baseline(tmp_db, p)["baseline"]


def graph(statement="Thanh toán hợp lệ được ghi nhận", with_fr2=True, run="r"):
    g = TsoGraph(F)
    P = lambda *b: Provenance("rule", list(b), [Evidence(b[0], "trích")], confidence=0.8, run_id=run)  # noqa: E731
    g.add(TsoEntity("TestBasisItem", "TBI-FR-001", {"tddCode": "FR-001", "tddClass": "FunctionalRequirement", "baselineId": "x",
                                                    "inScope": True}, prov=P("FR-001")))
    g.add(TsoEntity("TestOracle", "TORC-FR-001", {"oracleKind": "acceptanceCriterion", "expected": "201"}, prov=P("FR-001"))
          .link("source", "TBI-FR-001"))
    g.add(TsoEntity("TestObligation", "TOB-FR-001", {"question": "q", "origin": "seedRule", "applicability": "applicable",
                                                     "priority": "P1"}, prov=P("FR-001")).link("subject", "TBI-FR-001"))
    g.add(TsoEntity("TestCondition", "TCND-FR-001-AC1", {"conditionKind": "functional", "statement": statement,
                                                        "expectedEffect": "e", "key": "AC1"}, prov=P("FR-001"))
          .link("obligation", "TOB-FR-001").link("subject", "TBI-FR-001").link("oracle", "TORC-FR-001"))
    if with_fr2:
        g.add(TsoEntity("TestBasisItem", "TBI-FR-002", {"tddCode": "FR-002", "tddClass": "FunctionalRequirement", "baselineId": "x",
                                                        "inScope": True}, prov=P("FR-002")))
    return g


def test_commit_and_load_roundtrip(tmp_db):
    b = src(tmp_db)
    with tmp_db.tx():
        tmp_db.x("insert into run(run_id, feature, run_key, mode, status, created_at, updated_at) values ('r','PAY','k','none','x','t','t')")
        tso.stage_put(tmp_db, "r", "analysis", list(graph().entities.values()))
    staged = tso.stage_load(tmp_db, "r", F)
    assert set(staged.entities) == set(graph().entities)
    st = tso.commit(tmp_db, "r", staged, b)
    assert st["version"] == 1 and len(st["new"]) == 5 and not tso.stage_steps(tmp_db, "r")
    g = tso.load_spec(tmp_db, F)
    cnd = g.get("TCND-FR-001-AC1")
    assert cnd.rel("oracle") == ["TORC-FR-001"] and cnd.attrs["conditionKind"] == "functional"
    assert cnd.prov.based_on == ["FR-001"] and cnd.prov.evidence[0].quote == "trích"
    assert g.get("TBI-FR-001").attrs["inScope"] is True
    assert g.meta["lifecycle"]["TCND-FR-001-AC1"]["status"] == "Draft"


def test_draft_is_rewritten_in_place(tmp_db):
    b = src(tmp_db)
    tso.commit(tmp_db, "", graph(), b)
    st = tso.commit(tmp_db, "", graph("lời mới"), b)
    assert st["version"] == 1 and st["rewroteDraft"]
    assert [v["version"] for v in tso.versions(tmp_db, F)] == [1]
    assert tso.load_spec(tmp_db, F).get("TCND-FR-001-AC1").attrs["statement"] == "lời mới"
    assert tmp_db.one("select count(*) from tso_test_condition")[0] == 1  # no duplicated rows


def test_cr_reuses_unchanged_supersedes_changed_retires_removed(tmp_db):
    b1 = src(tmp_db)
    tso.commit(tmp_db, "", graph(), b1)
    lifecycle.set_review(tmp_db, F, "approved", "qa")
    lifecycle.baseline(tmp_db, F, "qa")
    b2 = src(tmp_db, text="Ghi nhận thanh toán (sửa)")
    st = tso.commit(tmp_db, "", graph("điều kiện đổi", with_fr2=False), b2, changed_by=["CR-1"])
    assert st["version"] == 2 and st["modified"] == ["TCND-FR-001-AC1"] and st["retiring"] == ["TBI-FR-002"]
    assert st["unchanged"] == 3
    res = lifecycle.baseline(tmp_db, F, "qa")
    assert res["retired"] == ["TBI-FR-002"] and res["superseded"] == ["TCND-FR-001-AC1"]
    v1 = tso.load_spec(tmp_db, F, 1)
    assert v1.meta["lifecycle"]["TBI-FR-002"]["status"] == "Retired" and v1.meta["lifecycle"]["TBI-FR-002"]["retired_by"] == "CR-1"
    assert v1.meta["lifecycle"]["TCND-FR-001-AC1"]["status"] == "Superseded"
    v2 = tso.load_spec(tmp_db, F, 2)
    assert v2.meta["nodeVersions"]["TCND-FR-001-AC1"] == 2 and v2.meta["nodeVersions"]["TOB-FR-001"] == 1
    assert [v["status"] for v in tso.versions(tmp_db, F)] == ["Superseded", "Baselined"]
    assert tso.diff_versions(tmp_db, F, 1, 2) == {"from": 1, "to": 2, "added": [], "removed": ["TBI-FR-002"],
                                                  "modified": ["TCND-FR-001-AC1"]}
    h = tso.history(tmp_db, F)
    assert h[1]["modified"] == 1 and h[1]["retired"] == 1


def test_baseline_refuses_retire_without_cr(tmp_db):
    b = src(tmp_db)
    tso.commit(tmp_db, "", graph(), b)
    lifecycle.baseline(tmp_db, F, "qa")
    tso.commit(tmp_db, "", graph(with_fr2=False), b)
    with pytest.raises(ImmutableError, match="retiredBy"):
        lifecycle.baseline(tmp_db, F, "qa")
    assert tso.versions(tmp_db, F)[-1]["status"] == "Draft"  # nothing half-written


def test_baselined_rows_are_immutable_even_via_sql(tmp_db):
    b = src(tmp_db)
    tso.commit(tmp_db, "", graph(), b)
    lifecycle.baseline(tmp_db, F, "qa")
    for sql in ("UPDATE tso_test_condition SET statement = 'x'", "DELETE FROM tso_test_oracle",
                "UPDATE tso_lifecycle SET review_status = 'rejected'", "DELETE FROM tso_spec_version_item",
                "UPDATE tso_spec_version SET graph_hash = 'x'", "DELETE FROM tso_test_obligation__subject",
                "DELETE FROM tso_based_on"):
        with pytest.raises(sqlite3.IntegrityError, match="immutable"):
            tmp_db.x(sql)
    with pytest.raises(sqlite3.IntegrityError):
        tmp_db.x("INSERT INTO tso_spec_version(feature, version, status, base_baseline_id, created_at) VALUES ('PAY', 2, 'Draft', ?, 't')", (b,))
        tmp_db.x("INSERT INTO tso_spec_version(feature, version, status, base_baseline_id, created_at) VALUES ('PAY', 3, 'Draft', ?, 't')", (b,))


def test_failed_commit_leaves_nothing(tmp_db):
    b = src(tmp_db)
    g = graph()
    g.get("TCND-FR-001-AC1").link("partition", "KHONG-CO")
    with pytest.raises(TsagentError, match="not in the version"):
        tso.commit(tmp_db, "", g, b)
    assert tso.versions(tmp_db, F) == [] and tmp_db.one("select count(*) from tso_node")[0] == 0


def test_review_filters(tmp_db):
    b = src(tmp_db)
    g = graph()
    g.get("TCND-FR-001-AC1").prov.derivation = "llm"
    g.get("TCND-FR-001-AC1").prov.confidence = 0.95
    tso.commit(tmp_db, "", g, b)
    assert [r["code"] for r in lifecycle.pending(tmp_db, F, derivation="llm")] == ["TCND-FR-001-AC1"]
    assert lifecycle.set_review(tmp_db, F, "approved", "auto", derivation="llm", min_confidence=0.9)["updated"] == 1
    assert lifecycle.set_review(tmp_db, F, "approved", "qa", classes=["TestOracle"])["updated"] == 1
    assert lifecycle.pending(tmp_db, F, derivation="llm") == []
    with pytest.raises(NotFound):
        lifecycle.set_review(tmp_db, F, "approved", "qa", codes=["KHONG-CO"])


def test_regression_and_introduced(tmp_db):
    b = src(tmp_db)
    tso.commit(tmp_db, "", graph(), b)
    lifecycle.baseline(tmp_db, F, "qa")
    tso.commit(tmp_db, "", graph("đổi"), b, changed_by=["CR-2"], regression=["TOB-FR-001"])
    assert tso.load_spec(tmp_db, F).regression == ["TOB-FR-001"]
    assert tso.introduced_codes(tmp_db, F, 2) == ["TCND-FR-001-AC1"]


def test_draft_rewrite_keeps_earlier_crs(tmp_db, minipay_pkg):
    from tsagent.analyze.seed_core import seed
    from tsagent.ingest.kg_json import load_kg_json
    from tsagent.store import source
    from tsagent.store import tso as store_tso
    pkg, _ = load_kg_json(minipay_pkg, "ST")
    bid = source.save_baseline(tmp_db, pkg)["baseline"]
    g = seed(pkg, bid).g
    store_tso.commit(tmp_db, "", g, bid, ["CR-001"])
    store_tso.commit(tmp_db, "", g, bid, ["CR-AUTO-x"])  # the follow-up pass rewrites the Draft
    assert store_tso.history(tmp_db, "ST")[-1]["changedBy"] == ["CR-001", "CR-AUTO-x"]
