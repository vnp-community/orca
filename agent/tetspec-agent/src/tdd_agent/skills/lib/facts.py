"""Build ontology elements from anchored source facts (deterministic)."""
from __future__ import annotations

import re
from typing import Iterable

from . import text as T

DEFAULT_ID_CLASS = {
    "FR": "Requirement", "SRS": "Requirement", "REQ": "Requirement", "BRL": "BusinessRule", "BR": "BusinessRule", "NFR": "QualityTarget",
    "API": "Operation", "ENT": "Entity", "ADR": "Decision", "SVC": "Component", "WRK": "Component", "M": "Module", "CT": "Constraint", "INT": "Constraint",
    "ALG": "Algorithm", "MIG": "Migration", "IF": "Interface", "HA": "HighAvailabilityTopology", "RTO": "RecoveryObjective",
    "BO": "Requirement",   # business objective of the URD: a statement with an indicator
}
# ids that name something without being a fact of their own: acceptance criteria live in the CR and in the "acceptance" of the requirement they verify
REFERENCE_ONLY_PREFIXES = ("AC",)
EDGE_REL = {("Module", "Module"): "depends_on"}
# a requirement row that names one of these (an interface, a component, an algorithm …) says what realizes it: the element satisfies the requirement
REALIZER_CLASSES = ("Operation", "Component", "Module", "Algorithm", "Interface", "Job", "Flow", "Event")


def relation_between(src_class: str, dst_class: str) -> str:
    """``satisfies`` when the ontology allows it (design element -> requirement-like target), else a plain trace."""
    if (src_class, dst_class) in EDGE_REL:
        return EDGE_REL[(src_class, dst_class)]
    from ...core import o5_path
    spec = o5_path.load().ontology5.legacy_relation_specs().get("satisfies", {})
    return "satisfies" if src_class in spec.get("domain", []) and dst_class in spec.get("range", []) else "traces_to"


CONTRACT_HEADERS = {"auth": ("auth", "authentication", "authorization", "xác thực", "xac thuc"), "request": ("request", "yêu cầu", "yeu cau", "input"),
                    "response": ("response", "phản hồi", "phan hoi", "output"), "errors": ("errors", "error", "lỗi", "loi", "error codes")}


def contract_columns(header: list[str], cells: list[str]) -> dict[str, str]:
    """Operation contract fields stated in dedicated table columns (never inferred)."""
    out = {}
    for name, value in zip(header, cells):
        key = next((k for k, aliases in CONTRACT_HEADERS.items() if name.strip().lower() in aliases), None)
        if key and value.strip() and value.strip() not in ("-", "n/a", "N/A"):
            out[key] = value.strip()
    return out


def range_columns(header: list[str], cells: list[str]) -> dict[str, float]:
    """A designed value range stated in ``min``/``max`` columns (used to prove a parameter change stays inside the design)."""
    out = {}
    for name, value in zip(header, cells):
        key = {"min": "min", "minimum": "min", "tối thiểu": "min", "max": "max", "maximum": "max", "tối đa": "max"}.get(name.strip().lower())
        if key:
            try:
                number = float(value.strip().replace(",", "."))
            except ValueError:
                continue
            out[key] = int(number) if number.is_integer() else number
    return out


def class_for_id(ident: str, id_class_map: dict[str, str] | None) -> str | None:
    prefix = ident.split("-", 1)[0]
    return {**DEFAULT_ID_CLASS, **(id_class_map or {})}.get(prefix)


BOOKKEEPING_PROPS = ("source_section", "logical_id", "revision", "source_columns", "upstream_definition", "sourced_props", "evidence_span", "declared_method", "logical_attribute_ids")


LIST_FIELDS = {"failure_domains", "stages", "invariants", "depends_on", "preconditions", "bound_projects"}
QUANTITY_FIELDS = {"rto", "rpo", "average", "peak", "payload", "growth"}


def _typed(value: str, key: str = ""):
    """Cell text to a value: lists, quantities, booleans and plain numbers become typed where the ontology expects it; otherwise text (no guessing)."""
    if key in LIST_FIELDS:
        return [p.strip() for p in re.split(r"[;,]", value) if p.strip()]
    if key in QUANTITY_FIELDS:
        found = T.quantities(value)
        if len(found) == 1 and found[0]["raw"].strip().lower() == value.strip().lower():
            return {"value": found[0]["value"], "unit": found[0]["unit"]}
        if re.fullmatch(r"-?\d+(?:\.\d+)?", value.strip()):
            return float(value) if "." in value else int(value)
    low = value.lower()
    if low in ("true", "yes", "có"):
        return True
    if low in ("false", "no", "không"):
        return False
    if re.fullmatch(r"-?\d+", value):
        return int(value)
    if re.fullmatch(r"-?\d+\.\d+", value):
        return float(value)
    return value


def sourced_prop_names(props: dict) -> list[str]:
    """Properties whose value was read from a source document: a prompt may not overwrite them (the source is the authority)."""
    return sorted(k for k in props if k not in BOOKKEEPING_PROPS)


def element_from_row(source_text: str, source_id: str, revision: str, block: T.Block, ident: str, cls: str) -> dict:
    cells = block.cells or []
    header = block.header or []
    columns = dict(zip(header, cells)) if header and len(header) == len(cells) else {"columns": cells}
    name = cells[1] if len(cells) > 1 and cells[1] else ident
    props: dict = {"definition": cells[1:], "source_section": " > ".join(block.heading_path), "logical_id": ident,
                   "revision": str(revision), "source_columns": columns}
    quantities = T.quantities(block.text)
    if quantities:
        props["quantities"] = quantities
    if cls == "Requirement":
        props["statement"] = name
        props["acceptance"] = cells[-1] if source_id.lower().startswith("srs") and len(cells) > 2 else None
    elif cls == "BusinessRule":
        props["predicate"] = name
    elif cls == "QualityTarget":
        props["target"] = cells[1:]
    elif cls == "Operation":
        m = T.METHOD_PATH_RE.search(block.text)
        if m:
            props.update(method=m.group(1), path=m.group(2))
        for field_name, value in contract_columns(header, cells).items():
            props[field_name] = value
    elif cls == "Interface":
        # an interface listed under "external interfaces / bên ngoài / third party" is a contract with a system we do not control
        props["external"] = bool(re.search(r"bên ngoài|external|third.?party|bên thứ ba|tích hợp", props["source_section"], re.I))
    elif cls == "Decision" and len(cells) >= 5:
        props.update(chosen=cells[1], rationale=cells[2], alternatives=cells[3], tradeoffs=cells[4])
    rng = range_columns(header, cells)
    if rng:
        props["range"] = rng
    for head, value in zip(header, cells):
        key = re.sub(r"[^a-z0-9]+", "_", head.strip().lower()).strip("_")
        if key and key not in props and key not in ("id", "name") and value.strip() not in ("", "-", "n/a", "N/A"):
            props[key] = _typed(value.strip(), key)
    props["sourced_props"] = sourced_prop_names(props)
    return {"id": ident, "class": cls, "name": name, "origin": "upstream", "maturity": "sketch", "owner_doc": source_id,
            "version_introduced": str(revision), "review_status": "needs_review", "props": props,
            "provenance": [T.evidence(source_text, source_id, revision, block.line)]}


ATTRIBUTE_HEADERS = ("attributes", "properties", "fields", "thuộc tính", "thuộc tính chính", "thuoc tinh")
FROM_HEADERS, TO_HEADERS = ("from", "từ", "source state", "tu"), ("to", "đến", "target state", "den")
STATE_STATEMENT = re.compile(r"^\*{0,2}(?P<id>" + T.ID_RE.pattern + r")\b[^.]*?\.?\*{0,2}\s*(?:Trạng thái|States?|Status)\s*:\s*(?P<states>[^.]+)\.", re.I)
ERROR_ID = re.compile(r"^`?([a-z][a-z0-9_]{2,})`?$")


def _split_names(value: str) -> list[str]:
    out = []
    for part in re.split(r",(?![^\[]*\])", value.replace("\\", "")):
        name = re.sub(r"\[\]$", "", part.strip().strip("`"))
        if re.fullmatch(r"[A-Za-z][A-Za-z0-9_/]*", name):
            out.append(name)
    return list(dict.fromkeys(out))


def _derived(source_text: str, source_id: str, revision: str, line: int) -> list[dict]:
    return [T.evidence(source_text, source_id, revision, line)]


def structured_facts(source_text: str, source_id: str, revision: str, blocks: Iterable[T.Block], id_class_map: dict[str, str] | None = None) -> dict:
    """Definition rows (an id in the first, or the second after a short type column) become elements; error-code rows,
    attribute columns and state-machine statements with their transition tables become typed elements too.
    References to ids that no row defines are kept as gaps, never as invented elements."""
    elements: dict[str, dict] = {}
    edges: list[dict] = []
    gaps: list[dict] = []
    machine: dict | None = None
    blocks = list(blocks)
    for b in blocks:
        if b.kind in ("heading",):
            machine = None
        if b.kind in ("paragraph", "list_item"):
            sm = STATE_STATEMENT.match(b.text.strip())
            if sm and {**DEFAULT_ID_CLASS, **(id_class_map or {})}.get(sm.group("id").split("-", 1)[0]):
                owner = sm.group("id")
                states = _split_names(sm.group("states"))
                if states:
                    mid = f"SM-{owner}"
                    ev = _derived(source_text, source_id, revision, b.line)
                    base = {"origin": "upstream", "maturity": "sketch", "owner_doc": source_id, "version_introduced": str(revision), "review_status": "needs_review"}
                    elements[mid] = {**base, "id": mid, "class": "StateMachine", "name": f"{owner} lifecycle", "props": {"states": states, "owner_entity": owner}, "provenance": ev}
                    for st in states:
                        elements[f"{mid}:{st}"] = {**base, "id": f"{mid}:{st}", "class": "State", "name": st, "props": {"machine": mid}, "provenance": ev}
                        edges.append({"src": mid, "rel": "has_state", "dst": f"{mid}:{st}", "provenance": ev})
                    machine = {"id": mid, "owner": owner, "states": set(states)}
            continue
        if b.kind != "table_row" or not b.cells:
            continue
        header = [h.strip().lower() for h in (b.header or [])]
        if machine and len(header) >= 2 and header[0] in FROM_HEADERS and header[1] in TO_HEADERS:
            frm, to = b.cells[0].strip(), b.cells[1].strip()
            rest = b.cells[2:]
            tid = f"TR-{machine['id']}:{frm}->{to}:{T.sha((rest[0] if rest else '') + '|' + (rest[1] if len(rest) > 1 else ''))[:6]}"  # stable across line shifts
            ev = _derived(source_text, source_id, revision, b.line)
            elements[tid] = {"origin": "upstream", "maturity": "sketch", "owner_doc": source_id, "version_introduced": str(revision), "review_status": "needs_review", "id": tid,
                             "class": "Transition", "name": f"{frm} → {to}", "props": {"from": frm, "to": to, "event": rest[0] if rest else "", "guard": rest[1] if len(rest) > 1 else "",
                                                                                       "side_effects": rest[2] if len(rest) > 2 else ""}, "provenance": ev}
            edges.append({"src": tid, "rel": "in_machine", "dst": machine["id"], "provenance": ev})
            for rel, st in (("transition_from", frm), ("transition_to", to)):
                if st in machine["states"]:
                    edges.append({"src": tid, "rel": rel, "dst": f"{machine['id']}:{st}", "provenance": ev})
            continue
        first = re.sub(r"[`*]", "", b.cells[0]).strip()
        if re.fullmatch(r"[45]\d\d", first) and len(b.cells) > 1 and ERROR_ID.match(b.cells[1].strip()):
            code = ERROR_ID.match(b.cells[1].strip()).group(1)
            eid = f"ERR-{code}"
            if eid not in elements:
                elements[eid] = {"origin": "upstream", "maturity": "sketch", "owner_doc": source_id, "version_introduced": str(revision), "review_status": "needs_review", "id": eid,
                                 "class": "ErrorCode", "name": code, "props": {"http_status": int(first), "meaning": b.cells[2] if len(b.cells) > 2 else ""}, "provenance": _derived(source_text, source_id, revision, b.line)}
            continue
        idx = 0
        m = re.match(r"^`?(" + T.ID_RE.pattern + r")`?$", b.cells[0].strip()) or re.match(r"^`?(" + T.ID_RE.pattern + r")`?\b", b.cells[0].strip())
        if not m and len(b.cells) > 1 and len(b.cells[0].strip()) <= 4:
            m, idx = re.match(r"^`?(" + T.ID_RE.pattern + r")`?$", b.cells[1].strip()), 1
        if not m:
            continue
        ident = m.group(1)
        cls = class_for_id(ident, id_class_map)
        if cls is None and ident.split("-", 1)[0] in REFERENCE_ONLY_PREFIXES:
            continue
        if cls is None:
            gaps.append({"id": f"GAP-{ident}-CLASS", "target": ident, "need": "class_mapping", "status": "missing",
                         "reason": f"id prefix {ident.split('-', 1)[0]!r} has no configured class", "block": b.id})
            continue
        if idx:
            b = T.Block(b.line, b.kind, b.text, b.heading_path, b.cells[1:], (b.header or [])[1:])
        if ident in elements:
            elements[ident].setdefault("variants", []).append(element_from_row(source_text, source_id, revision, b, ident, cls))
            continue
        el = elements[ident] = element_from_row(source_text, source_id, revision, b, ident, cls)
        if cls == "Entity":
            for name, value in zip(b.header or [], b.cells):
                if name.strip().lower() in ATTRIBUTE_HEADERS:
                    for attr in _split_names(value):
                        aid = f"{ident}:attribute:{attr}"
                        elements[aid] = {"origin": "upstream", "maturity": "sketch", "owner_doc": source_id, "version_introduced": str(revision), "review_status": "needs_review",
                                         "id": aid, "class": "Attribute", "name": attr, "props": {"level": "logical", "physical_type": "unknown", "entity": ident}, "provenance": _derived(source_text, source_id, revision, b.line)}
                        edges.append({"src": ident, "rel": "has_attribute", "dst": aid, "provenance": _derived(source_text, source_id, revision, b.line)})
                    el["props"]["logical_attribute_ids"] = [f"{ident}:attribute:{a}" for a in _split_names(value)]
    for ident, el in list(elements.items()):
        if el["class"] in ("StateMachine", "State", "Transition", "ErrorCode", "Attribute"):
            continue
        quote = el["provenance"][0]["quote"]
        for ref in T.ids_in(quote):
            if ref == ident or ref not in elements:
                continue
            edges.append({"src": ident, "rel": relation_between(el["class"], elements[ref]["class"]), "dst": ref, "provenance": [dict(el["provenance"][0])]})
            if el["class"] == "Requirement" and elements[ref]["class"] in REALIZER_CLASSES:
                edges.append({"src": ref, "rel": "satisfies", "dst": ident, "provenance": [dict(el["provenance"][0])]})
    owner_ids = {el["props"].get("owner_entity") for el in elements.values() if el["class"] == "StateMachine"}
    for sm in [e for e in elements.values() if e["class"] == "StateMachine"]:
        if sm["props"]["owner_entity"] in elements:
            edges.append({"src": sm["id"], "rel": "models_state_of", "dst": sm["props"]["owner_entity"], "provenance": list(sm["provenance"])})
    mapped = {**DEFAULT_ID_CLASS, **(id_class_map or {})}
    undefined: dict[str, str] = {}
    for el in list(elements.values()):
        if el["class"] in ("State", "Transition"):
            continue
        for ref in T.ids_in(el["provenance"][0]["quote"]):
            if ref not in elements and ref.split("-", 1)[0] in mapped and ref not in undefined:
                undefined[ref] = el["id"]
    for ref, by in sorted(undefined.items()):
        gaps.append({"id": f"GAP-{ref}-UNDEFINED", "target": ref, "need": "definition", "status": "missing", "referenced_by": by,
                     "reason": f"{ref} is referenced by {by} but none of the supplied documents defines it"})
    edges = [e for e in edges if e["src"] in elements and e["dst"] in elements]
    return {"elements": list(elements.values()), "edges": edges, "gaps": gaps}
