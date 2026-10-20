"""TDD generation agent (design section 3.1).

Pipeline: pin sources -> segment -> structured facts (rules) -> prose facts (LLM,
verified) -> entity matching -> verify evidence -> merge into the KG -> resolve
knowledge needs per source -> readiness -> render.  Missing design is recorded as
gaps and questions; nothing is invented to reach ``implementation_ready``.
"""
from __future__ import annotations

import re
from types import SimpleNamespace
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from ..core import o5_path
from ..orchestrator.context import GateContext
from ..orchestrator.model import GateOutcome
from ..skills.lib import text as T
from ..skills.steps import logic_step
from ..store.base import SourceDoc, digest
from .common import (NEED_SLOT_CLASSES, by_role, deep, head_graph, o5_source, pinned_sources, render_tdd, skills)

EXTRACT_ROLES = ("URD", "SRS", "SAD")
PROSE_CLASSES = {"Requirement": {"required": ["statement"]}, "BusinessRule": {"required": ["predicate"]}, "QualityTarget": {"required": ["target"]},
                 "Constraint": {"required": ["statement"]}, "Decision": {"required": ["chosen"]}, "Operation": {"required": ["method", "path"]},
                 "Scenario": {"required": ["when", "then"]}, "Entity": {"required": ["purpose"]}}
DEFAULT_PROSE_LIMIT = 300
GAP_OWNER = {"srs_clarification": "BA", "defect_target": "SA/Dev", "physical_schema": "SA/Dev", "verification": "QA", "code_target": "Dev", "endpoint_contract": "SA/Dev", "dependency_binding": "SA", "typed_fact": "BA",
             "class_mapping": "BA", "entity_resolution": "BA", "structured_delta": "BA"}


@dataclass
class Extraction:
    elements: list[dict] = field(default_factory=list)
    edges: list[dict] = field(default_factory=list)
    gaps: list[dict] = field(default_factory=list)
    conflicts: list[dict] = field(default_factory=list)
    stats: dict = field(default_factory=dict)
    blockers: list[str] = field(default_factory=list)
    # False when only some elements were read back (a change reads just the elements its delta names): what was not re-read is not "no longer stated"
    complete: bool = True
    scope: set[str] | None = None


def _line(el: dict) -> int | None:
    for p in el.get("provenance", []):
        if str(p.get("block", "")).startswith("line:"):
            return int(p["block"].split(":")[1])
    return None


def source_errors(ctx: GateContext, docs: list[SourceDoc], mode: str, params: dict) -> list[str]:
    o5 = o5_path.load()
    from dynamic import TddGenerationRequest
    sources = [{"id": d.id, "kind": d.role, "version": d.revision} for d in docs]
    request = TddGenerationRequest(id=ctx.run_id, mode=mode, scope=params.get("scope") or [params.get("tdd_id", "scope")], sources=sources,
                                   base_snapshot=params.get("base_snapshot"), change_request=params.get("change_request"))
    errors = request.validate_source_roles()
    if mode != "initial" and params.get("base_snapshot"):
        head = ctx.knowledge.head()
        if head is None or head.id != params["base_snapshot"]:
            errors.append("BASE_SNAPSHOT_STALE")
    return errors


def ingested_hashes(graph: dict) -> dict[str, set[str]]:
    """document id -> the content hashes of the revisions whose facts the graph already holds (read from the evidence of its elements)."""
    out: dict[str, set[str]] = {}
    for el in graph.get("elements", []):
        for p in el.get("provenance") or []:
            if p.get("kind") == "extracted" and p.get("source_id") and p.get("source_hash"):
                out.setdefault(str(p["source_id"]), set()).add(str(p["source_hash"]))
    return out


def extract_documents(ctx: GateContext, docs: list[SourceDoc], params: dict, *, only_ids: set[str] | None = None, prose: bool = True, known: dict[str, dict] | None = None) -> Extraction:
    """Run the extraction skills over the pinned documents (URD, SRS, SAD order)."""
    hub = skills(ctx)
    ob = ctx.observer
    out = Extraction(complete=only_ids is None, scope=set(only_ids) if only_ids is not None else None)
    id_class_map = params.get("id_class_map")
    merged: dict[str, dict] = {}
    edges: list[dict] = []
    ordered = sorted((d for d in docs if d.role in EXTRACT_ROLES), key=lambda d: EXTRACT_ROLES.index(d.role))
    for doc in ordered:
        payload = {"source_id": doc.id, "revision": doc.revision, "role": doc.role, "text": doc.text}
        if id_class_map:
            payload["id_class_map"] = id_class_map
        with logic_step(ob, "gen.segment"):
            seg = hub.run("segment-and-anchor", payload)
        out.stats.setdefault("blocks", {})[doc.id] = seg.output.get("counts", {}) if seg.ok else {}
        with logic_step(ob, "gen.extract-structured"):
            st = hub.run("extract-structured-facts", payload)
        if not st.ok:
            out.blockers.extend(st.blockers)
            continue
        out.gaps.extend(st.gaps)
        for el in st.output["elements"]:
            if only_ids is not None and el["id"] not in only_ids:
                continue
            prev = merged.get(el["id"])
            if prev is None:
                merged[el["id"]] = el
            elif prev.get("owner_doc") != el.get("owner_doc") and prev["class"] == el["class"]:
                # a later document refines an earlier one: keep both exact statements, never drop evidence
                el["provenance"] = el["provenance"] + prev["provenance"]
                el["props"]["upstream_definition"] = prev["props"]
                merged[el["id"]] = el
            elif prev["props"] != el["props"]:
                prev.setdefault("variants", []).append(el)
        edges.extend(e for e in st.output["edges"] if only_ids is None or e["src"] in only_ids)
        out.stats.setdefault("structured", {})[doc.id] = len(st.output["elements"])
        if prose and only_ids is None and params.get("prose", True):
            covered = [ln for e in st.output["elements"] if (ln := _line(e))]
            with logic_step(ob, "gen.select-prose"):
                sel = hub.run("select-prose-candidates", {"text": doc.text, "covered_lines": covered})
            spans = sel.output["spans"] if sel.ok else []
            limit = int(params.get("prose_limit", DEFAULT_PROSE_LIMIT))
            if len(spans) > limit:
                out.gaps.append({"id": f"GAP-{doc.id}-PROSE-TRUNCATED", "target": doc.id, "need": "prose_extraction", "status": "unknown",
                                 "reason": f"{len(spans) - limit} candidate line(s) beyond prose_limit={limit} were not read", "lines": [s["line"] for s in spans[limit:limit + 50]]})
                spans = spans[:limit]
            out.stats.setdefault("prose_spans", {})[doc.id] = len(spans)
            facts, blockers = _prose_facts(ctx, hub, doc, spans)
            out.blockers.extend(blockers)
            for el in facts["elements"]:
                merged.setdefault(el["id"], el)
            out.gaps.extend(facts["gaps"])
    out.elements = list(merged.values())
    out.edges = edges
    _resolve_cross_document_refs(out, known)
    # evidence must verify against the pinned text, otherwise the fact is a gap
    with logic_step(ob, "gen.verify-evidence"):
        verified = _verify_all(ctx, hub, out, docs)
    out.elements, out.edges = verified
    _match_entities(ctx, hub, out)
    return out


NO_REF_SOURCES = ("StateMachine", "State", "Transition", "ErrorCode", "Attribute")


def _resolve_cross_document_refs(out: Extraction, known: dict[str, dict] | None = None) -> None:
    """References are resolved against every supplied document, not only the one that mentions them.

    A rule defined in the URD and cited by an SRS row is a trace edge, never an "undefined" reference.
    """
    from ..skills.lib.facts import REALIZER_CLASSES, relation_between
    by_id = {**(known or {}), **{e["id"]: e for e in out.elements}}  # ``known``: elements of documents this run did not read again
    have = {(e["src"], e["dst"]) for e in out.edges}
    realized = {(e["src"], e["dst"]) for e in out.edges if e["rel"] == "satisfies"}
    for el in out.elements:
        if el["class"] in NO_REF_SOURCES or not el.get("provenance"):
            continue
        for ref in T.ids_in(el["provenance"][0].get("quote", "")):
            target = by_id.get(ref)
            if target is None or ref == el["id"]:
                continue
            if el["class"] == "Requirement" and target["class"] in REALIZER_CLASSES and (ref, el["id"]) not in realized:   # the row names what realizes the requirement
                out.edges.append({"src": ref, "rel": "satisfies", "dst": el["id"], "provenance": [dict(el["provenance"][0])]})
                realized.add((ref, el["id"]))
                have.add((ref, el["id"]))
            if (el["id"], ref) in have or (ref, el["id"]) in have:
                continue
            out.edges.append({"src": el["id"], "rel": relation_between(el["class"], target["class"]), "dst": ref, "provenance": [dict(el["provenance"][0])]})
            have.add((el["id"], ref))
    out.gaps = [g for g in out.gaps if not (g.get("need") == "definition" and g.get("id") == f"GAP-{g.get('target')}-UNDEFINED" and g.get("target") in by_id)]


def _prose_facts(ctx: GateContext, hub, doc: SourceDoc, spans: list[dict]) -> tuple[dict, list[str]]:
    result = {"elements": [], "gaps": []}
    if not spans:
        return result, []
    for sp in spans:
        r = hub.run("map-candidate-to-class", {"candidate": {"block": sp["block"], "text": sp["text"], "heading_path": sp.get("heading_path", [])},
                                                "allowed_classes": sorted(PROSE_CLASSES), "rules_only": True})
        if r.ok and r.output.get("class"):
            sp["hint"] = r.output["class"]
    jobs = [(PROSE_CLASSES, spans)]
    blockers: list[str] = []
    for allowed, group in jobs:
        res = hub.run("extract-facts-from-prose", {"source": {"id": doc.id, "revision": doc.revision, "text": doc.text}, "spans": group, "allowed_classes": allowed})
        if not res.ok:
            blockers.extend(res.blockers)
            continue
        result["elements"].extend(res.output["elements"])
        result["gaps"].extend(res.output["gaps"])
    return result, blockers


def _verify_all(ctx: GateContext, hub, ex: Extraction, docs: list[SourceDoc]) -> tuple[list[dict], list[dict]]:
    records, owners = [], []
    for el in ex.elements:
        for ev in el.get("provenance", []):
            records.append(ev); owners.append(el["id"])
    sources = [{"id": d.id, "version": d.revision, "content": d.text} for d in docs]
    res = hub.run("verify-evidence", {"records": records, "sources": sources})
    bad: dict[str, list[str]] = {}
    for item in res.output["results"]:
        if item["errors"]:
            bad.setdefault(owners[item["index"]], []).extend(item["errors"])
    keep = []
    for el in ex.elements:
        if el["id"] in bad:
            ex.gaps.append({"id": f"GAP-{el['id']}-EVIDENCE", "target": el["id"], "need": "valid_evidence", "status": "unknown", "reason": "evidence failed verification: " + ", ".join(sorted(set(bad[el["id"]])))})
        else:
            keep.append(el)
    ids = {e["id"] for e in keep}
    return keep, [e for e in ex.edges if e["src"] in ids and e["dst"] in ids]


def _match_entities(ctx: GateContext, hub, ex: Extraction, max_pairs: int = 20) -> None:
    pairs = []
    els = ex.elements
    for i, a in enumerate(els):
        for b in els[i + 1:]:
            if a["class"] != b["class"] or a.get("owner_doc") == b.get("owner_doc") or a["id"] == b["id"]:
                continue
            score = T.jaccard(a.get("name", ""), b.get("name", ""))
            if 0.2 < score < 1.0:
                pairs.append((score, a, b))
    pairs.sort(key=lambda t: (-t[0], t[1]["id"], t[2]["id"]))
    for score, a, b in pairs[:max_pairs]:
        r = hub.run("resolve-entity-match", {"a": {k: a[k] for k in ("id", "class", "name", "props") if k in a}, "b": {k: b[k] for k in ("id", "class", "name", "props") if k in b}})
        if not r.ok:
            ex.blockers.extend(r.blockers)
            continue
        same = r.output["same"]
        if same is True:
            ex.gaps.append({"id": f"GAP-DUP-{a['id']}-{b['id']}", "target": a["id"], "need": "entity_resolution", "status": "proposed",
                            "reason": f"{a['id']} and {b['id']} may be the same {a['class']}: {r.output['reason']}"})
        elif same is None:
            ex.conflicts.append({"type": "entity_match", "ids": [a["id"], b["id"]], "status": "unresolved", "reason": r.output["reason"]})


def merge_into_graph(head: dict, ex: Extraction, version: str) -> tuple[dict, dict]:
    """Union of the head graph and the new extraction.  Reviewed elements are never overwritten."""
    graph = deep(head)
    graph["version"] = version
    existing = {e["id"]: e for e in graph.setdefault("elements", [])}
    added = replaced = kept = 0
    conflicts = list(graph.get("conflicts", []))
    for el in ex.elements:
        cur = existing.get(el["id"])
        if cur is None:
            graph["elements"].append(el); existing[el["id"]] = el; added += 1
        elif _without_tdds(cur) == _without_tdds(el) or ((cur.get("review_status") or "needs_review") not in ("needs_review", "draft", "auto") and normalize_element(cur) == normalize_element(el)):
            cur["tdds"] = sorted(set(cur.get("tdds") or []) | set(el.get("tdds") or []))  # shared facts (e.g. the SAD) belong to every TDD that ingests them
            continue
        elif (cur.get("review_status") or "needs_review") in ("needs_review", "draft", "auto", None):
            _keep_evidence_of_other_sources(cur, el)
            graph["elements"][graph["elements"].index(cur)] = el; existing[el["id"]] = el; replaced += 1
        else:
            kept += 1
            conflicts.append({"type": "reviewed_element_differs", "id": el["id"], "status": "unresolved", "reason": "a reviewed element differs from the newly extracted statement"})
    graph["edges"] = _drop_superseded_edges(graph.setdefault("edges", []), ex)
    have = {digest(e) for e in graph["edges"]}
    for e in ex.edges:
        if digest(e) not in have and e["src"] in existing and e["dst"] in existing:
            graph["edges"].append(e); have.add(digest(e))
    named = lambda c: (c.get("type"), tuple(sorted(set(c.get("ids") or []) | ({c["id"]} if c.get("id") else set()))))  # a conflict somebody settled is not raised again
    settled = {named(c) for c in conflicts if c.get("status") == "resolved"}
    graph["conflicts"] = conflicts + [c for c in ex.conflicts if c not in conflicts and named(c) not in settled]
    graph["gaps"] = _merge_gaps(graph.get("gaps", []), ex.gaps)
    return graph, {"added": added, "replaced": replaced, "kept_reviewed": kept}


def _drop_superseded_edges(edges: list[dict], ex: Extraction) -> list[dict]:
    """A source document read again at a newer revision replaces what its older revision said.

    An edge is identified with its provenance, so without this the same relation would be kept once per revision of the document.
    Evidence from a document this run did not read is kept; an edge left with no evidence is dropped (the new revision no longer states it).
    A run that read only some elements (``ex.complete`` is false) judges only the edges of those elements: the rest of the document was not re-read.
    """
    read: dict[str, str] = {}
    for item in list(ex.elements) + list(ex.edges):
        for p in item.get("provenance") or []:
            read[str(p.get("source_id"))] = str(p.get("source_version"))
    out = []
    for e in edges:
        if not ex.complete and e.get("src") not in (ex.scope or ()):
            out.append(e)
            continue
        prov = e.get("provenance") or []
        live = [p for p in prov if str(p.get("source_id")) not in read or str(p.get("source_version")) == read[str(p.get("source_id"))]]
        if prov and not live:
            continue
        out.append(e if len(live) == len(prov) else {**e, "provenance": live})
    return out


def _keep_evidence_of_other_sources(cur: dict, new: dict) -> None:
    """A run that reads fewer documents (a defect run reads only SRS and SAD) must not drop what the other documents said about the element."""
    seen = {p.get("source_id") for p in new.get("provenance", [])}
    kept = [p for p in cur.get("provenance", []) if p.get("source_id") not in seen]
    if kept:
        new["provenance"] = list(new.get("provenance", [])) + kept
    old_def = (cur.get("props") or {}).get("upstream_definition")
    if old_def and "upstream_definition" not in (new.get("props") or {}):
        new.setdefault("props", {})["upstream_definition"] = old_def


_BOOKKEEPING = ("tdds", "review_status", "reviews", "change_history", "version_introduced")
_REVISION_FIELDS = ("source_version", "source_hash", "quote_hash")


def normalize_element(el: dict) -> dict:
    """The element as its sources state it: without review bookkeeping and without which revision of a document the evidence was read from.

    Two readings that differ only there are the same fact; a reviewed element is in conflict with a new reading only when a statement differs.
    """
    out = {k: v for k, v in el.items() if k not in _BOOKKEEPING}
    out["provenance"] = [{k: v for k, v in p.items() if k not in _REVISION_FIELDS} for p in el.get("provenance") or []]
    return out


def _without_tdds(el: dict) -> dict:
    return {k: v for k, v in el.items() if k != "tdds"}


def _merge_gaps(old: list[dict], new: list[dict]) -> list[dict]:
    seen = {g.get("id"): g for g in old}
    for g in new:
        seen[g["id"]] = g
    return [seen[k] for k in sorted(seen)]


_COND = re.compile(r"^(\w+)\.(\w+) is conditionally required$")


def _demote_conditional(graph: dict, errors: list) -> set[str]:
    """A write operation whose idempotency/concurrency is not stated cannot be typed as POST/PUT/...:
    keep the element, record the declared method as plain text and open a gap for each missing contract."""
    o5 = o5_path.load()
    rules = o5.ontology5.load().get("conditional_requirements", [])
    fixed: set[str] = set()
    by_id = {e["id"]: e for e in graph["elements"]}
    for err in errors:
        m = isinstance(err, dict) and _COND.match(str(err.get("message", "")))
        el = by_id.get(err.get("id")) if m else None
        if not el:
            continue
        cls, field_name = m.group(1), m.group(2)
        rule = next((r for r in rules if r.get("class") == cls and field_name in r.get("required", [])), None)
        trigger = (rule or {}).get("when", {}).get("property")
        props = el.setdefault("props", {})
        if trigger and trigger in props:
            props[f"declared_{trigger}"] = props.pop(trigger)
            fixed.add(el["id"])
        graph["gaps"] = _merge_gaps(graph.get("gaps", []), [{"id": f"GAP-{el['id']}-{field_name.upper()}", "target": el["id"], "need": field_name, "status": "missing",
                                                              "reason": f"{cls} {el['id']} declares {props.get('declared_' + trigger, '?') if trigger else '?'} but the sources do not state its {field_name} contract"}])
    return fixed


def prune_invalid(graph: dict, ex: Extraction) -> dict:
    """Make the graph ontology-valid without silent loss: demote what can be demoted, turn the rest into gaps."""
    o5 = o5_path.load()
    graph.setdefault("gaps", [])
    for _ in range(8):
        result = o5.validate_graph(graph)
        if result["ok"]:
            break
        if _demote_conditional(graph, result["errors"]):
            continue
        bad_el, bad_edge = set(), []
        for err in result["errors"]:
            if isinstance(err, dict) and err.get("edge"):
                bad_edge.append(err["edge"])
            elif isinstance(err, dict) and err.get("id"):
                bad_el.add(err["id"])
        if not bad_el and not bad_edge:
            break
        for ident in sorted(bad_el):
            reasons = "; ".join(str(e.get("message")) for e in result["errors"] if isinstance(e, dict) and e.get("id") == ident)
            graph["gaps"] = _merge_gaps(graph["gaps"], [{"id": f"GAP-{ident}-INVALID", "target": ident, "need": "valid_element", "status": "unknown",
                                                          "reason": "the ontology rejected this element: " + reasons[:300]}])
        graph["elements"] = [e for e in graph["elements"] if e["id"] not in bad_el]
        dropped = [e for e in graph["edges"] if e in bad_edge or e["src"] in bad_el or e["dst"] in bad_el]
        graph["edges"] = [e for e in graph["edges"] if e not in dropped]
        for e in dropped:
            graph["gaps"] = _merge_gaps(graph["gaps"], [{"id": f"GAP-EDGE-{digest(e)[:8]}", "target": e["src"], "need": "valid_relation", "status": "unknown",
                                                        "reason": f"relation {e['src']} -{e['rel']}-> {e['dst']} was rejected by the ontology"}])
    return graph


def knowledge_need_inputs(graph: dict, docs: list[SourceDoc], ex_ids_by_doc: dict[str, set[str]], params: dict) -> tuple[list[dict], list[dict]]:
    """Per-source facts for each mandatory need, built from typed elements (a rule table, no model)."""
    o5 = o5_path.load()
    profile = o5.ontology5.load()["profiles"]["tdd-generation"]
    na = params.get("not_applicable", {})
    needs = []
    for slot in profile["mandatory_needs"]:
        need = {"id": slot, "slot": slot, "required": True}
        if slot in na:
            need["applicability"] = {"applies": False, **na[slot]}
        needs.append(need)
    sources = []
    for doc in docs:
        facts, evidence = {}, {}
        mine = [e for e in graph["elements"] if e["id"] in ex_ids_by_doc.get(doc.id, set())]
        for slot in profile["mandatory_needs"]:
            classes = NEED_SLOT_CLASSES.get(slot)
            chosen = [e for e in mine if (classes is None or e["class"] in classes)]
            if slot == "provenance":
                chosen = [e for e in mine if e.get("provenance")]
            if chosen:
                # o5 compares a slot's value across sources; a constant keeps sources that agree from
                # reading as a conflict while ``searched_sources`` still records each source separately.
                facts[slot] = "defined"
                evidence[slot] = [p for e in chosen for p in e.get("provenance", [])][:50]
        sources.append({"id": doc.id, "version": doc.revision, "content": doc.text, "kind": doc.role, "facts": facts, "fact_evidence": evidence})
    return needs, sources


def gap_questions(gaps: list[dict]) -> list[dict]:
    out = []
    for g in sorted(gaps, key=lambda x: x["id"]):
        owner = GAP_OWNER.get(g.get("need", ""), "SA")
        out.append({"gap": g["id"], "owner": owner, "question": f"Provide or confirm '{g.get('need')}' for {g.get('target')}: {g.get('reason')}"})
    return out


NEED_OWNER = {"purpose": "BA", "scope": "BA", "requirements": "BA", "contracts": "SA/Dev", "data": "SA/Dev", "behavior": "SA", "decisions": "SA", "quality": "SA", "work": "Dev", "validation": "QA"}


def _same_content(a: dict, b: dict) -> bool:
    keys = ("elements", "edges", "gaps", "conflicts")
    return all(digest(a.get(k, [])) == digest(b.get(k, [])) for k in keys)


def need_questions(blockers: list[str]) -> list[dict]:
    """A blocked document always says what is missing: one question per unresolved knowledge need that no gap already explains."""
    return [{"gap": f"NEED-{slot}", "owner": NEED_OWNER.get(slot, "SA"),
             "question": f"No supplied document states the '{slot}' of this design (expected {', '.join(NEED_SLOT_CLASSES.get(slot, ('an element',)))}). Provide it, or confirm it is not applicable with a reason."}
            for slot in blockers if slot in NEED_SLOT_CLASSES]


def profile_gap_dicts(profile: str, graph: dict, only: set[str] | None = None) -> list[dict]:
    """Profile-aware contract gaps (missing failure policy, rollback, recovery objective ...) as KG gaps; ``only`` limits them to some elements."""
    from ..retrieval import expansion
    out = []
    for gap in expansion.gaps_for(profile, graph["elements"], graph["edges"]):
        if only is not None and gap["element"] not in only:
            continue
        out.append({"id": f"GAP-{gap['element']}-{gap['code']}" + (f"-{gap['field']}" if gap.get("field") else ""), "target": gap["element"], "need": gap.get("field") or gap["code"].lower(),
                    "status": "missing", "code": gap["code"], "reason": f"{gap['class']} {gap['element']}: {gap['code']}" + (f" ({gap['field']})" if gap.get("field") else "")})
    return out


DEFECT_CLASSES = ("Requirement", "QualityTarget", "Operation", "Constraint", "BusinessRule", "Decision")
_SECONDS = re.compile(r"(\d+(?:[.,]\d+)?)\s*(?:giây|s\b|seconds?)", re.I)
_TIMES = re.compile(r"(\d+)\s*(?:lần|times?)", re.I)
_FAIL_MODE = re.compile(r"fail-open|fail-closed|gate_failure_mode", re.I)


def _budgets(text: str) -> dict:
    return {"deadline_s": sorted({float(m.replace(",", ".")) for m in _SECONDS.findall(text)}), "retries": sorted({int(m) for m in _TIMES.findall(text)}),
            "failure_mode": sorted({m.lower() for m in _FAIL_MODE.findall(text)})}


def defect_record(ctx: GateContext, graph: dict, change: dict) -> tuple[dict, list[dict]]:
    """Design case 3: pin down which design facts govern the defect, what budget/retry/failure semantics the sources give, and which
    expected value no source supports (that is an SRS clarification, never a silent TDD edit).  Rules only; nothing is invented."""
    from ..retrieval.candidates import LexicalCandidates
    by_id = {e["id"]: e for e in graph["elements"]}
    text = f"{change.get('reproduction', '')} {change.get('expected_behavior', '')}"
    named = [i for i in T.ids_in(text) if i in by_id]
    lex = LexicalCandidates(ctx.knowledge, classes=DEFECT_CLASSES, use_index=bool(ctx.config.get("retrieval.lexical"))).search(text, k=6)
    affected_ids = list(dict.fromkeys(named + [i for i, _ in lex if i not in named]))[:8]
    affected = []
    for i in affected_ids:
        el = by_id[i]
        quote = next((str(p.get("quote") or "") for p in el.get("provenance", []) if p.get("quote")), "")
        affected.append({"id": i, "class": el.get("class"), "named_in_defect": i in named, "quote": quote[:400], "budgets": _budgets(quote)})
    basis = [a for a in affected if a["named_in_defect"]] or affected  # facts the defect names win over lexical look-alikes
    supported = {"deadline_s": sorted({v for a in basis for v in a["budgets"]["deadline_s"]}), "retries": sorted({v for a in basis for v in a["budgets"]["retries"]}),
                 "failure_mode": sorted({v for a in basis for v in a["budgets"]["failure_mode"]})}
    expected = _budgets(str(change.get("expected_behavior", "")))
    unsupported = [{"kind": k, "value": v} for k in ("deadline_s", "retries") for v in expected[k] if v not in supported[k]]
    gaps = [{"id": f"GAP-{change.get('id', 'DEFECT')}-CLARIFY-{n}", "target": (affected_ids[0] if affected_ids else change.get("id", "DEFECT")), "need": "srs_clarification", "status": "missing", "defect": change.get("id"),
             "reason": f"defect {change.get('id')} expects {u['kind']} = {u['value']} but no supplied source states it; ask for an SRS clarification before the TDD records it"} for n, u in enumerate(unsupported, 1)]
    if not affected_ids:
        gaps.append({"id": f"GAP-{change.get('id', 'DEFECT')}-TARGET", "target": change.get("id", "DEFECT"), "need": "defect_target", "status": "missing", "defect": change.get("id"),
                     "reason": "no design element of the knowledge graph matches the defect; name the Operation or requirement it concerns"})
    record = {"id": change.get("id"), "reproduction": change.get("reproduction"), "expected_behavior": change.get("expected_behavior"), "affected": affected, "supported_budgets": supported,
              "expected_values": expected, "unsupported_expected": unsupported, "srs_clarification_required": bool(unsupported),
              "test_scenario": {"given": change.get("reproduction"), "then": change.get("expected_behavior"), "evidence": affected_ids}}
    return record, gaps


def generation_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    ob = ctx.observer
    params = ctx.request.get("params", {})
    mode = ctx.request.get("mode") or params.get("mode", "initial")
    tdd_id = params.get("tdd_id") or f"TDD-{ctx.run_id}"
    with logic_step(ob, "gen.pin-sources"):
        docs = pinned_sources(ctx)
        errors = source_errors(ctx, docs, mode, params)
    if errors:
        return GateOutcome.blocked(*errors)
    head, head_g = head_graph(ctx.knowledge)
    # a document whose exact text the graph already holds is not read again: its facts stay as they were (the SAD is read-only, and a model reading it twice
    # would not word every fact the same way); ``force_extract`` reads everything
    done = {} if params.get("force_extract") else ingested_hashes(head_g)
    unread = [d for d in docs if d.content_hash in done.get(d.id, ())]
    ex = extract_documents(ctx, [d for d in docs if d not in unread], params, prose=params.get("prose", True), known={e["id"]: e for e in head_g.get("elements", [])})
    if unread:
        ex.stats["not_read_again"] = [f"{d.id}@{d.revision}" for d in unread]
    if ex.blockers:
        return GateOutcome.blocked(*dict.fromkeys(ex.blockers))
    for el in ex.elements:  # every fact extracted by this run belongs to the TDD it generates
        el["tdds"] = sorted(set(el.get("tdds") or []) | {tdd_id})
    version = f"{tdd_id}-v{(head.seq + 1) if head else 1}"
    graph, merge_report = merge_into_graph(head_g, ex, version)
    unread_ids = {d.id for d in unread}
    for el in graph["elements"]:  # the facts of a document not read again still belong to the TDD that is generated from it
        if unread_ids & {str(p.get("source_id")) for p in el.get("provenance") or []}:
            el["tdds"] = sorted(set(el.get("tdds") or []) | {tdd_id})
    with logic_step(ob, "gen.build-changeset"):
        graph = prune_invalid(graph, ex)
        graph["gaps"] = _merge_gaps(graph.get("gaps", []), profile_gap_dicts("tdd-generation", graph))
    defect = None
    if mode == "defect":
        with logic_step(ob, "gen.defect-record"):
            defect, defect_gaps = defect_record(ctx, graph, params.get("change_request") or {})
            graph["gaps"] = _merge_gaps(graph.get("gaps", []), defect_gaps)
    ids_by_doc: dict[str, set[str]] = {}
    for e in graph["elements"]:
        ids_by_doc.setdefault(e.get("owner_doc") or (e.get("props") or {}).get("owner_doc"), set()).add(e["id"])
    with logic_step(ob, "gen.resolve-needs"):
        needs, need_sources = knowledge_need_inputs(graph, docs, ids_by_doc, params)
        from dynamic import TddGenerationRequest, run_generation
        request = TddGenerationRequest(id=ctx.run_id, mode=mode, scope=params.get("scope") or [tdd_id], sources=need_sources,
                                       base_snapshot=params.get("base_snapshot"), change_request=params.get("change_request"))
    with logic_step(ob, "gen.validate-readiness"):
        result = run_generation(request, graph, needs)
    if head is not None and _same_content(graph, head_g):
        commit = SimpleNamespace(snapshot=head)  # nothing changed: no new snapshot
    else:
        commit = ctx.commit_graph(graph, base_snapshot=head.id if head else None, meta={"flow": "generation", "mode": mode, "tdd_id": tdd_id, "readiness": result["status"]})
    committed = ctx.knowledge.materialize(commit.snapshot.id)
    with logic_step(ob, "gen.render"):
        from .common import render_document
        markdown, roundtrip = render_document(ctx, committed, tdd_id, snapshot=commit.snapshot, readiness=result["status"])
    doc = ctx.register_generated_tdd(tdd_id, markdown, {"snapshot": commit.snapshot.id, "readiness": result["status"]})
    out_dir = ctx.config.work_dir / "runs" / ctx.run_id
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / f"{tdd_id}.r{doc.revision}.md").write_text(markdown, encoding="utf-8")
    with logic_step(ob, "gen.gap-questions"):
        readiness = result.get("readiness", {})
        questions = gap_questions(committed.get("gaps", [])) + need_questions(readiness.get("blockers", []))
    output = {"status": result["status"], "tdd": {"id": tdd_id, "revision": doc.revision, "content_hash": doc.content_hash}, "snapshot": commit.snapshot.id,
              "snapshot_hash": commit.snapshot.hash, "merge": merge_report, "elements": len(committed["elements"]), "edges": len(committed["edges"]),
              "gaps": len(committed.get("gaps", [])), "conflicts": len(committed.get("conflicts", [])), "blockers": readiness.get("blockers", result.get("blockers", [])),
              "questions": questions, "stats": ex.stats, "roundtrip": roundtrip, "artifact": str(out_dir / f"{tdd_id}.r{doc.revision}.md")}
    if defect is not None:
        output["defect"] = defect
    return GateOutcome.ok(output, evidence={"intake": result.get("intake", {}).get("reports", [])})
