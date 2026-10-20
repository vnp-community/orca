"""Deterministic text utilities shared by logic skills.

Nothing here calls a model.  Anchors are 1-based line numbers (``line:N``), the
same convention ``o5``'s evidence verifier uses.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field
from hashlib import sha256
from typing import Iterable

ID_RE = re.compile(r"\b[A-Z][A-Z0-9]{0,9}(?:-[A-Z0-9]{1,12}){1,4}\b")
HEADING_RE = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
SEPARATOR_RE = re.compile(r"^\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$")
LIST_RE = re.compile(r"^\s*(?:[-*+]|\d+[.)])\s+")
MODAL_RE = re.compile(r"\b(shall|must|should|will|may not|must not|shall not|phải|không được|cần|bắt buộc|tối đa|tối thiểu|không quá|ít nhất)\b", re.I)
CONDITION_RE = re.compile(r"\b(if|when|unless|otherwise|nếu|khi|trừ khi|ngược lại)\b", re.I)
METHOD_PATH_RE = re.compile(r"\b(GET|POST|PUT|PATCH|DELETE)\s+`?(/[^\s`|)]*)")

UNITS = {
    "ms": "ms", "msec": "ms", "milliseconds": "ms", "millisecond": "ms", "s": "s", "sec": "s", "secs": "s", "second": "s", "seconds": "s",
    "giây": "s", "giay": "s", "min": "min", "mins": "min", "minute": "min", "minutes": "min", "phút": "min", "phut": "min",
    "h": "h", "hr": "h", "hrs": "h", "hour": "h", "hours": "h", "giờ": "h", "gio": "h", "d": "d", "day": "d", "days": "d", "ngày": "d", "ngay": "d",
    "%": "%", "kb": "KB", "mb": "MB", "gb": "GB", "tb": "TB", "rps": "req/s", "req/s": "req/s", "tps": "tx/s", "tx/s": "tx/s", "qps": "req/s",
    "lần": "times", "lan": "times", "times": "times", "retries": "times", "retry": "times",
}
QUANTITY_RE = re.compile(r"(?<![\w.])(\d+(?:[.,]\d+)?)\s*(%|req/s|tx/s|[A-Za-zÀ-ỹ]+)", re.U)


def sha(text: str) -> str:
    return sha256(text.encode()).hexdigest()


def normalize_name(text: str) -> str:
    return re.sub(r"[^\w]+", " ", text.lower(), flags=re.U).strip()


def tokens(text: str) -> set[str]:
    return {t for t in normalize_name(text).split() if len(t) > 1}


def jaccard(a: str, b: str) -> float:
    ta, tb = tokens(a), tokens(b)
    return len(ta & tb) / len(ta | tb) if ta and tb else 0.0


def quantities(text: str) -> list[dict]:
    """Numbers with a recognised unit, e.g. ``30 seconds`` -> ``{"value": 30, "unit": "s"}``."""
    found = []
    for m in QUANTITY_RE.finditer(text):
        raw_unit = m.group(2)
        unit = UNITS.get(raw_unit.lower()) if raw_unit != "%" else "%"
        if unit is None:
            continue
        number = m.group(1).replace(",", ".")
        value = float(number)
        found.append({"value": int(value) if value.is_integer() else value, "unit": unit, "raw": m.group(0)})
    return found


def has_quantity(text: str, value, unit: str | None) -> bool:
    for q in quantities(text):
        if float(q["value"]) == float(value) and (unit is None or q["unit"] == UNITS.get(str(unit).lower(), unit)):
            return True
    return False


def ids_in(text: str) -> list[str]:
    return list(dict.fromkeys(ID_RE.findall(text)))


def table_cells(line: str) -> list[str]:
    return [c.strip() for c in re.split(r"(?<!\\)\|", line.strip())[1:-1]] if line.strip().startswith("|") else []


@dataclass
class Block:
    line: int
    kind: str
    text: str
    heading_path: list[str] = field(default_factory=list)
    cells: list[str] | None = None
    header: list[str] | None = None

    @property
    def id(self) -> str:
        return f"line:{self.line}"

    def as_dict(self) -> dict:
        d = {"id": self.id, "line": self.line, "kind": self.kind, "text": self.text, "heading_path": list(self.heading_path)}
        if self.cells is not None:
            d["cells"], d["header"] = self.cells, self.header or []
        return d


def segment(text: str) -> list[Block]:
    """Split Markdown into line-anchored blocks (headings, table rows, list items, paragraphs, code)."""
    blocks: list[Block] = []
    path: list[tuple[int, str]] = []
    in_code = False
    header: list[str] | None = None
    prev_table_line = 0
    lines = text.splitlines()
    for n, line in enumerate(lines, 1):
        stripped = line.strip()
        if FENCE_RE.match(line):
            in_code = not in_code
            blocks.append(Block(n, "code", line, [p[1] for p in path]))
            continue
        if in_code:
            blocks.append(Block(n, "code", line, [p[1] for p in path]))
            continue
        if not stripped:
            header = None
            continue
        m = HEADING_RE.match(line)
        if m:
            level = len(m.group(1))
            path = [p for p in path if p[0] < level] + [(level, m.group(2))]
            header = None
            blocks.append(Block(n, "heading", line, [p[1] for p in path]))
            continue
        hp = [p[1] for p in path]
        if stripped.startswith("|"):
            cells = table_cells(line)
            if SEPARATOR_RE.match(stripped):
                continue
            nxt = lines[n].strip() if n < len(lines) else ""
            if header is None and SEPARATOR_RE.match(nxt):
                header = cells
                blocks.append(Block(n, "table_header", line, hp, cells, cells))
            else:
                blocks.append(Block(n, "table_row", line, hp, cells, header or []))
            prev_table_line = n
            continue
        header = None
        blocks.append(Block(n, "list_item" if LIST_RE.match(line) else "paragraph", line, hp))
    return blocks


def prose_candidates(blocks: Iterable[Block]) -> list[Block]:
    """Paragraph/list lines worth sending to a model: they state an obligation, a condition or a quantity."""
    out = []
    for b in blocks:
        if b.kind not in ("paragraph", "list_item"):
            continue
        t = b.text
        if len(t.strip()) < 12:
            continue
        if MODAL_RE.search(t) or CONDITION_RE.search(t) or quantities(t) or METHOD_PATH_RE.search(t):
            out.append(b)
    return out


def evidence(source_text: str, source_id: str, revision: str, line: int) -> dict:
    """An ``o5``-compatible extracted-evidence record for one source line."""
    quote = source_text.splitlines()[line - 1]
    return {"kind": "extracted", "source_id": source_id, "source_version": str(revision), "source_hash": sha(source_text),
            "block": f"line:{line}", "quote": quote, "quote_hash": sha(quote)}


_TO_SECONDS = {"ms": 0.001, "s": 1, "min": 60, "h": 3600, "d": 86400}


def number_supported(quote: str, value) -> bool:
    """True when ``value`` appears in ``quote`` as a number, or equals a quoted duration converted to s/ms/min/h/d."""
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return True
    wanted = float(value)
    for m in re.finditer(r"(?<![\w.])(\d+(?:[.,]\d+)?)", quote):
        if float(m.group(1).replace(",", ".")) == wanted:
            return True
    for q in quantities(quote):
        if q["unit"] in _TO_SECONDS:
            seconds = float(q["value"]) * _TO_SECONDS[q["unit"]]
            if any(abs(seconds / f - wanted) < 1e-9 for f in _TO_SECONDS.values()):
                return True
    return False


def numbers_in_props(value) -> list:
    """Every numeric (non-bool) leaf of a props structure."""
    if isinstance(value, bool):
        return []
    if isinstance(value, (int, float)):
        return [value]
    if isinstance(value, dict):
        return [n for v in value.values() for n in numbers_in_props(v)]
    if isinstance(value, (list, tuple)):
        return [n for v in value for n in numbers_in_props(v)]
    return []


# ----------------------------------------------------------------------------- limits (capacity caps stated in a requirement)
_LIMIT_BOUND = re.compile(r"(?:tối đa|không quá|at most|up to|max(?:imum)?|<=|≤|vượt|exceed(?:s|ing)?|\bover\b)", re.I | re.U)
_LIMIT_PAIR = re.compile(r"(\d{1,3}(?:[.,]\d{3})+|\d+(?:[.,]\d+)?)\s*(components?|mb|gb|kb|bytes?|dòng|bản ghi|tệp|files?|dự án|người dùng|projects?|users?|records?|rows?|items?|requests?|req/s|tx/s)\b", re.I | re.U)
_THOUSANDS = re.compile(r"^\d{1,3}(?:[.,]\d{3})+$")


def _limit_number(raw: str) -> float:
    if _THOUSANDS.match(raw):
        return float(re.sub(r"[.,]", "", raw))
    return float(raw.replace(",", "."))


def limits_in(text: str) -> dict[str, float]:
    """Capacity caps a statement declares: ``unit -> value`` for ``<bound word> N <count/size unit>`` (time and % targets are not caps).

    "SBOM tối đa 50 MB và 20.000 component" -> {"mb": 50, "component": 20000}.  Lowering such a cap stays inside the
    envelope the design was made for; raising it does not.
    """
    found: dict[str, float] = {}
    for m in _LIMIT_BOUND.finditer(text):
        window = re.split(r"[;\n]", text[m.end(): m.end() + 110])[0]
        for num, unit in _LIMIT_PAIR.findall(window):
            key = unit.lower().rstrip("s") if unit.lower() not in ("mb", "gb", "kb", "req/s", "tx/s") else unit.lower()
            found[key] = max(found.get(key, 0.0), _limit_number(num))
    return found


def strip_limits(text: str) -> str:
    """The statement with the numbers of its caps removed (what is left must be identical for a cap-only change)."""
    caps = {m.group(0) for m in _LIMIT_PAIR.finditer(text)} if limits_in(text) else set()
    for c in caps:
        text = text.replace(c, " ")
    return text
