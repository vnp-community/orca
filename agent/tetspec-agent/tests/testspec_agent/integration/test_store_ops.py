"""T2.4: integrity tied to the right feature, snapshot-restore (TST-16), JSON export read back losslessly (TST-18)."""
import time

import pytest

from testspec_agent.integration.test_store_versions import graph, src
from tsagent.errors import TsagentError
from tsagent.model import SourcePackage, SrcNode
from tsagent.store import lifecycle, ops, source, tso

pytestmark = pytest.mark.integration


def build(tmp_db, feature="PAY"):
    b = src(tmp_db)
    g = graph()
    tso.commit(tmp_db, "", g, b)
    lifecycle.baseline(tmp_db, "PAY", "qa")
    return b


def test_clean_store_has_no_problems(tmp_db):
    build(tmp_db)
    assert ops.check(tmp_db) == []


def test_tampering_is_detected_per_feature(tmp_db):
    build(tmp_db)
    p = SourcePackage("OTHER")
    p.add_node(SrcNode("FR-9", "FunctionalRequirement", {}, {"text": "x"}))
    source.save_baseline(tmp_db, p)
    # simulate an unauthorised edit by dropping the trigger (as if someone edited the DB file with another tool)
    tmp_db.x("DROP TRIGGER trg_tso_test_condition_immutable_upd")
    tmp_db.x("UPDATE tso_test_condition SET statement = 'bị sửa'")
    probs = ops.check(tmp_db)
    assert [(p["feature"], p["rule"]) for p in probs] == [("PAY", "T16")]
    assert ops.check(tmp_db, "OTHER") == []  # PAY's errors do not block another feature


def test_snapshot_restore_roundtrip(tmp_db, tmp_path):
    build(tmp_db)
    t0 = time.time()
    snap = ops.snapshot(tmp_db, str(tmp_path / "snap" / "s1.db"))
    restored = ops.restore(snap["path"], str(tmp_path / "restored.db"))
    assert time.time() - t0 < 300  # NFR-009: RTO under 5 minutes on the sample DB
    assert tso.load_spec(restored, "PAY").get("TCND-FR-001-AC1").attrs["statement"] == "Thanh toán hợp lệ được ghi nhận"
    with pytest.raises(TsagentError, match="not empty"):
        ops.restore(snap["path"], str(tmp_path / "restored.db"))
    with pytest.raises(TsagentError, match="not overwritten"):
        ops.snapshot(tmp_db, snap["path"])


def test_restore_rejects_bad_hash(tmp_db, tmp_path):
    build(tmp_db)
    snap = ops.snapshot(tmp_db, str(tmp_path / "s.db"))
    with open(snap["path"] + ".sha256", "w") as fh:
        fh.write("0" * 64)
    with pytest.raises(TsagentError, match="hash"):
        ops.restore(snap["path"], str(tmp_path / "r.db"))


def test_export_reads_back_without_loss(tmp_db):
    build(tmp_db)
    exp = ops.export_json(tmp_db, "PAY")
    back = ops.graph_from_export(exp)
    orig = tso.load_spec(tmp_db, "PAY")
    assert set(back.entities) == set(orig.entities)
    for c, e in orig.entities.items():
        assert back.get(c).content_hash() == e.content_hash()
        assert back.get(c).prov.evidence == e.prov.evidence


def test_stats(tmp_db):
    build(tmp_db)
    s = ops.stats(tmp_db)
    assert s["tables"]["tso_test_condition"] == 1 and s["features"]["PAY"]["versions"] == [(1, "Baselined")]
