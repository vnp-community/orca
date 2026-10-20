"""Loader of `tdd-kg/v1` packages (ONTOLOGY-TDD section 8) and the input rules I1-I8 (section 6.2). Diagnostics carry a JSON pointer."""
import json
import os

import jsonschema

from ..errors import Diagnostic
from ..model import BASE_ATTRS, OntologyDecl, SourcePackage, SrcNode

SCHEMA_PATH = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "schemas", "tdd-kg-v1.schema.json")
SUPPORTED_ONTOLOGY = ("tdd-core-1",)  # prefix; "+ext-N" is the optional extension
_SCHEMA = None


def _schema():
    global _SCHEMA
    if _SCHEMA is None:
        with open(SCHEMA_PATH, encoding="utf-8") as fh:
            _SCHEMA = json.load(fh)
    return _SCHEMA


def _ptr(path):
    return "/" + "/".join(str(p) for p in path) if path else "/"


def load_kg_json(obj_or_path, feature=None, name=None):
    """Return (SourcePackage | None, diagnostics). The package is None when there is an error-level diagnostic."""
    if isinstance(obj_or_path, (str, os.PathLike)):
        name = name or os.path.basename(str(obj_or_path))
        try:
            with open(obj_or_path, encoding="utf-8") as fh:
                obj = json.load(fh)
        except json.JSONDecodeError as e:
            return None, [Diagnostic(name, f"{e.lineno}:{e.colno}", "I0", f"broken JSON: {e.msg}")]
    else:
        obj = obj_or_path
        name = name or "<tdd-kg>"
    diags = []

    def d(loc, rule, msg, severity="error"):
        diags.append(Diagnostic(name, loc, rule, msg, severity))

    # I8 first: an unsupported format or version stops further checks
    if isinstance(obj, dict):
        if obj.get("format") != "tdd-kg/v1":
            d("/format", "I8", f"unsupported format: {obj.get('format')!r} (only tdd-kg/v1)")
            return None, diags
        ov = str(obj.get("ontologyVersion", ""))
        if not any(ov == s or ov.startswith(s + "+") for s in SUPPORTED_ONTOLOGY):
            d("/ontologyVersion", "I8", f"unsupported ontologyVersion: {ov!r} (supported {', '.join(SUPPORTED_ONTOLOGY)})")
            return None, diags
    for err in sorted(jsonschema.Draft202012Validator(_schema()).iter_errors(obj), key=lambda e: list(e.absolute_path)):
        d(_ptr(err.absolute_path), "SCHEMA", err.message[:200])
    if diags:
        return None, diags

    feat = feature or obj["feature"]
    pkg = SourcePackage(feat)
    pkg.meta = {"format": obj["format"], "ontologyVersion": obj["ontologyVersion"], "package": obj.get("package", {}),
                "docs": [name], "sourceFeature": obj["feature"]}
    onto = OntologyDecl({c["name"]: {k: v for k, v in c.items() if k != "name"} for c in obj["ontology"]["classes"]},
                        {r["name"]: {k: v for k, v in r.items() if k != "name"} for r in obj["ontology"]["relations"]})
    pkg.ontology = onto
    seen = {}
    for i, n in enumerate(obj["nodes"]):
        code, cls = n["code"], n["class"]
        if code in seen:
            d(f"/nodes/{i}/code", "I4", f"duplicate code: {code} (already declared at /nodes/{seen[code]})")
            continue
        seen[code] = i
        if onto.classes and cls not in onto.classes:
            d(f"/nodes/{i}/class", "I2", f"{code}: class {cls} is not in the ontology sent along", "warning")
        base = {k: n[k] for k in BASE_ATTRS if k in n}
        props = dict(n.get("props", {}))
        node = SrcNode(code, cls, base, props, f"{name}#/nodes/{i}")
        for req in onto.required(cls):
            if req != "code" and node.get(req) in (None, "", []):
                d(f"/nodes/{i}", "I3", f"{code} ({cls}) lacks the required attribute {req}")
        for k, v in props.items():
            if k == "steps" and isinstance(v, str) and "->" in v:
                d(f"/nodes/{i}/props/{k}", "I7", f"{code}.{k} is a string joined with '->'; send a JSON array", "warning")
        pkg.nodes[code] = node
    for i, e in enumerate(obj.get("edges", [])):
        s, rel, t = e[0], e[1], e[2]
        props = e[3] if len(e) > 3 else {}
        missing = [x for x in (s, t) if x not in pkg.nodes]
        if missing:
            d(f"/edges/{i}", "I1", f"edge {s} -{rel}-> {t} points to nodes missing from the package: {', '.join(missing)}")
            continue
        rdef = onto.relations.get(rel)
        if rdef is None:
            d(f"/edges/{i}/1", "I2", f"relation {rel} is not in the ontology sent along", "warning")
        else:
            sc, tc = pkg.nodes[s].cls, pkg.nodes[t].cls
            dom, rng = rdef.get("domain", []), rdef.get("range", [])
            if dom and not any(onto.is_a(sc, x) for x in dom):
                d(f"/edges/{i}", "I2", f"{s} ({sc}) is not in the domain {dom} of {rel}")
            if rng and not any(onto.is_a(tc, x) for x in rng):
                d(f"/edges/{i}", "I2", f"{t} ({tc}) is not in the range {rng} of {rel}")
        pkg.add_edge(s, rel, t, props)
    for i, s in enumerate(obj.get("supersede", [])):
        o_, n_ = pkg.nodes.get(s["old"]), pkg.nodes.get(s["new"])
        if o_ and n_ and o_.cls != n_.cls:
            d(f"/supersede/{i}", "I5", f"supersede {s['old']} ({o_.cls}) -> {s['new']} ({n_.cls}) changes class")
        pkg.supersede.append({"old": s["old"], "new": s["new"]})
    docs = [n for n in pkg.nodes.values() if n.cls == "Document"]
    primary = [n for n in docs if n.base.get("primary") is True]
    if len(primary) > 1:
        d("/nodes", "I6", f"{len(primary)} Documents are marked primary: {', '.join(n.code for n in primary)}")
    elif not primary:
        pick = [n for n in docs if str(n.get("docType", "")).upper() == "TDD"] or \
               [n for n in docs if str(n.get("docType", "")).upper() == "CR"] or docs
        if not pick:
            d("/nodes", "I6", "the package has no Document; rule T17 needs a primary Document", "warning")
        else:
            pkg.meta["primaryDoc"] = sorted(pick, key=lambda n: n.code)[0].code
    if primary:
        pkg.meta["primaryDoc"] = primary[0].code
    if any(x.severity == "error" for x in diags):
        return None, diags
    return pkg, diags
