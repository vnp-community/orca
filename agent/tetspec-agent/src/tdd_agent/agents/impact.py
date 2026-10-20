"""Intake (G0), SRS readiness (G1) and the impact agent (G2).

Rules do the bulk of the work: delta derivation, trace-distance overlap, bounds
checks, report assembly.  The model is consulted only for TDDs whose coverage
cannot be proven by rules (``judge-design-coverage``) and, asymmetrically, for
prose architecture constraints (``detect-sad-conflict``).
"""
from __future__ import annotations

from typing import Any

from ..core import o5_path
from ..orchestrator.context import GateContext
from ..orchestrator.model import GateOutcome
from ..skills.lib.workflow_io import impact_to_dict
from ..skills.steps import logic_step
from ..store.base import SourceDoc, digest
from .common import upstream, approval_of, by_role, deep, head_graph, latest_tdd_docs, pinned_sources, skills

TRACE_CLASSES = ("Requirement", "RequirementVersion")
MAX_JUDGEMENTS = 40


def _params(ctx: GateContext) -> dict:
    return ctx.request.get("params", {})


def _doc(docs: list[SourceDoc], doc_id: str, revision: str) -> SourceDoc | None:
    return next((d for d in docs if d.id == doc_id and d.revision == str(revision)), None)


def cr_ref(cr: dict) -> dict:
    return {"document_id": cr["id"], "role": "CR", "revision": str(cr.get("revision", "1")), "content_hash": digest(cr), "section_id": "document"}


def intake_gate(ctx: GateContext) -> GateOutcome:
    p = _params(ctx)
    errors: list[str] = []
    with logic_step(ctx.observer, "gen.pin-sources"):
        docs = pinned_sources(ctx)
        roles = {d.role for d in docs}
        errors += [f"SOURCE_ROLE_MISSING:{r}" for r in ("SRS", "SAD") if r not in roles]
        srs = dict(p.get("srs") or {})
        cr = p.get("cr") or {}
        from_cr = not srs.get("after")  # no edited SRS: the change is described by the CR alone
        for key in ("before",) if from_cr else ("before", "after"):
            if not srs.get("id") or not srs.get(key) or _doc(docs, srs["id"], srs[key]) is None:
                errors.append(f"SRS_REVISION_NOT_PINNED:{key}")
        cr_doc = next((d for d in docs if d.role == "CR" and d.id == cr.get("id")), None)
        if from_cr and cr_doc is None:
            errors.append("CR_DOCUMENT_MISSING")
        if not cr.get("id"):
            errors.append("CR_REF_MISSING")
    if errors:
        return GateOutcome.blocked(*errors)
    extra: dict = {}
    if from_cr:
        from .crdelta import derive_srs_from_cr
        after, report, blockers = derive_srs_from_cr(ctx, _doc(docs, srs["id"], srs["before"]), cr_doc, cr, p.get("id_class_map"))
        if blockers:
            return GateOutcome.blocked(*blockers, output={"cr_edits": report.get("edits", [])})
        srs["after"] = after.revision
        docs = docs + [after]
        extra = {"cr_driven": True, "derived_pins": [{"id": after.id, "revision": after.revision, "role": after.role, "content_hash": after.content_hash}], "cr_proposal": report}
    head = ctx.knowledge.head()
    inventory = [{"id": d.id, "revision": d.revision, "content_hash": d.content_hash} for d in latest_tdd_docs(ctx.knowledge)]
    return GateOutcome.ok({"documents": [{"id": d.id, "revision": d.revision, "role": d.role, "content_hash": d.content_hash} for d in docs],
                           "baseline_snapshot": head.id if head else None, "tdd_inventory": inventory, "cr": cr, "srs": srs, **extra})


def srs_readiness_gate(ctx: GateContext) -> GateOutcome:
    p = _params(ctx)
    intake = upstream(ctx, "G0")
    docs = pinned_sources(ctx)
    srs = intake["srs"]
    before, after = _doc(docs, srs["id"], srs["before"]), _doc(docs, srs["id"], srs["after"])
    hub = skills(ctx)
    class_map = (ctx.request.get("params") or {}).get("id_class_map")
    with logic_step(ctx.observer, "imp.derive-delta"):
        res = hub.run("derive-srs-delta", {"before": {"source_id": before.id, "revision": before.revision, "role": "SRS", "text": before.text, **({"id_class_map": class_map} if class_map else {})},
                                           "after": {"source_id": after.id, "revision": after.revision, "role": "SRS", "text": after.text, **({"id_class_map": class_map} if class_map else {})},
                                           "cr_ref": cr_ref(intake["cr"])})
    if not res.ok:
        return GateOutcome.blocked(*res.blockers)
    delta = res.output
    if not delta["changes"] and not delta.get("unstructured"):
        if res.gaps:
            return GateOutcome.blocked("SRS_DELTA_UNMAPPED_IDS", output={"gaps": res.gaps, "unmapped_ids": sorted(g["target"] for g in res.gaps if g.get("need") == "class_mapping")})
        return GateOutcome.blocked("SRS_DELTA_EMPTY")
    delta["baseline_tdd_snapshot"] = intake.get("baseline_snapshot") or "empty"
    delta["acceptance"] = list(intake["cr"].get("acceptance", []))
    for c in delta["changes"]:
        c["cr_acceptance_refs"] = list(delta["acceptance"])
    blockers = []
    if not delta["changes"]:
        blockers.append("SRS_DELTA_UNSTRUCTURED_ONLY")
    if blockers:
        return GateOutcome.blocked(*blockers, output={"srs_delta": delta, "gaps": res.gaps})
    decision = {"srs_delta": {"ref": delta["ref"], "cr_ref": delta["cr_ref"], "changes": delta["changes"]}, "gaps": res.gaps, "unstructured": delta.get("unstructured", [])}
    if intake.get("cr_driven"):  # what the BA confirms is the reading of the CR: each edit with the sentence of the CR it comes from, and the resulting diff of the SRS
        proposal = intake["cr_proposal"]
        decision["from_change_request"] = {"edits": proposal["edits"], "not_mapped_to_a_requirement": proposal["unmatched"], "proposed_srs": proposal["proposed_srs"], "diff": proposal["diff"],
                                         "may_be_affected_by_a_rename": proposal.get("term_sweep", [])}
    return GateOutcome.ok({"srs_delta": delta, "gaps": res.gaps}, decision=decision)


def _light(el: dict) -> dict:
    p = el.get("props") or {}
    keep = {k: p[k] for k in ("logical_id", "range", "bounds", "min", "max", "tdds", "owner_doc", "statement", "summary", "definition") if k in p}
    return {"id": el["id"], "class": el.get("class"), "name": el.get("name"), "owner_doc": el.get("owner_doc"), "tdds": el.get("tdds"), "props": keep}


def build_inventory(graph: dict, tdd_docs: list[SourceDoc]) -> dict:
    """TDD documents with the section refs that prove what they contain, plus light KG elements/edges for the scan."""
    from .common import tdd_members
    docs = []
    for d in tdd_docs:
        members, _ = tdd_members(graph, d.id)
        sections = [{"document_id": d.id, "role": "TDD", "revision": d.revision, "content_hash": d.content_hash, "section_id": "document"}]
        sections += [{"document_id": d.id, "role": "TDD", "revision": d.revision, "content_hash": d.content_hash, "section_id": m["id"]} for m in members[:40]]
        docs.append({"id": d.id, "revision": d.revision, "content_hash": d.content_hash, "sections": sections})
    keep_rel = {"traces_to", "satisfies", "has_scenario", "derives_from", "implemented_by", "references_entity", "constrained_by"}
    return {"tdd_docs": docs, "elements": [_light(e) for e in graph.get("elements", [])], "edges": [e for e in graph.get("edges", []) if e.get("rel") in keep_rel]}


def _sad_dict(ctx: GateContext, sad_doc: SourceDoc, findings: list[dict]) -> dict:
    ref = {"document_id": sad_doc.id, "role": "SAD", "revision": sad_doc.revision, "content_hash": sad_doc.content_hash, "section_id": "document"}
    return {"ref": ref, "architecture_change": bool(findings)}


def impact_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    wf = o5.workflow
    p = _params(ctx)
    hub = skills(ctx)
    ob = ctx.observer
    g1 = upstream(ctx, "G1")
    delta = deep(g1.get("srs_delta") or p.get("srs_delta") or {})
    if not delta.get("changes"):
        return GateOutcome.blocked("SRS_DELTA_REQUIRED")
    docs = pinned_sources(ctx)
    sad_doc = by_role(docs, "SAD")
    if sad_doc is None:
        return GateOutcome.blocked("SOURCE_ROLE_MISSING:SAD")
    snap = p.get("baseline_snapshot") or upstream(ctx, "G0").get("baseline_snapshot")
    graph = ctx.knowledge.materialize(snap) if snap else head_graph(ctx.knowledge)[1]
    tdd_docs = latest_tdd_docs(ctx.knowledge)
    changes = delta["changes"]
    for c in changes:
        hinted = (p.get("tdd_for_new") or {}).get(c["id"])
        if hinted:
            c["tdd_ids"] = [hinted]
        c["new_tdd_id"] = hinted or f"TDD-NEW-{c['id']}"
    with logic_step(ob, "imp.candidates-q1"):
        inv = build_inventory(graph, tdd_docs)
    with logic_step(ob, "imp.scan-inventory"):
        scan = hub.run("scan-inventory-overlap", {"changes": changes, "tdd_docs": inv["tdd_docs"], "elements": inv["elements"], "edges": inv["edges"]})
    if not scan.ok:
        return GateOutcome.blocked(*scan.blockers)
    judged, judgements_made = {}, 0
    members = {e["id"]: e for e in graph.get("elements", [])}
    from .common import tdd_members
    for t in scan.output["tdds"]:
        if t["verdict"] != "residue":
            continue
        els, _ = tdd_members(graph, t["id"])
        sections = [{"id": e["id"], "class": e.get("class"), "name": e.get("name"), "props": {k: (e.get("props") or {}).get(k) for k in ("statement", "summary", "definition") if (e.get("props") or {}).get(k)}} for e in els if e.get("class") not in TRACE_CLASSES][:60]
        to_judge = [c for c in changes if c["id"] in (t["residue_changes"] or [x["id"] for x in changes]) and c.get("kind") != "removed"]
        decisions = {}
        for c in to_judge:
            if judgements_made >= int(p.get("max_judgements", MAX_JUDGEMENTS)):
                decisions[c["id"]] = {"decision": "update", "cited": [], "reason": "judgement budget exhausted; treated as needing an update"}
                continue
            judgements_made += 1
            r = hub.run("judge-design-coverage", {"change": c, "tdd_id": t["id"], "sections": sections, "distance_by_id": {}})
            if not r.ok:
                return GateOutcome.blocked(*r.blockers)
            decisions[c["id"]] = r.output
        judged[t["id"]] = decisions
    sad_elements = [e for e in graph.get("elements", []) if str(e.get("owner_doc") or "").lower() == sad_doc.id.lower() and e.get("class") in ("Decision", "Constraint", "Component")]
    sad_res = hub.run("detect-sad-conflict", {"changes": changes, "sad_elements": sad_elements, "sad_source": {"id": sad_doc.id, "revision": sad_doc.revision, "text": sad_doc.text}})
    if not sad_res.ok:
        return GateOutcome.blocked(*sad_res.blockers)
    # a waiver is the SA's decision, per (SAD element, requirement), with the reason; it applies to what the rules found as well as to what the model suspected
    waivers = {(w["sad_element"], w["change_id"]): w for w in p.get("sad_waivers", []) if w.get("rationale") and w.get("actor")}
    findings = [f for f in sad_res.output["findings"] if (f["sad_element"], f["change_id"]) not in waivers]
    waived = [{**{k: f[k] for k in ("code", "origin", "sad_element", "change_id", "message")}, "waived_by": waivers[(f["sad_element"], f["change_id"])]["actor"],
               "role": waivers[(f["sad_element"], f["change_id"])].get("role"), "rationale": waivers[(f["sad_element"], f["change_id"])]["rationale"]}
              for f in sad_res.output["findings"] if (f["sad_element"], f["change_id"]) in waivers]

    with logic_step(ob, "imp.assemble-report"):
        manifest = wf.tdd_inventory_manifest(inv["tdd_docs"])
        a_delta = {"ref": delta["ref"], "cr_ref": delta["cr_ref"], "changes": changes, "baseline_tdd_snapshot": manifest["snapshot"], "conflicts": delta.get("conflicts") or []}
        sad = _sad_dict(ctx, sad_doc, findings)
        items = []
        for t in scan.output["tdds"]:
            item = deep(t["inventory_item"])
            decisions = judged.get(t["id"], {})
            if any(d["decision"] == "update" for d in decisions.values()):
                item["affected_srs_ids"] = sorted(c for c, d in decisions.items() if d["decision"] == "update")
            items.append((t, item, decisions))
        report = wf.assess_tdd_impact(a_delta, [i for _, i, _ in items], sad, report_id=f"impact-{ctx.run_id}")
        entries = {e.tdd_id: e for e in report.entries}
        for t, item, decisions in items:
            base_kinds = {c["id"]: c.get("kind") for c in changes}
            if any(t["change_kinds"].get(cid, k) != k for cid, k in base_kinds.items() if cid in t["change_kinds"]):
                adj = deep(a_delta)
                for c in adj["changes"]:
                    c["kind"] = t["change_kinds"].get(c["id"], c.get("kind"))
                entries[t["id"]] = next(e for e in wf.assess_tdd_impact(adj, [item], sad, report_id="tmp").entries if e.tdd_id == t["id"])
        for t, item, decisions in items:
            entry = entries[t["id"]]
            if t["verdict"] == "no_overlap" and entry.action == "no_change":
                entry.rationale = t["rule_reason"]
            elif t["verdict"] == "residue" and entry.action == "no_change" and decisions:
                entry.rationale = "; ".join(f"{cid}: {d['reason']}" for cid, d in sorted(decisions.items()))
            elif entry.action == "no_change" and any(k == "parameter_within_bounds" for k in t["change_kinds"].values()):
                entry.rationale = "parameter-only change inside the range declared by the design: " + ", ".join(sorted(cid for cid, k in t["change_kinds"].items() if k == "parameter_within_bounds"))
            if entry.action == "update" and t.get("bound_notes"):
                notes = [f"{c['change']}: {x['unit']} cap {x['before']:g} -> {x['after']:g}" for c in t["bound_notes"] for x in c["caps"]]
                entry.rationale = (entry.rationale + "; " if entry.rationale else "") + "cap loosened beyond the envelope the design was made for (" + ", ".join(notes) + "): capacity and quality obligations must be revalidated"
                entry.evidence = [{"kind": "capacity_obligation", "caps": t["bound_notes"]}] + list(entry.evidence)
            if entry.action == "update":
                # an update is anchored to the document and to the design elements the delta actually reaches, not to every section
                near = {i for ov in t["overlap"] for i in ov["elements"]}
                doc_sections = next(d["sections"] for d in inv["tdd_docs"] if d["id"] == t["id"])
                entry.tdd_refs = [s_ for s_ in doc_sections if s_["section_id"] == "document" or s_["section_id"] in near]
            entry.evidence = [{"kind": "scan", "verdict": t["verdict"], "overlap": t["overlap"], "judgements": decisions, "elements": t["element_count"]}] + list(entry.evidence)
            if entry.action == "update":
                entry.evidence.insert(0, {"kind": "changes", "changes": sorted({c["id"] for c in changes if c["id"] in item["covered_srs_ids"] or c["id"] in item["affected_srs_ids"] or t["id"] in c.get("tdd_ids", [])})})
        for e in report.entries:
            if e.action == "add":
                e.evidence = [{"kind": "changes", "changes": sorted(c["id"] for c in changes if c["new_tdd_id"] == e.tdd_id)}]
        report.entries = [entries.get(e.tdd_id, e) for e in report.entries]
        for f in findings:
            report.findings.append({"code": f["code"], "origin": f["origin"], "sad_element": f["sad_element"], "change_id": f["change_id"], "message": f["message"]})
        errors = report.validate()
    output = {"report": impact_to_dict(report), "srs_delta": delta, "inventory_manifest": manifest, "scan": {t["id"]: t["verdict"] for t in scan.output["tdds"]},
              "delta_uncovered": scan.output["delta_uncovered"], "judgements": judged, "sad_findings": findings, "sad_waived": waived}
    if findings:
        output["sad_cr_proposal"] = {"title": f"SAD change request for {delta['cr_ref']['document_id']}", "reason": "SRS delta conflicts with the read-only SAD",
                                     "conflicts": findings, "next": "Raise a separate CR for the SAD, rebaseline, then re-run impact; or, if the SA judges the finding wrong, `tdd-agent change waive` records that decision with its reason"}
        return GateOutcome.blocked("SAD_CHANGE_REQUIRED", output=output)
    if errors:
        return GateOutcome.blocked(*errors, output=output)
    if report.findings:
        return GateOutcome.blocked(*[f"IMPACT_FINDING:{f.get('code')}" for f in report.findings], output=output)
    decision = report.decision_payload()
    if waived:
        decision = {**decision, "sad_waived": waived}  # what the SA set aside is part of what is approved
    return GateOutcome.ok(output, decision=decision)
