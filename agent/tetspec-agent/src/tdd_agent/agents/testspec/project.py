"""Projection of the design KG (one snapshot, one scope) into a tsagent ``tdd-kg/v1`` package (SOLUTION section 6).

No invented values: a required attribute missing from the source becomes a THIẾU ``OpenQuestion`` + a ``blockedBy`` edge.
Deterministic: the same snapshot + scope + mapping table give the same package byte for byte. Texts of generated nodes are Vietnamese (document content).
"""
from __future__ import annotations

import json
import re
from typing import Any

from .codes import CodeMap, norm_feature
from .context import Bundle, Item, clip, slug
from .mapping import load_mapping, tsagent_ontology
from .scope import Scope

ID_LIKE = re.compile(r"^[A-Za-z][A-Za-z0-9_.]*(-[A-Za-z0-9_.]+)+$")
NUM = re.compile(r"[-+]?\d+(?:[.,]\d+)?")
DURATION = {"ms": 1, "mili": 1, "s": 1000, "sec": 1000, "giây": 1000, "second": 1000, "m": 60000, "min": 60000, "phút": 60000, "minute": 60000,
            "h": 3600000, "giờ": 3600000, "hour": 3600000}
TRUE, FALSE = ("true", "yes", "có", "1", "y"), ("false", "no", "không", "0", "n")


def _blank(v: Any) -> bool:
    return v is None or v == "" or v == [] or v == {} or (isinstance(v, dict) and "slot_status" in v)


def lookup(e: dict, keys: list[str]) -> Any:
    """First available value of the keys, looked up in props, attrs, ext, then the element itself."""
    for k in keys:
        for src in (e.get("props") or {}, e.get("attrs") or {}, e.get("ext") or {}, e):
            v = src.get(k) if isinstance(src, dict) else None
            if not _blank(v):
                return v
    return None


def _num(v: Any):
    m = NUM.search(str(v))
    if not m:
        return None
    x = float(m.group(0).replace(",", "."))
    return int(x) if x.is_integer() else x


def apply_fn(fn: str | None, v: Any) -> Any:
    if v is None:
        return None
    if fn in (None, "str"):
        return "; ".join(map(str, v)) if isinstance(v, list) else (v if isinstance(v, (bool, int, float)) else str(v))
    if fn == "list":
        if isinstance(v, list):
            return [str(x) for x in v if not _blank(x)]
        parts = re.split(r"\s*(?:->|→|\n)\s*", str(v))
        return [p for p in (x.strip() for x in parts) if p]
    if fn in ("bool", "not"):
        b = v if isinstance(v, bool) else (True if str(v).strip().lower() in TRUE else False if str(v).strip().lower() in FALSE else None)
        return None if b is None else (not b if fn == "not" else b)
    if fn in ("num", "int"):
        n = _num(v)
        return int(n) if fn == "int" and n is not None else n
    if fn == "unit":
        m = NUM.search(str(v))
        rest = str(v)[m.end():].strip() if m else ""
        return rest.split()[0] if rest else None
    if fn == "ms":
        n = _num(v)
        if n is None:
            return None
        m = NUM.search(str(v))
        unit = str(v)[m.end():].strip().split(" ")[0].lower().rstrip(".") if m else ""
        factor = DURATION.get(unit, DURATION.get(unit[:1]) if unit else 1)
        return None if factor is None else int(n * factor)
    raise ValueError(f"unknown mapping function {fn}")


# Extra context (SOLUTION section 14): kind -> (tsagent class, relation from the anchor requirement); a Constraint hangs off the Decision.
CONTEXT_CLASS = {"decision": ("Decision", "satisfiedBy"), "constraint": ("Constraint", None), "assumption": ("Assumption", "assumes"),
                 "open_question": ("OpenQuestion", "blockedBy"), "risk": ("Risk", "hasRisk"), "acceptance": ("AcceptanceCriterion", "hasAcceptance")}
CONTEXT_PREFIX = {"decision": "ADR", "constraint": "CON", "assumption": "ASM", "open_question": "OQ", "risk": "RISK", "acceptance": "AC"}
# Kinds that make a TestGap (an unconfirmed assumption, an open question) go to ONE requirement; the rest go to every requirement named.
CONTEXT_PRIMARY = ("assumption", "open_question", "acceptance")
CONTEXT_ORDER = ("decision", "constraint", "risk", "assumption", "open_question", "acceptance")


class Projector:
    def __init__(self, graph: dict, mapping: dict | None = None):
        self.m = mapping or load_mapping()
        self.onto = tsagent_ontology()
        self.graph = graph
        self.by_id = {e["id"]: e for e in graph.get("elements", [])}
        self.out_edges: dict[tuple[str, str], list[str]] = {}
        self.in_edges: dict[tuple[str, str], list[str]] = {}
        for ed in graph.get("edges", []):
            self.out_edges.setdefault((ed.get("src"), ed.get("rel")), []).append(ed.get("dst"))
            self.in_edges.setdefault((ed.get("dst"), ed.get("rel")), []).append(ed.get("src"))

    # ------------------------------------------------------------------ rules
    def rule_for(self, e: dict) -> dict | None:
        for r in (self.m.get("classes") or {}).get(e["class"], []):
            w = r.get("when") or {}
            if "prop_in" in w and not all(str(lookup(e, [k]) or "").lower() in [str(x).lower() for x in vals] for k, vals in w["prop_in"].items()):
                continue
            if "id_prefix" in w and not any(str(e["id"]).startswith(p) for p in w["id_prefix"]):
                continue
            if "has" in w and any(lookup(e, [k]) is None for k in w["has"]):
                continue
            return r
        if e["class"] in (self.m.get("extended_passthrough") or []):
            return {"to": e["class"], "passthrough": True}
        return None

    def attrs_of(self, e: dict, spec: dict, cm: CodeMap) -> dict:
        out = {}
        for name, src in (spec or {}).items():
            if isinstance(src, list):
                v = apply_fn(None, lookup(e, src))
            elif "const" in src:
                v = src["const"]
            elif "edge_out" in src:
                dst = sorted(self.out_edges.get((e["id"], src["edge_out"]), []))
                v = cm(dst[0]) if dst else None
            else:
                v = apply_fn(src.get("fn"), lookup(e, src["from"]))
            if not _blank(v):
                out[name] = v
        return out

    # ------------------------------------------------------------------ package
    def build(self, scope: Scope, *, feature: str | None = None, cr: str | None = None, kind: str = "slice",
              generated_at: str = "", source_hash: str = "", context: Bundle | None = None) -> tuple[dict, dict]:
        cm = CodeMap()
        feat = feature or norm_feature(scope.tdd)
        nodes: dict[str, dict] = {}
        edges: set[tuple[str, str, str]] = set()
        pkg_code: dict[str, str] = {}      # tdd-agent element id -> node code
        pkg_cls: dict[str, str] = {}       # node code -> tsagent class
        report = {"unmapped": dict(scope.unmapped), "unmapped_relations": {}, "dropped_edges": 0, "missing": 0, "derived": 0}
        targets = set(scope.targets)
        derived: list[tuple[str, str, dict]] = []
        ext = False

        def add(code, cls, base, props):
            nodes[code] = {"code": code, "class": cls, **{k: v for k, v in base.items() if not _blank(v)}, "props": props}
            pkg_cls[code] = cls

        def missing(owner_code, owner_id, attr, owner_name, reason=""):
            q = f"{owner_code}-Q-{cm(attr)}"
            if q in nodes:
                return
            text = f"THIẾU {attr} của {owner_id} trong KG thiết kế" + (f": {reason}" if reason else "")
            add(q, "OpenQuestion", {"label": text[:200], "owner": owner_name or "SA"}, {"statement": text})
            edges.add((owner_code, "blockedBy", q))
            report["missing"] += 1

        for eid in list(scope.targets) + [x for x in scope.context if x not in targets]:
            e = self.by_id.get(eid)
            r = self.rule_for(e) if e else None
            if r is None:
                if e:
                    report["unmapped"][e["class"]] = report["unmapped"].get(e["class"], 0) + 1
                continue
            code = cm(eid)
            pkg_code[eid] = code
            in_scope = eid in targets
            props = dict(e.get("props") or {}) if r.get("passthrough") else self.attrs_of(e, r.get("attrs"), cm)
            ext = ext or bool(r.get("passthrough"))
            props["tddAgent"] = {"id": eid, "class": e["class"]}
            owner = lookup(e, ["owner"])
            base = {"label": str(e.get("name") or eid)[:300], "description": str(lookup(e, ["summary"]) or "")[:2000],
                    "sourceDoc": e.get("owner_doc") or scope.tdd, "sourceSection": str(lookup(e, ["source_section", "source_ref"]) or ""),
                    "owner": str(owner) if owner else None}
            if not in_scope:
                base["testScope"] = "context"
            add(code, r["to"], base, props)
            for d in r.get("derive") or []:   # derived nodes follow the scope of their element
                if any(lookup(e, [k]) is None for k in d.get("if") or []):
                    continue
                dcode = f"{code}-{d['suffix']}"
                dprops = self.attrs_of(e, d.get("attrs"), cm)
                dprops["tddAgent"] = {"id": eid, "class": e["class"], "derived": d["to"]}
                dbase = {"label": f"{d['to']} của {eid}", "sourceDoc": base["sourceDoc"]}
                if not in_scope:
                    dbase["testScope"] = "context"
                add(dcode, d["to"], dbase, dprops)
                derived.append((eid, dcode, d["link"]))
                report["derived"] += 1
                if in_scope:
                    for req in self.onto.required(d["to"]):
                        if req != "code" and req not in dprops:
                            missing(dcode, eid, req, base["owner"])
            if not in_scope:
                continue
            if not r.get("passthrough"):
                for req in self.onto.required(r["to"]):
                    if req != "code" and req not in props:
                        missing(code, eid, req, base["owner"])
            for gap in e.get("gaps") or []:
                slot = gap.get("slot") if isinstance(gap, dict) else str(gap)
                if slot:
                    missing(code, eid, slot, base["owner"], gap.get("reason", "") if isinstance(gap, dict) else "")
            self._acceptance(e, code, r, scope, cm, add, edges, missing, base["owner"])
        # element nodes are complete: link derived nodes, then the edges
        for eid, dcode, link in derived:
            for x in sorted(self.in_edges.get((eid, link["via"]), [])):
                self._edge(pkg_code.get(x), link["rel"], dcode, edges, pkg_cls, report)

        for ed in self.graph.get("edges", []):
            s, rel, d = ed.get("src"), ed.get("rel"), ed.get("dst")
            if s not in pkg_code and d not in pkg_code:
                continue
            if rel == "supersedes":
                continue
            cands = (self.m.get("relations") or {}).get(rel)
            if not cands:
                report["unmapped_relations"][rel] = report["unmapped_relations"].get(rel, 0) + 1
                continue
            done = False
            for c in cands:
                srcs = [s]
                if c.get("lift"):
                    srcs = sorted(self.in_edges.get((s, c["lift"]["via"]), []))
                for x in srcs:
                    a, b = (pkg_code.get(d), pkg_code.get(x)) if c.get("reverse") else (pkg_code.get(x), pkg_code.get(d))
                    if a and b and self._fits(c["to"], pkg_cls[a], pkg_cls[b]):
                        edges.add((a, c["to"], b)); done = True
                if done:
                    break
            if not done:
                report["dropped_edges"] += 1
        supersede = []
        for ed in self.graph.get("edges", []):
            if ed.get("rel") == "supersedes":
                new, old = pkg_code.get(ed.get("src")), pkg_code.get(ed.get("dst"))
                if new and old and pkg_cls[new] == pkg_cls[old]:
                    supersede.append({"old": old, "new": new})

        if context is not None:
            self._inject_context(context, nodes, edges, pkg_code, pkg_cls, report, add)

        doc = cm(f"DOC-{feat}")
        add(doc, "Document", {"label": scope.tdd, "testScope": "context", "primary": True, "sourceDoc": scope.tdd}, {"docType": "TDD", "title": scope.tdd})
        pkg = {
            "format": "tdd-kg/v1",
            "ontologyVersion": "tdd-core-1+ext-1" if ext else "tdd-core-1+ext-0",
            "feature": feat,
            "package": {"kind": kind, "cr": cr, "generatedAt": generated_at, "sourceHash": source_hash},
            "ontology": self._ontology(set(pkg_cls.values()), {r for _, r, _ in edges}),
            "nodes": [nodes[k] for k in sorted(nodes)],
            "edges": [[a, r, b] for a, r, b in sorted(edges)],
            "supersede": sorted(supersede, key=lambda x: (x["old"], x["new"])),
        }
        report.update(code_map=dict(sorted(cm.changed.items())), nodes=len(nodes), edges=len(edges), targets=len(targets),
                      context=len([x for x in scope.context if x in pkg_code and x not in targets]), extended=ext)
        return pkg, report

    def _acceptance(self, e, code, r, scope, cm, add, edges, missing, owner):
        items: list[tuple[str, str | None, str]] = []   # (code, text, origin)
        raw = lookup(e, r.get("acceptance") or []) if r.get("acceptance") else None
        vals = raw if isinstance(raw, list) else ([raw] if raw is not None else [])
        n = 0
        cr_text = {a["id"]: a.get("text") for a in scope.acceptance.get(e["id"], [])}
        for a in scope.acceptance.get(e["id"], []):    # CR criteria first: on the same code the "cr" origin wins
            items.append((cm(a["id"]), a.get("text"), "cr"))
        for v in vals:
            v = str(v).strip()
            if not v:
                continue
            if ID_LIKE.match(v):
                items.append((cm(v), cr_text.get(v), "tdd-agent"))
            else:
                n += 1
                items.append((f"{code}-AC{n}", v, "tdd-agent"))
        seen = set()
        for ac, text, origin in items:
            if ac in seen:
                continue
            seen.add(ac)
            props = {"derivation": origin}
            if text:
                props["statement"] = text
            add(ac, "AcceptanceCriterion", {"label": (text or ac)[:200], "sourceDoc": scope.tdd}, props)
            edges.add((code, "hasAcceptance", ac))
            if not text:
                missing(ac, ac, "statement", owner, "chưa có văn bản GIVEN/WHEN/THEN của tiêu chí")

    # ------------------------------------------------------------------ extra context (SOLUTION section 14)
    def _inject_context(self, b: Bundle, nodes: dict, edges: set, pkg_code: dict, pkg_cls: dict, report: dict, add) -> None:
        """Put the items of a Solution or a Plan into the package as ordinary nodes next to the requirements they are about.

        Anchors: an item goes to the in-scope requirements it names; an item that names none goes to the primary requirement (the first one
        the document names, or the only one in scope). An item with no anchor is listed under ``context.unanchored``, never guessed."""
        reqs = sorted(c for c, n in nodes.items() if n.get("testScope") != "context" and self.onto.is_a(pkg_cls[c], "Requirement"))
        known = {**pkg_code, **{c: c for c in nodes}}

        def resolve(codes):
            out: list[str] = []
            for c in codes:
                code = known.get(c)
                if code in reqs and code not in out:
                    out.append(code)
            return out
        hinted = resolve(b.hints)
        primary = hinted[:1] or (reqs if len(reqs) == 1 else [])
        meta = {"mode": b.mode, "ref": b.ref, "sha256": b.sha256}
        ref = slug(b.ref)
        rep = {**b.meta(), "nodes": 0, "anchors": [], "unanchored": [], "dropped_edges": 0, "notes": list(b.notes), "by_kind": {}}
        decision: str | None = None
        n_risk = 0
        ordered = sorted(b.items, key=lambda i: CONTEXT_ORDER.index(i.kind))
        for it in ordered:
            cls, rel = CONTEXT_CLASS[it.kind]
            key = slug(it.key) if it.kind != "risk" else f"R{(n_risk := n_risk + 1)}"
            code = f"{CONTEXT_PREFIX[it.kind]}-{ref}-{key}"
            if code in nodes:
                rep["notes"].append(f"{it.kind} {it.key}: code {code} already exists, skipped")
                continue
            attrs, base = self._context_attrs(it, b), {"label": clip(it.text, 200), "sourceDoc": b.label, "sourceSection": f"{b.label} {it.kind} {it.key}"}
            if it.kind != "acceptance":
                base["testScope"] = "context"     # read, never a subject of tests; an acceptance criterion is tested like any other
            if it.kind == "constraint":
                if decision is None:
                    rep["notes"].append(f"constraint {it.key} has no Decision to hang off: skipped")
                    continue
                add(code, cls, base, {**attrs, "context": meta})
                self._edge(decision, "constrainedBy", code, edges, pkg_cls, rep)
                rep["nodes"] += 1
                rep["by_kind"][it.kind] = rep["by_kind"].get(it.kind, 0) + 1
                continue
            named = resolve(it.anchors)
            anchors = named or (primary if it.kind in CONTEXT_PRIMARY else (hinted or primary))
            if it.kind in CONTEXT_PRIMARY and len(named) > 1:
                anchors = named[:1]
            if not anchors:
                rep["unanchored"].append(f"{it.kind}:{it.key}")
                continue
            add(code, cls, base, {**attrs, "context": meta})
            for a in anchors:
                self._edge(a, rel, code, edges, pkg_cls, rep)
                if a not in rep["anchors"]:
                    rep["anchors"].append(a)
            if it.kind == "decision" and decision is None:
                decision = code     # constraints (non-goals, confirmed assumptions) hang off the first decision
            rep["nodes"] += 1
            rep["by_kind"][it.kind] = rep["by_kind"].get(it.kind, 0) + 1
        report["dropped_edges"] += rep.pop("dropped_edges")
        rep["anchors"].sort()
        report["context_doc"] = rep

    @staticmethod
    def _context_attrs(it: Item, b: Bundle) -> dict:
        if it.kind == "decision":
            return {"statement": it.text, **{k: v for k, v in it.props.items() if not _blank(v)}}
        if it.kind == "constraint":
            return {"constraintType": it.props.get("constraintType", "confirmed-assumption"), "statement": it.text}
        if it.kind in ("assumption", "open_question"):
            return {"statement": it.text, "confirmStatus": "unconfirmed" if it.kind == "assumption" else "open"}
        if it.kind == "risk":
            return {"likelihood": "Medium", "impact": it.props.get("impact", "Medium"), "mitigation": "", "statement": it.text}
        return {"derivation": b.mode, "statement": it.text, **{k: v for k, v in it.props.items() if not _blank(v)}}

    def _fits(self, rel: str, a_cls: str, b_cls: str) -> bool:
        rdef = self.onto.relations.get(rel) or {}
        dom, rng = rdef.get("domain") or [], rdef.get("range") or []
        return (not dom or any(self.onto.is_a(a_cls, x) for x in dom)) and (not rng or any(self.onto.is_a(b_cls, x) for x in rng))

    def _edge(self, a, rel, b, edges, pkg_cls, report):
        if a and b and a in pkg_cls and b in pkg_cls and self._fits(rel, pkg_cls[a], pkg_cls[b]):
            edges.add((a, rel, b))
        else:
            report["dropped_edges"] += 1

    def _ontology(self, classes: set[str], rels: set[str]) -> dict:
        """Ontology declared with the package: only ``code`` is required (missing values already became OpenQuestions, section 6.4)."""
        need = set()
        for c in classes:
            need.update(self.onto.ancestors(c) if hasattr(self.onto, "ancestors") else [c])
            need.add(c)
        out = []
        for c in sorted(need):
            d = self.onto.classes.get(c, {})
            out.append({"name": c, "parent": d.get("parent"), "required": ["code"], "attrs": list(d.get("attrs") or []), "doc": d.get("doc")})
        rel_out = [{"name": r, "domain": list(self.onto.relations[r].get("domain") or []), "range": list(self.onto.relations[r].get("range") or [])}
                   for r in sorted(rels) if r in self.onto.relations]
        return {"classes": out, "relations": rel_out}


def build_package(graph: dict, scope: Scope, **kw) -> tuple[dict, dict]:
    return Projector(graph, kw.pop("mapping", None)).build(scope, **kw)


def dumps(pkg: dict) -> str:
    """Stable JSON (same package -> same bytes)."""
    return json.dumps(pkg, ensure_ascii=False, sort_keys=True, indent=1) + "\n"
