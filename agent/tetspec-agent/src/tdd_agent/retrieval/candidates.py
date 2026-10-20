"""Optional lexical candidate finder (off by default).

Lexical scoring only *proposes* seeds.  ``verify_candidates`` is the mandatory
gate: a candidate must exist in the snapshot, pass the authority checks of the
closure service and carry resolvable provenance before it can seed a closure.
"""
from __future__ import annotations

import math
import re
from collections import Counter
from typing import Iterable

from ..store.base import KnowledgeStore
from .closure import ClosureService, SeedSpec, provenance_of

_TOKEN = re.compile(r"[\w]+", re.U)
TEXT_FIELDS = ("name", "summary", "statement", "question", "rationale", "purpose")


def _tokens(text: str) -> list[str]:
    return [t.lower() for t in _TOKEN.findall(text) if len(t) > 1]


def _text(el: dict) -> str:
    p = el.get("props") or {}
    definition = p.get("definition")
    definition = " ".join(str(c) for c in definition) if isinstance(definition, list) else str(definition or "")
    quote = next((str(x.get("quote") or "") for x in el.get("provenance") or [] if x.get("quote")), "")
    return " ".join(str(el.get(f) or p.get(f) or "") for f in TEXT_FIELDS) + " " + definition + " " + quote + " " + el["id"]


class LexicalCandidates:
    """Seed proposals for a free-text query: the store's full-text index when asked for (``[retrieval] lexical``) and the store has one, else a BM25 scan in memory."""

    def __init__(self, store: KnowledgeStore, snapshot=None, classes: Iterable[str] | None = None, use_index: bool = False):
        self.store = store
        info = store.get_snapshot(snapshot)
        self.snapshot = info.id
        self.indexed = bool(use_index and getattr(store, "supports_search", False))
        self.classes = tuple(classes) if classes else ()
        if self.indexed:
            self.docs, self.avg, self.df, self.n = {}, 0.0, Counter(), 0
            return
        wanted = set(classes) if classes else None
        docs = {}
        for el in store.materialize(info.id)["elements"]:
            if wanted is None or el.get("class") in wanted:
                docs[el["id"]] = Counter(_tokens(_text(el)))
        self.docs = docs
        self.avg = (sum(sum(c.values()) for c in docs.values()) / len(docs)) if docs else 0.0
        df: Counter = Counter()
        for c in docs.values():
            df.update(c.keys())
        self.df, self.n = df, len(docs)

    def search(self, query: str, k: int = 10) -> list[tuple[str, float]]:
        if self.indexed:
            return [(h.id, round(h.score, 6)) for h in self.store.search(query, self.snapshot, self.classes or None, k)]
        terms = _tokens(query)
        scored = []
        for ident, tf in self.docs.items():
            length, score = sum(tf.values()), 0.0
            for t in terms:
                if t not in tf:
                    continue
                idf = math.log(1 + (self.n - self.df[t] + 0.5) / (self.df[t] + 0.5))
                score += idf * (tf[t] * 2.2) / (tf[t] + 1.2 * (0.25 + 0.75 * length / (self.avg or 1)))
            if score > 0:
                scored.append((ident, round(score, 6)))
        scored.sort(key=lambda x: (-x[1], x[0]))
        return scored[:k]


def verify_candidates(service: ClosureService, ids: Iterable[str], snapshot=None) -> SeedSpec:
    """Keep only candidates that survive graph + source verification."""
    info = service.store.get_snapshot(snapshot)
    role_of = {d.id: d.role for d in service.store.list_sources()}
    kept, rejected = [], []
    for el in service.store.get_elements(sorted(set(ids)), info.id):
        conflicts, exclude = service._authority(el, role_of, {})
        has_source = any(p.get("source_id") for p in provenance_of(el))
        if exclude or not has_source:
            rejected.append({"id": el["id"], "reason": [c["code"] for c in conflicts] or ["NO_RESOLVABLE_PROVENANCE"]})
        else:
            kept.append(el["id"])
    return SeedSpec("candidate_verified", tuple(kept), {"rejected": rejected, "snapshot": info.id})
