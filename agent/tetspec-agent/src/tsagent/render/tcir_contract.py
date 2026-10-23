"""The `contract` block of a TCIR test case: how the system under test is called, as far as the TDD says, and what it leaves open.

The test-script agent never reads the application, so this block is all it knows about the interface. It is built from the source
package only, in this order: Endpoint / Response / Field nodes when the TDD has an interface contract; else what the requirement and
acceptance-criterion sentences state ("sent with POST /notes", "refused with 422"). Whatever the TDD does not define is listed under
`unknown`, so a script can read it defensively instead of guessing, and an assertion never takes a value from it.
"""
from __future__ import annotations

import re

ENDPOINT_RE = re.compile(r"\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(/[\w/{}.:\-]*)")
# A status is a 1xx-5xx number said as one ("refused with 422", "nhận 405", "HTTP 200"), never a count ("1 to 280 characters").
STATUS_RE = re.compile(r"(?i)\b(?:HTTP|status|code|mã|nhận|trả về|trả|returns?|receives?|answers?|responds?|refused with|rejected with|with)"
                       r"\s*:?\s*([1-5]\d\d)\b(?!\s*(?:characters?|chars?|ký tự|ms|giây|seconds?|%))")
TEXT_KEYS = ("text", "statement", "summary", "name", "description", "trigger", "condition")
MAX_STATUS = 12


def _text(node) -> str:
    return " ".join(str(node.props.get(k)) for k in TEXT_KEYS if isinstance(node.props.get(k), str))


def _neighbours(pkg, seeds: set[str]) -> set[str]:
    """Seeds and the nodes one edge away in either direction."""
    out = set(seeds)
    for code in seeds:
        for ts in pkg.out_edges(code).values():
            out.update(ts)
        for ss in pkg.in_edges(code).values():
            out.update(ss)
    return {c for c in out if pkg.get(c) is not None}


def _endpoint_nodes(pkg) -> list[dict]:
    out = []
    for n in pkg.of("Endpoint"):
        method, path = n.props.get("method"), n.props.get("path")
        if not method or not path:
            continue
        ep = {"method": str(method).upper(), "path": str(path), "from": [n.code], "defined": True}
        fields = []
        for req in pkg.targets(n.code, "hasRequest"):
            fields += [_field(f) for f in pkg.targets(req.code, "hasField", "Field")]
        fields += [_field(f) for f in pkg.targets(n.code, "hasField", "Field")]
        if fields:
            ep["requestFields"] = sorted(fields, key=lambda f: f["name"])
        responses = [{"status": r.props.get("statusCode"), "errorCode": r.props.get("errorCode"), "condition": r.props.get("condition"),
                      "from": r.code} for r in pkg.targets(n.code, "hasResponse", "Response")]
        if responses:
            ep["responses"] = [{k: v for k, v in r.items() if v not in (None, "")} for r in responses]
        out.append(ep)
    return out


def _field(f) -> dict:
    out = {"name": f.props.get("name") or f.code}
    for k in ("type", "required", "enum"):
        if f.props.get(k) not in (None, "", []):
            out[k] = f.props[k]
    return out


def _endpoints_from_text(pkg) -> dict[tuple[str, str], list[str]]:
    """(method, path) -> codes of the requirement-like nodes whose sentence names it."""
    found: dict[tuple[str, str], list[str]] = {}
    for n in pkg.nodes.values():
        for m in ENDPOINT_RE.finditer(_text(n)):
            found.setdefault((m.group(1), m.group(2).rstrip(".,;")), []).append(n.code)
    return found


def _entities(pkg) -> list[dict]:
    """Data entities with their fields: by hasField edge, else by the tdd-agent id prefix '<ENT>:attribute:'."""
    out = []
    fields = list(pkg.of("Field"))
    for e in pkg.of("DataEntity"):
        mine = pkg.targets(e.code, "hasField", "Field")
        if not mine:
            tid = (e.props.get("tddAgent") or {}).get("id") or e.code
            mine = [f for f in fields if str((f.props.get("tddAgent") or {}).get("id", "")).startswith(f"{tid}:attribute:")]
        if mine:
            out.append({"code": e.code, "name": e.props.get("name") or e.code, "fields": sorted((_field(f) for f in mine), key=lambda f: f["name"])})
    return out


def _status_codes(pkg, near: set[str]) -> list[dict]:
    """HTTP statuses a sentence of the TDD states, nearest nodes first."""
    out, seen = [], set()
    for code in sorted(pkg.nodes, key=lambda c: (c not in near, c)):
        n = pkg.get(code)
        if n.cls == "Response" and n.props.get("statusCode"):
            hits = [(int(n.props["statusCode"]), str(n.props.get("condition") or n.props.get("errorCode") or ""))]
        elif n.cls in ("AcceptanceCriterion", "FunctionalRequirement", "NonFunctionalRequirement", "Constraint", "Decision"):
            text = _text(n)
            hits = [(int(m.group(1)), text) for m in STATUS_RE.finditer(text)]
        else:
            hits = []
        for status, where in hits:
            if (status, code) in seen:
                continue
            seen.add((status, code))
            out.append({"status": status, "where": where[:300], "from": code, "related": code in near})
    return out[:MAX_STATUS]


def build(pkg, seeds: set[str]) -> dict:
    """The contract block for a test case whose basis is `seeds` (TDD codes). Empty dict without a source package."""
    if pkg is None:
        return {}
    near = _neighbours(pkg, {s for s in seeds if pkg.get(s) is not None})
    near = _neighbours(pkg, near)   # two hops: rule -> requirement -> endpoint / acceptance criterion
    defined = _endpoint_nodes(pkg)
    endpoints = {(e["method"], e["path"]): e for e in defined}
    for key, codes in _endpoints_from_text(pkg).items():
        ep = endpoints.setdefault(key, {"method": key[0], "path": key[1], "from": [], "defined": False})
        ep["from"] = sorted(set(ep["from"]) | set(codes))
    for ep in endpoints.values():
        ep["related"] = bool(set(ep["from"]) & near)
    eps = sorted(endpoints.values(), key=lambda e: (not e["related"], e["path"], e["method"]))
    entities = _entities(pkg)
    unknown = []
    for ep in eps:
        if not ep.get("responses"):
            unknown.append(f"response body and status codes of {ep['method']} {ep['path']} (the TDD has no interface contract for it)")
        if ep["method"] in ("POST", "PUT", "PATCH") and not ep.get("requestFields"):
            unknown.append(f"request body of {ep['method']} {ep['path']}: only the entity fields below are known")
    for ent in entities:
        if any("type" not in f for f in ent["fields"]):
            unknown.append(f"types of the fields of {ent['code']}")
    if not eps:
        unknown.append("no endpoint is named anywhere in the TDD")
    return {"endpoints": eps, "entities": entities, "statusCodes": _status_codes(pkg, near), "unknown": unknown}
