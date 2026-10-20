"""Prompt-edit agent (design section 3.3).

The model only turns the prompt into a structured ``EditIntent`` (and, when rules
cannot, picks among target candidates).  Classification, authorization, conflict
with upstream values, version checks, planning and the atomic commit are rules.
"""
from __future__ import annotations

import re
from dataclasses import asdict

from ..core import o5_path
from ..orchestrator.context import GateContext
from ..orchestrator.model import GateOutcome
from ..skills.lib import text as T
from ..skills.steps import logic_step
from .common import deep, head_graph, latest_tdd_docs, render_document, render_tdd, skills, tdd_members

MAX_CANDIDATES = 25
# Names of these classes are single common words ("timeout", "target", "Open"); a name alone must never select them.
NAME_AMBIGUOUS_CLASSES = ("ErrorCode", "State", "Transition", "Attribute")


def _p(ctx: GateContext) -> dict:
    return ctx.request.get("params", {})


def stated_in_sources(element: dict, value) -> bool:
    """True when ``value`` occurs as a whole token in a quote the element was extracted from (a value the sources state is not the prompt's to change).

    Values the sources do not state (a physical type that is still ``unknown``, a derived level) stay editable: that is design work.
    """
    if element.get("origin") not in ("upstream", "external") or isinstance(value, (bool, dict, list)) or value is None:
        return False
    from ..render.rules import clean
    text = clean(value)
    if not text or text.lower() == "unknown" or (len(text) < 2 and not text.isdigit()):
        return False
    pattern = re.compile(rf"(?<![\w.]){re.escape(text)}(?![\w.])", re.I)
    return any(pattern.search(clean(p.get("quote"))) for p in element.get("provenance") or [])


def candidates_for(graph: dict, mention: str, hinted: list[str]) -> list[dict]:
    els = graph.get("elements", [])
    by_id = {e["id"]: e for e in els}
    if hinted:
        return [by_id[i] for i in hinted if i in by_id][:MAX_CANDIDATES]
    scored = []
    for e in els:
        name = str(e.get("name") or "")
        by_id_mention = e["id"].lower() in mention.lower()
        if e.get("class") in NAME_AMBIGUOUS_CLASSES and not by_id_mention:
            continue
        score = max(T.jaccard(mention, name), 1.0 if by_id_mention else 0.0)
        if name and T.normalize_name(name) in T.normalize_name(mention):
            score = max(score, 0.9)
        if score >= 0.25:
            scored.append((score, e))
    scored.sort(key=lambda t: (-t[0], t[1]["id"]))
    return [e for _, e in scored[:MAX_CANDIDATES]]


def _brief(e: dict) -> dict:
    p = e.get("props") or {}
    return {"id": e["id"], "name": e.get("name"), "class": e.get("class"), "summary": str(p.get("statement") or p.get("summary") or p.get("definition") or "")[:200]}


def _clarification(code: str, question: str, candidates: list[str] | None = None, **extra) -> GateOutcome:
    return GateOutcome.ok({"status": "clarification_required", "reason": code, "question": question, "candidates": candidates or [], **extra})


def _document_presentation_edit(ctx: GateContext, graph: dict, head, intent: dict, prompt: str, mention: str, p: dict) -> GateOutcome:
    """A presentation-only edit that names no design element (reorder sections, change the layout) applies to a TDD document."""
    import prompt_edit as pe
    docs = latest_tdd_docs(ctx.knowledge)
    named = [d for d in docs if d.id.lower() in prompt.lower()]
    pick = named[0] if len(named) == 1 else (docs[0] if len(docs) == 1 else None)
    audit = {"intent": intent, "classification": {"kind": "presentation", "reasons": ["the edit only changes how a document is laid out"]}}
    if pick is None:
        return _clarification("TARGET_UNRESOLVED", "Which TDD document should be re-laid out? Name its id.", [d.id for d in docs], **audit)
    request = pe.EditRequest(id=ctx.run_id, prompt=prompt, target=[pick.id], base_snapshot=graph.get("version") or head.id, authorization=p.get("authorization", {}),
                             user_id=p.get("user_id", "user"), intent="presentation")
    overlay = pe.presentation_overlay(request, {"id": pick.id, "revision": int(pick.revision) if str(pick.revision).isdigit() else 0, "semantic_graph": graph}, {"prompt": prompt, "intent": intent})
    errs = pe.validate_presentation_overlay(overlay, [], [])
    if errs:
        return GateOutcome.blocked(*errs, output=audit)
    return GateOutcome.ok({"status": "presentation_committed", "overlay": overlay, "snapshot": head.id, "target": pick.id, **audit})


def _section_edit(ctx: GateContext, graph: dict, head, prompt: str, p: dict) -> GateOutcome | None:
    """A free-form edit of one section of a TDD (\"sửa mục 1.3 …\"); ``None`` when the prompt names no section (or names an element), so the element edit goes on."""
    import prompt_edit as pe
    from . import sectionedit as se
    if any(i in {e["id"] for e in graph["elements"]} for i in T.ids_in(prompt)):
        return None
    docs = latest_tdd_docs(ctx.knowledge)
    named = [d for d in docs if d.id.lower() in prompt.lower()]
    doc = named[0] if len(named) == 1 else (docs[0] if len(docs) == 1 else None)
    if not docs:
        return None
    probe_text = (doc or docs[0]).text
    number, info = se.find_section(prompt, probe_text)
    if number is None:
        return _clarification("SECTION_NOT_FOUND", f"Which section do you mean? {info}".strip() + " Name it by its number, for example \"mục 1.3\".", []) if info else None
    if doc is None:
        return _clarification("TARGET_UNRESOLVED", "Which TDD document should be edited? Name its id.", [d.id for d in docs])
    nid = se.note_id(doc.id, number)
    request = pe.EditRequest(id=ctx.run_id, prompt=prompt, target=[nid], base_snapshot=graph.get("version") or head.id, authorization=p.get("authorization", {}),
                             user_id=p.get("user_id", "user"), intent="semantic")
    if not pe.authorized(request, "modify", [nid]):
        return GateOutcome.blocked("UNAUTHORIZED", output={"target": nid, "required": {"operation": "modify", "target": nid}})
    current = se.section_text(doc.text, info)
    written = skills(ctx).run("write-section", {"instruction": prompt, "section": {"number": number, "title": info["title"]}, "current": current})
    if not written.ok:
        return GateOutcome.blocked(*written.blockers, output={"section": number, "title": info["title"]})
    existing = next((e for e in graph["elements"] if e["id"] == nid), None)
    mode, markdown = se.merge_note(existing, written.output["mode"], written.output["markdown"])
    note = {"id": nid, "class": "Section", "name": f"{number} {info['title']}", "origin": "own", "maturity": "specified", "review_status": "approved", "owner_doc": doc.id, "tdds": [doc.id],
            "version_introduced": graph.get("version"), "props": {"section": number, "title": info["title"], "mode": mode, "markdown": markdown, "instruction": prompt, "summary": written.output["summary"],
                                                                     "tdd_id": doc.id},
            "reviews": [{"decision": "approved", "actor": p.get("user_id", "user"), "role": "editor", "rationale": "written by the person who made the edit request", "base_snapshot": head.id}],
            "provenance": [pe.prompt_provenance(request)]}
    new = deep(graph)
    new["elements"] = [e for e in new["elements"] if e["id"] != nid] + [note]
    new["version"] = f"{graph.get('version')}+edit-{ctx.run_id}"
    commit = ctx.commit_graph(new, base_snapshot=head.id, meta={"flow": "prompt-edit", "intent": "section", "section": number})
    committed = ctx.knowledge.materialize(commit.snapshot.id)
    text, check = render_document(ctx, committed, doc.id, snapshot=commit.snapshot)
    nd = ctx.register_generated_tdd(doc.id, text, {"snapshot": commit.snapshot.id, "run": ctx.run_id, "cause": "prompt-edit"})
    return GateOutcome.ok({"status": "committed", "target": nid, "affected": [nid], "snapshot": commit.snapshot.id, "snapshot_hash": commit.snapshot.hash, "before_snapshot": head.id,
                           "documents": [{"id": nd.id, "revision": nd.revision, "content_hash": nd.content_hash}], "readiness": None, "rendered_ok": bool(check.get("ok", True)),
                           "section_edit": {"note_id": nid, "section": number, "title": info["title"], "mode": written.output["mode"], "text_mode": mode, "summary": written.output["summary"], "previous": existing, "tdd_id": doc.id},
                           "intent": {"operation": "section", "target_mention": f"{number} {info['title']}"}})


def promptedit_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    import prompt_edit as pe
    p, hub, ob = _p(ctx), skills(ctx), ctx.observer
    prompt = p.get("prompt", "")
    if not prompt.strip():
        return GateOutcome.blocked("PROMPT_REQUIRED")
    head, graph = head_graph(ctx.knowledge)
    if head is None:
        return GateOutcome.blocked("KG_EMPTY")
    base = p.get("base_snapshot") or head.id
    with logic_step(ob, "pe.authorize-conflict-version"):
        if base != head.id:
            return GateOutcome.blocked("VERSION_CONFLICT", output={"base": base, "head": head.id})
    sectioned = _section_edit(ctx, graph, head, prompt, p)
    if sectioned is not None:
        return sectioned
    hinted = list(p.get("target") or [])
    parsed = hub.run("parse-edit-intent", {"prompt": prompt, "known_paths": sorted({f"props.{k}" for e in graph["elements"] if e["id"] in hinted for k in (e.get("props") or {})})})
    if not parsed.ok:
        return GateOutcome.blocked(*parsed.blockers)
    intent = parsed.output
    mention = intent["target_mention"] or prompt
    by_id = {e["id"]: e for e in graph["elements"]}
    named_ids = T.ids_in(mention) if not hinted else []
    unknown = [i for i in named_ids if i not in by_id]
    if unknown and not any(i in by_id for i in named_ids):
        # The user named an element by id and the KG has no such element: never fall back to guessing by words.
        gaps = [g for g in graph.get("gaps", []) if g.get("target") in unknown]
        why = "; ".join(f"{g['target']}: {g.get('reason', '')}" for g in gaps[:3]) or "no element with that id exists"
        with logic_step(ob, "pe.clarification"):
            return _clarification("TARGET_UNRESOLVED", f"{', '.join(unknown)} is not defined in the knowledge graph ({why}). Which defined element do you mean, or should the missing definition be supplied through a change request?",
                                  [], intent=intent, unknown_ids=unknown, gaps=[g["id"] for g in gaps])
    cands = candidates_for(graph, mention + " " + prompt if hinted == [] else mention, hinted)
    if not cands and not hinted:
        pre = hub.run("classify-edit-from-intent", {"intent": intent, "upstream": {"locked": {}}})
        if pre.ok and pre.output["kind"] == "presentation":
            return _document_presentation_edit(ctx, graph, head, intent, prompt, mention, p)
    resolved = hub.run("disambiguate-target", {"mention": mention, "context": prompt, "candidates": [_brief(c) for c in cands]})
    if not resolved.ok:
        return GateOutcome.blocked(*resolved.blockers)
    with logic_step(ob, "pe.clarification"):
        if resolved.output["id"] is None:
            if resolved.output.get("mismatch"):
                return _clarification("TARGET_MISMATCH", resolved.output["reason"] + ". Which element do you mean?", resolved.output["considered"], intent=intent)
            return _clarification("TARGET_UNRESOLVED", "Which design element should be edited? Name its id or a unique name.", [c["id"] for c in cands], intent=intent)
    target = next(e for e in graph["elements"] if e["id"] == resolved.output["id"])
    if intent.get("operation") in ("modify", "add", "remove"):
        # who may edit what is decided before anything about the content: an unauthorized user learns nothing about SRS conflicts
        with logic_step(ob, "pe.authorize-conflict-version"):
            probe = pe.EditRequest(id=ctx.run_id, prompt=prompt, target=[target["id"]], base_snapshot=graph.get("version") or base, authorization=p.get("authorization", {}),
                                   user_id=p.get("user_id", "user"), intent="semantic")
            if not pe.authorized(probe, intent["operation"], [target["id"]]):
                return GateOutcome.blocked("UNAUTHORIZED", output={"intent": intent, "target": target["id"], "required": {"operation": intent["operation"], "target": target["id"]}})
    locked = {}
    path = str(intent.get("path") or "")
    tprops = target.get("props") or {}
    key = path[len("props."):] if path.startswith("props.") else None
    if path in ("name", "") and target.get("origin") in ("upstream", "external") and target.get("provenance") and intent.get("operation") == "modify":
        locked["name"] = target.get("name")  # the name was read from a source document
        path = "name"
    if key and tprops.get("upstream_value") is not None:
        locked[key] = tprops["upstream_value"]
    elif key and key in (tprops.get("sourced_props") or []):
        locked[key] = tprops.get(key)  # read from an upstream document: only an SRS/CR clarification may change it
    elif key and stated_in_sources(target, tprops.get(key)):
        locked[key] = tprops.get(key)  # the value is quoted in the evidence the element was extracted from, so it too comes from a document
    with logic_step(ob, "pe.classify"):
        cls = hub.run("classify-edit-from-intent", {"intent": intent, "upstream": {"locked": locked}})
    kind = cls.output["kind"]
    request = pe.EditRequest(id=ctx.run_id, prompt=prompt, target=[target["id"]], base_snapshot=graph.get("version") or base, authorization=p.get("authorization", {}),
                             user_id=p.get("user_id", "user"), intent="presentation" if kind == "presentation" else "semantic")
    audit_base = {"intent": intent, "classification": cls.output, "target": target["id"], "resolution": resolved.output}
    if kind == "conflict":
        conflict = {"type": "Deviation", "status": "proposed", "element_id": target["id"], "property": key if (key := (path[len("props."):] if path.startswith("props.") else None)) else path,
                    "upstream": cls.output.get("upstream_value"), "requested": intent["value"], "resolution": "pending"}
        return GateOutcome.blocked("UPSTREAM_SRS_CLARIFICATION_REQUIRED", output={**audit_base, "conflicts": [conflict], "upstream_value": cls.output.get("upstream_value"),
                                                                               "next": "Raise an SRS/CR clarification through the impact gate; a prompt cannot override a value read from an upstream document"})
    if kind == "clarification":
        with logic_step(ob, "pe.clarification"):
            return _clarification("INTENT_NOT_ACTIONABLE", "Describe the property to change and its new value (for example: set <property> of <element> to <value>).", [target["id"]], **audit_base)
    if kind == "presentation":
        doc = {"id": target["id"], "semantic_graph": graph}
        overlay = pe.presentation_overlay(request, doc, {"prompt": prompt, "intent": intent})
        errs = pe.validate_presentation_overlay(overlay, [], [])
        if errs:
            return GateOutcome.blocked(*errs, output=audit_base)
        return GateOutcome.ok({"status": "presentation_committed", "overlay": overlay, "snapshot": head.id, **audit_base})
    operation = intent["operation"]
    if operation not in ("modify", "add", "remove"):
        return _clarification("UNSUPPORTED_OPERATION", f"Operation {operation!r} is not supported for semantic edits.", [target["id"]], **audit_base)
    with logic_step(ob, "pe.authorize-conflict-version"):
        if not pe.authorized(request, operation, [target["id"]]):
            return GateOutcome.blocked("UNAUTHORIZED", output={**audit_base, "required": {"operation": operation, "target": target["id"]}})
        value_path = path or "name"
        current = target
        for key in value_path.split("."):
            current = current.get(key) if isinstance(current, dict) else None
        conflict = pe.upstream_conflict(target, intent["value"]) if operation == "modify" and value_path.startswith("props.") else None
    with logic_step(ob, "pe.plan-commit-undo"):
        op = {"seq": 1, "op": operation, "target": target["id"], "provenance": [pe.prompt_provenance(request)]}
        if operation == "modify":
            op.update(path=value_path, before=current, after=intent["value"])
        elif operation == "remove":
            op.update(before=target.get("name"), cascade=(p.get("authorization") or {}).get("cascade", "restrict"))
        else:
            return _clarification("ADD_NEEDS_ELEMENT", "Adding an element needs its class and properties; use a change request.", [target["id"]], **audit_base)
        plan = {"status": "planned", "change_set": {"id": "CS-" + ctx.run_id, "request": ctx.run_id, "base_version": request.base_snapshot, "result_version": f"{request.base_snapshot}+1",
                                                    "ops": [op], "prompt_provenance": pe.prompt_provenance(request)}, "targets": [target["id"]]}
        if conflict:
            plan["conflicts"] = [conflict]
        result = pe.commit_edit(request, graph, plan, render_fn=lambda g: render_tdd(g, None))
    status = result["status"]
    if status in ("blocked", "rejected"):
        code = result.get("reason") or (result.get("result") or {}).get("code") or ("UPSTREAM_SRS_CLARIFICATION_REQUIRED" if result.get("conflicts") else "EDIT_BLOCKED")
        return GateOutcome.blocked(code, output={**audit_base, "conflicts": result.get("conflicts") or plan.get("conflicts", []), "upstream_value": locked or None})
    if status not in ("committed", "committed_pending_render"):
        return GateOutcome.blocked(f"EDIT_{status.upper()}", output=audit_base)
    commit = ctx.commit_changeset(plan["change_set"], base_snapshot=head.id, meta={"flow": "prompt-edit", "intent": kind})
    committed = ctx.knowledge.materialize(commit.snapshot.id)
    affected = sorted(set(result["audit"]["impact"].get("affected", [])) | {target["id"]})
    stale_docs, documents = [], []
    for d in latest_tdd_docs(ctx.knowledge):
        members, _ = tdd_members(committed, d.id)
        if {m["id"] for m in members} & set(affected):
            text, _ = render_document(ctx, committed, d.id, snapshot=commit.snapshot)
            nd = ctx.register_generated_tdd(d.id, text, {"snapshot": commit.snapshot.id, "run": ctx.run_id, "cause": "prompt-edit"})
            documents.append({"id": nd.id, "revision": nd.revision, "content_hash": nd.content_hash})
    out = {"status": status, "snapshot": commit.snapshot.id, "snapshot_hash": commit.snapshot.hash, "documents": documents, "affected": affected, "change_set": plan["change_set"],
           "audit": {k: v for k, v in result["audit"].items() if k in ("before_snapshot", "after_snapshot", "validation", "deviations", "render_error")},
           "readiness": result["audit"].get("readiness", {}).get("status"), **audit_base}
    if status == "committed_pending_render":
        out["note"] = "the edit is committed; rendering failed and the previously published documents are unchanged"
    return GateOutcome.ok(out)


def undo_section_edit(knowledge, run_output: dict, *, actor: str) -> dict:
    """Put a section back as it was before a free-form edit: the person's earlier text for it, or none.  Refuses when the graph moved on."""
    info = run_output["section_edit"]
    head = knowledge.head()
    if head is None or head.id != run_output["snapshot"]:
        return {"status": "conflict", "head": head.id if head else None}
    graph = knowledge.materialize(head.id)
    graph["elements"] = [e for e in graph["elements"] if e["id"] != info["note_id"]] + ([info["previous"]] if info.get("previous") else [])
    graph["version"] = f"{graph.get('version')}+undo"
    commit = knowledge.commit_graph(head.id, graph, actor=actor, meta={"undo_of": run_output.get("snapshot"), "flow": "prompt-edit"})
    return {"status": "undone", "snapshot": commit.snapshot.id}


def undo_last_edit(knowledge, run_output: dict, *, actor: str):
    """Compensating change (undo) with a version check; refuses when the graph moved on."""
    if run_output.get("section_edit"):
        return undo_section_edit(knowledge, run_output, actor=actor)
    o5_path.o5_root()  # a process that only undoes has not loaded the o5 modules yet
    import prompt_edit as pe
    head = knowledge.head()
    graph = knowledge.materialize(head.id)
    audit = {"change_set": run_output["change_set"], "after_snapshot": run_output["snapshot_hash"]}
    res = pe.undo(audit, graph)
    if res["status"] != "undone":
        return res
    commit = knowledge.commit_changeset(head.id, res["inverse"], actor=actor, meta={"undo_of": run_output["change_set"]["id"]})
    return {"status": "undone", "snapshot": commit.snapshot.id}
