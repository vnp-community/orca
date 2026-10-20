"""From a change request to a proposed SRS: the rules around the one step that needs a model.

The model only *proposes* edits to requirement rows (modify a fragment, add a row, remove a row), each justified by a verbatim
quote of the change request.  Everything else is rules: candidates are chosen by word overlap, every proposal is verified
(ids exist, the fragment to replace occurs once, numbers come from the CR, the quote is in the CR), new ids are assigned by the
id family, the rows are patched in the SRS text, and the result is checked by reading the patched SRS again.
"""
from __future__ import annotations

import difflib
import re
from dataclasses import dataclass

from . import facts as F, text as T

ROW_CLASSES = ("Requirement", "QualityTarget", "BusinessRule")
_AC_ID = re.compile(r"\bAC-[A-Za-z0-9][A-Za-z0-9_-]*")
_CR_ID = re.compile(r"\bCR-[A-Za-z0-9][A-Za-z0-9_-]*")
_NUMBER = re.compile(r"(?<![\w.-])\d+(?:[.,]\d+)?")
_SEPARATOR = re.compile(r"^\s*\|?[\s:|-]+\|[\s:|-]*$")
_ESCAPES = re.compile(r"\\([_\[\]*])")


def unescape(text: str) -> str:
    return _ESCAPES.sub(r"\1", text).strip()


# ----------------------------------------------------------------------------- the change request as a document
def parse_cr(text: str) -> dict:
    """Title (first heading or first line), id (a ``CR-…`` token) and acceptance criteria (``AC-…`` ids with the text that follows them)."""
    lines = [l for l in text.split("\n")]
    title = next((re.sub(r"^#+\s*", "", l).strip() for l in lines if l.strip()), "")
    ids = list(dict.fromkeys(_CR_ID.findall(text)))
    acceptance: dict[str, str] = {}
    for line in lines:
        for m in _AC_ID.finditer(line):
            tail = line[m.end():].strip(" \t:-–—)|*`")
            if m.group(0) not in acceptance or (tail and not acceptance[m.group(0)]):
                acceptance[m.group(0)] = tail if tail and not _AC_ID.match(tail) else acceptance.get(m.group(0), "")
    return {"title": title, "cr_id": ids[0] if ids else None, "acceptance": list(acceptance), "acceptance_text": acceptance}


# ----------------------------------------------------------------------------- requirement rows of an SRS
def requirement_rows(text: str, id_class_map: dict | None = None) -> list[dict]:
    """Rows of the SRS that state a requirement: id, statement, line, the index of the statement cell."""
    facts = F.structured_facts(text, "SRS", "0", T.segment(text), id_class_map)
    lines = text.split("\n")
    rows, seen = [], set()
    for el in facts["elements"]:
        if el["class"] not in ROW_CLASSES or el["id"] in seen:
            continue
        statement = unescape(str(el["name"]))
        for p in el["provenance"]:
            n = int(p["block"].split(":")[1])
            cells = T.table_cells(lines[n - 1])
            if cells and cells[0] == el["id"]:
                idx = next((i for i, c in enumerate(cells) if i and unescape(c) == statement), None)
                if idx is not None:
                    rows.append({"id": el["id"], "statement": statement, "line": n, "statement_cell": idx, "class": el["class"]})
                    seen.add(el["id"])
                    break
    return rows


def candidates(rows: list[dict], cr_text: str, limit: int = 60) -> list[dict]:
    """The requirements the model may touch: those the CR names by id first, then by word overlap with the CR (all of them when there are few)."""
    if len(rows) <= limit:
        return rows
    named = set(T.ids_in(cr_text))
    ranked = sorted(rows, key=lambda r: (r["id"] not in named, -T.jaccard(cr_text, r["statement"]), r["id"]))
    return sorted(ranked[:limit], key=lambda r: r["id"])


# ----------------------------------------------------------------------------- verification of the proposal
def _numbers(text: str) -> set[str]:
    return {n.replace(",", ".") for n in _NUMBER.findall(T.ID_RE.sub(" ", text))}


def _squash(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip().lower()


def verify_edits(edits: list[dict], cr_text: str, rows: list[dict], acceptance_ids: list[str], known_ids: set[str] = frozenset()) -> list[str]:
    """Error codes (empty when the proposal is acceptable).  Ids a new wording mentions must be in the CR or already in the SRS."""
    by_id = {r["id"]: r for r in rows}
    cr_numbers, cr_flat = _numbers(cr_text), _squash(cr_text)
    allowed_ids = set(T.ids_in(cr_text)) | set(known_ids)
    errors: list[str] = []
    if not edits:
        return ["CR_NO_REQUIREMENT_CHANGE"]
    touched: set[str] = set()
    spans: dict[str, list[tuple[int, int]]] = {}  # modified fragments of a row: several are merged into one change when they do not overlap
    for i, e in enumerate(edits, 1):
        tag = f"#{i}"
        op = e.get("op")
        quote = str(e.get("cr_quote") or "")
        if not quote.strip() or _squash(quote) not in cr_flat:
            errors.append(f"EDIT_QUOTE_NOT_IN_CR:{tag}")
        for ac in e.get("acceptance_ids") or []:
            if ac not in acceptance_ids:
                errors.append(f"EDIT_ACCEPTANCE_UNKNOWN:{tag}:{ac}")
        if op in ("modify", "remove"):
            row = by_id.get(e.get("id"))
            if row is None:
                errors.append(f"EDIT_TARGET_UNKNOWN:{tag}:{e.get('id')}")
                continue
            if row["id"] in touched and not (op == "modify" and row["id"] in spans):
                errors.append(f"EDIT_TARGET_REPEATED:{tag}:{row['id']}")
            elif op == "modify" and e.get("find") and row["statement"].count(str(e["find"])) == 1:
                start = row["statement"].index(str(e["find"]))
                end = start + len(str(e["find"]))
                if any(start < b and a < end for a, b in spans.get(row["id"], [])):
                    errors.append(f"EDIT_TARGET_OVERLAP:{tag}:{row['id']}")
                spans.setdefault(row["id"], []).append((start, end))
            touched.add(row["id"])
        if op == "modify" and row is not None:
            find, replace = str(e.get("find") or ""), str(e.get("replace") or "")
            if not find or find == replace:
                errors.append(f"EDIT_FRAGMENT_INVALID:{tag}")
            elif row["statement"].count(find) != 1:
                errors.append(f"EDIT_FIND_NOT_IN_STATEMENT:{tag}" if find not in row["statement"] else f"EDIT_FIND_AMBIGUOUS:{tag}")
            elif not (_numbers(replace) - _numbers(find)) <= cr_numbers:
                errors.append(f"EDIT_NUMBER_NOT_IN_CR:{tag}")
            elif not (set(T.ids_in(replace)) - set(T.ids_in(find))) <= allowed_ids:
                errors.append(f"EDIT_ID_NOT_IN_CR:{tag}")
        elif op == "add":
            near = by_id.get(e.get("near_id"))
            statement = str(e.get("statement") or "").strip()
            if near is None:
                errors.append(f"EDIT_NEAR_UNKNOWN:{tag}:{e.get('near_id')}")
            if not statement:
                errors.append(f"EDIT_STATEMENT_EMPTY:{tag}")
            elif not _numbers(statement) <= cr_numbers:
                errors.append(f"EDIT_NUMBER_NOT_IN_CR:{tag}")
            elif not set(T.ids_in(statement)) <= allowed_ids:
                errors.append(f"EDIT_ID_NOT_IN_CR:{tag}")
        elif op not in ("modify", "remove"):
            errors.append(f"EDIT_OP_UNKNOWN:{tag}")
    return sorted(set(errors))


# ----------------------------------------------------------------------------- applying the proposal to the SRS text
def _next_id(near_id: str, text: str, taken: set[str]) -> str | None:
    m = re.match(r"^(.*?)(\d+)$", near_id)
    if not m:
        return None
    prefix, width = m.group(1), len(m.group(2))
    numbers = [int(x[len(prefix):]) for x in set(T.ids_in(text)) | taken if x.startswith(prefix) and x[len(prefix):].isdigit()]
    return f"{prefix}{max(numbers, default=0) + 1:0{width}d}"


def _header_of(lines: list[str], index: int) -> list[str]:
    """Header cells of the table that contains line ``index`` (0-based), or []."""
    top = index
    while top > 0 and lines[top - 1].lstrip().startswith("|"):
        top -= 1
    if top + 1 < len(lines) and _SEPARATOR.match(lines[top + 1]) and lines[top].lstrip().startswith("|"):
        return T.table_cells(lines[top])
    return []


@dataclass
class Applied:
    text: str
    changes: list[dict]
    errors: list[str]


def apply_edits(text: str, edits: list[dict], rows: list[dict], acceptance_text: dict[str, str]) -> Applied:
    lines = text.split("\n")
    by_id = {r["id"]: r for r in rows}
    changes: list[dict] = []
    errors: list[str] = []
    inserts: dict[int, list[str]] = {}
    removals: set[int] = set()
    new_ids: set[str] = set()
    for e in edits:
        op = e["op"]
        if op in ("modify", "remove"):
            row = by_id[e["id"]]
            idx = row["line"] - 1
            if op == "remove":
                removals.add(idx)
                changes.append({"op": "remove", "id": row["id"], "line": row["line"], "before": row["statement"], "cr_quote": e["cr_quote"], "acceptance_ids": list(e.get("acceptance_ids") or [])})
                continue
            raw = re.split(r"(?<!\\)\|", lines[idx])
            cell = row["statement_cell"] + 1  # split() keeps the empty text before the first pipe
            if raw[cell].count(e["find"]) != 1:
                errors.append(f"EDIT_FIND_NOT_IN_ROW:{row['id']}")
                continue
            raw[cell] = raw[cell].replace(e["find"], e["replace"], 1)
            lines[idx] = "|".join(raw)
            earlier = next((c for c in changes if c["op"] == "modify" and c["id"] == row["id"]), None)
            if earlier:  # several fragments of one requirement are one change
                earlier["after"] = unescape(raw[cell])
                earlier["cr_quote"] = f"{earlier['cr_quote']} / {e['cr_quote']}"
                earlier["acceptance_ids"] = list(dict.fromkeys(earlier["acceptance_ids"] + list(e.get("acceptance_ids") or [])))
            else:
                changes.append({"op": "modify", "id": row["id"], "line": row["line"], "before": row["statement"], "after": unescape(raw[cell]), "cr_quote": e["cr_quote"], "acceptance_ids": list(e.get("acceptance_ids") or [])})
        else:
            near = by_id[e["near_id"]]
            idx = near["line"] - 1
            new_id = _next_id(near["id"], text, new_ids)
            header = _header_of(lines, idx)
            cells = T.table_cells(lines[idx])
            if new_id is None or not header or len(header) != len(cells):
                errors.append(f"EDIT_ROW_LAYOUT_UNKNOWN:{near['id']}")
                continue
            new_ids.add(new_id)
            ac_text = "; ".join(f"{a}: {acceptance_text.get(a) or ''}".rstrip(": ").strip() for a in e.get("acceptance_ids") or [])
            out = []
            for i, (name, old) in enumerate(zip(header, cells)):
                if i == 0:
                    out.append(new_id)
                elif i == near["statement_cell"]:
                    out.append(str(e["statement"]).strip())
                elif re.search(r"accept|nghiệm thu", name, re.I):
                    out.append(ac_text)
                else:
                    out.append("")
            inserts.setdefault(idx, []).append("| " + " | ".join(out) + " |")
            changes.append({"op": "add", "id": new_id, "near_id": near["id"], "after": str(e["statement"]).strip(), "cr_quote": e["cr_quote"], "acceptance_ids": list(e.get("acceptance_ids") or [])})
    result: list[str] = []
    for i, line in enumerate(lines):
        if i not in removals:
            result.append(line)
        result.extend(inserts.get(i, []))
    return Applied("\n".join(result), changes, errors)


_RENAME_PATTERNS = (
    re.compile(r"(?:đổi|chuyển|thay)\s+(?:tên\s+|thuật ngữ\s+|từ\s+)?[\"“'‘]?(?P<old>[^\"”'’,.;:\n→]{2,40}?)[\"”'’]?\s+(?:thành|sang|bằng)\s+[\"“'‘]?(?P<new>[^\"”'’,.;:\n→]{2,40}?)[\"”'’]?(?=[\s,.;:]|$)", re.I),
    re.compile(r"rename\s+[\"“'‘]?(?P<old>[^\"”'’,.;:\n]{2,40}?)[\"”'’]?\s+to\s+[\"“'‘]?(?P<new>[^\"”'’,.;:\n]{2,40}?)[\"”'’]?(?=[\s,.;:]|$)", re.I),
    re.compile(r"[\"“'‘](?P<old>[^\"”'’\n→]{2,40})[\"”'’]\s*(?:→|->)\s*[\"“'‘](?P<new>[^\"”'’\n]{2,40})[\"”'’]"),
)


def renames_in_cr(cr_text: str) -> list[tuple[str, str]]:
    """Terms the CR renames ("đổi "Handler" thành "Assignee"", "rename A to B", "A" → "B"), found by pattern, not by the model."""
    found: list[tuple[str, str]] = []
    for pattern in _RENAME_PATTERNS:
        for m in pattern.finditer(cr_text):
            pair = (m.group("old").strip(), m.group("new").strip())
            if pair[0] and pair[1] and pair[0].lower() != pair[1].lower() and pair not in found:
                found.append(pair)
    return found


def term_sweep(rows: list[dict], renames: list[tuple[str, str]], edited_ids: set[str]) -> list[dict]:
    """Requirements of the SRS that still use a renamed term and that no edit of the proposal changes: the BA decides whether they follow the rename."""
    out = []
    for old, new in renames:
        pattern = re.compile(rf"(?<!\w){re.escape(old)}(?!\w)", re.I)
        hits = [r["id"] for r in rows if pattern.search(r["statement"]) and r["id"] not in edited_ids]
        if hits:
            out.append({"term": old, "renamed_to": new, "rows_not_changed": hits})
    return out


def check_applied(new_text: str, changes: list[dict], id_class_map: dict | None = None) -> list[str]:
    """Read the patched SRS again: every added id is a row, every removed id is gone, every modified statement carries its new wording."""
    after = {r["id"]: r for r in requirement_rows(new_text, id_class_map)}
    errors = []
    for c in changes:
        if c["op"] == "add" and (c["id"] not in after or c["after"] not in after[c["id"]]["statement"] and unescape(c["after"]) != after[c["id"]]["statement"]):
            errors.append(f"SRS_PROPOSAL_ADD_NOT_READ_BACK:{c['id']}")
        elif c["op"] == "remove" and c["id"] in after:
            errors.append(f"SRS_PROPOSAL_REMOVE_NOT_APPLIED:{c['id']}")
        elif c["op"] == "modify" and (c["id"] not in after or after[c["id"]]["statement"] != c["after"]):
            errors.append(f"SRS_PROPOSAL_MODIFY_NOT_READ_BACK:{c['id']}")
    return errors


def diff_lines(before: str, after: str) -> list[str]:
    return [l for l in difflib.unified_diff(before.split("\n"), after.split("\n"), "SRS (before)", "SRS (proposed from the CR)", lineterm="", n=0)]


_GWT = re.compile(r"\bGIVEN\b.+\bWHEN\b.+\bTHEN\b", re.I | re.S)


def verifications_for(changes: list[dict], acceptance_text: dict[str, str], acceptance_ids: list[str]) -> dict[str, str]:
    """Verification of each changed requirement, read from the CR's acceptance criteria that are written as GIVEN / WHEN / THEN.

    The criteria an edit names are used; an edit that names none inherits the CR's only criterion when there is exactly one.  A criterion that is not
    in that form gives no verification (it is reported as missing downstream, never rewritten).
    """
    out = {}
    for c in changes:
        if c["op"] == "remove":
            continue
        ids = c.get("acceptance_ids") or (acceptance_ids if len(acceptance_ids) == 1 else [])
        texts = [acceptance_text[i].strip() for i in ids if _GWT.search(acceptance_text.get(i, ""))]
        if texts:
            out[c["id"]] = "; ".join(texts)
    return out
