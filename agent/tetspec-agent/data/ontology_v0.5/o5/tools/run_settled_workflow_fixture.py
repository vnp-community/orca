#!/usr/bin/env python3
"""Create a test-only CR→SRS→SAD→TDD→CS acceptance fixture.

The fixture exercises the settled document workflow with real content hashes,
an approved impact report, actual post-step-4 TDD sections and a canonical
change-spec table. It is intentionally not production approval evidence.
"""
from __future__ import annotations

import argparse
import json
from dataclasses import asdict
from hashlib import sha256
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from workflow import assess_tdd_impact, approve_impact, realize_tdd_set, build_change_spec, validate_workflow_context  # noqa: E402
from specgen.spec_change import project_change_spec_to_openspec  # noqa: E402


def digest(value: str) -> str:
    return sha256(value.encode()).hexdigest()


def document(document_id: str, role: str, revision: str, content: str, section_id: str) -> tuple[dict, dict]:
    ref = {"document_id": document_id, "role": role, "revision": revision, "content_hash": digest(content), "section_id": section_id, "evidence": [content]}
    source = {"id": document_id, "role": role, "revision": revision, "content_hash": ref["content_hash"], "content": content, "sections": [{"id": section_id, "content": content}]}
    return ref, source


def build_fixture() -> dict:
    cr_ref, cr_source = document("CR-PBOM-ENT-001", "CR", "1", "Require entitlement evidence for design assets.", "CR-AC-ENT-001")
    srs_ref, srs_source = document("SRS-PBOM-ENT-001", "SRS", "2", "The service SHALL require entitlement evidence before accepting a design asset.", "SRS-FR-ENT-001")
    sad_ref, sad_source = document("SAD-PBOM-001", "SAD", "1", "Reuse the existing entitlement boundary and evidence store.", "SAD-§3.1")
    tdd_before_ref, tdd_before_source = document("TDD-PBOM-ENT-001", "TDD", "1", "Existing design asset flow accepts entitlement evidence.", "TDD-§4.2")
    tdd_after_ref, tdd_after_source = document("TDD-PBOM-ENT-001", "TDD", "2", "Updated flow rejects a design asset without entitlement evidence and records the evidence reference.", "TDD-§4.2")
    srs_delta = {
        "id": "SRS-PBOM-ENT-001",
        "ref": srs_ref,
        "cr_ref": cr_ref,
        "baseline_tdd_snapshot": "tdd-pbom-v1",
        "acceptance": ["CR-AC-ENT-001"],
        "changes": [{
            "id": "SRS-FR-ENT-001", "kind": "modified", "section_ref": srs_ref,
            "tdd_ids": ["TDD-PBOM-ENT-001"], "code_target": "entitlement.evidence",
            "verification": "A design asset without entitlement evidence is rejected and the evidence reference is persisted.",
            "cr_acceptance_refs": ["CR-AC-ENT-001"], "rationale": "SRS changes the acceptance condition for design assets.",
        }],
    }
    sad = {"id": "SAD-PBOM-001", "ref": sad_ref, "architecture_change": False}
    inventory = [{"id": "TDD-PBOM-ENT-001", "covered_srs_ids": ["SRS-FR-ENT-001"], "sections": [tdd_before_ref], "evidence": [{"source_id": "TDD-PBOM-ENT-001"}]}]
    impact = assess_tdd_impact(srs_delta, inventory, sad, report_id="IMPACT-PBOM-ENT-001")
    approve_impact(impact, "sa-pbom", "SA")
    realization = realize_tdd_set(impact, [{
        "tdd_id": "TDD-PBOM-ENT-001", "action": "update", "before_revision": "1", "after_revision": "2",
        "actual_sections": [tdd_after_ref],
    }], "tdd-pbom-v2")
    cr = {"id": "CR-PBOM-ENT-001", "ref": cr_ref, "acceptance": ["CR-AC-ENT-001"]}
    sources = [cr_source, srs_source, sad_source, tdd_after_source]
    context = {
        "cr": cr, "srs_delta": srs_delta, "sad": sad, "impact": asdict(impact), "tdd_set": asdict(realization),
        "documents": [cr_ref, srs_ref, sad_ref], "source_documents": sources, "tdd_sources": [tdd_after_source],
    }
    context_errors = validate_workflow_context(context)
    change_spec = build_change_spec(srs_delta, realization, sad, impact, cr)
    # Materialize the same eligible table as a readable OpenSpec proposal.
    # The small KG below is derived from the actual SRS/TDD source sections so
    # the design text quotes evidence instead of inventing implementation
    # details from the CR alone.
    kg_context = {"version": "tdd-pbom-v2", "elements": [
        {"id": ref["section_id"], "class": ref["role"], "name": ref["section_id"], "props": {"statement": ref["evidence"][0]}, "provenance": [{"source_id": ref["document_id"], "source_version": ref["revision"], "block": ref["section_id"]}]}
        for ref in (cr_ref, srs_ref, sad_ref, tdd_after_ref)
    ], "edges": []}
    openspec = project_change_spec_to_openspec(change_spec, "pbom-entitlement-evidence", "Require entitlement evidence before accepting a design asset.", kg_context=kg_context)
    return {
        "fixture": "test-only-settled-workflow",
        "status": "pass" if not context_errors and change_spec["status"] == "eligible" else "blocked",
        "review": {"kind": "test_only", "status": "pass", "independent": False, "reviewer": "fixture-regression"},
        "context_errors": context_errors,
        "impact": asdict(impact), "realization": asdict(realization), "change_spec": change_spec,
        "source_documents": sources, "openspec_artifacts": openspec, "kg_context": kg_context,
    }


def main() -> None:
    parser = argparse.ArgumentParser(); parser.add_argument("--output", default="data/test-pbom/run-quality-authoring/settled-workflow-fixture.json")
    args = parser.parse_args(); result = build_fixture()
    path = Path(args.output); path.parent.mkdir(parents=True, exist_ok=True); path.write_text(json.dumps({k: v for k, v in result.items() if k != "openspec_artifacts"}, ensure_ascii=False, indent=2) + "\n")
    artifact_root = path.parent / "change-spec-from-kg"; artifact_root.mkdir(parents=True, exist_ok=True)
    for rel, content in result["openspec_artifacts"].items():
        artifact = artifact_root / rel; artifact.parent.mkdir(parents=True, exist_ok=True)
        artifact.write_text(content, encoding="utf-8")
    print(json.dumps({"status": result["status"], "context_errors": result["context_errors"], "change_spec": result["change_spec"]["status"], "output": str(path), "openspec": str(artifact_root)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
