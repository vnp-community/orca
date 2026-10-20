"""Approval policy (design D13).

Who may approve: actors in ``[[approvals.actors]]`` (humans and registered coding
agents) holding the gate's role.  Pipeline agents and skills never reach this
service: executors are not given a reference to it and the service also refuses
any call made inside an agent context.  An approval can only pass a gate whose
deterministic blockers are empty and whose decision hash the approver has read.
"""
from __future__ import annotations

from dataclasses import asdict

from ..config import Config
from ..obs import context
from ..obs.observer import Observer
from ..store.base import OperationalStore, now_iso
from .model import GATE_APPROVAL_ROLES, ApprovalRequest, ApprovalResult


class ApprovalService:
    def __init__(self, config: Config, operational: OperationalStore, observer: Observer):
        self.config = config
        self.store = operational
        self.observer = observer
        self.require_receipt = bool(config.get("approvals.require_review_receipt", True))

    # -- review receipts ----------------------------------------------------
    def record_receipt(self, run_id: str, actor_id: str, decision_hash: str, gate: str, source: str) -> dict:
        return self.store.add_receipt({"run_id": run_id, "actor_id": actor_id, "decision_hash": decision_hash, "gate": gate, "source": source})

    def _reject(self, req: ApprovalRequest, code: str, message: str) -> ApprovalResult:
        self.store.add_audit("approval_rejected", {"gate": req.gate, "actor": req.actor_id, "role": req.role, "code": code,
                                                   "message": message, "channel": req.channel}, req.run_id)
        self.observer.metrics.blocked(req.gate, code)
        self.observer.event("approval_rejected", gate=req.gate, actor=req.actor_id, code=code)
        return ApprovalResult(False, code, message)

    def check(self, req: ApprovalRequest, task: dict | None, run_status: str) -> ApprovalResult | None:
        """Return a rejection, or ``None`` when the request satisfies the policy."""
        if context.get("agent"):
            return self._reject(req, "PIPELINE_AGENT_CANNOT_APPROVE", "approval was attempted from inside a pipeline agent context")
        allowed = GATE_APPROVAL_ROLES.get(req.gate)
        if not allowed:
            return self._reject(req, "GATE_NOT_APPROVABLE", f"gate {req.gate} has no approval step")
        actor = next((a for a in self.config.actors() if a.id == req.actor_id), None)
        if actor is None:
            return self._reject(req, "ACTOR_NOT_ALLOWLISTED", f"{req.actor_id} is not an allowlisted approver")
        if req.claimed_kind and req.claimed_kind != actor.kind:
            return self._reject(req, "ACTOR_KIND_MISMATCH", f"{req.actor_id} is registered as {actor.kind}, not {req.claimed_kind}")
        if req.role not in actor.roles or req.role not in allowed:
            return self._reject(req, "ROLE_NOT_PERMITTED", f"role {req.role!r} cannot approve {req.gate} for {req.actor_id}")
        if not (req.rationale or "").strip():
            return self._reject(req, "RATIONALE_REQUIRED", "a non-empty rationale is required")
        if task is None or task.get("status") != "awaiting_approval":
            return self._reject(req, "NOT_AWAITING_APPROVAL", f"gate {req.gate} is not awaiting approval")
        if task.get("blockers") or run_status == "blocked":
            return self._reject(req, "BLOCKERS_PRESENT", "deterministic blockers remain; approval cannot override them")
        if req.decision_hash != task.get("decision_hash"):
            return self._reject(req, "DECISION_HASH_MISMATCH", "decision hash does not match the pending decision")
        if self.require_receipt and self.store.find_receipt(req.run_id, req.actor_id, req.decision_hash) is None:
            return self._reject(req, "REVIEW_RECEIPT_MISSING", "the approver has not read this decision (no review receipt)")
        return None

    def grade(self, actor_id: str) -> str:
        actor = next(a for a in self.config.actors() if a.id == actor_id)
        return "human" if actor.kind == "human" else "agent"

    AUTO_ACTOR = "tdd-agent:auto"

    def record_auto(self, run_id: str, gate: str, role: str, task: dict) -> dict:
        """The approval of a gate that no person is configured to approve: recorded as such (grade ``auto``), bound to the decision it covers."""
        record = {"run_id": run_id, "gate": gate, "actor_id": self.AUTO_ACTOR, "actor_kind": "automatic", "role": role, "approval_grade": "auto",
                  "decision_hash": task.get("decision_hash"), "rationale": f"auto mode: {gate} is not configured to need a person's approval (approvals.mode = auto)",
                  "channel": "auto", "task_input_hash": task.get("input_hash"), "ts": now_iso()}
        return self.store.add_approval(record)

    def record(self, req: ApprovalRequest, task: dict) -> dict:
        actor = next(a for a in self.config.actors() if a.id == req.actor_id)
        record = {"run_id": req.run_id, "gate": req.gate, "actor_id": actor.id, "actor_kind": actor.kind, "role": req.role,
                  "approval_grade": self.grade(actor.id), "decision_hash": req.decision_hash, "rationale": req.rationale,
                  "channel": req.channel, "task_input_hash": task.get("input_hash"), "ts": now_iso()}
        return self.store.add_approval(record)
