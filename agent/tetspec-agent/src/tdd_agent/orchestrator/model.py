"""Orchestrator data model: flows, gates, outcomes and approval requests."""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Protocol

# Roles allowed to approve each gate (G1 BA, G2 SA/Dev, G5 SA, G7 BA/owner).
GATE_APPROVAL_ROLES: dict[str, tuple[str, ...]] = {
    "G1": ("BA",),
    "G2": ("SA/Dev",),
    "G5": ("SA",),
    "G7": ("BA/owner",),
}
STATUSES = ("running", "awaiting_approval", "awaiting_evidence", "blocked", "completed", "failed")
STATE_ORDER = ("intake", "srs_ready", "impact_proposed", "impact_approved", "tdd_realized", "spec_eligible", "testspec_dispatched",
               "spec_authored", "implemented", "verified", "accepted")


@dataclass
class GateOutcome:
    """Result of a gate executor.

    ``status``: ``ok`` (done), ``blocked`` (machine-readable ``blockers``) or
    ``pending_evidence`` (needs data recorded outside the run, e.g. test receipts).
    ``decision`` is what a human/agent approver reviews; its hash is bound to the approval.
    """
    status: str
    output: dict = field(default_factory=dict)
    blockers: list[str] = field(default_factory=list)
    decision: dict | None = None
    evidence: dict = field(default_factory=dict)

    @staticmethod
    def ok(output: dict | None = None, decision: dict | None = None, evidence: dict | None = None) -> "GateOutcome":
        return GateOutcome("ok", output or {}, [], decision, evidence or {})

    @staticmethod
    def blocked(*blockers: str, output: dict | None = None, evidence: dict | None = None) -> "GateOutcome":
        return GateOutcome("blocked", output or {}, list(dict.fromkeys(blockers)), None, evidence or {})

    @staticmethod
    def pending_evidence(*blockers: str, output: dict | None = None) -> "GateOutcome":
        return GateOutcome("pending_evidence", output or {}, list(dict.fromkeys(blockers)), None, {})


Executor = Callable[["GateContext"], GateOutcome]  # noqa: F821 - defined in context.py


@dataclass(frozen=True)
class GateDef:
    """One gate of a flow.

    ``approval`` names the approver role required *after* the executor succeeds
    (``None`` = automatic).  ``state_after`` is the run state once the gate passes.
    ``extra_inputs`` lets a gate add values to its ledger input hash (skill hashes,
    profile versions, ...).
    """
    name: str
    state_after: str
    executor: Executor
    approval: str | None = None
    depends_on: tuple[str, ...] | None = None  # None = every earlier gate, () = none
    extra_inputs: Callable[["GateContext"], dict] | None = None  # noqa: F821
    evidence_gate: bool = False
    on_approved: Callable[["GateContext", dict], None] | None = None  # noqa: F821 - runs after an accepted approval (e.g. publication)
    source_roles: tuple[str, ...] | None = None  # roles whose pinned revision/hash feed the input hash (None = all)
    ignore_params: tuple[str, ...] = ()  # request params this gate does not read: changing them must not make it (and its approval) stale
    config_sections: tuple[str, ...] = ("llm", "render", "retrieval")  # the configuration this gate reads: changing any other section (who may approve, the project name, logging) leaves it valid


@dataclass(frozen=True)
class FlowDef:
    name: str
    gates: tuple[GateDef, ...]
    initial_state: str = "intake"

    def gate(self, name: str) -> GateDef:
        for g in self.gates:
            if g.name == name:
                return g
        raise KeyError(name)

    def index(self, name: str) -> int:
        return [g.name for g in self.gates].index(name)

    def state_rank(self, state: str) -> int:
        """Position of ``state`` along the flow (initial state = 0, after gate i = i + 1)."""
        if state == self.initial_state:
            return 0
        for i, g in enumerate(self.gates):
            if g.state_after == state:
                return i + 1
        return 0

    def dependencies(self, gate: "GateDef") -> tuple[str, ...]:
        if gate.depends_on is not None:
            return gate.depends_on
        return tuple(g.name for g in self.gates[:self.index(gate.name)])

    def dependents(self, names: set[str]) -> list["GateDef"]:
        """Gates that transitively depend on any gate in ``names`` (flow order)."""
        out: list[GateDef] = []
        seen = set(names)
        for g in self.gates:
            if g.name in seen:
                continue
            if set(self.dependencies(g)) & seen:
                seen.add(g.name); out.append(g)
        return out


@dataclass
class ApprovalRequest:
    run_id: str
    gate: str
    actor_id: str
    role: str
    decision_hash: str
    rationale: str
    claimed_kind: str | None = None
    channel: str = "cli"


@dataclass
class ApprovalResult:
    accepted: bool
    code: str
    message: str = ""
    run_status: str | None = None
    record: dict | None = None
