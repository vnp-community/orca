"""Store contracts.

``KnowledgeStore`` holds the knowledge graph as immutable snapshots plus the
pinned source documents.  ``OperationalStore`` holds run state, approvals, the
task ledger, spans and LLM audit rows.  They are separate interfaces so a
deployment can mix backends.

The graph algorithms that must behave identically on every backend (diffing a
new graph against the head, deterministic ordering, hash parity with
``o5.transaction5.snapshot_hash``) live here in the template methods of
``KnowledgeStore``; backends only provide small row primitives.  Any backend is
valid only if it passes ``tdd_agent.store.contract``.
"""
from __future__ import annotations

import abc
import json
from contextlib import contextmanager
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from hashlib import sha256
from typing import Any, Iterator, Sequence

from ..errors import SearchNotSupported, ChangeSetRejected, NotFound, SourceConflict, SourceRoleError, VersionConflict

STRIDE = 1 << 20
DIRECTIONS = ("out", "in", "both")


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="microseconds")


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":"))


def digest(value: Any) -> str:
    return sha256(canonical(value).encode()).hexdigest()


@dataclass(frozen=True)
class SnapshotInfo:
    id: str
    seq: int
    parent: str | None
    hash: str
    graph_version: str | None
    actor: str | None
    run_id: str | None
    changeset_id: str | None
    created_at: str
    graph_meta: dict = field(default_factory=dict)
    meta: dict = field(default_factory=dict)
    touched: tuple[str, ...] = ()

    def as_dict(self) -> dict:
        d = asdict(self)
        d["touched"] = list(self.touched)
        return d


@dataclass(frozen=True)
class CommitResult:
    snapshot: SnapshotInfo
    added: tuple[str, ...]
    modified: tuple[str, ...]
    removed: tuple[str, ...]
    edges_added: int
    edges_removed: int
    noop: bool = False


@dataclass(frozen=True)
class SourceDoc:
    id: str
    revision: str
    role: str
    text: str
    content_hash: str = ""
    meta: dict = field(default_factory=dict)
    registered_at: str = ""

    @staticmethod
    def make(id: str, revision: str, role: str, text: str, meta: dict | None = None) -> "SourceDoc":
        return SourceDoc(id=id, revision=str(revision), role=role.upper(), text=text,
                         content_hash=sha256(text.encode()).hexdigest(), meta=dict(meta or {}))

    def ref(self) -> dict:
        return {"document": self.id, "revision": self.revision, "role": self.role, "content_hash": self.content_hash}


@dataclass(frozen=True)
class ElementVersion:
    id: str
    first_snapshot: str
    last_snapshot: str | None
    content_hash: str
    data: dict


@dataclass
class LiveRow:
    rowid: Any
    ord: int
    data: dict


def assign_ords(olds: Sequence[int | None]) -> list[int]:
    """Give every item of the new ordered list a strictly increasing ordinal.

    Items that already have an ordinal keep it when it still fits; new items are
    placed in the gap before the next known ordinal.  Ordinals are sparse
    (``STRIDE``) so the common cases (append, remove, in-place edit) never
    renumber surviving rows.
    """
    n = len(olds)
    res: list[int] = [0] * n
    prev = -STRIDE
    i = 0
    while i < n:
        if olds[i] is not None:
            o = olds[i]
            if o <= prev:
                o = prev + STRIDE
            res[i] = o
            prev = o
            i += 1
            continue
        j = i
        while j < n and olds[j] is None:
            j += 1
        k = j - i
        nxt = olds[j] if j < n else None
        step = (nxt - prev) // (k + 1) if nxt is not None else STRIDE
        if step < 1:
            step = STRIDE
        for t in range(k):
            prev += step
            res[i + t] = prev
        i = j
    return res


def split_graph(graph: dict) -> tuple[list[dict], list[dict], dict]:
    elements = graph.get("elements", [])
    edges = graph.get("edges", [])
    meta = {k: v for k, v in graph.items() if k not in ("elements", "edges")}
    return elements, edges, meta


class KnowledgeStore(abc.ABC):
    """Snapshot store for the knowledge graph and pinned sources."""

    # a backend that can search sets this and implements ``_search``; one that cannot says so (never an empty answer that looks like "nothing found")
    supports_search = False

    def _search(self, seq: int, query: str, tokens: list[str], classes: tuple[str, ...], limit: int) -> list:
        raise SearchNotSupported(f"{type(self).__name__} has no full-text search", code="SEARCH_NOT_SUPPORTED")

    # ------------------------------------------------------------------ primitives
    @abc.abstractmethod
    def _txn(self) -> Iterator[None]:
        """Context manager: exclusive write transaction (rolls back on error)."""

    @abc.abstractmethod
    def _read(self) -> Iterator[None]:
        """Context manager: consistent read."""

    @abc.abstractmethod
    def _snapshot(self, ref: str | int | None) -> SnapshotInfo | None: ...

    @abc.abstractmethod
    def _all_snapshots(self) -> list[SnapshotInfo]: ...

    @abc.abstractmethod
    def _live_elements(self, seq: int) -> list[LiveRow]: ...

    @abc.abstractmethod
    def _live_edges(self, seq: int) -> list[LiveRow]: ...

    @abc.abstractmethod
    def _element_row(self, seq: int, element_id: str) -> dict | None: ...

    @abc.abstractmethod
    def _elements_by_class(self, seq: int, cls: str) -> list[dict]: ...

    @abc.abstractmethod
    def _elements_by_ids(self, seq: int, ids: Sequence[str]) -> list[dict]: ...

    @abc.abstractmethod
    def _edges_touching(self, seq: int, ids: Sequence[str], relations: Sequence[str] | None, direction: str) -> list[dict]: ...

    @abc.abstractmethod
    def _element_versions(self, element_id: str) -> list[ElementVersion]: ...

    @abc.abstractmethod
    def _write_snapshot(self, info: SnapshotInfo, close_elements: list[Any], open_elements: list[tuple[dict, int]],
                        close_edges: list[Any], open_edges: list[tuple[dict, int]]) -> None: ...

    @abc.abstractmethod
    def _put_source(self, doc: SourceDoc) -> SourceDoc: ...

    @abc.abstractmethod
    def _get_source(self, id: str, revision: str | None) -> SourceDoc | None: ...

    @abc.abstractmethod
    def _list_sources(self, role: str | None) -> list[SourceDoc]: ...

    @abc.abstractmethod
    def close(self) -> None: ...

    # ------------------------------------------------------------------ snapshots
    def head(self) -> SnapshotInfo | None:
        with self._read():
            return self._snapshot(None)

    def get_snapshot(self, ref: str | int | None = None) -> SnapshotInfo:
        with self._read():
            info = self._snapshot(ref)
        if info is None:
            raise NotFound(f"snapshot not found: {ref}", code="SNAPSHOT_NOT_FOUND")
        return info

    def snapshots(self, limit: int | None = None) -> list[SnapshotInfo]:
        with self._read():
            rows = self._all_snapshots()
        rows.sort(key=lambda s: s.seq, reverse=True)
        return rows[:limit] if limit else rows

    def materialize(self, snapshot: str | int | None = None) -> dict:
        """Return the graph dict (``elements``/``edges``/meta) exactly as committed."""
        with self._read():
            info = self._snapshot(snapshot)
            if info is None:
                if snapshot is None:
                    return {"version": None, "elements": [], "edges": []}
                raise NotFound(f"snapshot not found: {snapshot}", code="SNAPSHOT_NOT_FOUND")
            return self._materialize(info)

    def _materialize(self, info: SnapshotInfo) -> dict:
        graph: dict[str, Any] = dict(info.graph_meta)
        graph["version"] = info.graph_version
        graph["elements"] = [r.data for r in self._live_elements(info.seq)]
        graph["edges"] = [r.data for r in self._live_edges(info.seq)]
        return graph

    # ------------------------------------------------------------------ search
    def search(self, query: str, snapshot: str | int | None = None, classes: Sequence[str] | None = None, limit: int = 20) -> list:
        """Elements live at ``snapshot`` (the head by default) that match ``query``, best first.

        An id given in full comes first, then an id fragment, then the text (accents and capitals ignored).  Ties are broken by id, so the order is stable.
        The result only proposes elements: whether one counts as evidence is decided elsewhere.
        """
        from .search import SearchHit, excerpt, fold, tokens_of
        if not self.supports_search:
            raise SearchNotSupported(f"{type(self).__name__} has no full-text search", code="SEARCH_NOT_SUPPORTED")
        limit = max(1, int(limit))
        with self._read():
            info = self._snapshot(snapshot)
            if info is None:
                return []
            wanted = tuple(classes or ())
            tokens = tokens_of(query)
            exact = [el for el in (self._element_row(info.seq, query.strip()), self._element_row(info.seq, query.strip().upper())) if el]
            found = self._search(info.seq, query, tokens, wanted, limit * 3) if tokens or len(fold(query).strip()) >= 3 else []
        hits: dict[str, Any] = {}
        for el in exact:
            if (not wanted or el.get("class") in wanted) and el["id"] not in hits:
                hits[el["id"]] = (1000.0, ("id",), el)
        for cand in found:
            hits.setdefault(cand.element["id"], (cand.score, cand.matched, cand.element))
        ranked = sorted(hits.values(), key=lambda t: (-t[0], t[2]["id"]))[:limit]
        return [SearchHit(el["id"], el.get("class", "?"), score, matched, excerpt(el, tokens), info.id) for score, matched, el in ranked]

    # ------------------------------------------------------------------ reads
    def get_element(self, element_id: str, snapshot: str | int | None = None) -> dict | None:
        with self._read():
            info = self._snapshot(snapshot)
            return None if info is None else self._element_row(info.seq, element_id)

    def get_elements(self, ids: Sequence[str], snapshot: str | int | None = None) -> list[dict]:
        with self._read():
            info = self._snapshot(snapshot)
            return [] if info is None or not ids else self._elements_by_ids(info.seq, list(ids))

    def by_class(self, cls: str, snapshot: str | int | None = None) -> list[dict]:
        with self._read():
            info = self._snapshot(snapshot)
            return [] if info is None else self._elements_by_class(info.seq, cls)

    def neighbors(self, ids: Sequence[str], relations: Sequence[str] | None = None, direction: str = "both",
                  snapshot: str | int | None = None) -> list[dict]:
        if direction not in DIRECTIONS:
            raise ValueError(f"direction must be one of {DIRECTIONS}")
        with self._read():
            info = self._snapshot(snapshot)
            if info is None or not ids:
                return []
            edges = self._edges_touching(info.seq, list(ids), list(relations) if relations is not None else None, direction)
        return edges

    def history(self, element_id: str) -> list[ElementVersion]:
        with self._read():
            return self._element_versions(element_id)

    def stats(self, snapshot: str | int | None = None) -> dict:
        graph = self.materialize(snapshot)
        by_class: dict[str, int] = {}
        for el in graph["elements"]:
            by_class[el.get("class", "?")] = by_class.get(el.get("class", "?"), 0) + 1
        by_rel: dict[str, int] = {}
        for ed in graph["edges"]:
            by_rel[ed.get("rel", "?")] = by_rel.get(ed.get("rel", "?"), 0) + 1
        return {"elements": len(graph["elements"]), "edges": len(graph["edges"]), "by_class": by_class, "by_relation": by_rel}

    # ------------------------------------------------------------------ writes
    def commit_changeset(self, base_snapshot: str | None, changeset: dict, *, actor: str, run_id: str | None = None,
                         meta: dict | None = None) -> CommitResult:
        """Apply an o5 ChangeSet to ``base_snapshot`` atomically.

        ``base_snapshot`` must be the head, otherwise ``VersionConflict`` is
        raised and the graph is untouched.  The change is validated by
        ``o5.transaction5.apply_changeset`` (including the graph-version
        precondition and ontology validation).
        """
        from ..core import o5_path
        o5 = o5_path.load()
        with self._txn():
            head = self._snapshot(None)
            self._check_base(head, base_snapshot)
            graph = self._materialize(head) if head else {"version": None, "elements": [], "edges": []}
            new_graph, report = o5.apply_changeset(graph, changeset)
            if not report.get("ok"):
                code = report.get("code", "CHANGESET_REJECTED")
                if code == "VERSION_CONFLICT":
                    raise VersionConflict("graph version does not match changeset base_version", details=report)
                raise ChangeSetRejected(f"changeset rejected: {code}", code=code, details=report)
            return self._commit_locked(head, new_graph, actor=actor, run_id=run_id, changeset_id=changeset.get("id"),
                                       meta=meta, touched=report.get("touched", []))

    def commit_graph(self, base_snapshot: str | None, graph: dict, *, actor: str, run_id: str | None = None,
                     changeset_id: str | None = None, meta: dict | None = None, validate: bool = True) -> CommitResult:
        """Replace the graph with ``graph`` (import, generation output)."""
        if validate:
            from ..core import o5_path
            result = o5_path.load().validate_graph(graph)
            if not result["ok"]:
                raise ChangeSetRejected("graph failed ontology validation", code="VALIDATION_FAILED", details={"errors": result["errors"]})
        with self._txn():
            head = self._snapshot(None)
            self._check_base(head, base_snapshot)
            return self._commit_locked(head, graph, actor=actor, run_id=run_id, changeset_id=changeset_id, meta=meta, touched=[])

    @staticmethod
    def _check_base(head: SnapshotInfo | None, base: str | None) -> None:
        head_id = head.id if head else None
        if base != head_id:
            raise VersionConflict(f"base snapshot {base!r} is not the head {head_id!r}",
                                  details={"base": base, "head": head_id})

    def _commit_locked(self, head: SnapshotInfo | None, new_graph: dict, *, actor: str, run_id: str | None,
                       changeset_id: str | None, meta: dict | None, touched: Sequence[str]) -> CommitResult:
        from ..core import o5_path
        o5 = o5_path.load()
        new_els, new_edges, graph_meta = split_graph(new_graph)
        ids = [e["id"] for e in new_els]
        if len(set(ids)) != len(ids):
            raise ChangeSetRejected("duplicate element id in graph", code="DUPLICATE_ID")
        graph_hash = o5.snapshot_hash(new_graph)
        old_els = self._live_elements(head.seq) if head else []
        old_edges = self._live_edges(head.seq) if head else []

        # ---- elements: stable ids, versioned rows, sparse ordinals
        old_by_id = {r.data["id"]: r for r in old_els}
        new_ords = assign_ords([old_by_id[i].ord if i in old_by_id else None for i in ids])
        close_els, open_els = [], []
        added, modified = [], []
        for el, ordinal in zip(new_els, new_ords):
            old = old_by_id.pop(el["id"], None)
            if old is None:
                open_els.append((el, ordinal)); added.append(el["id"])
            elif old.data != el or old.ord != ordinal:
                close_els.append(old.rowid); open_els.append((el, ordinal))
                if old.data != el:
                    modified.append(el["id"])
        removed = sorted(old_by_id)
        close_els.extend(r.rowid for r in old_by_id.values())

        # ---- edges: identity is the canonical edge payload (multiset, order kept)
        pool: dict[str, list[LiveRow]] = {}
        for r in old_edges:
            pool.setdefault(canonical(r.data), []).append(r)
        matched: list[LiveRow | None] = []
        for ed in new_edges:
            bucket = pool.get(canonical(ed))
            matched.append(bucket.pop(0) if bucket else None)
        edge_ords = assign_ords([m.ord if m else None for m in matched])
        close_edges, open_edges = [], []
        for ed, m, ordinal in zip(new_edges, matched, edge_ords):
            if m is None:
                open_edges.append((ed, ordinal))
            elif m.ord != ordinal:
                close_edges.append(m.rowid); open_edges.append((ed, ordinal))
        leftovers = [r for bucket in pool.values() for r in bucket]
        close_edges.extend(r.rowid for r in leftovers)
        edges_added = sum(1 for m in matched if m is None)

        noop = head is not None and not (close_els or open_els or close_edges or open_edges) \
            and head.graph_version == new_graph.get("version") and head.graph_meta == graph_meta
        if noop:
            return CommitResult(head, (), (), (), 0, 0, noop=True)

        seq = (head.seq if head else 0) + 1
        info = SnapshotInfo(
            id=f"snap-{seq:06d}", seq=seq, parent=head.id if head else None, hash=graph_hash,
            graph_version=new_graph.get("version"), actor=actor, run_id=run_id, changeset_id=changeset_id,
            created_at=now_iso(), graph_meta=graph_meta, meta=dict(meta or {}),
            touched=tuple(sorted(set(touched) | set(added) | set(modified) | set(removed))),
        )
        self._write_snapshot(info, close_els, open_els, close_edges, open_edges)
        check = self._materialize(info)
        if o5.snapshot_hash(check) != graph_hash or check["elements"] != new_els or check["edges"] != new_edges:
            raise ChangeSetRejected("stored snapshot does not reproduce the committed graph", code="STORE_INTEGRITY")
        return CommitResult(info, tuple(added), tuple(modified), tuple(removed), edges_added, len(leftovers))

    # ------------------------------------------------------------------ sources
    def register_source(self, doc: SourceDoc) -> SourceDoc:
        """Pin a source document; re-registering identical content is idempotent."""
        if not doc.id or not doc.revision or not doc.role:
            raise ValueError("source needs id, revision and role")
        doc = SourceDoc.make(doc.id, doc.revision, doc.role, doc.text, doc.meta)
        with self._txn():
            existing = self._get_source(doc.id, doc.revision)
            if existing is not None:
                if existing.content_hash != doc.content_hash or existing.role != doc.role:
                    raise SourceConflict(
                        f"source {doc.id}@{doc.revision} already registered with different content or role",
                        details={"id": doc.id, "revision": doc.revision})
                return existing
            return self._put_source(doc)

    def get_source(self, id: str, revision: str | None = None, role: str | None = None) -> SourceDoc:
        with self._read():
            doc = self._get_source(id, str(revision) if revision is not None else None)
            if doc is None:
                raise NotFound(f"source not found: {id}@{revision}", code="SOURCE_NOT_FOUND")
        if role is not None and doc.role != role.upper():
            raise SourceRoleError(f"source {id} has role {doc.role}, not {role.upper()}",
                                  details={"id": id, "actual": doc.role, "expected": role.upper()})
        return doc

    def list_sources(self, role: str | None = None) -> list[SourceDoc]:
        with self._read():
            return self._list_sources(role.upper() if role else None)

    def __enter__(self) -> "KnowledgeStore":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()


# ====================================================================== operational
@dataclass
class RunRecord:
    run_id: str
    flow: str
    status: str
    state: str
    request: dict
    result: dict
    created_at: str
    updated_at: str
    trace_id: str = ""
    parent_run_id: str | None = None

    def as_dict(self) -> dict:
        return asdict(self)


class OperationalStore(abc.ABC):
    """Run state, ledger, approvals, spans, LLM audit and metrics."""

    # runs
    @abc.abstractmethod
    def create_run(self, run: RunRecord) -> RunRecord: ...
    @abc.abstractmethod
    def update_run(self, run_id: str, **fields: Any) -> RunRecord: ...
    @abc.abstractmethod
    def get_run(self, run_id: str) -> RunRecord: ...
    @abc.abstractmethod
    def list_runs(self, *, flow: str | None = None, status: str | None = None, limit: int = 50, offset: int = 0) -> list[RunRecord]: ...
    @abc.abstractmethod
    def count_runs_since(self, iso_timestamp: str) -> int: ...

    # transitions
    @abc.abstractmethod
    def add_transition(self, run_id: str, record: dict) -> dict: ...
    @abc.abstractmethod
    def list_transitions(self, run_id: str) -> list[dict]: ...

    # ledger
    @abc.abstractmethod
    def get_task(self, run_id: str, task_id: str) -> dict | None: ...
    @abc.abstractmethod
    def put_task(self, run_id: str, task_id: str, record: dict) -> None: ...
    @abc.abstractmethod
    def list_tasks(self, run_id: str) -> list[dict]: ...

    # approvals and receipts
    @abc.abstractmethod
    def add_approval(self, record: dict) -> dict: ...
    @abc.abstractmethod
    def list_approvals(self, run_id: str) -> list[dict]: ...
    @abc.abstractmethod
    def add_receipt(self, record: dict) -> dict: ...
    @abc.abstractmethod
    def find_receipt(self, run_id: str, actor_id: str, decision_hash: str) -> dict | None: ...
    @abc.abstractmethod
    def add_audit(self, kind: str, payload: dict, run_id: str | None = None) -> dict: ...
    @abc.abstractmethod
    def list_audit(self, *, run_id: str | None = None, kind: str | None = None, limit: int = 200) -> list[dict]: ...

    # spans and llm audit
    @abc.abstractmethod
    def add_span(self, span: dict) -> None: ...
    @abc.abstractmethod
    def end_span(self, span_id: str, **fields: Any) -> None: ...
    @abc.abstractmethod
    def list_spans(self, *, trace_id: str | None = None, run_id: str | None = None) -> list[dict]: ...
    @abc.abstractmethod
    def add_llm_call(self, record: dict) -> None: ...
    @abc.abstractmethod
    def list_llm_calls(self, run_id: str | None = None, limit: int = 1000) -> list[dict]: ...

    # metrics
    @abc.abstractmethod
    def incr_metric(self, name: str, labels: dict[str, str] | None = None, value: float = 1.0) -> None: ...
    @abc.abstractmethod
    def list_metrics(self, name: str | None = None) -> list[dict]: ...

    @abc.abstractmethod
    def close(self) -> None: ...

    def __enter__(self) -> "OperationalStore":
        return self

    def __exit__(self, *exc: object) -> None:
        self.close()
