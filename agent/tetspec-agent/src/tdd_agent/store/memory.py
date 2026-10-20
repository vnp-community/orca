"""In-memory stores.

Fully functional backends (not test doubles): useful for ephemeral runs and as
the second implementation that proves the interfaces are not bound to SQLite.
They pass the same contract suite as the SQLite backends.
"""
from __future__ import annotations

import copy
import json
import re
import threading
from collections import defaultdict
from contextlib import contextmanager
from dataclasses import replace
from typing import Any, Iterator, Sequence

from ..errors import NotFound
from .base import (ElementVersion, KnowledgeStore, LiveRow, OperationalStore, RunRecord, SnapshotInfo, SourceDoc,
                   canonical, digest, now_iso)


def _live(first: int, last: int | None, seq: int) -> bool:
    return first <= seq and (last is None or last > seq)


class MemoryKnowledgeStore(KnowledgeStore):
    supports_search = True

    def _search(self, seq: int, query: str, tokens: list[str], classes: tuple[str, ...], limit: int) -> list:
        from .search import fold, rank_in_memory
        compact = re.sub(r"\s+", "", fold(query))
        live = [r["data"] for r in self._els if _live(r["first"], r["last"], seq) and (not classes or r["data"].get("class") in classes)]
        return [type(c)(copy.deepcopy(c.element), c.score, c.matched) for c in rank_in_memory(live, tokens, compact, limit)]

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._depth = 0
        self._snaps: list[SnapshotInfo] = []
        self._els: list[dict] = []     # rowid, id, first, last, ord, data
        self._edges: list[dict] = []   # rowid, first, last, ord, data
        self._sources: list[SourceDoc] = []
        self._next_row = 1

    @contextmanager
    def _txn(self) -> Iterator[None]:
        with self._lock:
            if self._depth:
                self._depth += 1
                try:
                    yield
                finally:
                    self._depth -= 1
                return
            saved = (list(self._snaps), copy.deepcopy(self._els), copy.deepcopy(self._edges), list(self._sources), self._next_row)
            self._depth = 1
            try:
                yield
            except BaseException:
                self._snaps, self._els, self._edges, self._sources, self._next_row = saved
                raise
            finally:
                self._depth = 0

    @contextmanager
    def _read(self) -> Iterator[None]:
        with self._lock:
            yield

    def _snapshot(self, ref: str | int | None) -> SnapshotInfo | None:
        if not self._snaps:
            return None
        if ref is None:
            return self._snaps[-1]
        for s in self._snaps:
            if (isinstance(ref, int) and s.seq == ref) or (isinstance(ref, str) and s.id == ref):
                return s
        return None

    def _all_snapshots(self) -> list[SnapshotInfo]:
        return list(self._snaps)

    def _write_snapshot(self, info: SnapshotInfo, close_elements: list[Any], open_elements: list[tuple[dict, int]],
                        close_edges: list[Any], open_edges: list[tuple[dict, int]]) -> None:
        closing, closing_edges = set(close_elements), set(close_edges)
        for r in self._els:
            if r["rowid"] in closing:
                r["last"] = info.seq
        for r in self._edges:
            if r["rowid"] in closing_edges:
                r["last"] = info.seq
        for el, o in open_elements:
            self._els.append({"rowid": self._next_row, "id": el["id"], "first": info.seq, "last": None, "ord": o, "data": json.loads(canonical(el))})
            self._next_row += 1
        for e, o in open_edges:
            self._edges.append({"rowid": self._next_row, "first": info.seq, "last": None, "ord": o, "data": json.loads(canonical(e))})
            self._next_row += 1
        self._snaps.append(info)

    def _live_elements(self, seq: int) -> list[LiveRow]:
        rows = sorted((r for r in self._els if _live(r["first"], r["last"], seq)), key=lambda r: r["ord"])
        return [LiveRow(r["rowid"], r["ord"], copy.deepcopy(r["data"])) for r in rows]

    def _live_edges(self, seq: int) -> list[LiveRow]:
        rows = sorted((r for r in self._edges if _live(r["first"], r["last"], seq)), key=lambda r: r["ord"])
        return [LiveRow(r["rowid"], r["ord"], copy.deepcopy(r["data"])) for r in rows]

    def _element_row(self, seq: int, element_id: str) -> dict | None:
        for r in self._els:
            if r["id"] == element_id and _live(r["first"], r["last"], seq):
                return copy.deepcopy(r["data"])
        return None

    def _elements_by_class(self, seq: int, cls: str) -> list[dict]:
        rows = sorted((r for r in self._els if _live(r["first"], r["last"], seq) and r["data"].get("class") == cls), key=lambda r: r["ord"])
        return [copy.deepcopy(r["data"]) for r in rows]

    def _elements_by_ids(self, seq: int, ids: Sequence[str]) -> list[dict]:
        order = {i: n for n, i in enumerate(ids)}
        rows = [r for r in self._els if r["id"] in order and _live(r["first"], r["last"], seq)]
        return [copy.deepcopy(r["data"]) for r in sorted(rows, key=lambda r: order[r["id"]])]

    def _edges_touching(self, seq: int, ids: Sequence[str], relations: Sequence[str] | None, direction: str) -> list[dict]:
        wanted = set(ids)
        rels = None if relations is None else set(relations)
        out = []
        for r in self._edges:
            if not _live(r["first"], r["last"], seq):
                continue
            d = r["data"]
            if rels is not None and d.get("rel") not in rels:
                continue
            hit_out = d.get("src") in wanted
            hit_in = d.get("dst") in wanted
            if (direction == "out" and hit_out) or (direction == "in" and hit_in) or (direction == "both" and (hit_out or hit_in)):
                out.append(r)
        return [copy.deepcopy(r["data"]) for r in sorted(out, key=lambda r: r["ord"])]

    def _element_versions(self, element_id: str) -> list[ElementVersion]:
        names = {s.seq: s.id for s in self._snaps}
        rows = sorted((r for r in self._els if r["id"] == element_id), key=lambda r: r["first"])
        return [ElementVersion(element_id, names[r["first"]], names.get(r["last"]), digest(r["data"]), copy.deepcopy(r["data"])) for r in rows]

    def _put_source(self, doc: SourceDoc) -> SourceDoc:
        stamped = replace(doc, registered_at=now_iso())
        self._sources.append(stamped)
        return stamped

    def _get_source(self, id: str, revision: str | None) -> SourceDoc | None:
        matches = [d for d in self._sources if d.id == id and (revision is None or d.revision == revision)]
        return matches[-1] if matches else None

    def _list_sources(self, role: str | None) -> list[SourceDoc]:
        return [d for d in self._sources if role is None or d.role == role]

    def close(self) -> None:
        return None


class MemoryOperationalStore(OperationalStore):
    def __init__(self) -> None:
        self._lock = threading.RLock()
        self.runs: dict[str, RunRecord] = {}
        self.transitions: dict[str, list[dict]] = defaultdict(list)
        self.tasks: dict[tuple[str, str], dict] = {}
        self.approvals: list[dict] = []
        self.receipts: list[dict] = []
        self.audit: list[dict] = []
        self.spans: dict[str, dict] = {}
        self.llm_calls: list[dict] = []
        self.metrics: dict[tuple[str, str], dict] = {}

    def create_run(self, run: RunRecord) -> RunRecord:
        with self._lock:
            if run.run_id in self.runs:
                raise ValueError(f"run exists: {run.run_id}")
            self.runs[run.run_id] = copy.deepcopy(run)
            return run

    def update_run(self, run_id: str, **fields: Any) -> RunRecord:
        with self._lock:
            run = self.get_run(run_id)
            for k, v in fields.items():
                if not hasattr(run, k):
                    raise ValueError(f"unknown run field {k}")
                setattr(run, k, copy.deepcopy(v))
            run.updated_at = now_iso()
            self.runs[run_id] = run
            return copy.deepcopy(run)

    def get_run(self, run_id: str) -> RunRecord:
        with self._lock:
            if run_id not in self.runs:
                raise NotFound(f"run not found: {run_id}", code="RUN_NOT_FOUND")
            return copy.deepcopy(self.runs[run_id])

    def list_runs(self, *, flow: str | None = None, status: str | None = None, limit: int = 50, offset: int = 0) -> list[RunRecord]:
        with self._lock:
            rows = [r for r in self.runs.values() if (flow is None or r.flow == flow) and (status is None or r.status == status)]
            rows.sort(key=lambda r: r.created_at, reverse=True)
            return [copy.deepcopy(r) for r in rows[offset:offset + limit]]

    def count_runs_since(self, iso_timestamp: str) -> int:
        with self._lock:
            return sum(1 for r in self.runs.values() if r.created_at >= iso_timestamp)

    def add_transition(self, run_id: str, record: dict) -> dict:
        with self._lock:
            rec = dict(record, run_id=run_id, ts=record.get("ts") or now_iso(), n=len(self.transitions[run_id]) + 1)
            self.transitions[run_id].append(rec)
            return copy.deepcopy(rec)

    def list_transitions(self, run_id: str) -> list[dict]:
        with self._lock:
            return copy.deepcopy(self.transitions.get(run_id, []))

    def get_task(self, run_id: str, task_id: str) -> dict | None:
        with self._lock:
            return copy.deepcopy(self.tasks.get((run_id, task_id)))

    def put_task(self, run_id: str, task_id: str, record: dict) -> None:
        with self._lock:
            self.tasks[(run_id, task_id)] = copy.deepcopy(record)

    def list_tasks(self, run_id: str) -> list[dict]:
        with self._lock:
            return [dict(v, task_id=k[1]) for k, v in sorted(self.tasks.items()) if k[0] == run_id]

    def add_approval(self, record: dict) -> dict:
        with self._lock:
            rec = dict(record, ts=record.get("ts") or now_iso())
            self.approvals.append(rec)
            return copy.deepcopy(rec)

    def list_approvals(self, run_id: str) -> list[dict]:
        with self._lock:
            return [copy.deepcopy(a) for a in self.approvals if a.get("run_id") == run_id]

    def add_receipt(self, record: dict) -> dict:
        with self._lock:
            rec = dict(record, ts=record.get("ts") or now_iso())
            self.receipts.append(rec)
            return copy.deepcopy(rec)

    def find_receipt(self, run_id: str, actor_id: str, decision_hash: str) -> dict | None:
        with self._lock:
            for r in reversed(self.receipts):
                if r["run_id"] == run_id and r["actor_id"] == actor_id and r["decision_hash"] == decision_hash:
                    return copy.deepcopy(r)
            return None

    def add_audit(self, kind: str, payload: dict, run_id: str | None = None) -> dict:
        with self._lock:
            rec = {"kind": kind, "run_id": run_id, "payload": copy.deepcopy(payload), "ts": now_iso()}
            self.audit.append(rec)
            return copy.deepcopy(rec)

    def list_audit(self, *, run_id: str | None = None, kind: str | None = None, limit: int = 200) -> list[dict]:
        with self._lock:
            rows = [a for a in self.audit if (run_id is None or a["run_id"] == run_id) and (kind is None or a["kind"] == kind)]
            return copy.deepcopy(rows[-limit:])

    def add_span(self, span: dict) -> None:
        with self._lock:
            self.spans[span["span_id"]] = copy.deepcopy(span)

    def end_span(self, span_id: str, **fields: Any) -> None:
        with self._lock:
            if span_id in self.spans:
                self.spans[span_id].update(copy.deepcopy(fields))

    def list_spans(self, *, trace_id: str | None = None, run_id: str | None = None) -> list[dict]:
        with self._lock:
            rows = [s for s in self.spans.values() if (trace_id is None or s.get("trace_id") == trace_id) and (run_id is None or s.get("run_id") == run_id)]
            return copy.deepcopy(sorted(rows, key=lambda s: (s.get("start", ""), s["span_id"])))

    def add_llm_call(self, record: dict) -> None:
        with self._lock:
            self.llm_calls.append(copy.deepcopy(record))

    def list_llm_calls(self, run_id: str | None = None, limit: int = 1000) -> list[dict]:
        with self._lock:
            rows = [r for r in self.llm_calls if run_id is None or r.get("run_id") == run_id]
            return copy.deepcopy(rows[-limit:])

    def incr_metric(self, name: str, labels: dict[str, str] | None = None, value: float = 1.0) -> None:
        with self._lock:
            key = (name, canonical(labels or {}))
            row = self.metrics.setdefault(key, {"name": name, "labels": dict(labels or {}), "value": 0.0})
            row["value"] += value

    def list_metrics(self, name: str | None = None) -> list[dict]:
        with self._lock:
            return copy.deepcopy([v for v in self.metrics.values() if name is None or v["name"] == name])

    def close(self) -> None:
        return None
