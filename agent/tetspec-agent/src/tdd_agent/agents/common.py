"""Helpers shared by the four agents."""
from __future__ import annotations

import copy
import re
from dataclasses import dataclass
from functools import lru_cache
from typing import Any

from ..core import o5_path
from ..errors import NotFound
from ..orchestrator.context import GateContext
from ..skills import SkillRegistry, SkillResult, SkillRunner
from ..store.base import KnowledgeStore, SourceDoc, digest

DOC_ROLES = ("URD", "SRS", "SAD", "TDD", "CR")
NEED_SLOT_CLASSES = {
    "purpose": ("Capability",), "scope": ("Capability", "Constraint"), "requirements": ("Requirement", "RequirementVersion"),
    "contracts": ("Operation", "Interface", "Event"), "data": ("Entity", "Attribute"),
    "behavior": ("StateMachine", "Flow", "Algorithm", "BusinessRule", "Transition", "Decision"),
    "decisions": ("Decision",), "quality": ("QualityTarget",), "work": ("WorkItem",), "validation": ("Scenario", "TestCase"),
}


@lru_cache(maxsize=1)
def registry() -> SkillRegistry:
    return SkillRegistry.load()


def skills(ctx: GateContext) -> SkillRunner:
    return SkillRunner(registry(), ctx.observer, ctx.gateway)


def skill_fingerprint(ids: list[str] | None = None) -> str:
    return registry().fingerprint(ids)


def pinned_sources(ctx: GateContext) -> list[SourceDoc]:
    """The documents pinned to this run (id, revision, role, hash verified against the store)."""
    out = []
    for pin in ctx.request.get("pins", []):
        doc = ctx.knowledge.get_source(pin["id"], pin["revision"])
        if doc.content_hash != pin["content_hash"]:
            raise NotFound(f"source {pin['id']}@{pin['revision']} changed after it was pinned", code="SOURCE_PIN_MISMATCH")
        out.append(doc)
    for pin in (ctx.upstream.get("G0") or {}).get("derived_pins", []):  # the SRS a change request proposed (registered by G0 of this run)
        doc = ctx.knowledge.get_source(pin["id"], pin["revision"])
        if doc.content_hash != pin["content_hash"]:
            raise NotFound(f"source {pin['id']}@{pin['revision']} changed after it was derived", code="SOURCE_PIN_MISMATCH")
        out.append(doc)
    return out


def by_role(docs: list[SourceDoc], role: str) -> SourceDoc | None:
    found = [d for d in docs if d.role == role.upper()]
    return found[0] if found else None


def o5_source(doc: SourceDoc) -> dict:
    return {"id": doc.id, "version": doc.revision, "content": doc.text, "kind": doc.role, "content_hash": doc.content_hash}


def blocked_from(result: SkillResult) -> list[str]:
    return result.blockers or ["SKILL_FAILED"]


def tdd_members(graph: dict, tdd_id: str | None) -> tuple[list[dict], list[dict]]:
    """Elements and edges of one TDD document (``tdds`` membership or ``owner_doc``); the whole KG when ``tdd_id`` is None."""
    if not tdd_id:
        return list(graph.get("elements", [])), list(graph.get("edges", []))
    els = [e for e in graph.get("elements", []) if tdd_id in (e.get("tdds") or (e.get("props") or {}).get("tdds") or []) or (e.get("owner_doc") or (e.get("props") or {}).get("owner_doc")) == tdd_id]
    ids = {e["id"] for e in els}
    return els, [e for e in graph.get("edges", []) if e.get("src") in ids and e.get("dst") in ids]


def render_tdd(graph: dict, tdd_id: str | None) -> str:
    o5 = o5_path.load()
    import render as o5_render  # o5 harness (path added by o5_path)
    els, edges = tdd_members(graph, tdd_id)
    sub = {"version": graph.get("version"), "elements": els, "edges": edges, "gaps": graph.get("gaps", []), "conflicts": graph.get("conflicts", []),
           "known_ids": [e["id"] for e in graph.get("elements", [])]}
    return o5_render.render_tdd(sub, None)


def render_checked(graph: dict, tdd_id: str | None) -> tuple[str, dict]:
    """Render a TDD and verify the semantic round trip (nothing missing, changed or invented)."""
    import render as o5_render
    els, edges = tdd_members(graph, tdd_id)
    sub = {"version": graph.get("version"), "elements": els, "edges": edges, "gaps": graph.get("gaps", []), "conflicts": graph.get("conflicts", []),
           "known_ids": [e["id"] for e in graph.get("elements", [])]}
    text = o5_render.render_tdd(sub, None)
    check = o5_render.round_trip(sub, text)
    return text, {"ok": bool(check.get("ok")), "missing": len(check.get("missing", [])), "extra": len(check.get("extra", [])), "errors": list(check.get("errors", []))[:5]}


def render_document(ctx: GateContext, graph: dict, tdd_id: str | None, *, snapshot, readiness: str = "") -> tuple[str, dict]:
    """The TDD text to publish for ``tdd_id`` and its check report.

    ``render.style = "standard"`` writes the reader-facing document (the eleven parts of design v0.6 + appendix) (rules over the KG, no payload);
    ``"kg"`` keeps the lossless knowledge-graph projection with its round-trip check.
    """
    cfg = ctx.config.section("render")
    if cfg["style"] == "kg":
        text, check = render_checked(graph, tdd_id)
        return text, {**check, "style": "kg"}
    from ..render import RenderInfo, render_standard
    els, edges = tdd_members(graph, tdd_id)
    sub = {"version": graph.get("version"), "elements": els, "edges": edges, "gaps": graph.get("gaps", []), "conflicts": graph.get("conflicts", [])}
    store = ctx.knowledge._store
    texts: dict[tuple[str, str], str] = {}
    for e in els:
        for p in e.get("provenance") or []:
            key = (str(p.get("source_id")), str(p.get("source_version")))
            if key not in texts and key[0] != "None":
                try:
                    texts[key] = store.get_source(key[0], key[1]).text
                except NotFound:
                    continue
    info = RenderInfo(tdd_id=tdd_id or "KG", revision=next_revision(store, tdd_id) if tdd_id else "", snapshot_id=snapshot.id, snapshot_hash=snapshot.hash,
                      readiness=readiness, language=cfg["language"], source_detail=cfg["source_detail"], source_texts=texts)
    text, report = render_standard(sub, info, cfg["format"])
    return text, {**report, "style": "standard", "missing": len(report["missing_elements"]) if not report["ok"] else 0, "extra": 0}


def tdd_section_refs(doc: SourceDoc, element_ids: list[str]) -> list[dict]:
    """SectionRef dicts for rendered sections (the renderer emits ``## <id> — <name>`` headings)."""
    return [{"document_id": doc.id, "role": "TDD", "revision": doc.revision, "content_hash": doc.content_hash, "section_id": i} for i in sorted(set(element_ids))]


def latest_tdd_docs(store: KnowledgeStore) -> list[SourceDoc]:
    latest: dict[str, SourceDoc] = {}
    for d in store.list_sources("TDD"):
        latest[d.id] = d  # list is in registration order
    return [latest[k] for k in sorted(latest)]


def next_revision(store: KnowledgeStore, doc_id: str) -> str:
    try:
        cur = store.get_source(doc_id)
    except NotFound:
        return "1"
    m = re.match(r"^(\d+)", cur.revision)
    return str(int(m.group(1)) + 1) if m else cur.revision + ".1"


def graph_hash(graph: dict) -> str:
    return o5_path.load().snapshot_hash(graph)


def deep(x: Any) -> Any:
    return copy.deepcopy(x)


def head_graph(store: KnowledgeStore) -> tuple[Any, dict]:
    head = store.head()
    return head, (store.materialize(head.id) if head else {"version": None, "elements": [], "edges": []})


def upstream(ctx: GateContext, gate: str) -> dict:
    """Output of an earlier gate; isolated agent runs supply it as ``params.frozen[gate]`` instead."""
    return ctx.upstream.get(gate) or (ctx.request.get("params", {}).get("frozen") or {}).get(gate) or {}


def approval_of(ctx: GateContext, gate: str) -> dict | None:
    task = ctx.operational.get_task(ctx.run_id, gate) or {}
    return task.get("approval") or (ctx.request.get("params", {}).get("frozen") or {}).get(f"{gate}_approval")
