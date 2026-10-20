"""The author and the critic get a scoped pack: the change, the facts it points at and their nearest neighbours, within a budget."""
from __future__ import annotations

import json

from tdd_agent.agents.authoring import scoped_pack


def pack(n=600, width=400):
    els = [{"id": f"E{i}", "class": "Entity", "name": "x" * width, "props": {}} for i in range(n)]
    return {"request": {"workflow_context": {"big": "y" * 5000}, "intent": "i"}, "repo": {"contents": {}}, "graph": {"elements": els, "edges": [{"src": f"E{i}", "rel": "traces_to", "dst": f"E{i + 1}"} for i in range(n - 1)], "gaps": [
        {"id": "G1", "target": "E10", "need": "definition"}, {"id": "G2", "target": "E599", "need": "definition"}]},
        "design": els, "versions": [], "scenarios": [], "work": [], "tests": [], "provenance": {e["id"]: [{"quote": "q"}] for e in els},
        "impact": {"changed": ["E10"], "affected": [], "truncated": False}, "deltas": [], "workflow_context": {"cs": 1},
        "canonical_change_spec": {"items": [{"srs_refs": [{"section_id": "E10"}], "tdd_refs": [{"section_id": "E300"}, {"section_id": "NOT-IN-KG"}]}]}}


def test_a_small_pack_is_passed_unchanged():
    p = pack(n=5, width=10)
    shown, rec = scoped_pack(p)
    assert shown is p and rec["scoped"] is False


def test_a_large_pack_is_cut_to_the_budget_around_what_the_change_points_at_and_the_cut_is_recorded():
    p = pack()
    shown, rec = scoped_pack(p, budget=120_000)
    ids = {e["id"] for e in shown["graph"]["elements"]}
    assert {"E10", "E300"} <= ids, "the elements the change spec points at are always shown first"
    assert "E11" in ids or "E9" in ids, "then their nearest neighbours"
    assert rec["scoped"] and rec["truncated"] and rec["included"] == len(ids) < rec["of"] == 600
    assert len(json.dumps(shown, ensure_ascii=False)) <= 120_000 * 1.05
    assert shown["pack_scope"]["truncated"] is True and "cite only facts" in shown["pack_scope"]["note"]
    assert all(e["src"] in ids and e["dst"] in ids for e in shown["graph"]["edges"]), "edges only between shown facts"
    assert [g["id"] for g in shown["graph"]["gaps"]] == ["G1"], "a gap is shown only when its target is shown"
    assert set(shown["provenance"]) == ids and {e["id"] for e in shown["design"]} == ids
    assert "workflow_context" not in shown["request"] and shown["workflow_context"] == {"cs": 1}, "the context is not sent twice"


def test_scoping_is_deterministic():
    a, _ = scoped_pack(pack(), budget=120_000)
    b, _ = scoped_pack(pack(), budget=120_000)
    assert json.dumps(a, sort_keys=True) == json.dumps(b, sort_keys=True)
