"""Parser of v1 md templates (tables keyed by prefixed codes, entity headings, solution appendix, supplement) -> SourcePackage.

Vietnamese literals here match the headings and column names of Vietnamese source documents."""
import os
import re

from ...errors import Diagnostic
from ...model import SourcePackage, SrcNode
from .doc import parse_md, parse_md_file
from .vocab import CODE_RE, CONTRACT_REL, HEADING_RE, class_of, classify, clean, codes_in, coerce, parse_kv

SUPPLEMENT_PREFIXES = ("AC", "FLD", "RSP", "TRN")
SUPPLEMENT_MAX_ROWS = 80
KV_LINE = re.compile(r"^\s*`?[A-Za-z_]\w*`?\s*=")


def _status(front):
    s = str(front.get("status") or "").lower()
    return "Baselined" if s in ("active", "baselined", "approved", "released") else "Draft"


def _doc_kind(doc):
    k = str(doc.front.get("doc_type") or "").lower()
    if not k and "supplement" in doc.name.lower():
        k = "supplement"
    return k or "doc"


class _Builder:
    def __init__(self, feature, known):
        self.pkg = SourcePackage(feature)
        self.known = set(known)       # codes present in other sources (JSON package loaded alongside)
        self.diags = []
        self.rev = []                 # (target in the cell, relation, row code, file, line)
        self.refs = []                # (code, relation, target, file, line) to check dangling references
        self.inline_ac = []

    def d(self, f, line, rule, msg, sev="error"):
        self.diags.append(Diagnostic(f, str(line), rule, msg, sev))

    def entity(self, code, cls, f, line, doc_code, section):
        node = SrcNode(code, cls, {"sourceDoc": doc_code, "sourceSection": section}, {"sourceFile": f, "sourceLine": line}, f"{f}:{line}")
        try:
            node = self.pkg.add_node(node)
        except ValueError as e:
            self.d(f, line, "MD-DUP-CLASS", str(e))
            return self.pkg.nodes[code]
        self.pkg.add_edge(code, "describedIn", doc_code)
        return node

    def set_attr(self, node, key, text):
        v = coerce(key, text)
        if v is not None:
            node.props[key] = v

    def link(self, node, rel, text, f, line):
        for t in codes_in(text, self.known | set(self.pkg.nodes)):
            self.refs.append((node.code, rel, t, f, line))


def _section(num, text):
    return f"§{num}" if num else clean(text)


def parse_documents(docs, feature, known=()):
    """`docs`: list of (name, content | path). Returns (SourcePackage, diagnostics)."""
    b = _Builder(feature, known)
    parsed = []
    for name, src in docs:
        doc = parse_md_file(src, name) if os.path.exists(str(src)) and len(str(src)) < 4096 else parse_md(src, name)
        b.diags += doc.diags
        parsed.append(doc)
    primary = None
    for doc in parsed:
        kind = _doc_kind(doc)
        stem = os.path.splitext(doc.name)[0]
        doc_code = str(doc.front.get("doc_id") or f"DOC-{stem}")
        b.pkg.add_node(SrcNode(doc_code, "Document", {"status": _status(doc.front), "label": str(doc.front.get("title") or stem)},
                               {"docType": kind, "title": str(doc.front.get("title") or stem), "version": str(doc.front.get("version", "")),
                                "path": doc.name}, doc.name))
        if kind == "tdd" and primary is None:
            primary = doc_code
        _walk(b, doc, doc_code, kind)
    _finalize(b)
    b.pkg.meta.update({"format": "md-v1", "ontologyVersion": "tdd-core-1+ext-0", "docs": [d.name for d in parsed], "primaryDoc": primary})
    if not [n for n in b.pkg.nodes.values() if n.cls != "Document"]:
        b.d(",".join(d.name for d in parsed) or "-", 0, "MD-EMPTY", "no entity recognised; check the v1 md template (the first column is a code with a known prefix)")
    return b.pkg, b.diags


def _walk(b, doc, doc_code, kind):
    f = doc.name
    in_appendix = kind != "solution"
    if kind == "solution" and not any(blk.kind == "heading" and _is_appendix(blk.heading.text) for blk in doc.blocks):
        b.d(f, 1, "MD-SOLUTION-APPENDIX", "the solution has no 'Phụ lục: đặc tả có cấu trúc' section; reading every table", "warning")
        in_appendix = True
    cur, cur_int, int_level, section, appendix_level = None, None, 99, "", 99
    supplement_rows = 0
    for blk in doc.blocks:
        if blk.kind == "heading":
            h = blk.heading
            if kind == "solution":
                if _is_appendix(h.text):
                    in_appendix, appendix_level = True, h.level
                elif h.level <= appendix_level and appendix_level != 99:
                    in_appendix = False
            if cur_int is not None and h.level <= int_level:
                cur_int, int_level = None, 99
            m = HEADING_RE.match(h.text)
            cls = class_of(m["code"]) if m else None
            if m and cls and in_appendix:
                section = _section(m["num"], h.text)
                cur = b.entity(m["code"], cls, f, h.line, doc_code, section)
                if m["title"]:
                    cur.props.setdefault("name", clean(m["title"]))
                if cls == "Integration":
                    cur_int, int_level = cur, h.level
            else:
                cur = None
                mm = re.match(r"^(\d+(?:\.\d+)*)\.?\s+", h.text)
                section = _section(mm.group(1) if mm else None, h.text)
        elif blk.kind == "para" and cur is not None and in_appendix and KV_LINE.match(blk.text):
            for k, v in parse_kv(blk.text):
                rel = classify(f"`{k}`")
                if rel[0] == "rel":
                    b.link(cur, rel[1], v, f, blk.line)
                else:
                    b.set_attr(cur, k, v)
        elif blk.kind == "table" and in_appendix:
            supplement_rows += _table(b, blk.table, f, doc_code, section, cur, cur_int, kind)
    if kind == "supplement" and supplement_rows > SUPPLEMENT_MAX_ROWS:
        b.d(f, 1, "MD-SUPPLEMENT-SIZE", f"the supplement has {supplement_rows} rows, at most {SUPPLEMENT_MAX_ROWS}")


def _is_appendix(text):
    t = text.lower()
    return "đặc tả có cấu trúc" in t or ("phụ lục" in t and "test" in t)


def _table(b, t, f, doc_code, section, cur, cur_int, kind):
    hdr = [clean(h).lower() for h in t.header]
    if hdr and hdr[0] == "#" and cur is not None:
        idx = next((i for i, h in enumerate(hdr) if h in ("bước", "step", "steps")), 1)
        steps = [clean(r[idx]) for r in t.rows if len(r) > idx and clean(r[idx])]
        if steps:
            cur.props["steps"] = steps
        return 0
    cols = [classify(h) for h in t.header]
    rows = 0
    for r, line in zip(t.rows, t.lines, strict=False):
        code = clean(r[0]) if r else ""
        m = CODE_RE.match(code)
        cls = class_of(code) if m else None
        if not cls:
            continue
        if kind == "supplement":
            if m.group(1) not in SUPPLEMENT_PREFIXES:
                b.d(f, line, "MD-SUPPLEMENT-PREFIX", f"{code}: the supplement only accepts AC, FLD, RSP, TRN tables")
                continue
            basis = next((clean(c) for (k, v), c in zip(cols, r, strict=False) if k == "attr" and v == "basis"), "")
            if not basis:
                b.d(f, line, "MD-SUPPLEMENT-BASIS", f"{code}: missing the 'Căn cứ' (source quote) column")
                continue
        rows += 1
        node = b.entity(code, cls, f, line, doc_code, section)
        if kind == "supplement":
            node.props["derivation"] = "supplement"
        for (ck, cv), cell in zip(cols[1:], r[1:], strict=False):
            if ck == "skip":
                continue
            if ck == "attr":
                b.set_attr(node, cv, cell)
            elif ck == "kv":
                for k, v in parse_kv(cell):
                    rk = classify(f"`{k}`")
                    if rk[0] == "rel":
                        b.link(node, rk[1], v, f, line)
                    else:
                        b.set_attr(node, k, v)
            elif ck == "rel":
                b.link(node, cv, cell, f, line)
            elif ck == "rev":
                for tgt in codes_in(cell, b.known | set(b.pkg.nodes)):
                    b.rev.append((tgt, cv, code, f, line))
            elif ck == "ac":
                text = clean(cell)
                if text:
                    b.inline_ac.append((node, text, f, line))
        if cur_int is not None and cls in CONTRACT_REL and code != cur_int.code:
            b.pkg.add_edge(cur_int.code, CONTRACT_REL[cls], code)
    return rows


def _finalize(b):
    pkg = b.pkg
    exists = lambda c: c in pkg.nodes or c in b.known  # noqa: E731
    for src, rel, tgt, f, line in b.refs:
        if rel == "removes" or exists(tgt):
            if tgt in pkg.nodes or rel == "removes":
                pkg.add_edge(src, rel, tgt) if tgt in pkg.nodes else pkg.get(src).props.setdefault("removes", []).append(tgt)
            else:
                pkg.meta.setdefault("externalEdges", []).append((src, rel, tgt))
        else:
            b.d(f, line, "MD-DANGLING", f"{src}.{rel} points to {tgt}, which does not exist")
    for tgt, rel, src, f, line in b.rev:
        if tgt in pkg.nodes:
            pkg.add_edge(tgt, rel, src)
        elif tgt in b.known:
            pkg.meta.setdefault("externalEdges", []).append((tgt, rel, src))
        else:
            b.d(f, line, "MD-DANGLING", f"{src}: reverse column {rel} points to {tgt}, which does not exist")
    for node, text, f, line in b.inline_ac:
        parts = [p.strip() for p in re.split(r";\s+", text) if p.strip()]
        for i, part in enumerate(parts, 1):
            code = f"AC-{node.code}-{i}"
            pkg.add_node(SrcNode(code, "AcceptanceCriterion", {"sourceDoc": node.base.get("sourceDoc"), "sourceSection": node.base.get("sourceSection")},
                                 {"statement": part, "derivation": "stated", "sourceFile": f, "sourceLine": line}, f"{f}:{line}"))
            pkg.add_edge(node.code, "hasAcceptance", code)
    for n in pkg.of("AcceptanceCriterion"):
        n.props.setdefault("derivation", "stated")
    for tr in list(pkg.of("Transition")):
        for sm in pkg.targets(tr.code, "inMachine"):
            pkg.add_edge(sm.code, "hasTransition", tr.code)


__all__ = ["parse_documents"]
