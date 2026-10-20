"""Synthetic graphs for retrieval/query/agent tests (committed with validate=False)."""
from __future__ import annotations

from tdd_agent.store.base import KnowledgeStore, SourceDoc


def el(id, cls, **props):
    top = {k: props.pop(k) for k in ("provenance", "review_status") if k in props}
    return {"id": id, "class": cls, "name": props.pop("name", id), "props": props, **top}


def edge(src, rel, dst):
    return {"src": src, "rel": rel, "dst": dst}


def workflow_graph() -> dict:
    elements = [
        el("CR-1", "ChangeRequest", status="open"), el("CR-2", "ChangeRequest", status="open"), el("CR-3", "ChangeRequest", status="accepted"),
        el("DELTA-1", "SrsDelta", cr_id="CR-1"), el("DELTA-2", "SrsDelta", cr_id="CR-2"), el("DELTA-3", "SrsDelta", cr_id="CR-3"),
        el("RD-1", "RequirementDelta", change_request_id="CR-1", kind="add"),
        el("RV-1", "RequirementVersion", statement="Evidence MUST be stored"), el("RV-2", "RequirementVersion", statement="unrelated"),
        el("SC-1", "Scenario"),
        el("OP-1", "Operation", method="POST", revision="3"), el("OP-2", "Operation", integration=True), el("OP-3", "Operation", integration=True),
        el("OP-9", "Operation", method="GET"), el("EN-1", "Entity"), el("IF-1", "Interface", external=True),
        el("FP-1", "FailurePolicy"),
        el("MG-1", "Migration", from_schema="1", to_schema="2", rollback=None, backfill=["x"], stages=[]),
        el("MG-2", "Migration", from_schema="2", to_schema="3", rollback="down.sql", backfill=["y"], stages=["s1"]),
        el("SRS-DOC", "DocumentRef", role="SRS", revision="2", content_hash="h2"),
        el("TDD-A", "DocumentRef", role="TDD", revision="5", content_hash="t5"),
        el("SEC-SRS-1", "SectionRef", document_id="SRS-DOC", role="SRS", revision="2", content_hash="h2", section_id="3.1"),
        el("SEC-SRS-OLD", "SectionRef", document_id="SRS-DOC", role="SRS", revision="1", content_hash="h1", section_id="3.2"),
        el("SEC-TDD-1", "SectionRef", document_id="TDD-A", role="TDD", revision="5", content_hash="t5", section_id="6.3"),
        el("SEC-BAD", "SectionRef", document_id="TDD-A", role="SRS", revision="5", content_hash="t5", section_id="6.4"),
        el("CS-1", "ChangeSpecItem", action="modify", code_target="pbom.store()", srs_refs=["SEC-SRS-1"], tdd_refs=["SEC-TDD-1"],
           verification={"precondition": "p", "stimulus": "s", "expected": "e"}, cr_acceptance_refs=["AC-1", "AC-2"]),
        el("CS-2", "ChangeSpecItem", action="add", code_target="x", srs_refs=["SEC-SRS-1"], tdd_refs=["SEC-TDD-1"], verification={}, cr_acceptance_refs=["AC-9"]),
        el("CS-3", "ChangeSpecItem", action="add", code_target="y", srs_refs=["SEC-SRS-1"], tdd_refs=["SEC-TDD-1"], verification={}, cr_acceptance_refs=["AC-8"]),
        el("CS-BADROLE", "ChangeSpecItem", action="add", code_target="z", srs_refs=["SEC-BAD"], tdd_refs=["SEC-TDD-1"], verification={}, cr_acceptance_refs=["AC-7"]),
        el("AR-1", "AcceptanceReceipt", cr_id="CR-1", criterion_refs=["AC-1", "AC-2"], status="pending"),
        el("WI-1", "WorkItem"), el("TR-1", "TestReceipt", cs_id="CS-1", criterion_refs=["AC-1"], result="pass", executed_at="2026-10-10"),
        el("IR-1", "ImpactReport", entries=[
            {"tdd_id": "TDD-A", "action": "no_change", "rationale": "audit schema already covers it", "srs_refs": ["SEC-SRS-1"], "tdd_refs": [{"document_id": "TDD-A", "revision": "5"}]},
            {"tdd_id": "TDD-B", "action": "no_change", "rationale": "", "srs_refs": ["SEC-SRS-1"], "tdd_refs": [{"document_id": "TDD-A", "revision": "5"}]},
            {"tdd_id": "TDD-C", "action": "no_change", "rationale": "ok", "srs_refs": [], "tdd_refs": [{"document_id": "TDD-A", "revision": "5"}]},
            {"tdd_id": "TDD-D", "action": "no_change", "rationale": "ok", "srs_refs": ["SEC-SRS-1"], "tdd_refs": [{"document_id": "TDD-A", "revision": "4"}]},
            {"tdd_id": "TDD-E", "action": "update", "rationale": "changes", "srs_refs": ["SEC-SRS-1"], "tdd_refs": []}]),
        el("IR-2", "ImpactReport", entries=[]), el("IR-3", "ImpactReport", entries=[]),
        el("TS-1", "TddSetRealization"), el("TS-2", "TddSetRealization"), el("TS-3", "TddSetRealization"),
        el("OB-1", "DesignObligation", question="q1", applicability="applicable", status="open", tdd_id="TDD-A"),
        el("OB-2", "DesignObligation", question="q2", applicability="applicable", status="open", tdd_id="TDD-A"),
        el("OB-3", "DesignObligation", question="q3", applicability="not_applicable", status="open", tdd_id="TDD-A"),
        el("OR-1", "ObligationResolution", obligation_id="OB-2", status="satisfied", design_refs=["SEC-TDD-1"]),
    ]
    edges = [
        edge("RD-1", "after_requirement", "RV-1"), edge("RV-1", "has_scenario", "SC-1"),
        edge("OP-1", "satisfies", "RV-1"), edge("OP-2", "traces_to", "RV-1"), edge("EN-1", "traces_to", "SC-1"),
        edge("OP-3", "governed_by", "FP-1"), edge("OP-2", "uses", "IF-1"), edge("OP-3", "uses", "IF-1"),
        edge("OB-2", "has_obligation_resolution", "OR-1"),
        edge("WI-1", "implements", "CS-1"), edge("TR-1", "verifies", "WI-1"),
        edge("CR-1", "has_srs_delta", "DELTA-1"), edge("DELTA-1", "has_impact_report", "IR-1"), edge("IR-1", "realizes_tdd_set", "TS-1"),
        edge("TS-1", "has_change_spec_item", "CS-1"), edge("CS-1", "traces_to", "IF-1"),
        edge("CR-2", "has_srs_delta", "DELTA-2"), edge("DELTA-2", "has_impact_report", "IR-2"), edge("IR-2", "realizes_tdd_set", "TS-2"),
        edge("TS-2", "has_change_spec_item", "CS-2"), edge("CS-2", "traces_to", "IF-1"),
        edge("CR-3", "has_srs_delta", "DELTA-3"), edge("DELTA-3", "has_impact_report", "IR-3"), edge("IR-3", "realizes_tdd_set", "TS-3"),
        edge("TS-3", "has_change_spec_item", "CS-3"), edge("CS-3", "traces_to", "IF-1"),
    ]
    return {"version": "wf-1", "elements": elements, "edges": edges}


def load_workflow(store: KnowledgeStore) -> str:
    return store.commit_graph(None, workflow_graph(), actor="test", validate=False).snapshot.id


def register_sources(store: KnowledgeStore) -> dict[str, SourceDoc]:
    docs = {"SRS-DOC": store.register_source(SourceDoc.make("SRS-DOC", "2", "SRS", "# SRS rev 2")),
            "TDD-A": store.register_source(SourceDoc.make("TDD-A", "5", "TDD", "# TDD rev 5"))}
    return docs
