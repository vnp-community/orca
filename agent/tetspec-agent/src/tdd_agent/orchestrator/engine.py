"""Durable G0-G7 engine.

The engine owns run state; gate logic lives in executors.  Every gate execution
is recorded in the task ledger with an input hash: identical input => the stored
result is reused (no LLM call), changed input => the gate and everything after it
become ``stale`` and any approval on them is void.
"""
from __future__ import annotations

import subprocess
import threading
from typing import Any, Sequence

from ..config import Config
from ..errors import NotFound, TddAgentError
from ..llm.gateway import LlmGateway
from ..obs import context
from ..obs.observer import Observer
from ..store.base import KnowledgeStore, OperationalStore, RunRecord, SourceDoc, digest, now_iso
from .approvals import ApprovalService
from .context import ChildRuns, GateContext, GuardedKnowledge
from .model import ApprovalRequest, ApprovalResult, FlowDef, GateDef, GateOutcome

TERMINAL = ("completed", "failed")
# an interruption or a provider that did not answer says nothing about the run: it stops where it was and can be advanced again
TRANSIENT = (KeyboardInterrupt, TimeoutError, ConnectionError, subprocess.TimeoutExpired)


def _deep_merge(base: dict, patch: dict) -> dict:
    for key, value in patch.items():
        base[key] = _deep_merge(dict(base[key]), value) if isinstance(value, dict) and isinstance(base.get(key), dict) else value
    return base


class Orchestrator:
    def __init__(self, config: Config, knowledge: KnowledgeStore, operational: OperationalStore, gateway: LlmGateway,
                 observer: Observer, flows: dict[str, FlowDef]):
        self.config, self.knowledge, self.operational = config, knowledge, operational
        self.gateway, self.observer, self.flows = gateway, observer, dict(flows)
        self.approvals = ApprovalService(config, operational, observer)
        self._guarded = GuardedKnowledge(knowledge)
        self._locks: dict[str, threading.RLock] = {}
        self._guard = threading.Lock()

    # ------------------------------------------------------------------ helpers
    def _lock(self, run_id: str) -> threading.RLock:
        with self._guard:
            return self._locks.setdefault(run_id, threading.RLock())

    def _flow(self, run: RunRecord) -> FlowDef:
        if run.flow not in self.flows:
            raise NotFound(f"flow {run.flow!r} is not registered", code="FLOW_UNKNOWN")
        return self.flows[run.flow]

    def _pins(self, run: RunRecord, roles: tuple[str, ...] | None) -> list[dict]:
        pins = run.request.get("pins", [])
        return [p for p in pins if roles is None or p["role"] in roles]

    def _upstream(self, run: RunRecord, flow: FlowDef, gate: GateDef) -> dict[str, dict]:
        names = flow.dependencies(gate)
        out = {}
        for n in names:
            t = self.operational.get_task(run.run_id, n)
            if t and t.get("status") in ("completed", "approved") and "output" in t:
                out[n] = t["output"]
        return out

    def _input_hash(self, ctx: GateContext, gate: GateDef, flow: FlowDef) -> str:
        pins = [(p["id"], p["revision"], p["content_hash"]) for p in self._pins(ctx.run, gate.source_roles)]
        params = {k: v for k, v in ctx.run.request.get("params", {}).items() if k not in gate.ignore_params}
        base = {"gate": gate.name, "params": params, "mode": ctx.run.request.get("mode"), "pins": sorted(pins),
                "upstream": {k: digest(v) for k, v in sorted(ctx.upstream.items())}, "config": self.config.fingerprint(gate.config_sections)}
        if gate.extra_inputs:
            base["extra"] = gate.extra_inputs(ctx)
        return digest(base)

    def _ctx(self, run: RunRecord, flow: FlowDef, gate: GateDef) -> GateContext:
        sad = frozenset(p["content_hash"] for p in run.request.get("pins", []) if p["role"] == "SAD")
        children = ChildRuns(start=lambda child_flow, req: self.start(child_flow, req, parent_run_id=run.run_id).run_id,
                             advance=lambda child_id: self.advance(child_id).status)
        return GateContext(run=run, gate=gate.name, config=self.config, knowledge=GuardedKnowledge(self.knowledge, sad), operational=self.operational,
                           gateway=self.gateway, observer=self.observer, upstream=self._upstream(run, flow, gate), children=children)

    def _transition(self, run: RunRecord, to_state: str, gate: str, actor: str, evidence: dict) -> None:
        self.operational.add_transition(run.run_id, {"from": run.state, "to": to_state, "gate": gate, "actor": actor, "evidence": evidence})

    def _set(self, run_id: str, **fields: Any) -> RunRecord:
        return self.operational.update_run(run_id, **fields)

    # ------------------------------------------------------------------ lifecycle
    def start(self, flow: str, request: dict, *, run_id: str | None = None, sources: list[SourceDoc] | None = None, parent_run_id: str | None = None) -> RunRecord:
        if flow not in self.flows:
            raise NotFound(f"flow {flow!r} is not registered (have {sorted(self.flows)})", code="FLOW_UNKNOWN")
        rid = run_id or f"run-{context.new_id(6)}"
        trace_id = context.new_id(16)
        ts = now_iso()
        request = dict(request)
        pins = []
        for doc in sources or []:
            stored = self.knowledge.register_source(doc)
            pins.append({"id": stored.id, "revision": stored.revision, "role": stored.role, "content_hash": stored.content_hash})
        request["pins"] = sorted(pins, key=lambda p: (p["role"], p["id"]))
        run = RunRecord(rid, flow, "running", self.flows[flow].initial_state, request, {"gates": {}, "blockers": []}, ts, ts, trace_id, parent_run_id)
        quota = self.gateway.check_run_quota()
        if quota:
            run.status = "blocked"; run.result["blockers"] = [quota]
        self.operational.create_run(run)
        self.observer.metrics.run_status(flow, run.status)
        self.observer.event("run_started", run_id=rid, flow=flow, status=run.status)
        return run

    def update_sources(self, run_id: str, sources: list[SourceDoc], *, keep_revisions: bool = False, unpin: Sequence[tuple[str, str, str]] = (),
                       params: dict | None = None) -> RunRecord:
        """Pin new source revisions; gates whose input hash changes become stale on the next advance.

        By default a new revision replaces the pin of the same (role, id).  ``keep_revisions`` keeps the older revisions pinned
        (a change request pins a *before* and an *after* revision of one SRS), ``unpin`` drops (role, id, revision) pins and
        ``params`` is merged into the request params (for example the ``srs.after`` revision the gates should now read).
        """
        with self._lock(run_id):
            run = self.operational.get_run(run_id)
            pins = {(p["role"], p["id"], p["revision"]): p for p in run.request.get("pins", [])}
            for doc in sources:
                stored = self.knowledge.register_source(doc)
                if not keep_revisions:
                    for key in [k for k in pins if k[0] == stored.role and k[1] == stored.id]:
                        del pins[key]
                pins[(stored.role, stored.id, stored.revision)] = {"id": stored.id, "revision": stored.revision, "role": stored.role, "content_hash": stored.content_hash}
            for key in unpin:
                pins.pop(tuple(key), None)
            request = dict(run.request); request["pins"] = sorted(pins.values(), key=lambda p: (p["role"], p["id"], p["revision"]))
            if params:
                request["params"] = _deep_merge(dict(request.get("params", {})), params)
            return self._set(run_id, request=request)

    def get_run(self, run_id: str) -> RunRecord:
        return self.operational.get_run(run_id)

    def advance(self, run_id: str) -> RunRecord:
        """Run gates until the run waits for an approval/evidence, blocks, fails or completes."""
        with self._lock(run_id):
            run = self.operational.get_run(run_id)
            if run.status in TERMINAL:
                return run
            flow = self._flow(run)
            with self.observer.bind(run_id=run_id, trace_id=run.trace_id):
                with self.observer.span(f"run:{run.flow}", "run", flow=run.flow) as run_span:
                    result = self._advance_locked(run, flow, run_span)
            return result

    def retry(self, run_id: str) -> RunRecord:
        """Run again a run that ``failed`` on a program error (an interruption or a provider that did not answer leaves the run ``blocked`` and ``advance`` continues it)."""
        with self._lock(run_id):
            run = self.operational.get_run(run_id)
            if run.status != "failed":
                return run
            flow = self._flow(run)
            for gate in flow.gates:
                task = self.operational.get_task(run_id, gate.name)
                if task and task.get("status") == "failed":
                    self.operational.put_task(run_id, gate.name, dict(task, status="blocked", blockers=["RETRY_REQUESTED"]))
            self.operational.add_audit("run_retried", {"previous": "failed"}, run_id)
            self._set(run_id, status="blocked", result=self._result(run, blockers=["RETRY_REQUESTED"]))
        return self.advance(run_id)

    def _advance_locked(self, run: RunRecord, flow: FlowDef, run_span) -> RunRecord:
        invalid: set[str] = set()
        for idx, gate in enumerate(flow.gates):
            run = self.operational.get_run(run.run_id)
            task = self.operational.get_task(run.run_id, gate.name)
            ctx = self._ctx(run, flow, gate)
            h = self._input_hash(ctx, gate, flow)
            changed = bool(task) and task.get("status") != "stale" and task.get("input_hash") != h and not gate.evidence_gate
            if changed:
                task = self._mark_stale(run, gate, task, "input hash changed")
                invalid.add(gate.name)
                # void every transitive dependent (and any approval on it) before doing anything else
                for later in flow.dependents({gate.name}):
                    t = self.operational.get_task(run.run_id, later.name)
                    if t and t.get("status") not in ("stale", None):
                        self._mark_stale(run, later, t, f"upstream gate {gate.name} was invalidated")
                previous = flow.gates[idx - 1].state_after if idx else flow.initial_state
                if run.state != previous and flow.state_rank(run.state) > flow.state_rank(previous):
                    self._transition(run, previous, gate.name, "orchestrator", {"reason": "input changed; gate and dependents invalidated"})
                    run = self._set(run.run_id, state=previous, status="running")
            task = self.operational.get_task(run.run_id, gate.name)
            if task and task.get("input_hash") == h and task["status"] in ("completed", "approved"):
                if flow.state_rank(run.state) < idx + 1 and not gate.evidence_gate:
                    self._transition(run, gate.state_after, gate.name, "orchestrator", {"reason": "gate still valid after upstream re-run"})
                    run = self._set(run.run_id, state=gate.state_after)
                if not gate.evidence_gate:
                    continue
                regression = self._recheck_evidence(run, flow, gate, ctx, h, idx)
                if regression is None:
                    continue
                return regression
            if task and task.get("input_hash") == h and task["status"] == "awaiting_approval":
                if gate.approval and self.auto_approves(gate.name):
                    run = self._auto_approve(run, flow, gate)
                    if run.status != "running":
                        return run
                    continue
                run = self._set(run.run_id, status="awaiting_approval", result=self._result(run, blockers=[], pending_gate=gate.name))
                run_span.set(waiting=gate.name)
                return run
            if gate.name in self.AUTO_REVIEW_BEFORE and self.auto_approves(gate.name):
                self._auto_review(run, gate)
            run = self._execute(run, flow, gate, ctx, h, task, run_span)
            if run.status != "running":
                return run
        done = self._set(run.run_id, status="completed")
        self.observer.metrics.run_status(run.flow, "completed")
        self.observer.event("run_completed", run_id=run.run_id)
        return done

    def _recheck_evidence(self, run: RunRecord, flow: FlowDef, gate: GateDef, ctx: GateContext, input_hash: str, idx: int) -> RunRecord | None:
        """Evidence gates are re-verified on every advance without re-entering the state machine.

        Returns ``None`` while the evidence still holds; otherwise the run drops back to
        ``awaiting_evidence`` and the gate is reopened (later approvals become stale).
        """
        with self.observer.bind(gate=gate.name, agent=gate.name.lower()):
            with self.observer.span(f"{gate.name}:recheck", "gate") as span:
                outcome = gate.executor(ctx)
                if outcome.status == "ok":
                    return None
                span.block(*outcome.blockers)
        previous = flow.gates[idx - 1].state_after if idx else flow.initial_state
        self.operational.put_task(run.run_id, gate.name, {"status": "pending_evidence", "input_hash": input_hash, "blockers": outcome.blockers, "output": outcome.output})
        self._transition(run, previous, gate.name, "orchestrator", {"reason": "evidence no longer holds", "blockers": outcome.blockers})
        for later in flow.gates[idx + 1:]:
            t = self.operational.get_task(run.run_id, later.name)
            if t and t.get("status") not in ("stale", None):
                self._mark_stale(run, later, t, f"evidence gate {gate.name} reopened")
        return self._set(run.run_id, state=previous, status="awaiting_evidence", result=self._result(run, blockers=outcome.blockers, blocked_gate=gate.name))

    def _result(self, run: RunRecord, **updates: Any) -> dict:
        result = dict(run.result or {"gates": {}, "blockers": []}); result.update(updates); return result

    def _mark_stale(self, run: RunRecord, gate: GateDef, task: dict, reason: str) -> dict:
        task = dict(task, status="stale", stale_reason=reason, stale_at=now_iso(), previous_status=task.get("status"))
        self.operational.put_task(run.run_id, gate.name, task)
        self.operational.add_audit("gate_stale", {"gate": gate.name, "reason": reason}, run.run_id)
        self.observer.event("gate_stale", gate=gate.name, reason=reason)
        return task

    def _execute(self, run: RunRecord, flow: FlowDef, gate: GateDef, ctx: GateContext, input_hash: str, prior: dict | None, run_span) -> RunRecord:
        attempt = (prior or {}).get("attempt", 0) + 1
        self.operational.put_task(run.run_id, gate.name, {"status": "running", "input_hash": input_hash, "attempt": attempt, "started": now_iso()})
        with self.observer.bind(gate=gate.name, agent=gate.name.lower()):
            with self.observer.span(gate.name, "gate", attempt=attempt) as span:
                try:
                    outcome = gate.executor(ctx)
                except TddAgentError as exc:
                    codes = [exc.code]
                    if exc.code == "SAD_READONLY" and (exc.details or {}).get("elements"):
                        codes.append("SAD_CHANGED:" + ",".join(map(str, exc.details["elements"][:10])))  # which SAD elements, so the person can see what changed
                    outcome = GateOutcome.blocked(*codes, evidence={"error": str(exc), "details": exc.details})
                except TRANSIENT as exc:
                    code = "INTERRUPTED" if isinstance(exc, KeyboardInterrupt) else f"TRANSIENT:{type(exc).__name__}"
                    self.operational.put_task(run.run_id, gate.name, {"status": "blocked", "input_hash": input_hash, "attempt": attempt, "blockers": [code], "output": {}, "evidence": {"interrupted": True}})
                    self._set(run.run_id, status="blocked", result=self._result(run, blockers=[code], blocked_gate=gate.name))
                    self.operational.add_audit("gate_interrupted", {"gate": gate.name, "error": type(exc).__name__}, run.run_id)
                    raise
                except BaseException as exc:
                    self.operational.put_task(run.run_id, gate.name, {"status": "failed", "input_hash": input_hash, "attempt": attempt, "error": type(exc).__name__})
                    self._set(run.run_id, status="failed", result=self._result(run, blockers=[f"GATE_ERROR:{type(exc).__name__}"]))
                    self.observer.metrics.run_status(run.flow, "failed")
                    raise
                if outcome.status == "blocked":
                    span.block(*outcome.blockers)
                    for code in outcome.blockers:
                        self.observer.metrics.blocked(gate.name, code.split(":")[0])
                    self.operational.put_task(run.run_id, gate.name, {"status": "blocked", "input_hash": input_hash, "attempt": attempt, "blockers": outcome.blockers,
                                                                  "output": outcome.output, "evidence": outcome.evidence})
                    result = self._result(run, blockers=outcome.blockers, blocked_gate=gate.name)
                    self.observer.metrics.run_status(run.flow, "blocked")
                    return self._set(run.run_id, status="blocked", result=result)
                if outcome.status == "pending_evidence":
                    span.block(*outcome.blockers)
                    self.operational.put_task(run.run_id, gate.name, {"status": "pending_evidence", "input_hash": input_hash, "attempt": attempt,
                                                                  "blockers": outcome.blockers, "output": outcome.output})
                    return self._set(run.run_id, status="awaiting_evidence", result=self._result(run, blockers=outcome.blockers, blocked_gate=gate.name))
                record = {"input_hash": input_hash, "attempt": attempt, "output": outcome.output, "evidence": outcome.evidence, "finished": now_iso(), "blockers": []}
                if gate.approval:
                    decision = outcome.decision if outcome.decision is not None else outcome.output
                    record.update(status="awaiting_approval", decision=decision, decision_hash=digest(decision), approval_role=gate.approval)
                    self.operational.put_task(run.run_id, gate.name, record)
                    span.set(awaiting_approval=gate.approval)
                    waiting = self._set(run.run_id, status="awaiting_approval", result=self._result(run, blockers=[], pending_gate=gate.name, blocked_gate=None))
                    if self.auto_approves(gate.name):
                        return self._auto_approve(waiting, flow, gate)
                    return waiting
                record["status"] = "completed"
                self.operational.put_task(run.run_id, gate.name, record)
                self._transition(run, gate.state_after, gate.name, f"agent:{gate.name}", {"input_hash": input_hash})
                gates = dict(run.result.get("gates", {})); gates[gate.name] = {"status": "completed", "input_hash": input_hash}
                return self._set(run.run_id, state=gate.state_after, status="running", result=self._result(run, gates=gates, blockers=[], blocked_gate=None, pending_gate=None))

    # ------------------------------------------------------------------ review + approval
    def get_decision(self, run_id: str, gate: str, actor_id: str, *, source: str = "cli") -> dict:
        """Return the pending decision for a gate and record that ``actor_id`` has read it."""
        task = self.operational.get_task(run_id, gate)
        if task is None or task.get("status") != "awaiting_approval":
            raise NotFound(f"gate {gate} of run {run_id} has no pending decision", code="NO_PENDING_DECISION")
        self.approvals.record_receipt(run_id, actor_id, task["decision_hash"], gate, source)
        return {"run_id": run_id, "gate": gate, "decision_hash": task["decision_hash"], "decision": task["decision"],
                "approval_role": task.get("approval_role"), "blockers": task.get("blockers", [])}

    def cancel(self, run_id: str, reason: str, actor: str) -> list[str]:
        """Fail ``run_id`` and its unfinished child runs with ``CANCELLED_BY_CLIENT``; returns the cancelled run ids.

        A background advance must be stopped first (``dispatch.terminate``): this only records the outcome.
        """
        cancelled: list[str] = []
        children = [r for r in self.operational.list_runs(limit=100000) if r.parent_run_id == run_id]
        for child in children:
            cancelled += self.cancel(child.run_id, reason, actor)
        with self._lock(run_id):
            run = self.operational.get_run(run_id)
            if run.status in TERMINAL:
                return cancelled
            for task in self.operational.list_tasks(run_id):
                if task.get("status") == "running":
                    gate = task["task_id"]
                    rec = {k: v for k, v in task.items() if k != "task_id"}
                    self.operational.put_task(run_id, gate, dict(rec, status="failed", error="CANCELLED_BY_CLIENT"))
            self.operational.add_audit("run_cancelled", {"reason": reason, "actor": actor}, run_id)
            self._set(run_id, status="failed", result=self._result(run, blockers=["CANCELLED_BY_CLIENT"], cancel={"reason": reason, "actor": actor, "at": now_iso()}))
            self.observer.metrics.run_status(run.flow, "failed")
            self.observer.event("run_cancelled", run_id=run_id, actor=actor)
        return cancelled + [run_id]

    def submit_approval(self, req: ApprovalRequest, *, advance: bool = True) -> ApprovalResult:
        """Record an approval; ``advance=False`` leaves the run ``running`` for a background advance (remote clients)."""
        with self._lock(req.run_id):
            run = self.operational.get_run(req.run_id)
            task = self.operational.get_task(req.run_id, req.gate)
            rejection = self.approvals.check(req, task, run.status)
            if rejection is not None:
                return rejection
            flow = self._flow(run)
            gate = flow.gate(req.gate)
            record = self.approvals.record(req, task)
            blocked = self._apply_approval(run, flow, gate, req.actor_id, record, task)
            if blocked:
                return ApprovalResult(True, "APPROVED_HOOK_BLOCKED", f"approved, but the post-approval step failed: {blocked}", "blocked", record)
        run = self.advance(req.run_id) if advance else self.operational.get_run(req.run_id)
        return ApprovalResult(True, "APPROVED", "gate approved", run.status, record)

    def _apply_approval(self, run: RunRecord, flow: FlowDef, gate: GateDef, actor_id: str, record: dict, task: dict) -> str | None:
        """Mark the gate approved, move the run on and run the post-approval step; returns the blocker code when that step fails."""
        run_id = run.run_id
        task = dict(task, status="approved", approval=record, approved_at=now_iso())
        self.operational.put_task(run_id, gate.name, task)
        self._transition(run, gate.state_after, gate.name, actor_id, {"decision_hash": record.get("decision_hash"), "approval_grade": record["approval_grade"]})
        gates = dict(run.result.get("gates", {})); gates[gate.name] = {"status": "approved", "input_hash": task.get("input_hash"), "approval_grade": record["approval_grade"]}
        self._set(run_id, state=gate.state_after, status="running", result=self._result(run, gates=gates, pending_gate=None, blockers=[]))
        self.observer.event("approval_accepted", run_id=run_id, gate=gate.name, actor=actor_id, grade=record["approval_grade"])
        if gate.on_approved is not None:
            hook_ctx = self._ctx(self.operational.get_run(run_id), flow, gate)
            hook_ctx.upstream["_approval"] = record
            with self.observer.bind(run_id=run_id, trace_id=run.trace_id, gate=gate.name, agent=gate.name.lower()):
                with self.observer.span(f"{gate.name}:on-approved", "gate") as span:
                    try:
                        gate.on_approved(hook_ctx, task)
                    except TddAgentError as exc:
                        span.block(exc.code)
                        self.operational.put_task(run_id, gate.name, dict(task, hook_error=exc.code, hook_details=exc.details))
                        self._set(run_id, status="blocked", result=self._result(self.operational.get_run(run_id), blockers=[exc.code], blocked_gate=gate.name))
                        return exc.code
        return None

    # ------------------------------------------------------------------ automatic mode
    def auto_approves(self, gate: str) -> bool:
        """``approvals.mode = auto`` (the default): gates are approved by the engine, except those listed in ``approvals.required``.

        Only the person's decision is replaced: a gate with blockers never reaches approval, so rules still stop the flow, and
        what is recorded is an approval of grade ``auto`` that names the decision it covers.
        """
        cfg = self.config.section("approvals")
        return self.config.approval_mode() == "auto" and gate not in cfg["required"]

    def _auto_approve(self, run: RunRecord, flow: FlowDef, gate: GateDef) -> RunRecord:
        task = self.operational.get_task(run.run_id, gate.name)
        record = self.approvals.record_auto(run.run_id, gate.name, gate.approval or "", task)
        self.operational.add_audit("gate_auto_approved", {"gate": gate.name, "decision_hash": task.get("decision_hash")}, run.run_id)
        self.observer.event("gate_auto_approved", gate=gate.name)
        self._apply_approval(run, flow, gate, self.approvals.AUTO_ACTOR, record, task)
        return self.operational.get_run(run.run_id)

    # the readiness rules of these gates need the facts they read to be reviewed; in auto mode the engine does that review
    AUTO_REVIEW_BEFORE = ("G5",)

    def _auto_review(self, run: RunRecord, gate: GateDef) -> None:
        head = self.knowledge.head()
        if head is None:
            return
        graph = self.knowledge.materialize(head.id)
        pending = [e for e in graph["elements"] if (e.get("review_status") or "needs_review") == "needs_review"]
        if not pending:
            return
        import copy as _copy
        new = _copy.deepcopy(graph)
        ids = {e["id"] for e in pending}
        for el in new["elements"]:
            if el["id"] in ids:
                el["review_status"] = "approved"
                el.setdefault("reviews", []).append({"decision": "approved", "actor": self.approvals.AUTO_ACTOR, "role": "auto", "grade": "auto",
                                                     "rationale": f"auto mode: facts read by {gate.name} are accepted without a person's review (approvals.mode = auto)", "base_snapshot": head.id})
        new["version"] = f"{graph['version']}+reviewed"
        self.knowledge.commit_graph(head.id, new, actor=self.approvals.AUTO_ACTOR, meta={"flow": "element-review", "decision": "approved", "role": "auto", "grade": "auto", "run": run.run_id})
        self.operational.add_audit("elements_auto_reviewed", {"gate": gate.name, "count": len(ids)}, run.run_id)

    # ------------------------------------------------------------------ introspection
    def run_view(self, run_id: str) -> dict:
        run = self.operational.get_run(run_id)
        tasks = {t.pop("task_id"): t for t in self.operational.list_tasks(run_id)}
        return {"run": run.as_dict(), "tasks": tasks, "transitions": self.operational.list_transitions(run_id),
                "approvals": self.operational.list_approvals(run_id)}

    # ------------------------------------------------------------------ evidence (G6)
    def record_test_receipt(self, run_id: str, cs_id: str, criterion_refs: list[str], result: str, actor_id: str, *, executed_at: str | None = None,
                            evidence_ref: str | None = None) -> dict:
        """Record that the change-spec row ``cs_id`` was tested.

        Only allowlisted actors may record receipts, except in a project that declared no approver at all (automatic mode): there the name given is the person's
        own statement of who ran the tests, and the receipt says so.
        """
        declared = bool(self.config.actors())
        if declared and not any(a.id == actor_id for a in self.config.actors()):
            raise TddAgentError(f"{actor_id} is not an allowlisted actor", code="ACTOR_NOT_ALLOWLISTED")
        if not declared and not (actor_id or "").strip():
            raise TddAgentError("say who ran the tests (--actor)", code="ACTOR_NOT_ALLOWLISTED")
        run = self.operational.get_run(run_id)
        cr_id = (run.request.get("params", {}).get("cr") or {}).get("id", "CR")
        head = self.knowledge.head()
        graph = self.knowledge.materialize(head.id)
        items = {e["id"]: e for e in graph["elements"] if e.get("class") == "ChangeSpecItem" and (e.get("props") or {}).get("cr_id") == cr_id}
        key = cs_id if cs_id in items else next((i for i, e in items.items() if e["props"].get("table_id") == cs_id), None)
        if key is None:
            raise NotFound(f"change-spec row {cs_id} not found for {cr_id}", code="CS_NOT_FOUND")
        allowed = set(items[key]["props"].get("cr_acceptance_refs", []))
        unknown = [c for c in criterion_refs if c not in allowed]
        if unknown:
            raise TddAgentError(f"criteria {unknown} are not accepted by row {cs_id}", code="CRITERION_NOT_IN_ROW", details={"allowed": sorted(allowed)})
        if result not in ("pass", "fail"):
            raise TddAgentError("result must be 'pass' or 'fail'", code="RESULT_INVALID")
        n = sum(1 for e in graph["elements"] if e.get("class") == "TestReceipt") + 1
        el = {"id": f"TR-{run_id}-{n}", "class": "TestReceipt", "name": f"Test of {key}", "origin": "own", "maturity": "sketch", "review_status": "needs_review", "owner_doc": "CS", "tdds": [],
              "version_introduced": graph["version"],
              "props": {"cs_id": key, "criterion_refs": sorted(criterion_refs), "result": result, "executed_at": executed_at or now_iso(), "cs_version": items[key].get("version_introduced"),
                        "cr_version": str((run.request.get("params", {}).get("cr") or {}).get("revision", "1")), "recorded_by": actor_id, "recorded_by_declared": declared, "evidence_ref": evidence_ref},
              "provenance": [{"kind": "derived", "source_id": run_id, "source_version": str(graph["version"]), "source_hash": digest({"cs": key, "criteria": sorted(criterion_refs), "result": result}),
                              "block": f"derivation:{key}", "inputs": [key], "derivation": f"test receipt recorded by {actor_id}"}]}
        cs = {"id": f"receipt-{el['id']}", "base_version": graph["version"], "result_version": f"{graph['version']}+receipt", "ops": [{"seq": 1, "op": "add", "after": el}]}
        self.knowledge.commit_changeset(head.id, cs, actor=actor_id, run_id=run_id, meta={"flow": "test-receipt"})
        self.operational.add_audit("test_receipt", {"cs": key, "criteria": criterion_refs, "result": result, "actor": actor_id}, run_id)
        return el

    # ------------------------------------------------------------------ element review
    REVIEW_ROLES = ("BA", "SA", "SA/Dev", "BA/owner")

    def review_elements(self, element_ids: list[str], decision: str, actor_id: str, role: str, rationale: str) -> dict:
        """Mark extracted/derived facts ``approved`` (or send them back); requires an allowlisted actor holding a review role.

        Readiness refuses facts that were never reviewed, so this is the human (or registered coding agent)
        sign-off that turns intake/derived knowledge into baseline knowledge.
        """
        if decision not in ("approved", "needs_review", "rejected"):
            raise TddAgentError("decision must be approved, needs_review or rejected", code="REVIEW_DECISION_INVALID")
        actor = next((a for a in self.config.actors() if a.id == actor_id), None)
        if actor is None or role not in actor.roles or role not in self.REVIEW_ROLES:
            self.operational.add_audit("review_rejected", {"actor": actor_id, "role": role, "elements": len(element_ids)})
            raise TddAgentError(f"{actor_id} may not review as {role}", code="REVIEW_NOT_PERMITTED")
        if not (rationale or "").strip():
            raise TddAgentError("a rationale is required", code="RATIONALE_REQUIRED")
        from ..obs import context as _ctx
        if _ctx.get("agent"):
            raise TddAgentError("pipeline agents cannot review elements", code="PIPELINE_AGENT_CANNOT_APPROVE")
        head = self.knowledge.head()
        graph = self.knowledge.materialize(head.id)
        by_id = {e["id"]: e for e in graph["elements"]}
        missing = [i for i in element_ids if i not in by_id]
        if missing:
            raise NotFound(f"elements not found: {missing[:5]}", code="ELEMENT_NOT_FOUND")
        import copy as _copy
        wanted = {i for i in element_ids if by_id[i].get("review_status") != decision}
        if not wanted:
            return {"changed": 0, "snapshot": head.id}
        new = _copy.deepcopy(graph)
        grade = "human" if actor.kind == "human" else "agent"
        for el in new["elements"]:
            if el["id"] in wanted:
                el["review_status"] = decision
                el.setdefault("reviews", []).append({"decision": decision, "actor": actor_id, "role": role, "grade": grade, "rationale": rationale, "base_snapshot": head.id})
        # review is bookkeeping on unchanged facts, so it is committed as a graph replacement; a ChangeSet would reset the
        # status of every touched element (and its neighbours) to needs_review by design.
        new["version"] = f"{graph['version']}+reviewed"
        result = self.knowledge.commit_graph(head.id, new, actor=actor_id, meta={"flow": "element-review", "decision": decision, "role": role, "grade": grade})
        ops = wanted
        self.operational.add_audit("elements_reviewed", {"actor": actor_id, "role": role, "decision": decision, "count": len(ops), "grade": "human" if actor.kind == "human" else "agent"})
        return {"changed": len(ops), "snapshot": result.snapshot.id}

    # ------------------------------------------------------------------ conflicts
    CONFLICT_ROLES = ("SA", "BA")

    @staticmethod
    def conflict_key(conflict: dict) -> str:
        named = sorted(set(conflict.get("ids") or []) | ({conflict["id"]} if conflict.get("id") else set()))
        return digest({"type": conflict.get("type"), "named": named, "reason": conflict.get("reason")})[:10]

    def list_conflicts(self, status: str | None = None) -> list[dict]:
        graph = self.knowledge.materialize()
        rows = [{"key": self.conflict_key(c), **c} for c in graph.get("conflicts", [])]
        return [r for r in rows if status is None or r.get("status") == status]

    def resolve_conflicts(self, keys: list[str], actor_id: str, role: str, decision: str, rationale: str) -> dict:
        """Settle conflicts the knowledge graph records (``decision``: what was decided, in a sentence); the elements are not touched.

        Only a registered person or agent holding the SA or BA role may, and the reason is kept with the conflict and in the audit log.
        """
        actor = next((a for a in self.config.actors() if a.id == actor_id), None)
        if actor is None or role not in actor.roles or role not in self.CONFLICT_ROLES:
            self.operational.add_audit("conflict_resolution_rejected", {"actor": actor_id, "role": role, "keys": keys})
            raise TddAgentError(f"{actor_id} may not resolve conflicts as {role}", code="RESOLVE_NOT_PERMITTED")
        if not (rationale or "").strip() or not (decision or "").strip():
            raise TddAgentError("the decision and the reason are required", code="RATIONALE_REQUIRED")
        from ..obs import context as _ctx
        if _ctx.get("agent"):
            raise TddAgentError("pipeline agents cannot resolve conflicts", code="PIPELINE_AGENT_CANNOT_APPROVE")
        head = self.knowledge.head()
        graph = self.knowledge.materialize(head.id) if head else {"elements": [], "edges": [], "conflicts": []}
        known = {self.conflict_key(c): c for c in graph.get("conflicts", [])}
        missing = [k for k in keys if k not in known]
        if missing:
            raise NotFound(f"conflicts not found: {missing[:5]}", code="CONFLICT_NOT_FOUND")
        import copy as _copy
        new = _copy.deepcopy(graph)
        grade = "human" if actor.kind == "human" else "agent"
        done = []
        for c in new["conflicts"]:
            key = self.conflict_key(c)
            if key in keys and c.get("status") != "resolved":
                c["status"] = "resolved"
                c["resolution"] = {"actor": actor_id, "role": role, "grade": grade, "decision": decision, "rationale": rationale, "ts": now_iso()}
                done.append(key)
        if not done:
            return {"resolved": [], "snapshot": head.id if head else None}
        new["version"] = f"{graph['version']}+conflicts"
        result = self.knowledge.commit_graph(head.id, new, actor=actor_id, meta={"flow": "conflict-resolution", "role": role, "grade": grade})
        self.operational.add_audit("conflicts_resolved", {"actor": actor_id, "role": role, "keys": done, "decision": decision, "rationale": rationale, "grade": grade})
        return {"resolved": done, "snapshot": result.snapshot.id}

    def elements_needing_review(self) -> list[dict]:
        graph = self.knowledge.materialize()
        return [{"id": e["id"], "class": e.get("class"), "owner_doc": e.get("owner_doc"), "review_status": e.get("review_status")} for e in graph["elements"] if e.get("review_status") != "approved"]
