import json

import pytest

from harness import doc, make, prompt_blocks, props_items
from tdd_agent.agents.flows import default_flows
from tdd_agent.store.base import SourceDoc

SRS_PROSE = "# SRS\n## 1. Overview\n| ID | Statement |\n|---|---|\n| FR-1 | The system MUST store evidence. |\n## 2. Behaviour\nThe SBOM upload MUST time out after 2 minutes.\nThe notice shall be retried 3 times if delivery fails.\nThe audit log must keep 7 days of data.\nPlain text without any obligation here.\n"


def docs(srs=SRS_PROSE):
    return [doc("urd", "URD"), SourceDoc.make("SRS", "1", "SRS", srs), doc("sad", "SAD")]


def fact_handler(req, n):
    skill = req.metadata.get("skill")
    if skill == "extract-facts-from-prose":
        out = []
        for blk, text in prompt_blocks(req.prompt):
            if "time out" in text:
                out.append({"block": blk, "class": "Constraint", "name": "upload timeout", "props_items": props_items({"statement": "upload times out", "timeout_seconds": 120}), "quote": "MUST time out after 2 minutes"})
            if "retried" in text:
                out.append({"block": blk, "class": "Constraint", "name": "notice retry", "props_items": props_items({"statement": "retry delivery", "retries": 3}), "quote": "retried 3 times"})
            if "audit log" in text:
                out.append({"block": blk, "class": "Constraint", "name": "bogus", "props_items": props_items({"statement": "x", "limit": 99}), "quote": "not in the line"})
        return {"facts": out}
    if skill == "resolve-entity-match":
        return {"same": False, "reason": "different"}
    raise AssertionError(skill)


def run_gen(o, sources, **params):
    o.start("tdd-generation", {"mode": params.pop("mode", "initial"), "params": {"tdd_id": "TDD-X", **params}}, run_id="g1", sources=sources)
    return o.advance("g1")


def test_generation_builds_typed_kg_with_verified_prose_facts_gaps_and_questions(tmp_path):
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run = run_gen(o, docs())
    assert run.status == "completed"
    out = ops.get_task("g1", "G3")["output"]
    assert out["status"] == "blocked" and out["tdd"] == {"id": "TDD-X", "revision": "1", "content_hash": know.get_source("TDD-X").content_hash}
    g = know.materialize()
    facts = [e for e in g["elements"] if e["id"].startswith("FACT-")]
    assert {e["name"] for e in facts} == {"upload timeout", "notice retry"} and {e["props"]["timeout_seconds"] for e in facts if "timeout_seconds" in e["props"]} == {120}
    o5 = __import__("tdd_agent.core.o5_path", fromlist=["load"]).load()
    srs_text = docs()[1].text
    for e in facts:   # every accepted fact is anchored to the exact source line
        assert o5.verify_evidence(e["provenance"][0], [{"id": "SRS", "version": "1", "content": srs_text}]) == []
        assert e["review_status"] == "needs_review" and e["maturity"] == "sketch" and "TDD-X" in e["tdds"]
    assert any(gp["need"] == "typed_fact" and "QUOTE_NOT_IN_LINE" in gp["reason"] for gp in g["gaps"]), "the model's unverifiable fact became a gap"
    assert out["questions"] and all(q["owner"] and q["question"] for q in out["questions"])
    text = know.get_source("TDD-X").text
    assert '<a id="FR-1"></a>' in text and "upload timeout" in text, "standard document: every element has an anchor"
    assert (tmp_path / "work" / "runs" / "g1" / "TDD-X.r1.md").read_text() == text
    assert {"G3"} == {s["name"] for s in ops.list_spans(run_id="g1") if s["kind"] == "gate"}


def test_prose_can_be_switched_off_and_truncation_is_a_gap(tmp_path):
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, docs(), prose=False)
    assert not [e for e in know.materialize()["elements"] if e["id"].startswith("FACT-")] and not [c for c in provider.calls if c.metadata.get("skill") == "extract-facts-from-prose"]
    o2, know2, ops2, p2 = make(default_flows(), fact_handler, tmp_path / "b")
    run_gen(o2, docs(), prose_limit=1)
    assert any(g["need"] == "prose_extraction" for g in know2.materialize()["gaps"])


@pytest.mark.parametrize("sources,mode,extra,blocker", [
    ([doc("urd", "URD"), doc("srs", "SRS")], "initial", {}, "SOURCE_ROLE_MISSING:SAD"),
    (docs(), "defect", {}, "DEFECT_EXPECTED_BEHAVIOR_REQUIRED"),
    (docs(), "change-request", {}, "BASE_TDD_SNAPSHOT_REQUIRED"),
    (docs(), "change-request", {"base_snapshot": "snap-000009"}, "BASE_SNAPSHOT_STALE"),
])
def test_generation_blocks_instead_of_guessing(tmp_path, sources, mode, extra, blocker):
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run = run_gen(o, sources, mode=mode, **extra)
    assert run.status == "blocked" and blocker in run.result["blockers"], run.result
    assert know.head() is None and provider.calls == []


def test_not_applicable_needs_require_reason_and_evidence(tmp_path):
    o, know, ops, p = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, docs(), prose=False, not_applicable={"data": {"reason": "no persistent data", "evidence": [{"kind": "proposed", "decision_id": "DEC-1"}]}, "work": {"reason": "no reason evidence"}})
    reports = {r["need_id"]: r for r in ops.get_task("g1", "G3")["evidence"]["intake"]}
    assert reports["data"]["status"] == "not_applicable" and reports["work"]["status"] == "missing", "N/A without evidence stays missing"


def test_review_then_regeneration_keeps_reviewed_facts(tmp_path):
    o, know, ops, p = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, docs(), prose=False)
    ids = [e["id"] for e in know.materialize()["elements"] if e["id"] == "FR-1"]
    assert o.review_elements(ids, "approved", "alice", "BA", "checked against the SRS")["changed"] == 1
    changed = docs(SRS_PROSE.replace("MUST store evidence", "MUST store evidence and logs"))
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-X", "prose": False}}, run_id="g2", sources=[doc("urd", "URD"), SourceDoc.make("SRS", "2", "SRS", changed[1].text), doc("sad", "SAD")])
    o.advance("g2")
    g = know.materialize()
    assert next(e for e in g["elements"] if e["id"] == "FR-1")["review_status"] == "approved", "a reviewed fact is never silently overwritten"
    assert any(c["type"] == "reviewed_element_differs" and c["id"] == "FR-1" for c in g["conflicts"])


def test_review_requires_an_allowlisted_reviewer_and_rationale(tmp_path):
    from tdd_agent.errors import TddAgentError
    o, know, ops, p = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, docs(), prose=False)
    for who, role, why, code in (("mallory", "BA", "ok", "REVIEW_NOT_PERMITTED"), ("alice", "SA", "ok", "REVIEW_NOT_PERMITTED"), ("alice", "BA", "  ", "RATIONALE_REQUIRED")):
        with pytest.raises(TddAgentError) as e:
            o.review_elements(["FR-1"], "approved", who, role, why)
        assert e.value.code == code
    assert know.get_element("FR-1")["review_status"] == "needs_review"


def test_references_resolve_across_documents_not_only_within_one(tmp_path):
    urd = SourceDoc.make("URD", "1", "URD", "# URD\n| ID | Statement |\n|---|---|\n| BRL-2 | A payment is captured at most once per key. |\n")
    srs = SourceDoc.make("SRS", "1", "SRS", "# SRS\n| ID | Statement | Rules |\n|---|---|---|\n| FR-1 | The system MUST capture a payment. | BRL-2 |\n| FR-2 | The system MUST refund. | BRL-7 |\n")
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    assert run_gen(o, [urd, srs, doc("sad", "SAD")], prose=False).status == "completed"
    g = know.materialize()
    assert any(e["src"] == "FR-1" and e["dst"] == "BRL-2" for e in g["edges"]), "defined in the URD, cited by the SRS"
    gaps = {x["target"] for x in g["gaps"] if x["need"] == "definition"}
    assert "BRL-2" not in gaps and "BRL-7" in gaps, "only the reference nobody defines is a gap"


def test_a_blocked_document_names_every_missing_knowledge_need_as_a_question(tmp_path):
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, docs(), prose=False)
    out = ops.get_task("g1", "G3")["output"]
    assert out["status"] == "blocked" and out["blockers"]
    asked = {q["gap"] for q in out["questions"]}
    assert {f"NEED-{b}" for b in out["blockers"]} <= asked and all(q["owner"] for q in out["questions"])


def test_the_rendered_tdd_shows_the_gaps_of_undefined_references(tmp_path):
    srs = SourceDoc.make("SRS", "1", "SRS", "# SRS\n| ID | Statement | Rules |\n|---|---|---|\n| FR-2 | The system MUST refund. | BRL-7 |\n")
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, [doc("urd", "URD"), srs, doc("sad", "SAD")], prose=False)
    text = know.get_source("TDD-X").text
    assert "BRL-7" in text.split("### A.2 ", 1)[1], "a gap that explains why the document is blocked must be in the document"
    assert "- **Gaps:** 0" not in text


def test_defect_mode_pins_down_the_governing_facts_and_asks_for_srs_clarification_instead_of_inventing(tmp_path):
    srs = SourceDoc.make("SRS", "1", "SRS", "# SRS\n| ID | Statement |\n|---|---|\n| NFR-1 | Nộp SBOM: p95 ≤ 30 giây; thử lại 3 lần khi 429; quá 30 giây áp gate_failure_mode |\n| FR-2 | The system MUST store evidence. |\n")
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, [doc("urd", "URD"), srs, doc("sad", "SAD")], prose=False)
    head = know.head().id
    supported = {"id": "DEFECT-1", "reproduction": "CI gets a timeout when submitting an SBOM", "expected_behavior": "p95 ≤ 30 giây and retry 3 lần (NFR-1)"}
    o.start("tdd-generation", {"mode": "defect", "params": {"tdd_id": "TDD-X", "prose": False, "base_snapshot": head, "change_request": supported}}, run_id="d1", sources=[srs, doc("sad", "SAD")])
    assert o.advance("d1").status == "completed"
    rec = ops.get_task("d1", "G3")["output"]["defect"]
    assert rec["affected"][0]["id"] == "NFR-1" and rec["supported_budgets"]["deadline_s"] == [30.0] and rec["supported_budgets"]["retries"] == [3]
    assert not rec["srs_clarification_required"] and rec["test_scenario"]["evidence"][0] == "NFR-1"
    assert know.head().id == head, "nothing changed, so no new snapshot"
    unsupported = dict(supported, id="DEFECT-2", expected_behavior="timeout 10 giây and retry 5 lần (NFR-1)")
    o.start("tdd-generation", {"mode": "defect", "params": {"tdd_id": "TDD-X", "prose": False, "base_snapshot": head, "change_request": unsupported}}, run_id="d2", sources=[srs, doc("sad", "SAD")])
    assert o.advance("d2").status == "completed"
    out = ops.get_task("d2", "G3")["output"]
    assert out["defect"]["srs_clarification_required"] and {(u["kind"], u["value"]) for u in out["defect"]["unsupported_expected"]} == {("deadline_s", 10.0), ("retries", 5)}
    assert any(q["gap"].startswith("GAP-DEFECT-2-CLARIFY") and q["owner"] == "BA" for q in out["questions"])
    missing = dict(supported, id="DEFECT-3", reproduction="completely unrelated zebra", expected_behavior="zebra")
    o.start("tdd-generation", {"mode": "defect", "params": {"tdd_id": "TDD-X", "prose": False, "base_snapshot": know.head().id, "change_request": missing}}, run_id="d3", sources=[srs, doc("sad", "SAD")])
    o.advance("d3")
    assert any(q["gap"] == "GAP-DEFECT-3-TARGET" for q in ops.get_task("d3", "G3")["output"]["questions"])


def test_a_run_that_reads_fewer_documents_keeps_what_the_other_documents_said(tmp_path):
    """Found on PBOM (phase 11): a defect run pinned only SRS and SAD and dropped the URD evidence of every shared element."""
    urd = SourceDoc.make("URD", "1", "URD", "# URD\n| ID | Rule |\n|---|---|\n| BRL-1 | A payment is captured once per key. |\n")
    srs = SourceDoc.make("SRS", "1", "SRS", "# SRS\n| ID | Rule |\n|---|---|\n| BRL-1 | Capture once per idempotency key |\n")
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    run_gen(o, [urd, srs, doc("sad", "SAD")], prose=False)
    before = know.get_element("BRL-1")
    assert {p["source_id"] for p in before["provenance"]} == {"URD", "SRS"}
    head = know.head().id
    o.start("tdd-generation", {"mode": "defect", "params": {"tdd_id": "TDD-X", "prose": False, "base_snapshot": head, "change_request": {"id": "D", "reproduction": "BRL-1 is violated", "expected_behavior": "BRL-1 holds"}}}, run_id="d", sources=[srs, doc("sad", "SAD")])
    assert o.advance("d").status == "completed"
    after = know.get_element("BRL-1")
    assert {p["source_id"] for p in after["provenance"]} == {"URD", "SRS"}, "the URD evidence survived a run that did not read the URD"
    assert know.head().id == head, "nothing changed, so nothing was committed"


def test_reading_a_source_again_at_a_new_revision_replaces_its_edges_instead_of_duplicating_them(tmp_path):
    """The same relation must not be kept once per revision of the document (it doubled the relations of PBOM on the second generate)."""
    o, know, ops, provider = make(default_flows(), fact_handler, tmp_path)
    srs_v1 = "# SRS\n| ID | Statement | Rules |\n|---|---|---|\n| FR-1 | The system MUST store evidence. | BRL-1 |\n\n| ID | Rule | FR |\n|---|---|---|\n| BRL-1 | Keep it | FR-1 |\n"
    assert run_gen(o, [doc("urd", "URD"), SourceDoc.make("SRS", "1", "SRS", srs_v1), doc("sad", "SAD")], prose=False).status == "completed"
    first = know.materialize()
    assert first["edges"], "the fixture must produce relations"
    srs_v2 = srs_v1.replace("MUST store evidence", "MUST store evidence for 5 years")
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-X", "prose": False}}, run_id="g2", sources=[doc("urd", "URD"), SourceDoc.make("SRS", "2", "SRS", srs_v2), doc("sad", "SAD")])
    assert o.advance("g2").status == "completed"
    second = know.materialize()
    key = lambda g: sorted((e["src"], e["rel"], e["dst"]) for e in g["edges"])
    assert key(second) == key(first), "same relations, not one per revision"
    versions = {p["source_version"] for e in second["edges"] for p in e.get("provenance", []) if p["source_id"] == "SRS"}
    assert versions == {"2"}, "the evidence now points at the revision that was read"


def test_a_partial_read_of_a_new_revision_keeps_the_relations_of_the_elements_it_did_not_read():
    """Found on TMS (issue 12a): a modified CR re-read one requirement at SRS r2 and the relations of every other requirement vanished."""
    from tdd_agent.agents.generation import Extraction, merge_into_graph

    def prov(version):
        return [{"kind": "source", "source_id": "SRS", "source_version": version, "block": "line:3"}]
    el = lambda i: {"id": i, "class": "Requirement", "name": i, "origin": "own", "review_status": "reviewed", "props": {"statement": i}, "provenance": prov("1")}
    head = {"version": "v1", "elements": [el("FR-1"), el("FR-2")], "edges": [
        {"src": "FR-1", "rel": "satisfies", "dst": "FR-2", "provenance": prov("1")}, {"src": "FR-2", "rel": "satisfies", "dst": "FR-1", "provenance": prov("1")}]}
    new_el = dict(el("FR-1"), provenance=prov("2"), props={"statement": "FR-1 changed"}, review_status="needs_review")
    partial = Extraction(elements=[new_el], edges=[{"src": "FR-1", "rel": "satisfies", "dst": "FR-2", "provenance": prov("2")}], complete=False, scope={"FR-1"})
    graph, _ = merge_into_graph(head, partial, "v2")
    edges = {(e["src"], e["dst"]): {p["source_version"] for p in e["provenance"]} for e in graph["edges"]}
    assert edges == {("FR-1", "FR-2"): {"2"}, ("FR-2", "FR-1"): {"1"}}, "FR-2 was not re-read, so its relation stays; FR-1's moves to the revision that was read"


def test_a_document_whose_text_is_already_in_the_graph_is_not_read_again_and_its_facts_do_not_change(tmp_path):
    """Found on TMS (issue 13): editing the SRS made the model read the SAD again and word its facts differently, which the read-only SAD refuses."""
    reads = []

    def handler(req, n):
        reads.append(req.metadata.get("skill"))
        return fact_handler(req, n)
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    srs_v1 = "# SRS\n| ID | Statement |\n|---|---|\n| FR-1 | The system MUST store evidence. |\n"
    run_gen(o, [doc("urd", "URD"), SourceDoc.make("SRS", "1", "SRS", srs_v1), doc("sad", "SAD")], prose=True)
    sad_before = sorted((e["id"], str(e["props"])) for e in know.materialize()["elements"] if e.get("owner_doc") == "SAD")
    reads.clear()
    srs_v2 = srs_v1.replace("MUST store evidence", "MUST store evidence for 5 years")
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-X", "prose": True}}, run_id="g2", sources=[doc("urd", "URD"), SourceDoc.make("SRS", "2", "SRS", srs_v2), doc("sad", "SAD")])
    assert o.advance("g2").status == "completed"
    sad_after = sorted((e["id"], str(e["props"])) for e in know.materialize()["elements"] if e.get("owner_doc") == "SAD")
    assert sad_after == sad_before and sad_before, "the SAD facts are exactly as they were"
    assert ops.get_task("g2", "G3")["output"]["stats"]["not_read_again"] == ["URD@1", "SAD@1"] or set(ops.get_task("g2", "G3")["output"]["stats"]["not_read_again"]) >= {"SAD@1"}


def test_a_reviewed_element_read_again_from_a_new_revision_is_the_same_fact_not_a_conflict():
    from tdd_agent.agents.generation import Extraction, merge_into_graph
    prov = lambda v: [{"kind": "extracted", "source_id": "SRS", "source_version": v, "source_hash": "h" + v, "block": "line:3", "quote": "| FR-1 | MUST store |", "quote_hash": "q" + v}]
    base = {"id": "FR-1", "class": "Requirement", "name": "FR-1", "origin": "own", "props": {"statement": "MUST store"}}
    head = {"version": "v1", "elements": [dict(base, review_status="approved", reviews=[{"actor": "bob"}], provenance=prov("1"), version_introduced="1")], "edges": []}
    graph, report = merge_into_graph(head, Extraction(elements=[dict(base, review_status="needs_review", provenance=prov("2"), version_introduced="2")]), "v2")
    assert graph.get("conflicts", []) == [] and report["kept_reviewed"] == 0 and graph["elements"][0]["review_status"] == "approved"
    changed = dict(base, props={"statement": "MUST store for 5 years"}, review_status="needs_review", provenance=prov("2"))
    graph, report = merge_into_graph(head, Extraction(elements=[changed]), "v2")
    assert report["kept_reviewed"] == 1 and graph["conflicts"][0]["type"] == "reviewed_element_differs"


def test_a_baseline_requirement_in_vietnamese_is_carried_whole_after_a_fixed_lead_in(tmp_path):
    """TMS: 'Hệ thống phải …' has no SHALL/MUST, which OpenSpec needs; the exact statement must still be in the baseline (rule TS-02 checks that)."""
    from tdd_agent.agents.baseline import ensure_baseline
    statement = "Hệ thống phải đẩy bug sang Jira."
    made = ensure_baseline(tmp_path, "cap", {"FR-1": statement, "FR-2": "The system MUST store.", "FR-3": "  "})
    text = (tmp_path / "openspec" / "specs" / "cap" / "spec.md").read_text(encoding="utf-8")
    assert made["written"] == ["FR-1", "FR-2"] and [u["id"] for u in made["unwritable"]] == ["FR-3"]
    assert f"SHALL satisfy this requirement as the SRS states it: {statement}" in text and "The system MUST store." in text and text.count("SHALL satisfy") == 1
