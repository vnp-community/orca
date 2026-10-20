"""The acceptance harness itself: approvals gate grading, isolated runs need the TDD report, oracles are deterministic."""
import json

import pytest

from harness import make
from tdd_agent.acceptance import oracles
from tdd_agent.acceptance.common import Check
from tdd_agent.acceptance.isolated import PREREQUISITE, require_tdd_acceptance, seed_runtime, _stability
from tdd_agent.agents.flows import default_flows
from tdd_agent.config import load_config
from tdd_agent.errors import TddAgentError
from tdd_agent.runtime import Runtime
from tdd_agent.store.base import SourceDoc

ACTORS = [{"id": "alice", "kind": "human", "roles": ["BA", "SA"]}, {"id": "claude", "kind": "coding_agent", "roles": ["SA"]}]


class Rt:
    def __init__(self):
        self.config = load_config(overrides={"approvals": {"actors": ACTORS}}, env={})


def test_oracle_approval_needs_an_allowlisted_actor_role_and_rationale_and_records_the_grade(tmp_path):
    rt = Rt()
    rec = oracles.approve(rt, tmp_path, "second-generation", "claude", "SA", "authored before the run")
    assert rec["grade"] == "agent" and oracles.approval_for(tmp_path, "second-generation")["actor"] == "claude"
    assert oracles.approve(rt, tmp_path, "pbom-generation", "alice", "SA", "reviewed")["grade"] == "human"
    for bad in (("mallory", "SA", "x"), ("claude", "BA", "x"), ("claude", "SA", "  ")):
        with pytest.raises(TddAgentError) as e:
            oracles.approve(rt, tmp_path, "prose-generation", *bad)
        assert e.value.code == "ORACLE_APPROVAL_REJECTED"
    assert oracles.approval_for(tmp_path, "prose-generation") is None, "an unapproved oracle grades nothing"


def test_a_changed_oracle_invalidates_its_approval(tmp_path, monkeypatch):
    oracles.approve(Rt(), tmp_path, "second-generation", "claude", "SA", "ok")
    original = oracles.load
    monkeypatch.setattr(oracles, "load", lambda name: {**original(name), "revision_note": "edited after approval"})
    assert oracles.approval_for(tmp_path, "second-generation") is None


def test_oracle_predicates_are_deterministic_and_check_evidence():
    graph = {"elements": [{"id": "FR-1", "class": "Requirement", "origin": "upstream", "props": {"statement": "s"}, "provenance": [{"kind": "extracted", "source_id": "SRS", "source_version": "1", "block": "line:3", "quote": "The system MUST x", "quote_hash": "h"}]},
                          {"id": "X-1", "class": "Entity", "origin": "upstream", "props": {}, "provenance": []}],
             "edges": [{"src": "FR-1", "rel": "traces_to", "dst": "BRL-1"}], "gaps": [{"need": "definition", "target": "BRL-1"}]}
    oracle = {"checks": [{"id": "e", "kind": "element", "element": "FR-1", "class": "Requirement", "props": {"statement": "s"}},
                         {"id": "r", "kind": "relation", "src": "FR-1", "rel": "traces_to", "dst": "BRL-1"},
                         {"id": "g", "kind": "gap", "need": "definition", "target_prefix": "BRL"},
                         {"id": "n", "kind": "no_invented_facts"}, {"id": "nr", "kind": "not_ready"}, {"id": "q", "kind": "question_for_gap"}]}
    checks = {c.id: c for c in oracles.evaluate_generation(oracle, graph, {"status": "blocked", "questions": []}, [])}
    assert checks["e"].passed and checks["r"].passed and checks["g"].passed and checks["nr"].passed
    assert not checks["n"].passed, "an upstream element without provenance is an invented fact"
    assert not checks["q"].passed
    with pytest.raises(TddAgentError):
        oracles.evaluate_generation({"checks": [{"id": "z", "kind": "bogus"}]}, graph, {}, [])


def test_isolated_runs_refuse_to_start_without_the_tdd_acceptance_report(tmp_path):
    with pytest.raises(TddAgentError) as e:
        require_tdd_acceptance(tmp_path)
    assert e.value.code == "ACCEPTANCE_PREREQUISITE_MISSING"
    (tmp_path / PREREQUISITE).write_text(json.dumps({"inputs": {}}))
    assert require_tdd_acceptance(tmp_path) == {"inputs": {}}


def test_an_unstable_case_is_never_a_pass():
    assert _stability([True, True, True]) == "stable_pass" and _stability([False] * 3) == "stable_fail" and _stability([True, False, True]) == "unstable"


def test_seeded_runtime_partitions_a_generated_kg_into_one_tdd_per_requirement_group(tmp_path):
    base = load_config(overrides={"approvals": {"actors": ACTORS}}, env={})
    from tdd_agent.store import open_knowledge_store
    src = tmp_path / "src"; src.mkdir()
    k = open_knowledge_store(f"sqlite:{src}/knowledge.db")
    prov = [{"kind": "extracted", "source_id": "SRS", "source_version": "1", "source_hash": "h", "block": "line:1", "quote": "q", "quote_hash": "qh"}]
    def req(i):
        return {"id": i, "class": "Requirement", "name": i, "origin": "upstream", "maturity": "sketch", "owner_doc": "srs", "tdds": ["TDD-ALL"], "review_status": "approved", "version_introduced": "1", "props": {"statement": "s " + i}, "provenance": prov}
    k.commit_graph(None, {"version": "v1", "elements": [req("FR-AAA-01"), req("FR-BBB-01")], "edges": []}, actor="t", validate=False)
    k.register_source(SourceDoc.make("TDD-ALL", "1", "TDD", "# all\n"))
    k.close()
    try:
        rt, groups = seed_runtime(base, src, tmp_path / "out", "seeded", partition=True)
    except Exception as exc:  # the real provider is not needed to seed; surface anything else
        raise AssertionError(exc)
    try:
        assert set(groups) == {"TDD-AAA", "TDD-BBB"}
        assert {d.id for d in rt.knowledge.list_sources("TDD")} == {"TDD-AAA", "TDD-BBB"}, "the single generated TDD is replaced by the partition"
        assert rt.knowledge.get_element("FR-AAA-01")["tdds"] == ["TDD-AAA"]
    finally:
        rt.close()


# ---- the real-case grader (phase 11): deterministic, safety separate from design
def test_real_case_grading_separates_safety_from_design_and_never_calls_a_data_limit_a_pass():
    from tdd_agent.acceptance import real_cases as rc
    case = {"checks": [{"id": "s", "kind": "falsy", "level": "safety", "path": "edit.mutated"}, {"id": "d", "kind": "in", "level": "design", "path": "edit.outcome", "values": ["blocked"]},
                       {"id": "pub", "kind": "eq", "level": "design", "path": "published", "value": True, "data_limited": True}]}
    ok = {"edit": {"mutated": False, "outcome": "blocked"}, "published": True}
    assert rc.grade(case, ok)["verdict"] == "pass"
    assert rc.grade(case, {**ok, "edit": {"mutated": True, "outcome": "blocked"}})["verdict"] == "fail", "a safety failure is a fail whatever the design checks say"
    assert rc.grade(case, {**ok, "edit": {"mutated": False, "outcome": "committed"}})["verdict"] == "partial"
    limited = rc.grade(case, {**ok, "published": False, "g5": {"status": "blocked"}})
    assert limited["verdict"] == "blocked_by_data" and limited["design_unmet"] == ["pub"]
    assert rc.grade(case, {**ok, "published": False})["verdict"] == "partial", "only a G5 block makes a missing publication data-limited"


def test_real_case_inputs_fail_loudly_when_the_pbom_text_drifts():
    from tdd_agent.acceptance import real_cases as rc
    srs = SourceDoc.make("SRS", "1.0", "SRS", "| FR-1 | a |\n| FR-2 | b |\n")
    assert "| FR-1.5 | x |" in rc.srs_variant(srs, "2.0", insert_after=("| FR-1 |", "| FR-1.5 | x |")).text
    with pytest.raises(TddAgentError) as e:
        rc.srs_variant(srs, "2.0", insert_after=("| FR-9 |", "| x |"))
    assert e.value.code == "CASE_INPUT_DRIFT"
    with pytest.raises(TddAgentError):
        rc.srs_variant(srs, "2.0", replace=[("not in the text", "y")])


def test_real_cases_need_an_approved_oracle_and_a_real_provider(tmp_path):
    from tdd_agent.acceptance import real_cases as rc
    cfg = load_config(overrides={"approvals": {"actors": ACTORS}}, env={})
    with pytest.raises(TddAgentError) as e:
        rc.run_real_cases(cfg, tmp_path)
    assert e.value.code == "ORACLE_NOT_APPROVED"
    oracles.approve(Rt(), tmp_path, "real-cases", "claude", "SA", "authored before the runs")
    fake = load_config(overrides={"approvals": {"actors": ACTORS}, "llm": {"default": "claude_code"}}, env={})
    fake.data["llm"]["default"] = "fake"
    with pytest.raises(TddAgentError) as e:
        rc.run_real_cases(fake, tmp_path)
    assert e.value.code == "REAL_PROVIDER_REQUIRED"
