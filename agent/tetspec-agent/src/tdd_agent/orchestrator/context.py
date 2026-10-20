"""What a gate executor may touch.

Executors get read access to the knowledge store and one guarded write path
(``commit_changeset`` / ``commit_graph``) that enforces the SAD read-only rule.
They never receive the approval service.
"""
from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from ..config import Config
from ..core import o5_path
from ..errors import ChangeSetRejected
from ..llm.gateway import LlmGateway
from ..obs.observer import Observer
from ..retrieval import ClosureService
from ..retrieval.queries import QueryService
from ..store.base import CommitResult, KnowledgeStore, OperationalStore, RunRecord


def sad_owned(element: dict) -> bool:
    props = element.get("props") or {}
    return str(element.get("owner_doc") or props.get("owner_doc") or "").upper() == "SAD"


_VOLATILE = ("review_status", "change_history", "tdds")


def _normalized(el: dict) -> dict:
    return {k: v for k, v in el.items() if k not in _VOLATILE}


def sad_violations(before_graph: dict, after_graph: dict, ingestible_hashes: frozenset[str] = frozenset()) -> list[str]:
    """SAD-owned elements may be *ingested* from the pinned SAD but never edited or removed.

    A new SAD-owned element is acceptable only if every provenance record points at a
    pinned SAD revision (``ingestible_hashes``); changing or deleting an existing one needs
    a separate SAD CR.  Review-status bookkeeping is ignored.
    """
    b = {e["id"]: e for e in before_graph.get("elements", []) if sad_owned(e)}
    a = {e["id"]: e for e in after_graph.get("elements", []) if sad_owned(e)}
    out = [i for i in b if i not in a or _normalized(b[i]) != _normalized(a[i])]
    for i, el in a.items():
        if i in b:
            continue
        hashes = {p.get("source_hash") for p in (el.get("provenance") or []) if isinstance(p, dict)}
        if not hashes or not hashes <= set(ingestible_hashes):
            out.append(i)
    return sorted(set(out))


class GuardedKnowledge:
    """Read-through facade over ``KnowledgeStore`` with SAD-protected writes."""

    def __init__(self, store: KnowledgeStore, sad_hashes: frozenset[str] = frozenset()):
        self._store = store
        self._sad_hashes = frozenset(sad_hashes)

    def __getattr__(self, name: str) -> Any:
        if name in {"commit_changeset", "commit_graph", "register_source"}:
            raise AttributeError(f"{name} is not available directly; use the guarded GateContext writers")
        return getattr(self._store, name)

    def commit_changeset(self, base_snapshot: str | None, changeset: dict, *, actor: str, run_id: str | None = None, meta: dict | None = None) -> CommitResult:
        head = self._store.head()
        before = self._store.materialize(head.id) if head else {"elements": []}
        o5 = o5_path.load()
        import copy
        candidate, report = o5.apply_changeset(copy.deepcopy(before), changeset)
        if report.get("ok"):
            violations = sad_violations(before, candidate, self._sad_hashes)
            if violations:
                raise ChangeSetRejected("change touches SAD-owned elements; a separate SAD CR is required", code="SAD_READONLY",
                                        details={"elements": violations})
        return self._store.commit_changeset(base_snapshot, changeset, actor=actor, run_id=run_id, meta=meta)

    def commit_graph(self, base_snapshot: str | None, graph: dict, *, actor: str, run_id: str | None = None,
                     changeset_id: str | None = None, meta: dict | None = None, validate: bool = True) -> CommitResult:
        head = self._store.head()
        before = self._store.materialize(head.id) if head else {"elements": []}
        violations = sad_violations(before, graph, self._sad_hashes)
        if violations:
            raise ChangeSetRejected("graph replacement changes SAD-owned elements; a separate SAD CR is required", code="SAD_READONLY",
                                    details={"elements": violations})
        return self._store.commit_graph(base_snapshot, graph, actor=actor, run_id=run_id, changeset_id=changeset_id, meta=meta, validate=validate)


@dataclass(frozen=True)
class ChildRuns:
    """Lets a gate start and advance a child run (``parent_run_id`` = the current run) through the engine."""
    start: Callable[[str, dict], str]  # (flow, request) -> child run id
    advance: Callable[[str], str]      # child run id -> status after advancing


@dataclass
class GateContext:
    run: RunRecord
    gate: str
    config: Config
    knowledge: GuardedKnowledge
    operational: OperationalStore
    gateway: LlmGateway
    observer: Observer
    upstream: dict[str, dict] = field(default_factory=dict)
    children: ChildRuns | None = None

    @property
    def request(self) -> dict:
        return self.run.request

    @property
    def run_id(self) -> str:
        return self.run.run_id

    def closure(self) -> ClosureService:
        return ClosureService(self.knowledge)

    def queries(self) -> QueryService:
        return QueryService(self.knowledge)

    def register_generated_tdd(self, doc_id: str, text: str, meta: dict | None = None):
        """Register a TDD revision produced by this run (the only source an executor may register)."""
        from ..store.base import SourceDoc
        from ..agents.common import next_revision
        store = self.knowledge._store
        doc = SourceDoc.make(doc_id, next_revision(store, doc_id), "TDD", text, {**(meta or {}), "generated_by_run": self.run_id})
        return store.register_source(doc)

    def register_proposed_srs(self, base, text: str, meta: dict):
        """Register the SRS a change request would produce, as the next revision of ``base`` (the same text from the same CR is registered once)."""
        from ..store.base import SourceDoc
        from ..agents.common import next_revision
        store = self.knowledge._store
        probe = SourceDoc.make(base.id, "0", "SRS", text)
        for d in store.list_sources("SRS"):
            if d.id == base.id and d.content_hash == probe.content_hash and (d.meta or {}).get("proposed_from_cr") == meta.get("proposed_from_cr"):
                return d
        return store.register_source(SourceDoc.make(base.id, next_revision(store, base.id), "SRS", text, {**meta, "proposed_by_run": self.run_id}))

    def commit_changeset(self, changeset: dict, *, base_snapshot: str | None = None, meta: dict | None = None) -> CommitResult:
        head = self.knowledge.head()
        return self.knowledge.commit_changeset(base_snapshot if base_snapshot is not None else (head.id if head else None), changeset,
                                               actor=f"agent:{self.gate}", run_id=self.run_id, meta=meta)

    def commit_graph(self, graph: dict, *, base_snapshot: str | None = None, meta: dict | None = None, validate: bool = True) -> CommitResult:
        head = self.knowledge.head()
        return self.knowledge.commit_graph(base_snapshot if base_snapshot is not None else (head.id if head else None), graph,
                                           actor=f"agent:{self.gate}", run_id=self.run_id, meta=meta, validate=validate)
