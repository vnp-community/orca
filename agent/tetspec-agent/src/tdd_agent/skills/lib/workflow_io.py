"""Dict <-> o5 workflow dataclasses (ImpactReport etc.)."""
from __future__ import annotations

from dataclasses import asdict
from typing import Any

from ...core import o5_path


def impact_from_dict(data: dict[str, Any]):
    wf = o5_path.load().workflow
    entries = [wf.ImpactEntry(**{k: v for k, v in e.items() if k in wf.ImpactEntry.__dataclass_fields__}) for e in data.get("entries", [])]
    fields = {k: v for k, v in data.items() if k in wf.ImpactReport.__dataclass_fields__ and k != "entries"}
    return wf.ImpactReport(entries=entries, **fields)


def impact_to_dict(report) -> dict[str, Any]:
    return asdict(report)


def finalize_impact_approval(report, approval: dict[str, Any]):
    """Bind an accepted approval to the report (status ``approved``).

    o5's own ``approve_impact`` refuses non-human actors by design; the runtime's
    policy (D13) lets allowlisted coding agents approve too, so the approval is
    recorded here with its grade and the report's own decision hash.
    """
    report.status = "approved"
    report.approval = {"approver_id": approval["actor_id"], "role": approval["role"], "decision_hash": report.decision_hash,
                       "approval_grade": approval.get("approval_grade"), "actor_kind": approval.get("actor_kind"),
                       "orchestrator_decision_hash": approval.get("decision_hash")}
    errors = report.validate()
    if report.findings:
        errors.append("IMPACT_FINDINGS_REQUIRE_RESOLUTION")
    return report, sorted(set(errors))
