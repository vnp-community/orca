"""MOD-Ingest: load tdd-kg/v1 packages and v1 md templates into one SourcePackage (FR-001, FR-002).

A document is a path (must be inside TSAGENT_SOURCE_ROOTS) or {name, content} (written to the inbox, then read). Error-level
diagnostics raise InputError with every diagnostic; nothing is written to the DB.
"""
import os
import re

from ..errors import Diagnostic, InputError
from ..model import SourcePackage
from .kg_json import load_kg_json
from .md import parse_documents
from .tdd_ontology import core_ontology, extension_ontology

SAFE_NAME = re.compile(r"[^A-Za-z0-9._-]+")


def _within(path, roots):
    real = os.path.realpath(path)
    return any(real == r or real.startswith(r.rstrip(os.sep) + os.sep) for r in roots)


def resolve_documents(documents, cfg):
    """Return the checked list of (name, path). Content sent inline is written to the inbox."""
    out, diags = [], []
    inbox = cfg.inbox()
    for i, d in enumerate(documents or []):
        if isinstance(d, dict):
            name = SAFE_NAME.sub("_", os.path.basename(str(d.get("name") or f"doc-{i}")))[:120] or f"doc-{i}"
            content = d.get("content")
            if not isinstance(content, str) or not content.strip():
                diags.append(Diagnostic(name, "0", "INGEST-EMPTY-DOC", "inline document has no content"))
                continue
            os.makedirs(inbox, exist_ok=True)
            path = os.path.join(inbox, name)
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(content)
            out.append((name, path))
        else:
            path = str(d)
            if not _within(path, cfg.source_roots):
                diags.append(Diagnostic(os.path.basename(path), "0", "INGEST-ROOT",
                                        f"path outside TSAGENT_SOURCE_ROOTS: {path}"))
                continue
            if not os.path.isfile(path):
                diags.append(Diagnostic(os.path.basename(path), "0", "INGEST-NOFILE", "file not found"))
                continue
            out.append((os.path.basename(path), path))
    return out, diags


def _merge_into(target, other, diags):
    for n in other.nodes.values():
        try:
            target.add_node(n)
        except ValueError as e:
            diags.append(Diagnostic(n.origin or "-", "0", "INGEST-DUP-CLASS", str(e)))
    for e in other.edges:
        target.add_edge(e.src, e.rel, e.dst, e.props_dict)


def ingest(documents, feature, cfg):
    """documents: paths or {name, content}; .json (tdd-kg/v1) and .md. Returns (SourcePackage, diagnostics)."""
    docs, diags = resolve_documents(documents, cfg)
    js = [(n, p) for n, p in docs if p.lower().endswith(".json")]
    md = [(n, p) for n, p in docs if p.lower().endswith((".md", ".markdown"))]
    other = [(n, p) for n, p in docs if (n, p) not in js + md]
    for n, p in other:
        hint = "; Turtle/RDF is no longer supported: export the TDD KG as a tdd-kg/v1 package" if p.lower().endswith((".ttl", ".trig")) else ""
        diags.append(Diagnostic(n, "0", "INGEST-TYPE", "only .json (tdd-kg/v1) and .md are accepted" + hint))
    pkg = SourcePackage(feature)
    kg_loaded = False
    for name, path in js:
        p, ds = load_kg_json(path, feature, name)
        diags += ds
        if p is None:
            continue
        if not kg_loaded:
            pkg, kg_loaded = p, True
        else:
            _merge_into(pkg, p, diags)
            pkg.ontology.merge(p.ontology)
            pkg.meta["docs"] += p.meta["docs"]
    if md:
        mp, ds = parse_documents(md, feature, known=set(pkg.nodes))
        diags += ds
        _merge_into(pkg, mp, diags)
        for s, rel, t in mp.meta.get("externalEdges", []):
            if s in pkg.nodes and t in pkg.nodes:
                pkg.add_edge(s, rel, t)
            else:
                diags.append(Diagnostic("-", "0", "MD-DANGLING", f"{s}.{rel} points to {t}, which does not exist"))
        pkg.meta.setdefault("docs", [])
        pkg.meta["docs"] = list(pkg.meta["docs"]) + mp.meta["docs"]
        if not pkg.meta.get("primaryDoc"):
            pkg.meta["primaryDoc"] = mp.meta.get("primaryDoc")
        pkg.meta["format"] = "tdd-kg/v1+md" if kg_loaded else "md-v1"
        pkg.meta.setdefault("ontologyVersion", mp.meta["ontologyVersion"])
    if kg_loaded:
        pkg.ontology.merge(extension_ontology())
    else:
        pkg.ontology = core_ontology()
    if not pkg.meta.get("primaryDoc"):
        pick = sorted(n.code for n in pkg.of("Document") if str(n.get("docType", "")).lower() == "tdd")
        pkg.meta["primaryDoc"] = pick[0] if pick else None
    if not docs and not diags:
        diags.append(Diagnostic("-", "0", "INGEST-NODOCS", "no documents"))
    if not pkg.nodes and not any(d.severity == "error" for d in diags):
        diags.append(Diagnostic("-", "0", "INGEST-EMPTY", "no nodes after loading"))
    errors = [d for d in diags if d.severity == "error"]
    if errors:
        raise InputError(diags)
    pkg.feature = feature
    return pkg, diags
