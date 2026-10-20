"""T3.4: delta, affected set, implicit CR (TST-04, failure scenario 4.1 of v1)."""
import copy

import pytest

from tsagent.delta import diff, impact, prepare_change
from tsagent.ingest.kg_json import load_kg_json
from tsagent.store import source

pytestmark = pytest.mark.unit


def pkg_of(obj):
    p, diags = load_kg_json(obj)
    assert p is not None, diags
    return p


def changed_copy(obj, fn):
    o = copy.deepcopy(obj)
    fn(o)
    return o


def edit_text(o, code, text):
    next(n for n in o["nodes"] if n["code"] == code)["props"]["text"] = text


def drop(o, code):
    o["nodes"] = [n for n in o["nodes"] if n["code"] != code]
    o["edges"] = [e for e in o["edges"] if code not in (e[0], e[2])]


def test_added_modified_removed(minipay_pkg):
    a = pkg_of(minipay_pkg)
    b = pkg_of(changed_copy(minipay_pkg, lambda o: (edit_text(o, "FR-046", "mới"), drop(o, "FR-049"),
                                                     o["nodes"].append({"code": "FR-099", "class": "FunctionalRequirement",
                                                                        "props": {"text": "x"}}))))
    d = diff(a, b)
    assert d.added == {"FR-099"} and "FR-046" in d.modified and d.removed == {"FR-049"}


def test_supersede_rename_is_modification(minipay_pkg):
    def f(o):
        n = next(n for n in o["nodes"] if n["code"] == "FR-046")
        n["code"] = "FR-046-v2"
        for e in o["edges"]:
            e[0] = "FR-046-v2" if e[0] == "FR-046" else e[0]
            e[2] = "FR-046-v2" if e[2] == "FR-046" else e[2]
        o["supersede"] = [{"old": "FR-046", "new": "FR-046-v2"}]
    d = diff(pkg_of(minipay_pkg), pkg_of(changed_copy(minipay_pkg, f)))
    assert "FR-046-v2" in d.modified and "FR-046" not in d.removed and d.renamed == {"FR-046-v2": "FR-046"}


def test_impact_reaches_neighbours(minipay_pkg):
    a = pkg_of(minipay_pkg)
    b = pkg_of(changed_copy(minipay_pkg, lambda o: edit_text(o, "FR-046", "mới")))
    imp = impact(b, a, diff(a, b).changed)
    assert {"FR-046", "MOD-Scheduler", "SCN-ScheduledRun"} <= imp
    assert all(b.get(c).cls not in ("Document", "Mechanism") for c in imp if b.get(c))


def test_ingest_first_then_generate_keeps_implicit_cr(tmp_db, minipay_full):
    # the full package has no ChangeRequest node: a change without a CR must produce an implicit CR
    source.save_baseline(tmp_db, pkg_of(minipay_full))
    changed = changed_copy(minipay_full, lambda o: drop(o, "FR-049"))
    source.save_baseline(tmp_db, pkg_of(changed))            # `kg ingest` stores the new baseline first
    res = prepare_change(tmp_db, pkg_of(changed))             # then `gen`
    assert res["reused"] and res["delta"].removed == {"FR-049"}
    cr = res["crs"][0]
    assert cr.startswith("CR-AUTO-") and res["source"].get(cr).props["removes"] == ["FR-049"]
    again = prepare_change(tmp_db, pkg_of(changed))           # rerun: the CR is still there, no extra one
    assert again["crs"] == [cr]


def test_declared_cr_is_used(minipay_pkg, tmp_db):
    source.save_baseline(tmp_db, pkg_of(minipay_pkg))
    def f(o):
        edit_text(o, "FR-046", "mới")
        o["nodes"].append({"code": "CR-009", "class": "ChangeRequest", "props": {"description": "đổi FR-046"}})
        o["edges"].append(["CR-009", "affects", "FR-046", {}])
    res = prepare_change(tmp_db, pkg_of(changed_copy(minipay_pkg, f)))
    assert res["crs"] == ["CR-009"] and res["delta"].declared["CR-009"]["affects"] == ["FR-046"]
