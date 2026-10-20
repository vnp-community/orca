"""T6.4: render the three documents: PAY golden, byte-identical (TST-11), refuse to overwrite hand edits, render old versions."""
import os
import pathlib

import pytest

from testspec_agent.conftest import FIX
from testspec_agent.helpers.pipeline import commit_full
from tsagent.errors import HandEditedError
from tsagent.render import build, render
from tsagent.store import tso as store_tso

pytestmark = pytest.mark.integration
GOLDEN = pathlib.Path(FIX) / "golden" / "pay"


@pytest.fixture
def pay_db(cfg, tmp_db, pay_docs):
    commit_full(cfg, tmp_db, pay_docs, "PAY")
    return tmp_db


def test_golden_pay(cfg, pay_db):
    docs, _ = build(pay_db, cfg, "PAY")
    for _, (name, text) in docs.items():
        gold = GOLDEN / name
        if os.environ.get("UPDATE_GOLDEN") == "1":
            gold.parent.mkdir(parents=True, exist_ok=True)
            gold.write_text(text, encoding="utf-8")
        assert text == gold.read_text(encoding="utf-8"), f"{name} khác golden (UPDATE_GOLDEN=1 để cập nhật có chủ đích)"


def test_byte_identical_across_runs(cfg, tmp_path, pay_docs):
    from testspec_agent.conftest import make_cfg
    from tsagent.store.db import Database
    outs = []
    for i in range(2):
        c = make_cfg(tmp_path / f"run{i}")
        db = Database(c.db)
        db.init_schema()
        commit_full(c, db, pay_docs, "PAY")
        res = render(db, c, "PAY")
        outs.append({os.path.basename(f): open(f, "rb").read() for f in res["files"]})
        db.close()
    assert outs[0] == outs[1] and len(outs[0]) == 3


def test_structure(cfg, pay_db):
    docs, meta = build(pay_db, cfg, "PAY")
    tr, td, tc = (docs[k][1] for k in ("TestRequirements", "TestDesign", "TestCases"))
    assert tr.startswith("---\nspec: TSD-PAY-TestRequirements\n") and "contentHash: " in tr
    for s in ("## 3. Phân tích (tầng A)", "## 4. Test Requirements theo mức", "## 5. Truy vết", "## 6. Khoảng trống mở", "## 7. Báo cáo kiểm"):
        assert s in tr
    assert "Tiêu chí đạt:" in tr and "\n1. " in tr
    assert "## 3. Oracle" in td and "## 5. Mô hình lỗi và tiêm lỗi" in td
    assert "| when |" in tc and "`TORC-PAY-" in tc
    assert "createdAt" not in tr and "2026-" not in tr.split("---", 2)[2]  # no timestamps in the body


def test_hand_edit_rejected_then_force(cfg, pay_db):
    files = render(pay_db, cfg, "PAY")["files"]
    p = next(f for f in files if f.endswith("01-test-requirements.md"))
    render(pay_db, cfg, "PAY")  # nothing changed: writing again is allowed
    with open(p, "a", encoding="utf-8") as fh:
        fh.write("\nsửa tay\n")
    with pytest.raises(HandEditedError):
        render(pay_db, cfg, "PAY")
    render(pay_db, cfg, "PAY", force=True)
    assert "sửa tay" not in open(p, encoding="utf-8").read()


def test_old_version_needs_out_dir(cfg, pay_db, pay_docs, tmp_path):
    from tsagent.store import lifecycle
    pay_db.x("UPDATE tso_lifecycle SET review_status = 'approved'")
    lifecycle.baseline(pay_db, "PAY", "qa")
    g = store_tso.load_spec(pay_db, "PAY")
    store_tso.commit(pay_db, "", g, g.meta["baseline"], ["CR-X"])  # v2 Draft
    with pytest.raises(ValueError):
        render(pay_db, cfg, "PAY", version=1)
    res = render(pay_db, cfg, "PAY", version=1, out_dir=str(tmp_path / "v1"))
    assert res["version"] == 1 and all(f.startswith(str(tmp_path / "v1")) for f in res["files"])
    assert not os.path.exists(os.path.join(cfg.specs_root, "PAY"))
