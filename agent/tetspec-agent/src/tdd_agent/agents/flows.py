"""Flow definitions: how gates, agents and approvals fit together.

* ``tdd-generation``  - G0 pin sources -> generate/readiness/render
* ``change-request``  - G0..G7 of the settled document workflow (CR/SRS delta + fixed SAD)
* ``impact-agent`` / ``changespec-agent`` - single agents on frozen inputs (isolated acceptance)
* ``prompt-edit``     - one controlled edit
* ``testspec``        - TS1: test specs by tsagent (child run started by GT after G4, or manual)
"""
from __future__ import annotations

from ..orchestrator.model import FlowDef, GateDef
from .changespec import (acceptance_gate, acceptance_on_approved, authoring_gate, cs_table_gate, evidence_gate, publish_on_approved)
from .common import skill_fingerprint
from .generation import generation_gate
from .impact import impact_gate, intake_gate, srs_readiness_gate
from .promptedit import promptedit_gate
from .realization import realization_gate
from .testspec import gt_extra_inputs, gt_gate, ts1_extra_inputs, ts1_gate


def _skills(*ids):
    return lambda ctx: {"skills": skill_fingerprint(list(ids))}


# what a change needs only from G4 on: supplying it late must not make the approved gates before it stale
LATE_PARAMS = ("code_targets", "verifications", "sad_waivers", "capability", "acceptance_extra", "satisfied_by")


def default_flows() -> dict[str, FlowDef]:
    generation_skills = ["segment-and-anchor", "extract-structured-facts", "select-prose-candidates", "extract-facts-from-prose", "map-candidate-to-class", "resolve-entity-match", "verify-evidence"]
    change = FlowDef("change-request", (
        GateDef("G0", "intake", intake_gate, source_roles=("SRS", "SAD", "CR"), extra_inputs=_skills("propose-srs-edits-from-cr"), ignore_params=LATE_PARAMS),
        GateDef("G1", "srs_ready", srs_readiness_gate, approval="BA", source_roles=("SRS",), depends_on=("G0",), extra_inputs=_skills("derive-srs-delta"), ignore_params=LATE_PARAMS),
        GateDef("G2", "impact_approved", impact_gate, approval="SA/Dev", source_roles=("SRS", "SAD", "TDD"), depends_on=("G0", "G1"),
                extra_inputs=_skills("scan-inventory-overlap", "judge-design-coverage", "detect-sad-conflict"), ignore_params=LATE_PARAMS),
        GateDef("G3", "tdd_realized", realization_gate, source_roles=("SRS", "SAD"), depends_on=("G0", "G1", "G2"), extra_inputs=_skills(*generation_skills), ignore_params=LATE_PARAMS),
        GateDef("G4", "spec_eligible", cs_table_gate, source_roles=("SAD",), depends_on=("G0", "G3"),
                extra_inputs=_skills("derive-verification", "select-code-target", "build-cs-table", "propose-acceptance-from-cr", "propose-satisfying-element"), config_sections=("llm", "render", "retrieval", "change")),
        # nhánh test specs: chạy ngay sau G4, khởi chạy run con `testspec` rồi trả về; G5-G7 không phụ thuộc GT
        GateDef("GT", "testspec_dispatched", gt_gate, source_roles=(), depends_on=("G0", "G3", "G4"), extra_inputs=gt_extra_inputs, ignore_params=LATE_PARAMS),
        GateDef("G5", "spec_authored", authoring_gate, approval="SA", source_roles=("SAD",), depends_on=("G0", "G2", "G3", "G4"), on_approved=publish_on_approved, config_sections=("llm", "render", "retrieval", "openspec"),
                extra_inputs=_skills("author-spec-prose", "critique-spec")),
        GateDef("G6", "verified", evidence_gate, depends_on=("G0", "G4"), evidence_gate=True),
        GateDef("G7", "accepted", acceptance_gate, approval="BA/owner", depends_on=("G0", "G6"), on_approved=acceptance_on_approved),
    ))
    generation = FlowDef("tdd-generation", (
        GateDef("G3", "tdd_realized", generation_gate, source_roles=("URD", "SRS", "SAD"), extra_inputs=_skills(*generation_skills)),
    ), initial_state="intake")
    edit = FlowDef("prompt-edit", (GateDef("P1", "accepted", promptedit_gate, extra_inputs=_skills("parse-edit-intent", "disambiguate-target", "classify-edit-from-intent", "write-section")),))
    impact_only = FlowDef("impact-agent", (GateDef("G2", "impact_approved", impact_gate, approval="SA/Dev", extra_inputs=_skills("scan-inventory-overlap", "judge-design-coverage", "detect-sad-conflict")),))
    changespec_only = FlowDef("changespec-agent", (
        GateDef("G4", "spec_eligible", cs_table_gate, extra_inputs=_skills("derive-verification", "select-code-target", "build-cs-table", "propose-acceptance-from-cr", "propose-satisfying-element"),
                config_sections=("llm", "render", "retrieval", "change")),
        GateDef("G5", "spec_authored", authoring_gate, approval="SA", depends_on=("G4",), on_approved=publish_on_approved, extra_inputs=_skills("author-spec-prose", "critique-spec"))))
    testspec = FlowDef("testspec", (GateDef("TS1", "testspec_written", ts1_gate, source_roles=(), extra_inputs=ts1_extra_inputs),), initial_state="intake")
    return {f.name: f for f in (change, generation, edit, impact_only, changespec_only, testspec)}
