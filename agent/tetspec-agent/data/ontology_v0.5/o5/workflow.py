"""Settled CR/URD -> SRS -> TDD impact -> change-spec workflow.

This module is deliberately dependency-free.  It is the small, typed boundary
between the ontology graph and the three agent flows.  A graph/KG can represent
these records, but it cannot replace the source documents or the human impact
approval required by stage 3.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass, field
from hashlib import sha256
import json
from typing import Any, Iterable


class WorkflowError(ValueError):
    """Input is not eligible for the settled document workflow."""


@dataclass
class DesignObligation:
    """A typed design question derived from an SRS/CR requirement."""

    id: str
    question: str
    srs_refs: list[dict[str, Any]]
    cr_acceptance_refs: list[str]
    applicability: str = "applicable"
    resolution_refs: list[dict[str, Any]] = field(default_factory=list)
    status: str = "open"
    profile_id: str = "core-tdd"

    def validate(self) -> list[str]:
        errors = []
        if not self.question or not self.srs_refs:
            errors.append("OBLIGATION_SOURCE_REQUIRED:" + self.id)
        if not self.cr_acceptance_refs:
            errors.append("OBLIGATION_CR_ACCEPTANCE_REQUIRED:" + self.id)
        if self.applicability not in {"applicable", "not_applicable", "unknown"}:
            errors.append("OBLIGATION_APPLICABILITY_INVALID:" + self.id)
        if self.status in {"verified", "accepted"} and not self.resolution_refs:
            errors.append("OBLIGATION_RESOLUTION_REQUIRED:" + self.id)
        if self.applicability == "not_applicable" and not self.resolution_refs:
            errors.append("OBLIGATION_NA_EVIDENCE_REQUIRED:" + self.id)
        return errors


def derive_obligations(srs_delta: dict[str, Any], impact: ImpactReport | None = None, profile_id: str = "core-tdd") -> list[DesignObligation]:
    """Create design questions without certifying their answers."""
    result = []
    for change in srs_delta.get("changes", srs_delta.get("requirements", [])) or []:
        ident = str(change.get("id", ""))
        if not ident:
            continue
        refs = [dict(change.get("section_ref", srs_delta.get("ref", {})))]
        criteria = change.get("cr_acceptance_refs", srs_delta.get("acceptance", [])) or []
        if isinstance(criteria, str):
            criteria = [criteria]
        result.append(DesignObligation("OBL-" + ident, str(change.get("design_question", "How will this requirement be designed and verified?")), refs, list(criteria), profile_id=profile_id))
    return result


OBLIGATION_TRANSITIONS = {
    "open": {"proposed", "not_applicable"},
    "proposed": {"open", "verified", "not_applicable", "stale"},
    "verified": {"accepted", "stale"},
    "accepted": {"stale"},
    "not_applicable": {"stale"},
    "stale": {"open", "proposed"},
}


def transition_obligation(obligation: DesignObligation | dict[str, Any], target_status: str, *, actor: str, base_revision: str, expected_revision: str | None = None, evidence: dict[str, Any] | None = None) -> dict[str, Any]:
    """Apply a guarded obligation lifecycle transition.

    Transitions are immutable records.  A new revision is returned; callers
    must persist it through their ChangeSet/transaction boundary.
    """
    current = asdict(obligation) if isinstance(obligation, DesignObligation) else dict(obligation)
    current_status = str(current.get("status", "open"))
    if target_status not in OBLIGATION_TRANSITIONS.get(current_status, set()):
        raise WorkflowError(f"OBLIGATION_TRANSITION_INVALID:{current_status}->{target_status}")
    if not actor or actor.startswith("agent:"):
        raise WorkflowError("OBLIGATION_AGENT_TRANSITION_FORBIDDEN")
    revision = str(current.get("revision", "1"))
    if expected_revision is not None and revision != str(expected_revision):
        raise WorkflowError("OBLIGATION_REVISION_CONFLICT")
    if str(base_revision) != revision:
        raise WorkflowError("OBLIGATION_BASE_REVISION_CONFLICT")
    updated = dict(current)
    updated["status"] = target_status
    updated["revision"] = str(int(revision) + 1)
    updated["updated_by"] = actor
    updated["evidence"] = dict(evidence or current.get("evidence") or {})
    updated["transition_hash"] = _hash({"before": current, "after": updated, "actor": actor})
    return updated


def issue_acceptance_token(obligation: DesignObligation | dict[str, Any], *, actor: str, role: str, resolution_hash: str, source_hashes: dict[str, str]) -> dict[str, Any]:
    """Issue a human-bound token for evaluator acceptance, never by an agent."""
    current = asdict(obligation) if isinstance(obligation, DesignObligation) else dict(obligation)
    if role not in {"SA", "Dev", "BA", "SA/Dev"} or not actor or actor.startswith("agent:"):
        raise WorkflowError("OBLIGATION_ACCEPTANCE_HUMAN_REQUIRED")
    if current.get("status") not in {"verified", "accepted", "not_applicable"}:
        raise WorkflowError("OBLIGATION_NOT_VERIFIED")
    payload = {"obligation_id": current.get("id"), "revision": current.get("revision", "1"), "resolution_hash": resolution_hash, "source_hashes": dict(sorted(source_hashes.items())), "actor": actor, "role": role}
    return {"token_id": "TOK-" + _hash(payload)[:16], "payload": payload, "token_hash": _hash(payload), "status": "issued"}


def verify_acceptance_token(token: dict[str, Any], obligation: DesignObligation | dict[str, Any], *, resolution_hash: str, source_hashes: dict[str, str]) -> list[str]:
    current = asdict(obligation) if isinstance(obligation, DesignObligation) else dict(obligation)
    errors = []
    payload = token.get("payload", {})
    if token.get("status") != "issued": errors.append("ACCEPTANCE_TOKEN_NOT_ISSUED")
    if payload.get("obligation_id") != current.get("id") or str(payload.get("revision")) != str(current.get("revision", "1")): errors.append("ACCEPTANCE_TOKEN_OBLIGATION_STALE")
    if payload.get("resolution_hash") != resolution_hash: errors.append("ACCEPTANCE_TOKEN_RESOLUTION_STALE")
    if payload.get("source_hashes") != dict(sorted(source_hashes.items())): errors.append("ACCEPTANCE_TOKEN_SOURCE_STALE")
    if token.get("token_hash") != _hash(payload): errors.append("ACCEPTANCE_TOKEN_TAMPERED")
    return sorted(set(errors))


def _hash(value: Any) -> str:
    return sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def _content_hash(content: Any) -> str:
    if isinstance(content, bytes):
        return sha256(content).hexdigest()
    if isinstance(content, str):
        return sha256(content.encode()).hexdigest()
    return _hash(content)


def _nonempty(value: Any) -> bool:
    return value not in (None, "", [], {}, ())


def validate_sad_readonly(before: Any, after: Any) -> list[str]:
    """SAD is a constraint in this flow; any mutation requires a SAD CR."""
    return [] if _hash(before) == _hash(after) else ["SAD_MUTATION_REQUIRES_SEPARATE_CR"]


@dataclass(frozen=True)
class SectionRef:
    document_id: str
    role: str
    revision: str
    content_hash: str
    section_id: str
    display: str = ""
    evidence: tuple[str, ...] = ()

    def validate(self, expected_role: str | None = None) -> list[str]:
        errors = []
        for name in ("document_id", "role", "revision", "content_hash", "section_id"):
            if not _nonempty(getattr(self, name)):
                errors.append(f"SECTION_REF_MISSING:{name}")
        if expected_role and self.role != expected_role:
            errors.append(f"SECTION_REF_ROLE:{self.section_id}:{self.role}!={expected_role}")
        return errors

    @classmethod
    def from_dict(cls, value: dict[str, Any], expected_role: str | None = None) -> "SectionRef":
        ref = cls(
            document_id=str(value.get("document_id", "")),
            role=str(value.get("role", "")),
            revision=str(value.get("revision", "")),
            content_hash=str(value.get("content_hash", value.get("hash", ""))),
            section_id=str(value.get("section_id", value.get("id", ""))),
            display=str(value.get("display", value.get("display_number", ""))),
            evidence=tuple(value.get("evidence", ()) or ()),
        )
        errors = ref.validate(expected_role)
        if errors:
            raise WorkflowError(";".join(errors))
        return ref


@dataclass(frozen=True)
class DocumentRef:
    document_id: str
    role: str
    revision: str
    content_hash: str

    def validate(self, expected_role: str | None = None) -> list[str]:
        errors = [f"DOCUMENT_REF_MISSING:{x}" for x in ("document_id", "role", "revision", "content_hash") if not _nonempty(getattr(self, x))]
        if expected_role and self.role != expected_role:
            errors.append(f"DOCUMENT_REF_ROLE:{self.document_id}:{self.role}!={expected_role}")
        return errors

    @classmethod
    def from_dict(cls, value: dict[str, Any], expected_role: str | None = None) -> "DocumentRef":
        ref = cls(str(value.get("document_id", value.get("id", ""))), str(value.get("role", "")), str(value.get("revision", "")), str(value.get("content_hash", value.get("hash", ""))))
        errors = [f"DOCUMENT_REF_MISSING:{x}" for x in ("document_id", "role", "revision", "content_hash") if not _nonempty(getattr(ref, x))]
        if expected_role and ref.role != expected_role:
            errors.append(f"DOCUMENT_REF_ROLE:{ref.document_id}:{ref.role}!={expected_role}")
        if errors:
            raise WorkflowError(";".join(errors))
        return ref


@dataclass
class ImpactEntry:
    tdd_id: str
    action: str
    rationale: str
    srs_refs: list[dict[str, Any]] = field(default_factory=list)
    tdd_refs: list[dict[str, Any]] = field(default_factory=list)
    evidence: list[dict[str, Any]] = field(default_factory=list)
    status: str = "pending"

    def validate(self) -> list[str]:
        errors = []
        if self.action not in {"add", "update", "no_change"}:
            errors.append(f"INVALID_ACTION:{self.tdd_id}:{self.action}")
        if not self.rationale:
            errors.append(f"MISSING_RATIONALE:{self.tdd_id}")
        if not self.srs_refs:
            errors.append(f"MISSING_SRS_REF:{self.tdd_id}")
        if self.action == "no_change" and not self.tdd_refs:
            errors.append(f"NO_CHANGE_MISSING_TDD_REF:{self.tdd_id}")
        if self.action in {"update", "no_change"} and self.tdd_id in ("", None):
            errors.append("MISSING_BASELINE_TDD_ID")
        return errors


@dataclass
class ImpactReport:
    id: str
    cr_ref: dict[str, Any]
    srs_ref: dict[str, Any]
    sad_ref: dict[str, Any]
    baseline_tdd_snapshot: str
    entries: list[ImpactEntry]
    findings: list[dict[str, Any]] = field(default_factory=list)
    status: str = "pending_approval"
    approval: dict[str, Any] | None = None

    def decision_payload(self) -> dict[str, Any]:
        data = asdict(self)
        data["approval"] = None
        return data

    @property
    def decision_hash(self) -> str:
        return _hash(self.decision_payload())

    def validate(self) -> list[str]:
        errors = []
        for label, ref in (("CR", self.cr_ref), ("SRS", self.srs_ref), ("SAD", self.sad_ref)):
            if not isinstance(ref, dict) or not _nonempty(ref.get("document_id", ref.get("id"))) or not _nonempty(ref.get("revision")):
                errors.append(f"MISSING_{label}_REF")
        if not self.baseline_tdd_snapshot:
            errors.append("MISSING_TDD_BASELINE")
        for entry in self.entries:
            errors.extend(entry.validate())
        if self.status == "approved":
            if not self.approval or self.approval.get("decision_hash") != self.decision_hash:
                errors.append("INVALID_IMPACT_APPROVAL")
        return sorted(set(errors))


@dataclass
class TddSetRealization:
    baseline_snapshot: str
    actual_snapshot: str
    mappings: list[dict[str, Any]]
    status: str = "pending"

    def validate(self, report: ImpactReport | None = None) -> list[str]:
        errors = []
        if not self.baseline_snapshot or not self.actual_snapshot:
            errors.append("MISSING_REALIZATION_SNAPSHOT")
        if report and self.baseline_snapshot != report.baseline_tdd_snapshot:
            errors.append("REALIZATION_BASELINE_MISMATCH")
        if report:
            expected = {entry.tdd_id: entry.action for entry in report.entries}
            actual = {str(item.get("tdd_id")): item for item in self.mappings}
            for tdd_id in sorted(set(expected) - set(actual)):
                errors.append(f"REALIZATION_MISSING_TDD:{tdd_id}")
            for tdd_id in sorted(set(actual) - set(expected)):
                errors.append(f"REALIZATION_UNPLANNED_TDD:{tdd_id}")
            for tdd_id, action in expected.items():
                if tdd_id in actual and actual[tdd_id].get("action") != action:
                    errors.append(f"REALIZATION_ACTION_MISMATCH:{tdd_id}")
                if tdd_id in actual:
                    expected_sections = {str(ref.get("section_id", ref.get("id", ""))) for entry in report.entries if entry.tdd_id == tdd_id for ref in entry.tdd_refs}
                    actual_sections = {str(ref.get("section_id", ref.get("id", ""))) for ref in actual[tdd_id].get("actual_sections", [])}
                    if expected_sections and not expected_sections.issubset(actual_sections):
                        errors.append(f"REALIZATION_PLANNED_ANCHOR_MISSING:{tdd_id}")
        for mapping in self.mappings:
            if mapping.get("action") not in {"add", "update", "no_change"}:
                errors.append(f"INVALID_REALIZATION_ACTION:{mapping.get('tdd_id')}")
            if not mapping.get("actual_sections"):
                errors.append(f"MISSING_ACTUAL_TDD_SECTIONS:{mapping.get('tdd_id')}")
            if mapping.get("action") == "no_change" and mapping.get("before_revision") != mapping.get("after_revision"):
                errors.append(f"NO_CHANGE_REVISION_CHANGED:{mapping.get('tdd_id')}")
            if mapping.get("action") == "no_change" and not mapping.get("design_reuse_proof"):
                errors.append(f"NO_CHANGE_DESIGN_PROOF_MISSING:{mapping.get('tdd_id')}")
            if mapping.get("action") in {"update", "no_change"} and not mapping.get("before_revision"):
                errors.append(f"MISSING_BEFORE_TDD_REVISION:{mapping.get('tdd_id')}")
            if mapping.get("action") in {"add", "update"} and not mapping.get("after_revision"):
                errors.append(f"MISSING_AFTER_TDD_REVISION:{mapping.get('tdd_id')}")
        return sorted(set(errors))


def _ref(value: Any, role: str) -> dict[str, Any]:
    if isinstance(value, SectionRef):
        return asdict(value)
    if isinstance(value, DocumentRef):
        return asdict(value)
    if not isinstance(value, dict):
        raise WorkflowError(f"INVALID_{role}_REF")
    result = dict(value)
    result.setdefault("role", role)
    if not result.get("document_id") and result.get("id"):
        result["document_id"] = result["id"]
    return result


def assess_tdd_impact(srs_delta: dict[str, Any], tdd_inventory: Iterable[dict[str, Any]], sad: dict[str, Any], *, report_id: str = "impact-1") -> ImpactReport:
    """Classify every baseline TDD and every unmatched SRS change.

    The input intentionally accepts plain dicts so it can consume KG exports,
    while the returned report is typed and serializable. Explicit ``tdd_ids``
    or ``covered_srs_ids`` are preferred; no match is a finding, never an
    implicit no_change.
    """
    srs_ref = _ref(srs_delta.get("ref", srs_delta), "SRS")
    cr_ref = _ref(srs_delta.get("cr_ref", srs_delta.get("change_request", {})), "CR")
    sad_ref = _ref(sad.get("ref", sad), "SAD")
    changes = list(srs_delta.get("changes", srs_delta.get("requirements", [])) or [])
    if not changes:
        changes = [{"id": x} for x in srs_delta.get("changed_ids", [])]
    entries: list[ImpactEntry] = []
    covered: set[str] = set()
    inventory = list(tdd_inventory)
    for tdd in inventory:
        tdd_id = str(tdd.get("id", ""))
        covered_ids = set(tdd.get("covered_srs_ids", tdd.get("srs_ids", [])) or [])
        explicit = set(tdd.get("affected_srs_ids", []) or [])
        hits = [c for c in changes if c.get("id") in covered_ids or c.get("id") in explicit or tdd_id in (c.get("tdd_ids", []) or [])]
        if not hits:
            # An untouched TDD is still represented, but with a neutral source ref.
            hits = [{"id": "SRS-unchanged", "section_ref": srs_ref}]
            action = "no_change"
        else:
            covered.update(str(c.get("id")) for c in hits if c.get("id") != "SRS-unchanged")
            action = "update" if any(c.get("kind", c.get("action", "modified")) in {"modified", "removed", "renamed", "update"} for c in hits) else "no_change"
            if any(c.get("kind", c.get("action")) in {"added", "add"} and (not c.get("parameter_only") or c.get("within_bounds") is False) for c in hits):
                action = "update"
        refs = [dict(c.get("section_ref", c.get("ref", srs_ref))) for c in hits]
        trefs = list(tdd.get("sections", tdd.get("section_refs", [])) or [])
        entries.append(ImpactEntry(tdd_id=tdd_id, action=action, rationale="; ".join(str(c.get("rationale", "reuse existing design")) for c in hits), srs_refs=refs, tdd_refs=trefs, evidence=list(tdd.get("evidence", []))))
    missing = [c for c in changes if c.get("id") not in covered]
    for change in missing:
        if change.get("id") == "SRS-unchanged":
            continue
        proposed_id = str(change.get("new_tdd_id", "TDD-NEW-" + str(change.get("id"))))
        entries.append(ImpactEntry(tdd_id=proposed_id, action="add", rationale=str(change.get("rationale", "no existing design covers SRS change")), srs_refs=[dict(change.get("section_ref", change.get("ref", srs_ref)))], tdd_refs=[], evidence=list(change.get("evidence", []))))
    findings = []
    if not changes:
        findings.append({"code": "SRS_DELTA_EMPTY", "message": "No SRS delta was supplied; impact cannot be inferred."})
    if sad.get("architecture_change") or srs_delta.get("architecture_change"):
        findings.append({"code": "SAD_CHANGE_REQUIRED", "message": "Architecture change requires a separate SAD CR."})
    if srs_delta.get("conflicts"):
        findings.append({"code": "SRS_CONFLICT", "message": "SRS contains unresolved conflicting statements.", "evidence": srs_delta.get("conflicts")})
    for tdd in inventory:
        if tdd.get("conflicts"):
            findings.append({"code": "TDD_CONFLICT", "tdd_id": tdd.get("id"), "message": "TDD has unresolved conflicting design evidence.", "evidence": tdd.get("conflicts")})
    report = ImpactReport(report_id, cr_ref, srs_ref, sad_ref, str(srs_delta.get("baseline_tdd_snapshot", srs_delta.get("tdd_snapshot", ""))), entries, findings)
    return report


def approve_impact(report: ImpactReport, approver_id: str, role: str, *, actor: str = "human") -> ImpactReport:
    if actor != "human":
        raise WorkflowError("AGENT_SELF_APPROVAL_FORBIDDEN")
    if role not in {"SA", "Dev", "SA/Dev"} or not approver_id:
        raise WorkflowError("IMPACT_APPROVER_ROLE_REQUIRED")
    errors = report.validate()
    if report.findings:
        errors.append("IMPACT_FINDINGS_REQUIRE_RESOLUTION")
    if errors:
        raise WorkflowError(";".join(errors))
    report.status = "approved"
    report.approval = {"approver_id": approver_id, "role": role, "decision_hash": report.decision_hash}
    return report


def realize_tdd_set(report: ImpactReport, mappings: list[dict[str, Any]], actual_snapshot: str) -> TddSetRealization:
    if report.status != "approved":
        raise WorkflowError("IMPACT_APPROVAL_REQUIRED")
    realization = TddSetRealization(report.baseline_tdd_snapshot, actual_snapshot, mappings, "materialized")
    errors = realization.validate(report)
    if errors:
        raise WorkflowError(";".join(errors))
    return realization


@dataclass
class ChangeSpecItem:
    id: str
    action: str
    code_target: str
    srs_refs: list[dict[str, Any]]
    tdd_refs: list[dict[str, Any]]
    verification: str
    cr_acceptance_refs: list[str]

    def validate(self) -> list[str]:
        errors = []
        if self.action not in {"add", "modify", "remove"}:
            errors.append(f"INVALID_CS_ACTION:{self.id}")
        for name, value in (("code_target", self.code_target), ("verification", self.verification), ("srs_refs", self.srs_refs), ("tdd_refs", self.tdd_refs), ("cr_acceptance_refs", self.cr_acceptance_refs)):
            if not _nonempty(value): errors.append(f"CS_{name.upper()}_MISSING:{self.id}")
        for ref in self.srs_refs:
            if ref.get("role") not in (None, "SRS"):
                errors.append(f"CS_SRS_ROLE_INVALID:{self.id}")
            for field in ("document_id", "revision", "content_hash", "section_id"):
                if not ref.get(field):
                    errors.append(f"CS_SRS_REF_MISSING_{field.upper()}:{self.id}")
        for ref in self.tdd_refs:
            if ref.get("role") not in (None, "TDD"):
                errors.append(f"CS_TDD_ROLE_INVALID:{self.id}")
            for field in ("document_id", "revision", "content_hash", "section_id"):
                if not ref.get(field):
                    errors.append(f"CS_TDD_REF_MISSING_{field.upper()}:{self.id}")
        return errors


def build_change_spec(srs_delta: dict[str, Any], tdd_set: dict[str, Any] | TddSetRealization, sad: dict[str, Any], impact: ImpactReport, cr: dict[str, Any] | None = None) -> dict[str, Any]:
    """Build the canonical stage-5 table; OpenSpec is a later projection."""
    if impact.status != "approved":
        raise WorkflowError("IMPACT_APPROVAL_REQUIRED")
    errors = impact.validate()
    if errors:
        raise WorkflowError(";".join(errors))
    if sad.get("architecture_change"):
        raise WorkflowError("SAD_CHANGE_REQUIRES_SEPARATE_CR")
    realized = tdd_set if isinstance(tdd_set, dict) else asdict(tdd_set)
    if realized.get("status") not in {"materialized", "approved", "realized"}:
        raise WorkflowError("TDD_REALIZATION_REQUIRED")
    maps = {str(x.get("tdd_id")): x for x in realized.get("mappings", [])}
    changes = list(srs_delta.get("changes", srs_delta.get("requirements", [])) or [])
    source_for_criteria = cr or {}
    criteria = source_for_criteria.get("acceptance", source_for_criteria.get("acceptance_criteria", [])) or srs_delta.get("acceptance", srs_delta.get("acceptance_criteria", [])) or []
    if isinstance(criteria, str): criteria = [criteria]
    items: list[ChangeSpecItem] = []
    for i, change in enumerate(changes, 1):
        tdd_ids = list(change.get("tdd_ids", []) or [])
        if not tdd_ids:
            tdd_ids = [e.tdd_id for e in impact.entries if any(r.get("section_id") == change.get("id") for r in e.srs_refs)]
        refs = [maps[x].get("actual_sections", []) for x in tdd_ids if x in maps]
        tdd_refs = [r for group in refs for r in group]
        srs_refs = [dict(change.get("section_ref", change.get("ref", impact.srs_ref)))]
        action = str(change.get("code_action", change.get("action", "modify")))
        if action == "update": action = "modify"
        item = ChangeSpecItem(f"CS-{i:03d}", action, str(change.get("code_target", "")), srs_refs, tdd_refs, str(change.get("verification", change.get("check", ""))), list(change.get("cr_acceptance_refs", criteria)))
        item_errors = item.validate()
        if item_errors: errors.extend(item_errors)
        items.append(item)
    if not changes:
        errors.append("SRS_DELTA_HAS_NO_CODE_ITEMS")
    return {"status": "eligible" if not errors else "blocked", "items": [asdict(x) for x in items if not x.validate()], "pending": [asdict(x) for x in items if x.validate()], "errors": sorted(set(errors)), "sources": {"cr": cr or impact.cr_ref, "srs": impact.srs_ref, "sad": sad, "impact": impact.decision_hash, "tdd_set": realized.get("actual_snapshot")}}


def validate_workflow_context(context: dict[str, Any]) -> list[str]:
    """Cheap preflight used by spec generation and publication."""
    required = ("cr", "srs_delta", "sad", "impact", "tdd_set")
    errors = [f"WORKFLOW_CONTEXT_MISSING:{x}" for x in required if not context.get(x)]
    impact = context.get("impact")
    if impact and ((isinstance(impact, dict) and impact.get("status") != "approved") or (not isinstance(impact, dict) and getattr(impact, "status", None) != "approved")):
        errors.append("IMPACT_APPROVAL_REQUIRED")
    if context.get("sad", {}).get("architecture_change"):
        errors.append("SAD_CHANGE_REQUIRES_SEPARATE_CR")
    if impact:
        findings = impact.get("findings", []) if isinstance(impact, dict) else getattr(impact, "findings", [])
        if findings:
            errors.append("IMPACT_FINDINGS_REQUIRE_RESOLUTION")
        approval = impact.get("approval") if isinstance(impact, dict) else getattr(impact, "approval", None)
        if isinstance(impact, dict) and impact.get("status") == "approved" and not approval:
            errors.append("INVALID_IMPACT_APPROVAL")
    if "sad_before" in context and "sad_after" in context:
        errors.extend(validate_sad_readonly(context["sad_before"], context["sad_after"]))
    tdd_set = context.get("tdd_set", {})
    tdd_status = tdd_set.get("status") if isinstance(tdd_set, dict) else getattr(tdd_set, "status", None)
    if tdd_status not in {"materialized", "approved", "realized"}:
        errors.append("TDD_REALIZATION_REQUIRED")
    if context.get("documents"):
        errors.extend(validate_document_freshness(context.get("documents", []), context.get("source_documents", [])))
    if context.get("tdd_sources"):
        errors.extend(validate_realization_freshness(context.get("tdd_set", {}), context.get("tdd_sources", [])))
    if context.get("profile") and context["profile"].get("status") == "blocked":
        errors.append("SPECIALIST_PROFILE_COMPOSITION_BLOCKED")
    if context.get("profile") and context["profile"].get("hash") and context["profile"].get("expected_hash"):
        if context["profile"]["hash"] != context["profile"]["expected_hash"]:
            errors.append("SPECIALIST_PROFILE_HASH_STALE")
    return sorted(set(errors))


def validate_document_freshness(refs: Iterable[dict[str, Any]], sources: Iterable[dict[str, Any]]) -> list[str]:
    """Resolve document IDs, roles, revisions and hashes before authoring."""
    by_id = {str(source.get("id", source.get("document_id", ""))): source for source in sources}
    errors = []
    for raw in refs:
        if isinstance(raw, SectionRef):
            ref = asdict(raw)
        else:
            ref = dict(raw)
        ident = str(ref.get("document_id", ref.get("id", "")))
        source = by_id.get(ident)
        if not source:
            errors.append(f"SOURCE_NOT_FOUND:{ident}")
            continue
        if str(ref.get("revision", "")) != str(source.get("revision", source.get("version", ""))):
            errors.append(f"SOURCE_VERSION_MISMATCH:{ident}")
        expected = ref.get("content_hash", ref.get("hash"))
        actual = source.get("content_hash") or _content_hash(source.get("content", ""))
        if expected and expected != actual:
            errors.append(f"SOURCE_HASH_MISMATCH:{ident}")
        if ref.get("role") and source.get("role", source.get("kind")) and ref["role"] != source.get("role", source.get("kind")):
            errors.append(f"SOURCE_ROLE_MISMATCH:{ident}")
        sections = source.get("sections")
        if sections and ref.get("section_id"):
            section_ids = {str(x.get("id", x.get("section_id", ""))) for x in sections if isinstance(x, dict)}
            aliases = {str(x.get("renamed_from")): str(x.get("id", x.get("section_id", ""))) for x in sections if isinstance(x, dict) and x.get("renamed_from")}
            section_id = str(ref.get("section_id"))
            if section_id not in section_ids and section_id not in aliases:
                errors.append(f"SECTION_NOT_FOUND:{ident}:{section_id}")
            if ref.get("evidence") and isinstance(source.get("content"), str):
                for quote in ref.get("evidence", []):
                    if quote and quote not in source["content"]: errors.append(f"SECTION_EVIDENCE_NOT_FOUND:{ident}:{section_id}")
    return sorted(set(errors))


def validate_realization_freshness(realization: TddSetRealization | dict[str, Any], sources: Iterable[dict[str, Any]]) -> list[str]:
    """Validate actual post-step-4 TDD sections against pinned documents."""
    value = asdict(realization) if isinstance(realization, TddSetRealization) else realization
    refs = [ref for mapping in value.get("mappings", []) for ref in mapping.get("actual_sections", [])]
    return validate_document_freshness(refs, sources)


def validate_change_spec_coverage(change_spec: dict[str, Any], test_receipts: Iterable[dict[str, Any]] = ()) -> dict[str, Any]:
    """Check semantic CR→SRS→TDD→CS→test closure without ID-only shortcuts."""
    errors = list(change_spec.get("errors", []))
    items = list(change_spec.get("items", []))
    receipts = list(test_receipts)
    by_cs = {str(x.get("cs_id", x.get("change_spec_id", ""))): x for x in receipts}
    for item in items:
        ident = item.get("id", "")
        for field in ("srs_refs", "tdd_refs", "cr_acceptance_refs", "verification"):
            if not item.get(field):
                errors.append(f"COVERAGE_MISSING_{field.upper()}:{ident}")
        receipt = by_cs.get(ident)
        if not receipt:
            errors.append(f"COVERAGE_MISSING_TEST:{ident}")
        elif receipt.get("result") not in {"pass", "passed", "accepted"}:
            errors.append(f"COVERAGE_TEST_NOT_PASSED:{ident}")
        elif not receipt.get("executed_at"):
            errors.append(f"COVERAGE_TEST_EXECUTION_TIME_MISSING:{ident}")
        elif not receipt.get("cs_version") or not receipt.get("cr_version"):
            errors.append(f"COVERAGE_RECEIPT_VERSION_MISSING:{ident}")
        elif receipt.get("criterion_refs") and not set(item.get("cr_acceptance_refs", [])).issubset(set(receipt["criterion_refs"])):
            errors.append(f"COVERAGE_CR_CRITERION_MISMATCH:{ident}")
    return {"status": "complete" if not errors else "blocked", "errors": sorted(set(errors)), "covered": sorted(by_cs)}


def validate_impact_consistency(graph: dict[str, Any], impacted_ids: Iterable[str]) -> list[str]:
    """Ensure semantic edits/CRs carry implementation and test closure."""
    impacted = {str(x) for x in impacted_ids}; by_id = {str(x.get("id")): x for x in graph.get("elements", [])}
    outgoing: dict[str, list[dict[str, Any]]] = {}
    for edge in graph.get("edges", []): outgoing.setdefault(str(edge.get("src")), []).append(edge)
    errors = []
    for ident in sorted(impacted):
        node = by_id.get(ident, {}); cls = node.get("class")
        if cls not in {"Requirement", "RequirementVersion", "Operation"}: continue
        targets = outgoing.get(ident, [])
        if not any(edge.get("rel") == "implemented_by" and by_id.get(str(edge.get("dst")), {}).get("class") == "WorkItem" for edge in targets):
            errors.append(f"IMPACT_WORK_MISSING:{ident}")
        if not any(edge.get("rel") == "verified_by" and by_id.get(str(edge.get("dst")), {}).get("class") == "TestCase" for edge in targets):
            errors.append(f"IMPACT_TEST_MISSING:{ident}")
    return errors


def pre_authoring_gate(context: dict[str, Any], stage: str = "spec") -> dict[str, Any]:
    """Single gate shared by dynamic, prompt and stage-5 authoring APIs."""
    errors = validate_workflow_context(context or {})
    if stage == "spec" and context and context.get("change_spec"):
        errors.extend(context["change_spec"].get("errors", []))
    return {"status": "approved" if not errors else "blocked", "stage": stage, "errors": sorted(set(errors)), "context_hash": _hash(context)}


def tdd_inventory_manifest(tdd_inventory: Iterable[dict[str, Any]]) -> dict[str, Any]:
    """Pin the complete baseline TDD inventory used by impact assessment."""
    items = []
    for tdd in tdd_inventory:
        items.append({"id": str(tdd.get("id", "")), "revision": str(tdd.get("revision", "")), "content_hash": tdd.get("content_hash", tdd.get("hash", "")), "section_ids": sorted(str(x.get("section_id", x.get("id", ""))) for x in tdd.get("sections", tdd.get("section_refs", [])) or [])})
    items.sort(key=lambda x: x["id"])
    return {"ids": [x["id"] for x in items], "items": items, "snapshot": _hash(items)}


def validate_realized_inventory(manifest: dict[str, Any], realization: TddSetRealization | dict[str, Any]) -> list[str]:
    """Detect stale baseline selection and unplanned TDD drift."""
    actual = asdict(realization) if isinstance(realization, TddSetRealization) else realization
    errors = []
    expected_ids = set(manifest.get("ids", []))
    actual_ids = {str(x.get("tdd_id", "")) for x in actual.get("mappings", []) if x.get("action") != "add"}
    if expected_ids != actual_ids:
        errors.append("TDD_INVENTORY_DRIFT")
    planned_added = set(manifest.get("planned_added_ids", []))
    for mapping in actual.get("mappings", []):
        if mapping.get("action") == "add" and str(mapping.get("tdd_id")) not in planned_added:
            errors.append("TDD_INVENTORY_DRIFT")
    if not actual.get("actual_snapshot"):
        errors.append("MISSING_REALIZED_SNAPSHOT")
    baseline_by_id = {str(item.get("id")): item for item in manifest.get("items", [])}
    for mapping in actual.get("mappings", []):
        ident = str(mapping.get("tdd_id")); baseline = baseline_by_id.get(ident, {})
        action = mapping.get("action")
        if action in {"update", "no_change"} and baseline.get("revision") and str(mapping.get("before_revision")) != str(baseline.get("revision")):
            errors.append(f"TDD_BASELINE_REVISION_STALE:{ident}")
        if action == "no_change" and mapping.get("after_revision") != mapping.get("before_revision"):
            errors.append(f"TDD_NO_CHANGE_REVISION_DRIFT:{ident}")
        if action == "update" and mapping.get("after_revision") and mapping.get("before_revision") and mapping.get("after_revision") == mapping.get("before_revision"):
            errors.append(f"TDD_UPDATE_NOT_REALIZED:{ident}")
    return sorted(set(errors))
