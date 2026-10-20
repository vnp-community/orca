"""Full-text search over the elements of a knowledge-graph snapshot: what the backends share.

The text of an element is indexed in three fields (its id, its name and everything else it says); Vietnamese is folded to plain letters on both sides,
so "canh bao thue bao" finds "cảnh báo thuê bao".  A search only *proposes* elements: nothing here decides that an element is evidence.
"""
from __future__ import annotations

import re
import unicodedata
from dataclasses import dataclass, field
from typing import Any

BODY_LIMIT = 20_000
SKIPPED_KEYS = ("source_columns", "sourced_props", "evidence_span")  # copies of text already indexed from the field they were read from


def require_search(store) -> None:
    """A store that cannot search says so (``SEARCH_NOT_SUPPORTED``) instead of answering with nothing found."""
    from ..errors import SearchNotSupported
    if not getattr(store, "supports_search", False):
        raise SearchNotSupported("this SQLite build has no FTS5, so the knowledge graph cannot be searched: use a Python whose sqlite3 has FTS5 enabled", code="SEARCH_NOT_SUPPORTED")


def fold(text: Any) -> str:
    """Lower-case letters without diacritics (``đ`` is ``d``): the form both the index and the query use."""
    s = unicodedata.normalize("NFKD", str(text).replace("đ", "d").replace("Đ", "D"))
    return "".join(ch for ch in s if not unicodedata.combining(ch)).lower()


def tokens_of(query: str) -> list[str]:
    return re.findall(r"[a-z0-9]+", fold(query))


def _strings(value: Any, out: list[str]) -> None:
    if isinstance(value, str):
        out.append(value)
    elif isinstance(value, dict):
        for k, v in value.items():
            if k not in SKIPPED_KEYS:
                _strings(v, out)
    elif isinstance(value, (list, tuple)):
        for v in value:
            _strings(v, out)
    elif isinstance(value, (int, float)) and not isinstance(value, bool):
        out.append(str(value))


def element_text(el: dict) -> tuple[str, str, str]:
    """(id, name, body) of an element, each folded; the body is everything in ``props`` that is text, capped."""
    parts: list[str] = [str(el.get("summary") or "")]   # what the element says about itself outside ``props`` (a requirement's summary repeats its sentence)
    _strings(el.get("props") or {}, parts)
    quote = next((str(x.get("quote") or "") for x in el.get("provenance") or [] if x.get("quote")), "")   # the sentence it was read from
    return fold(el.get("id", "")), fold(el.get("name", "")), fold(" ".join(parts + [quote]))[:BODY_LIMIT]


def excerpt(el: dict, tokens: list[str], width: int = 160) -> str:
    """A window of the element's own wording around the first query token (the original text, not the folded one)."""
    parts: list[str] = [str(el.get("name", ""))]
    _strings(el.get("props") or {}, parts)
    for text in parts:
        folded = fold(text)
        for t in tokens:
            at = folded.find(t)
            if at >= 0 and len(folded) == len(text):
                lo = max(0, at - width // 3)
                return ("…" if lo else "") + text[lo:lo + width].strip() + ("…" if lo + width < len(text) else "")
    return str(el.get("name", ""))[:width]


@dataclass(frozen=True)
class SearchHit:
    id: str
    cls: str
    score: float
    matched: tuple[str, ...]
    excerpt: str
    snapshot: str

    def as_dict(self) -> dict:
        return {"id": self.id, "class": self.cls, "score": round(self.score, 4), "matched": list(self.matched), "excerpt": self.excerpt, "snapshot": self.snapshot}


@dataclass
class Candidate:
    """What a backend returns for one element: the element and how well it matched."""
    element: dict
    score: float
    matched: tuple[str, ...] = field(default_factory=tuple)


def rank_in_memory(elements: list[dict], tokens: list[str], compact: str, limit: int) -> list[Candidate]:
    """Ranking for a store that keeps no index: BM25 over the three fields (id counts 10 times, name 6, text once), words matched as prefixes, plus
    100 for an id fragment.  An element with every word scores above one with some."""
    import math
    docs = []
    for el in elements:
        eid, name, body = element_text(el)
        words = {"id": re.findall(r"[a-z0-9]+", eid), "name": re.findall(r"[a-z0-9]+", name), "body": re.findall(r"[a-z0-9]+", body)}
        docs.append((el, words, eid))
    n = len(docs) or 1
    avg = (sum(len(w["id"]) * 10 + len(w["name"]) * 6 + len(w["body"]) for _, w, _ in docs) / n) or 1.0
    df = {t: sum(1 for _, w, _ in docs if any(x.startswith(t) for part in w.values() for x in part)) for t in tokens}
    out = []
    for el, words, eid in docs:
        length = len(words["id"]) * 10 + len(words["name"]) * 6 + len(words["body"])
        score, matched = 0.0, []
        if compact and len(compact) >= 3 and compact in eid.replace(" ", ""):
            score += 100
            matched.append("id")
        for t in tokens:
            tf = sum(weight * sum(1 for x in words[label] if x.startswith(t)) for label, weight in (("id", 10), ("name", 6), ("body", 1)))
            if not tf:
                continue
            idf = math.log(1 + (n - df[t] + 0.5) / (df[t] + 0.5))
            score += idf * (tf * 2.2) / (tf + 1.2 * (0.25 + 0.75 * length / avg))
            matched += [label for label in ("id", "name", "body") if any(x.startswith(t) for x in words[label])]
        if score > 0:
            out.append(Candidate(el, score, tuple(dict.fromkeys(matched))))
    out.sort(key=lambda c: (-c.score, c.element["id"]))
    return out[:limit]


def score_in_memory(el: dict, tokens: list[str], compact: str, require_all: bool = False) -> Candidate | None:
    """Kept for callers that rank one element on its own (see ``rank_in_memory`` for a whole snapshot)."""
    got = rank_in_memory([el], tokens, compact, 1)
    return got[0] if got else None
