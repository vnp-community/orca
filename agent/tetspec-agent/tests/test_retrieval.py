import copy

import pytest

from graphs import edge, el, load_workflow, register_sources, workflow_graph
from tdd_agent.core import o5_path
from tdd_agent.errors import NotFound, RelationNotRegistered, SourceRoleError
from tdd_agent.retrieval import ClosureService, SeedSpec
from tdd_agent.retrieval.candidates import LexicalCandidates, verify_candidates
from tdd_agent.retrieval.policy import registered_relations
from tdd_agent.retrieval.queries import QueryService
from tdd_agent.retrieval.seeds import seeds_from_capability, seeds_from_change_spec_item, seeds_from_srs_delta
from tdd_agent.store import open_knowledge_store
from tdd_agent.store.base import SourceDoc
from tdd_agent.store.contract import pbom_graph


@pytest.fixture()
def store():
    s = open_knowledge_store("memory:")
    yield s
    s.close()


def chain(n, rel="depends_on"):
    return {"version": "c", "elements": [el(f"N{i:02d}", "Component") for i in range(n)], "edges": [edge(f"N{i:02d}", rel, f"N{i + 1:02d}") for i in range(n - 1)]}


# ---------------------------------------------------------- the ten characteristics
def test_01_graph_first_only_declared_relations_unsupported_is_reported(store):
    g = chain(3); g["edges"].append(edge("N00", "totally_made_up", "N02"))
    store.commit_graph(None, g, actor="t", validate=False)
    c = ClosureService(store).resolve(["N00"])
    assert c.selected_ids == ["N00", "N01", "N02"]
    assert {"type": "unsupported_relation", "src": "N00", "rel": "totally_made_up", "dst": "N02"} in c.unresolved


def test_02_source_bound_closure_carries_document_revision_hash(store):
    g = chain(2)
    g["elements"][0]["provenance"] = [{"kind": "extracted", "source_id": "SRS-DOC", "source_version": "2", "source_hash": "abc", "block": "line:10", "quote": "x"}]
    store.commit_graph(None, g, actor="t", validate=False)
    c = ClosureService(store).resolve(["N00"])
    assert c.source_refs == [{"document": "SRS-DOC", "revision": "2", "section": "line:10", "content_hash": "abc"}]


def test_03_scope_aware_only_reachable_elements(store):
    g = chain(3); g["elements"].append(el("ISOLATED", "Component"))
    store.commit_graph(None, g, actor="t", validate=False)
    assert "ISOLATED" not in ClosureService(store).resolve(["N00"]).selected_ids


def test_04_bidirectional_from_either_end(store):
    store.commit_graph(None, chain(4), actor="t", validate=False)
    svc = ClosureService(store)
    assert svc.resolve(["N03"]).selected_ids == svc.resolve(["N00"]).selected_ids or set(svc.resolve(["N03"]).selected_ids) == {"N00", "N01", "N02", "N03"}


def test_05_profile_aware_gaps(store):
    g = {"version": "p", "elements": [el("W", "Operation", method="POST"), el("T", "Operation", transactional=True),
                                      el("IN", "Operation", integration=True), el("M", "Migration", from_schema="a", to_schema="b", rollback=None, backfill=None, stages=[]),
                                      el("HA", "HighAvailabilityTopology")], "edges": [edge("W", "depends_on", "T"), edge("T", "depends_on", "IN"), edge("IN", "depends_on", "M"), edge("M", "depends_on", "HA")]}
    store.commit_graph(None, g, actor="t", validate=False)
    svc = ClosureService(store)
    codes = {(x["code"], x["element"], x.get("field")) for x in svc.resolve(["W"], profile="tdd-generation").gaps}
    assert ("MISSING_CONDITIONAL_FIELD", "W", "idempotency") in codes and ("MISSING_CONDITIONAL_FIELD", "W", "concurrency") in codes
    assert ("MISSING_CONDITIONAL_FIELD", "T", "failure_policy") in codes and ("MISSING_FAILURE_POLICY", "IN", None) in codes
    assert {c for c, e, f in codes if e == "M"} == {"MISSING_ROLLBACK", "MISSING_BACKFILL", "MISSING_MIGRATION_STAGES"} and ("MISSING_RECOVERY_OBJECTIVE", "HA", None) in codes
    edit_codes = {x["code"] for x in svc.resolve(["W"], profile="prompt-tdd-edit").gaps}
    assert edit_codes == {"MISSING_CONDITIONAL_FIELD"}
    assert svc.resolve(["W"], profile="none").gaps == []


def test_06_temporal_stale_source_is_excluded_and_blocks(store):
    register_sources(store)
    g = chain(3)
    g["elements"][1]["provenance"] = [{"source_id": "SRS-DOC", "source_version": "2", "source_hash": "WRONG"}]
    store.commit_graph(None, g, actor="t", validate=False)
    c = ClosureService(store).resolve(["N00"])
    assert "N01" not in c.selected_ids and [x["id"] for x in c.excluded] == ["N01"] and "N02" not in c.selected_ids
    assert any(x["code"] == "STALE_SOURCE" for x in c.conflicts) and "STALE_SOURCE" in c.blockers
    pinned = ClosureService(store).resolve(["N00"], pins={"SRS-DOC": ("3", "newhash")})
    assert "STALE_SOURCE" in pinned.blockers


def test_07_negative_evidence_is_preserved_not_dropped(store):
    g = chain(2); g["edges"].append(edge("N01", "depends_on", "GHOST"))
    store.commit_graph(None, g, actor="t", validate=False)
    c = ClosureService(store).resolve(["N00", "NOT-THERE"], mandatory_ids=["NEVER-REACHED"])
    assert {"type": "dangling_reference", "src": "N01", "rel": "depends_on", "dst": "GHOST"} in c.unresolved
    codes = {(g["code"], g["element"]) for g in c.gaps}
    assert ("SEED_NOT_FOUND", "NOT-THERE") in codes and ("MANDATORY_NOT_IN_CLOSURE", "NEVER-REACHED") in codes


def test_08_fixed_point_has_no_depth_cap(store):
    store.commit_graph(None, chain(60), actor="t", validate=False)
    c = ClosureService(store).resolve(["N00"])
    assert len(c.selected_ids) == 60 and not c.truncated


def test_09_bounded_truncation_is_explicit_and_distance_ordered(store):
    store.commit_graph(None, chain(10), actor="t", validate=False)
    c = ClosureService(store).resolve(["N00"], budget=4, mandatory_ids=["N09"])
    assert c.selected_ids == ["N00", "N01", "N02", "N03"] and c.truncated and c.missing_ids[0] == "N04"
    assert c.mandatory_truncated == ["N09"] and "TRUNCATED_MANDATORY_NEED" in c.blockers
    assert any(b["reason"] == "budget" for b in c.boundary)
    assert not ClosureService(store).resolve(["N00"], budget=4, mandatory_ids=["N01"]).blockers


def test_10_reproducible_resolution_hash(store):
    info = store.commit_graph(None, chain(5), actor="t", validate=False).snapshot
    svc = ClosureService(store)
    a, b = svc.resolve(["N00"]), svc.resolve(["N00"])
    assert a.resolution_hash == b.resolution_hash and a.to_dict() == b.to_dict()
    assert svc.resolve(["N00"], budget=3).resolution_hash != a.resolution_hash and svc.resolve(["N01"]).resolution_hash != a.resolution_hash
    g = chain(5); g["elements"][2]["name"] = "changed"
    store.commit_graph(info.id, g, actor="t", validate=False)
    assert svc.resolve(["N00"]).resolution_hash != a.resolution_hash
    assert svc.resolve(["N00"], snapshot=info.id).resolution_hash == a.resolution_hash


def test_wrong_role_section_is_excluded_with_conflict(store):
    register_sources(store)
    store.commit_graph(None, workflow_graph(), actor="t", validate=False)
    c = ClosureService(store).resolve(["CS-BADROLE"])
    assert "SEC-BAD" in [x["id"] for x in c.excluded] and "SEC-BAD" not in c.selected_ids
    conflict = next(x for x in c.conflicts if x["code"] == "SOURCE_ROLE_MISMATCH")
    assert conflict["declared_role"] == "SRS" and conflict["actual_role"] == "TDD" and "SOURCE_ROLE_MISMATCH" in c.blockers


def test_closure_equals_o5_resolve_scope_on_real_graph(store):
    g = pbom_graph(); store.commit_graph(None, g, actor="t")
    svc = ClosureService(store)
    seed = next(e["src"] for e in g["edges"] if e["rel"] == "has_scenario")
    mine = svc.resolve([seed])
    ref = o5_path.load().semantic_resolver.resolve_scope(g, [seed], policies=svc.policies)
    assert sorted(mine.selected_ids) == sorted(ref["ids"])
    assert mine.reasons and all(r["from"] in mine.selected_ids for r in mine.reasons.values())


def test_closure_on_sqlite_equals_memory(tmp_path):
    g = pbom_graph(); seed = next(e["src"] for e in g["edges"] if e["rel"] == "has_scenario")
    results = []
    for url in ("memory:", f"sqlite:{tmp_path}/c.db"):
        s = open_knowledge_store(url); s.commit_graph(None, g, actor="t")
        results.append(ClosureService(s).resolve([seed]).resolution_hash); s.close()
    assert results[0] == results[1]


# ------------------------------------------------------------------ queries
@pytest.fixture()
def wf(store):
    register_sources(store)
    snap = load_workflow(store)
    return QueryService(store), snap


def test_q1_candidates_are_design_elements_within_three_hops(wf):
    q, snap = wf
    r = q.q1_candidate_tdds("DELTA-1")
    assert r.kind == "input" and r.meta["candidate_only"] is True
    assert {x["id"] for x in r.rows} == {"OP-1", "OP-2", "EN-1"} and "OP-9" not in {x["id"] for x in r.rows}
    assert r.rows[[x["id"] for x in r.rows].index("OP-1")]["via_requirements"] == ["RV-1"]
    with pytest.raises(NotFound):
        q.q1_candidate_tdds("NOPE")


def test_q2_unresolved_obligations_including_na_without_evidence(wf):
    q, _ = wf
    assert {x["id"] for x in q.q2_unresolved_obligations("TDD-A").rows} == {"OB-1", "OB-3"}
    assert q.q2_unresolved_obligations("OTHER").rows == []


def test_q3_context_pack_and_role_enforcement(wf):
    q, _ = wf
    row = q.q3_context_pack("CS-1").rows[0]
    assert row["cs"]["id"] == "CS-1" and row["srs"][0]["role"] == "SRS" and row["tdd"][0]["role"] == "TDD" and row["errors"] == []
    assert [a["id"] for a in row["acceptance"]] == ["AC-1", "AC-2"] and all(a.get("missing") for a in row["acceptance"])
    with pytest.raises(SourceRoleError) as e:
        q.q3_context_pack("CS-BADROLE")
    assert e.value.details["errors"][0]["code"] == "SOURCE_ROLE_MISMATCH"
    with pytest.raises(NotFound):
        q.q3_context_pack("OP-1")


def test_q4_q5_gaps(wf):
    q, _ = wf
    assert [r["operation"] for r in q.q4_missing_failure_policy().rows] == ["OP-2"]
    assert [(r["id"], r["missing"]) for r in q.q5_incomplete_migrations().rows] == [("MG-1", ["rollback", "stages"])]


def test_q6_stale_refs(wf):
    q, _ = wf
    rows = {r["ref"]: r for r in q.q6_stale_refs().rows}
    assert set(rows) == {"SEC-SRS-OLD"} and rows["SEC-SRS-OLD"]["reasons"] == ["CONTENT_HASH_DIFFERS", "REVISION_DIFFERS"]


def test_q7_no_change_validity(wf):
    q, _ = wf
    r = q.q7_no_change_validity("IR-1")
    by = {x["tdd_id"]: x for x in r.rows}
    assert by["TDD-A"]["valid"] and by["TDD-B"]["errors"] == ["NO_CHANGE_REASON_MISSING"] and by["TDD-C"]["errors"] == ["NO_CHANGE_SRS_REFS_MISSING"]
    assert by["TDD-D"]["errors"] == ["NO_CHANGE_REVISION_STALE"] and "TDD-E" not in by and r.meta["all_valid"] is False


def test_q8_acceptance_coverage_and_registry_guard(store):
    register_sources(store); load_workflow(store)
    rows = {r["criterion"]: r for r in QueryService(store).q8_acceptance_coverage("CR-1").rows}
    assert rows["AC-1"]["status"] == "covered" and rows["AC-1"]["work_items"] == ["WI-1"] and rows["AC-1"]["passing_tests"] == ["TR-1"]
    assert rows["AC-2"]["status"] == "uncovered"
    with pytest.raises(RelationNotRegistered) as e:
        QueryService(store, registry={"traces_to"}).q8_acceptance_coverage("CR-1")
    assert e.value.code == "RELATION_NOT_REGISTERED" and set(e.value.details["missing"]) == {"implements", "verifies"}
    assert {"implements", "verifies"} <= registered_relations()


def test_q9_two_open_crs_on_one_interface_but_not_accepted_ones(wf):
    q, _ = wf
    rows = q.q9_shared_contract_conflicts().rows
    assert len(rows) == 1 and rows[0]["interface"] == "IF-1" and rows[0]["change_requests"] == ["CR-1", "CR-2"]


def test_query_dispatch_by_name(wf):
    q, snap = wf
    assert q.run("q6").name == "Q6" and q.run("Q7", report_id="IR-1").kind == "valid"
    with pytest.raises(NotFound) as e:
        q.run("Q10")
    assert e.value.code == "QUERY_UNKNOWN"
    assert q.run("Q6", snapshot=snap).snapshot == snap


# --------------------------------------------------------------------- seeds / candidates
def test_seeds_by_intent(wf, store):
    spec = seeds_from_srs_delta(store, "DELTA-1")
    assert spec.kind == "srs_delta" and set(spec.ids) == {"DELTA-1", "RD-1", "RV-1"}
    assert set(seeds_from_change_spec_item(store, "CS-1").ids) == {"CS-1", "SEC-SRS-1", "SEC-TDD-1", "AC-1", "AC-2"}
    store2 = open_knowledge_store("memory:")
    store2.commit_graph(None, {"version": "1", "elements": [el("CAP", "Capability"), el("R1", "RequirementVersion")], "edges": [edge("CAP", "has_requirement", "R1")]}, actor="t", validate=False)
    assert set(seeds_from_capability(store2, "CAP").ids) == {"CAP", "R1"}


def test_lexical_candidates_are_only_proposals_until_verified(store):
    register_sources(store)
    g = {"version": "1", "elements": [
        el("A", "Operation", name="submit sbom evidence", provenance=[{"source_id": "SRS-DOC", "source_version": "2", "source_hash": "WRONG"}]),
        el("B", "Operation", name="submit sbom evidence quickly", provenance=[{"source_id": "SRS-DOC", "source_version": "2", "source_hash": store.get_source("SRS-DOC").content_hash}]),
        el("C", "Operation", name="submit sbom evidence nosource"), el("D", "Operation", name="unrelated login flow")], "edges": []}
    store.commit_graph(None, g, actor="t", validate=False)
    found = LexicalCandidates(store).search("submit sbom evidence", k=5)
    assert {i for i, _ in found} == {"A", "B", "C"} and found == sorted(found, key=lambda x: (-x[1], x[0]))
    spec = verify_candidates(ClosureService(store), [i for i, _ in found])
    assert spec.ids == ("B",) and {r["id"]: r["reason"] for r in spec.meta["rejected"]} == {"A": ["STALE_SOURCE"], "C": ["NO_RESOLVABLE_PROVENANCE"]}


def test_seeds_proposed_by_the_full_text_index_still_pass_verification_and_give_the_same_closure(store):
    register_sources(store)
    g = {"version": "1", "elements": [
        el("A", "Operation", name="submit sbom evidence", provenance=[{"source_id": "SRS-DOC", "source_version": "2", "source_hash": "WRONG"}]),
        el("B", "Operation", name="submit sbom evidence quickly", provenance=[{"source_id": "SRS-DOC", "source_version": "2", "source_hash": store.get_source("SRS-DOC").content_hash}]),
        el("C", "Operation", name="submit sbom evidence nosource"), el("D", "Operation", name="unrelated login flow")], "edges": []}
    store.commit_graph(None, g, actor="t", validate=False)
    scan = LexicalCandidates(store).search("submit sbom evidence", k=5)
    indexed = LexicalCandidates(store, use_index=True)
    if not getattr(store, "supports_search", False):
        pytest.skip("this backend has no index")
    assert indexed.indexed and {i for i, _ in indexed.search("submit sbom evidence", k=5)} == {i for i, _ in scan} == {"A", "B", "C"}
    from_index = verify_candidates(ClosureService(store), [i for i, _ in indexed.search("submit sbom evidence", k=5)])
    from_scan = verify_candidates(ClosureService(store), [i for i, _ in scan])
    assert from_index.ids == from_scan.ids == ("B",) and {r["id"]: r["reason"] for r in from_index.meta["rejected"]} == {"A": ["STALE_SOURCE"], "C": ["NO_RESOLVABLE_PROVENANCE"]}, "the index proposes, verification decides"


def test_workload_and_availability_targets_stated_as_text_need_a_load_profile_and_a_recovery_objective():
    from tdd_agent.retrieval import expansion
    def qt(i, text):
        return {"id": i, "class": "QualityTarget", "name": i, "props": {"target": [text]}, "provenance": []}
    els = [qt("NFR-1", "Evaluate chịu 500 TPS đỉnh, p99 ≤ 300 ms"), qt("NFR-2", "khả dụng 99,95% active-active"), qt("NFR-3", "p95 ≤ 2 giây"), qt("NFR-4", "60 lần nộp/phút")]
    gaps = {(g["element"], g["code"]) for g in expansion.gaps_for("tdd-generation", els, [])}
    assert gaps == {("NFR-1", "MISSING_LOAD_PROFILE"), ("NFR-2", "MISSING_RECOVERY_OBJECTIVE")}
    typed = els + [{"id": "LP-1", "class": "LoadProfile", "props": {}}, {"id": "RTO-1", "class": "RecoveryObjective", "props": {}}]
    assert expansion.gaps_for("tdd-generation", typed, []) == []
