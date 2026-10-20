"""Change-spec agent (G4-G5) and the evidence/acceptance gates (G6-G7)."""
from __future__ import annotations

import json
import re
import shutil
import subprocess
from dataclasses import asdict
from pathlib import Path

from ..core import o5_path
from ..errors import TddAgentError
from ..orchestrator.context import GateContext
from ..orchestrator.model import GateOutcome
from ..skills.lib.workflow_io import impact_from_dict
from ..skills.steps import logic_step
from ..store.base import digest
from .authoring import ProviderAuthoringRunner
from .common import upstream, approval_of, by_role, deep, head_graph, pinned_sources, skills

CHANGE_NAME = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def _p(ctx: GateContext) -> dict:
    return ctx.request.get("params", {})


def _cr(ctx: GateContext) -> dict:
    """The change request as the gates from G4 on see it: its acceptance criteria are those of the CR plus the ones given later (``change update --acceptance``)."""
    cr = dict(upstream(ctx, "G0").get("cr") or _p(ctx).get("cr") or {})
    cr["acceptance"] = list(dict.fromkeys(list(cr.get("acceptance") or []) + list(_p(ctx).get("acceptance_extra") or [])))
    return cr


def cs_element_id(cr_id: str, cs_id: str) -> str:
    return f"{cr_id}/{cs_id}"


def _scenarios_for(graph: dict, change_id: str) -> list[dict]:
    by_id = {e["id"]: e for e in graph["elements"]}
    ids = {e["dst"] for e in graph["edges"] if e["rel"] == "has_scenario" and e["src"] == change_id}
    return [by_id[i] for i in sorted(ids) if i in by_id and by_id[i].get("class") == "Scenario"]


def _name_hints(graph: dict, change_id: str) -> list[str]:
    by_id = {e["id"]: e for e in graph["elements"]}
    near = {e["src"] if e["dst"] == change_id else e["dst"] for e in graph["edges"] if change_id in (e["src"], e["dst"]) and e["rel"] in ("satisfies", "traces_to", "implemented_by")}
    return sorted({by_id[i].get("name", "") for i in near if i in by_id and by_id[i].get("class") in ("Operation", "Component", "Module")} - {""})


REQUIREMENT_CLASSES = ("Requirement", "BusinessRule", "QualityTarget")
MODALITY = (("shall not", "shall_not"), ("must not", "shall_not"), ("shall", "shall"), ("must", "shall"), ("should", "should"), ("may", "may"), ("phải", "shall"))


def _modality(text: str) -> str:
    low = text.lower()
    return next((m for k, m in MODALITY if k in low), "shall")


def _gwt(verification: str) -> dict:
    m = re.match(r"^(?:GIVEN\s+(?P<given>.*?)\s+)?WHEN\s+(?P<when>.*?)\s+THEN\s+(?P<then>.*)$", verification.split("; ")[0].strip(), re.S)
    return {k: (m.group(k) or "") for k in ("given", "when", "then")} if m else {"given": "", "when": "", "then": verification}


def build_spine(ctx: GateContext, graph: dict, delta: dict, table: dict, cr: dict, capability: dict, before_rev: str, after_rev: str, srs_id: str, verifications: dict, migrations: dict,
                satisfied_by: dict | None = None) -> tuple[list[dict], list[dict], list[str]]:
    """Rule-derived KG spine (capability, requirement versions, deltas, scenarios, work, tests) that the stage-5 readiness rules read.

    Everything comes from the sourced delta, the canonical table and explicit inputs; a missing
    input (capability data, removal migration) is a blocker, never a default.
    """
    errors: list[str] = []
    cap_id = capability["id"]
    version = graph.get("version")
    by_id = {e["id"]: e for e in graph["elements"]}
    els: list[dict] = []
    edges: list[dict] = []
    prov = lambda ident: [{"kind": "derived", "source_id": ctx.run_id, "source_version": str(version), "source_hash": digest({"change": ident, "table": [i["id"] for i in table["items"]]}),
                           "block": f"derivation:{ident}", "inputs": [ident], "derivation": "stage-5 spine from the sourced SRS delta and canonical change-spec table"}]
    base = {"origin": "own", "maturity": "sketch", "review_status": "needs_review", "owner_doc": "CS", "tdds": [], "version_introduced": str(version)}
    els.append({**base, "id": cap_id, "class": "Capability", "name": capability.get("name", cap_id), "props": {"name": capability.get("name", cap_id), "stable_key": capability.get("stable_key", cap_id),
                "purpose": capability["purpose"], "scope": capability["scope"], "owner": capability["owner"], "lifecycle": capability.get("lifecycle", "active")}, "provenance": prov(cap_id)})
    rows = {i["srs_refs"][0]["section_id"]: i for i in table["items"]}
    impl_domain = set(o5_path.load().ontology5.legacy_relation_specs()["implemented_by"]["domain"])
    for ch in delta["changes"]:
        cid, kind = ch["id"], ch["kind"]
        row = rows.get(cid)
        src_el = by_id.get(cid)
        requirement_like = bool(src_el) and src_el.get("class") in REQUIREMENT_CLASSES or kind == "removed"
        after_id = f"{cid}@{after_rev}" if kind != "removed" and requirement_like else None
        before_id = f"{cid}@{before_rev}" if kind in ("modified", "removed") and requirement_like else None
        if requirement_like:
            stmt_after = " ".join(str(x) for x in ((src_el or {}).get("props", {}).get("definition") or [ch.get("after_quote", "")]))
            for rid, rev, statement in ((after_id, after_rev, stmt_after), (before_id, before_rev, _row_statement(ch.get("before_quote", "")))):
                if not rid:
                    continue
                els.append({**base, "id": rid, "class": "RequirementVersion", "name": cid, "props": {"logical_requirement_id": cid, "revision": rev, "statement": statement or cid, "modality": _modality(statement or cid),
                            "acceptance": "; ".join(ch.get("cr_acceptance_refs") or cr.get("acceptance", [])), "baseline_id": f"{srs_id}@{rev}", "capability_id": cap_id}, "provenance": prov(cid)})
                edges.append({"src": cap_id, "rel": "has_requirement", "dst": rid})
            rd = {**base, "id": f"{ctx.run_id}/RD-{cid}", "class": "RequirementDelta", "name": cid,
                  "props": {"kind": kind, "rationale": ch.get("rationale", ""), "change_request_id": cr.get("id"), "capability_id": cap_id, "before_requirement_id": before_id, "after_requirement_id": after_id}, "provenance": prov(cid)}
            rd["props"] = {k: v for k, v in rd["props"].items() if v is not None}
            if kind == "removed":
                mig = migrations.get(cid)
                if not mig:
                    errors.append(f"REMOVAL_MIGRATION_REQUIRED:{cid}")
                rd["props"]["migration"] = mig or ""
            els.append(rd)
        if not row:
            continue
        gwt = _gwt(verifications.get(cid) or row["verification"])
        wi, tc = f"{ctx.run_id}/WI-{row['id']}", f"{ctx.run_id}/TC-{row['id']}"
        els.append({**base, "id": wi, "class": "WorkItem", "name": f"{row['action']} {row['code_target']}", "props": {"deliverable": f"{row['action']} {row['code_target']} for {cid}", "definition_of_done": row["verification"], "dependencies": [], "cs_id": row["id"]}, "provenance": prov(cid)})
        els.append({**base, "id": tc, "class": "TestCase", "name": f"Test for {row['id']}", "props": {"cs_id": row["id"], "expected": gwt["then"], "verification": row["verification"]}, "provenance": prov(cid)})
        if after_id:
            sc = f"{ctx.run_id}/SC-{cid}"
            els.append({**base, "id": sc, "class": "Scenario", "name": f"Scenario for {cid}", "props": {"requirement_id": after_id, **gwt}, "provenance": prov(cid)})
            edges += [{"src": after_id, "rel": "has_scenario", "dst": sc}, {"src": after_id, "rel": "verified_by", "dst": tc}]
            if src_el is not None and src_el.get("class") == "Requirement":
                edges.append({"src": cid, "rel": "verified_by", "dst": tc})  # the test checks the requirement as it now stands, not only its new version
            for e in graph["edges"]:
                if e["rel"] == "satisfies" and e["dst"] == cid:
                    edges.append({"src": e["src"], "rel": "satisfies", "dst": after_id})
                    if by_id.get(e["src"], {}).get("class") in impl_domain:
                        edges.append({"src": e["src"], "rel": "implemented_by", "dst": wi})
            given = (satisfied_by or {}).get(cid)   # the SA's statement that an existing design element satisfies the requirement (the SRS rows did not say so)
            if given:
                if given not in by_id:
                    errors.append(f"SATISFIED_BY_UNKNOWN:{cid}:{given}")
                else:
                    edges += [{"src": given, "rel": "satisfies", "dst": after_id}, {"src": given, "rel": "satisfies", "dst": cid}]
                    if by_id[given].get("class") in impl_domain:
                        edges.append({"src": given, "rel": "implemented_by", "dst": wi})
        elif src_el is not None:
            if src_el.get("class") in impl_domain:
                edges.append({"src": cid, "rel": "implemented_by", "dst": wi})
            edges.append({"src": cid, "rel": "verified_by", "dst": tc})
    existing = {e["id"] for e in graph["elements"]}
    return [e for e in els if e["id"] not in existing], edges, errors


def _row_statement(quote: str) -> str:
    cells = [c.strip() for c in quote.strip().strip("|").split("|")]
    return cells[1] if len(cells) > 1 else quote


_ID_LIST = re.compile(r"^(?:[A-Z][A-Z0-9]*-[\w.\-]+(?:[,; ]+|$))+$")


def _row_acceptance(quote: str) -> str:
    """The acceptance an SRS requirement row states (its last cell, when that cell is a sentence and not a list of ids), else nothing."""
    cells = [c.strip() for c in quote.strip().strip("|").split("|")]
    last = cells[-1].replace("\\[", "[").replace("\\]", "]") if len(cells) >= 4 else ""
    return last if len(last) >= 15 and " " in last and not _ID_LIST.match(last) else ""


def cs_table_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    wf = o5.workflow
    p, hub, ob = _p(ctx), skills(ctx), ctx.observer
    g3 = upstream(ctx, "G3")
    if not g3.get("tdd_set") or not g3.get("report"):
        return GateOutcome.blocked("TDD_REALIZATION_REQUIRED")
    docs = pinned_sources(ctx)
    sad_doc = by_role(docs, "SAD")
    delta = deep(g3["srs_delta"])
    cr = _cr(ctx)
    from . import proposals
    propose = ctx.config.proposes_missing()   # nobody to ask (automatic mode): what the CR left out is proposed, marked as proposed, and listed for review
    proposed: dict = {"acceptance": [], "satisfied_by": {}, "code_targets": {}}
    cr_doc = next((d for d in docs if d.role == "CR"), None)
    statements = [{"id": c["id"], "kind": c["kind"], "statement": _row_statement(c.get("after_quote") or c.get("before_quote") or "")} for c in delta["changes"] if c["kind"] != "removed"]
    proposed_gwt: dict[str, list[str]] = {}
    if not cr.get("acceptance"):
        if not (propose and cr_doc and statements):
            return GateOutcome.blocked("CR_ACCEPTANCE_MISSING")  # known at the start, but only needed here: what the rows are verified against
        made, blockers = proposals.acceptance(ctx, cr_doc.text, statements)
        if blockers or not made:
            return GateOutcome.blocked("CR_ACCEPTANCE_MISSING", *blockers)
        proposed["acceptance"] = made
        cr["acceptance"] = [c["id"] for c in made]
        for c in made:
            proposed_gwt.setdefault(c["change_id"], []).append(proposals.gwt(c))
    delta["acceptance"] = list(cr["acceptance"])
    for c in delta["changes"]:
        c["cr_acceptance_refs"] = list(dict.fromkeys(list(c.get("cr_acceptance_refs") or []) + delta["acceptance"]))
    graph = ctx.knowledge.materialize(g3["snapshot"])
    code_targets, verifications, gaps = dict(p.get("code_targets", {})), {}, []
    repo_root = p.get("repo_root") or str(ctx.config.repo_root)
    from_cr = (upstream(ctx, "G0").get("cr_proposal") or {}).get("verifications", {})  # GIVEN/WHEN/THEN criteria of the CR (change requests without an edited SRS)
    for change in delta["changes"]:
        cid = change["id"]
        with logic_step(ob, "cs.verification"):
            v = hub.run("derive-verification", {"change_id": cid, "scenarios": _scenarios_for(graph, cid), "acceptance": delta.get("acceptance", [])})
        gaps.extend(v.gaps)
        if v.output.get("verification"):
            verifications[cid] = v.output["verification"]
        elif change.get("verification") or p.get("verifications", {}).get(cid) or from_cr.get(cid):
            verifications[cid] = change.get("verification") or p.get("verifications", {}).get(cid) or from_cr[cid]
        elif proposed_gwt.get(cid):
            verifications[cid] = "; ".join(proposed_gwt[cid])
        if cid not in code_targets:
            r = hub.run("select-code-target", {"change": change, "repo_root": repo_root, "name_hints": _name_hints(graph, cid)})
            if not r.ok:
                return GateOutcome.blocked(*r.blockers)
            gaps.extend(r.gaps)
            if r.output.get("code_target"):
                code_targets[cid] = r.output["code_target"]
    by_id = {e["id"]: e for e in graph["elements"]}
    given_satisfiers = dict(p.get("satisfied_by") or {})
    if propose:
        missing_v = [s_ for s_ in statements if s_["id"] not in verifications]
        if missing_v and cr_doc:   # an acceptance criterion exists but not for every changed requirement
            made, blockers = proposals.acceptance(ctx, cr_doc.text, missing_v)
            for c in made:
                verifications.setdefault(c["change_id"], proposals.gwt(c))
                proposed["acceptance"].append(c)
        has_satisfier = {e["dst"] for e in graph["edges"] if e["rel"] == "satisfies"}
        for st in statements:
            cid = st["id"]
            if cid in given_satisfiers or cid in has_satisfier or by_id.get(cid, {}).get("class") not in REQUIREMENT_CLASSES:
                continue
            chosen = proposals.satisfier(ctx, graph, st)
            if chosen:
                proposed["satisfied_by"][cid] = chosen
        for st in statements:
            cid = st["id"]
            if cid not in code_targets:
                holder = given_satisfiers.get(cid) or (proposed["satisfied_by"].get(cid) or {}).get("id")
                if holder in by_id:
                    code_targets[cid] = proposals.place_of_change(by_id[holder])
                    proposed["code_targets"][cid] = code_targets[cid]
    satisfied_by = {**{k: v["id"] for k, v in proposed["satisfied_by"].items()}, **given_satisfiers}
    sad = {"ref": {"document_id": sad_doc.id, "role": "SAD", "revision": sad_doc.revision, "content_hash": sad_doc.content_hash, "section_id": "document"}, "architecture_change": False}
    with logic_step(ob, "cs.build-table"):
        built = hub.run("build-cs-table", {"srs_delta": delta, "tdd_set": g3["tdd_set"], "sad": sad, "impact": g3["report"], "cr": {"id": cr.get("id"), "acceptance": cr.get("acceptance", [])},
                                           "code_targets": code_targets, "verifications": verifications})
    if not built.ok:
        return GateOutcome.blocked(*built.blockers, output={"gaps": gaps + built.gaps})
    table = built.output
    gaps = gaps + built.gaps
    with logic_step(ob, "cs.migration-ownership"):
        # migration/rollback/ownership come from the TDD sections the rows cite; absent => gap, never invented
        for item in table["items"]:
            sections = {r.get("section_id") for r in item["tdd_refs"]}
            facts = [e for e in graph["elements"] if e["id"] in sections and e.get("class") in ("Migration", "Operation")]
            item["migration"] = [{"id": e["id"], "rollback": (e.get("props") or {}).get("rollback")} for e in facts if e.get("class") == "Migration"]
    out = {"change_spec": table, "gaps": gaps, "proposed": proposed, "srs_delta_cs": {**delta, "changes": [{**c, **({"code_target": code_targets[c["id"]]} if c["id"] in code_targets else {}),
                                                                                        **({"verification": verifications[c["id"]]} if c["id"] in verifications else {})} for c in delta["changes"]]}}
    if table["status"] != "eligible":
        return GateOutcome.blocked(*table["errors"], output=out)
    # persist the canonical rows and the acceptance receipt so coverage queries (Q8) can run on the KG
    head, base = head_graph(ctx.knowledge)
    new = deep(base)
    new["version"] = f"{ctx.run_id}-cs-v{head.seq + 1}"
    cr_id = cr.get("id", "CR")
    stale = {i for i in [e["id"] for e in new["elements"]] if i.startswith(f"{cr_id}/") or i.startswith(f"{ctx.run_id}/")}
    new["elements"] = [e for e in new["elements"] if e["id"] not in stale]  # re-running G4 replaces this run's own records
    new["edges"] = [e for e in new["edges"] if e["src"] not in stale and e["dst"] not in stale]
    for item in table["items"]:
        new["elements"].append({"id": cs_element_id(cr_id, item["id"]), "class": "ChangeSpecItem", "name": f"{item['id']} {item['code_target']}", "origin": "own", "maturity": "sketch",
                                "review_status": "needs_review", "owner_doc": "CS", "tdds": [], "version_introduced": new["version"],
                                "props": {"action": item["action"], "code_target": item["code_target"], "srs_refs": item["srs_refs"], "tdd_refs": item["tdd_refs"],
                                          "verification": item["verification"], "cr_acceptance_refs": item["cr_acceptance_refs"], "table_id": item["id"], "cr_id": cr_id, "run_id": ctx.run_id},
                                "provenance": [{"kind": "derived", "source_id": ctx.run_id, "source_version": new["version"], "source_hash": digest(item), "block": f"derivation:{item['id']}",
                                                "inputs": [r.get("section_id") for r in item["srs_refs"] + item["tdd_refs"]], "derivation": "build_change_spec"}]})
    capability = {**(p.get("capability") or {}), "id": p.get("change_name") or ""}  # o5 projects deltas into specs/<change_name>/spec.md, so the capability is the change name
    missing_cap = [k for k in ("id", "purpose", "scope", "owner") if not capability.get(k)]
    if missing_cap:
        return GateOutcome.blocked(*[f"CAPABILITY_FIELD_REQUIRED:{k}" for k in missing_cap], output=out)
    srs = (upstream(ctx, "G0") or {}).get("srs") or p.get("srs") or {}
    spine_els, spine_edges, spine_errors = build_spine(ctx, new, delta, table, cr, capability, str(srs.get("before")), str(srs.get("after")), srs.get("id", "SRS"), verifications, p.get("removal_migrations", {}), satisfied_by)
    if spine_errors:
        return GateOutcome.blocked(*spine_errors, output=out)
    have = {e["id"] for e in new["elements"]}
    new["elements"] = [e for e in new["elements"] if e["id"] not in {x["id"] for x in spine_els}] + spine_els
    for e in spine_edges:
        if e not in new["edges"]:
            new["edges"].append(e)
    out["spine"] = {"elements": [e["id"] for e in spine_els], "capability": capability["id"]}
    records, record_edges = workflow_records(ctx, new, cr, srs, delta, g3, table)
    new["elements"] += records
    new["edges"] += [e for e in record_edges if e not in new["edges"]]
    criteria = sorted({c for item in table["items"] for c in item["cr_acceptance_refs"]})
    new["elements"].append({"id": f"{cr_id}/acceptance", "class": "AcceptanceReceipt", "name": f"Acceptance of {cr_id}", "origin": "own", "maturity": "sketch", "review_status": "needs_review",
                            "owner_doc": "CS", "tdds": [], "version_introduced": new["version"],
                            "props": {"cr_id": cr_id, "criterion_refs": criteria, "accepted_by": "", "status": "pending"},
                            "provenance": [{"kind": "derived", "source_id": ctx.run_id, "source_version": new["version"], "source_hash": digest(criteria), "block": "derivation:acceptance",
                                            "inputs": criteria, "derivation": "CR acceptance criteria"}]})
    commit = ctx.commit_graph(new, base_snapshot=head.id if head else None, meta={"flow": "change-spec-table", "run": ctx.run_id})
    out.update(snapshot=commit.snapshot.id, kg_version=new["version"], cs_elements=[cs_element_id(cr_id, i["id"]) for i in table["items"]])
    shared = [r for r in ctx.queries().q9_shared_contract_conflicts().rows if cr_id in r["change_requests"]]
    if shared:
        out["shared_contract_conflicts"] = shared
        return GateOutcome.blocked(*[f"SHARED_CONTRACT_CONFLICT:{r['interface']}:{'+'.join(c for c in r['change_requests'] if c != cr_id)}" for r in shared], output=out)
    return GateOutcome.ok(out)


def workflow_records(ctx: GateContext, graph: dict, cr: dict, srs: dict, delta: dict, g3: dict, table: dict) -> tuple[list[dict], list[dict]]:
    """KG records of the settled workflow (CR -> SRS delta -> impact -> TDD set -> CS rows) so Q8/Q9 can query real data."""
    cr_id = cr.get("id", "CR")
    version = graph["version"]
    base = {"origin": "own", "maturity": "sketch", "review_status": "needs_review", "owner_doc": "CS", "tdds": [], "version_introduced": version}
    prov = lambda tag, payload: [{"kind": "derived", "source_id": ctx.run_id, "source_version": version, "source_hash": digest(payload), "block": f"derivation:{tag}", "inputs": [tag],
                                   "derivation": "workflow record of the settled document workflow"}]
    report, tdd_set = g3["report"], g3["tdd_set"]
    ids = {"cr": cr_id, "delta": f"{cr_id}/delta", "impact": f"{cr_id}/impact", "set": f"{cr_id}/tdd-set"}
    existing = {e["id"] for e in graph["elements"]}
    els = []
    if cr_id not in existing:
        els.append({**base, "id": cr_id, "class": "ChangeRequest", "name": cr.get("title") or cr_id, "props": {"status": "open", "acceptance": cr.get("acceptance", []), "revision": str(cr.get("revision", "1"))}, "provenance": prov(cr_id, cr)})
    els += [
        {**base, "id": ids["delta"], "class": "SrsDelta", "name": f"SRS delta of {cr_id}", "props": {"cr_id": cr_id, "before_revision": str(srs.get("before")), "after_revision": str(srs.get("after")), "sections": [c["id"] for c in delta["changes"]]}, "provenance": prov("delta", delta["changes"])},
        {**base, "id": ids["impact"], "class": "ImpactReport", "name": f"Impact of {cr_id}", "props": {k: report[k] for k in ("cr_ref", "srs_ref", "sad_ref", "baseline_tdd_snapshot", "entries", "findings", "status", "approval") if k in report}, "provenance": prov("impact", report["entries"])},
        {**base, "id": ids["set"], "class": "TddSetRealization", "name": f"TDD set of {cr_id}", "props": {k: tdd_set[k] for k in ("baseline_snapshot", "actual_snapshot", "mappings", "status")}, "provenance": prov("tdd-set", tdd_set["mappings"])},
    ]
    edges = [{"src": cr_id, "rel": "has_srs_delta", "dst": ids["delta"]}, {"src": ids["delta"], "rel": "has_impact_report", "dst": ids["impact"]},
             {"src": ids["impact"], "rel": "realizes_tdd_set", "dst": ids["set"]}]
    by_id = {e["id"] for e in graph["elements"]}
    for item in table["items"]:
        cs = cs_element_id(cr_id, item["id"])
        edges.append({"src": ids["set"], "rel": "has_change_spec_item", "dst": cs})
        for ref in item["tdd_refs"]:
            sid = ref.get("section_id")
            if sid in by_id and sid != "document":
                edges.append({"src": cs, "rel": "traces_to", "dst": sid})
    return els, edges


def _workflow_context(ctx: GateContext, g4: dict, g3: dict) -> dict:
    docs = pinned_sources(ctx)
    return {"cr": _cr(ctx), "srs_delta": g4["srs_delta_cs"], "sad": {"ref": {"document_id": by_role(docs, "SAD").id, "role": "SAD", "revision": by_role(docs, "SAD").revision,
                                                                                    "content_hash": by_role(docs, "SAD").content_hash, "section_id": "document"}, "architecture_change": False},
            "impact": g3["report"], "tdd_set": g3["tdd_set"]}


def staging_dir(ctx: GateContext) -> Path:
    return ctx.config.work_dir / "staging" / ctx.run_id / "changes"


def _same_modulo_review(a: dict, b: dict) -> bool:
    """True when two snapshots differ only in review bookkeeping (status/history), i.e. not in any fact."""
    def norm(g):
        return ([{k: v for k, v in e.items() if k not in ("review_status", "change_history", "provenance", "reviews")} for e in g["elements"]], g["edges"])
    ea, eb = norm(a), norm(b)
    return ea == eb


def authoring_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    from spec_change import SpecChangeRequest, publish
    p = _p(ctx)
    g3, g4 = upstream(ctx, "G3"), upstream(ctx, "G4")
    name = p.get("change_name") or ""
    if not CHANGE_NAME.match(name):
        return GateOutcome.blocked("CHANGE_NAME_INVALID")
    head, graph = head_graph(ctx.knowledge)
    if head is None or (head.id != g4.get("snapshot") and not _same_modulo_review(ctx.knowledge.materialize(g4["snapshot"]), graph)):
        return GateOutcome.blocked("KG_SNAPSHOT_CHANGED")
    wc = _workflow_context(ctx, g4, g3)
    # o5 expects the impact as an *approved* typed report; rebuild it from the recorded approval
    approval = approval_of(ctx, "G2")
    from ..skills.lib.workflow_io import finalize_impact_approval, impact_to_dict
    report, errors = finalize_impact_approval(impact_from_dict(g3["report"]), approval)
    if errors:
        return GateOutcome.blocked(*errors)
    wc["impact"] = impact_to_dict(report)
    allow_draft = bool(p.get("allow_draft"))  # the requester asked for the staged bundle even though the spec is not eligible (inspection only; see below)
    request = SpecChangeRequest(id=ctx.run_id, change_name=name, intent=(wc["cr"].get("title") or wc["cr"].get("id", name)), scope=sorted({c["id"] for c in wc["srs_delta"]["changes"]} | {f"{ctx.run_id}/RD-{c['id']}" for c in wc["srs_delta"]["changes"]}),
                                kg_snapshot=graph["version"], repo_root=p.get("repo_root") or str(ctx.config.repo_root), allow_draft=allow_draft, workflow_context=wc,
                                code_paths=p.get("code_paths"), context_budget=p.get("context_budget"), run_root=str(ctx.config.work_dir / "runs" / ctx.run_id / "spec"))
    touched = {c["id"]: _row_statement(c.get("before_quote", "")) for c in wc["srs_delta"]["changes"] if c["kind"] in ("modified", "removed")}
    if touched and p.get("baseline_from_srs"):
        from .baseline import ensure_baseline
        made = ensure_baseline(p.get("repo_root") or str(ctx.config.repo_root), name, touched,
                               acceptance={c["id"]: _row_acceptance(c.get("before_quote", "")) for c in wc["srs_delta"]["changes"] if c["kind"] in ("modified", "removed")})
        if made["written"]:
            ctx.observer.event("baseline_written", capability=name, requirements=made["written"])
        if made["unwritable"]:
            return GateOutcome.blocked(*[f"BASELINE_UNWRITABLE:{u['id']}" for u in made["unwritable"]], output={"baseline": made})
    staging = staging_dir(ctx)
    if staging.parent.exists():  # a previous attempt may have left a draft here (it is written beside, not inside, "changes")
        shutil.rmtree(staging.parent)
    attempts: list[dict] = []
    runner = ProviderAuthoringRunner(ctx, language=str(ctx.config.get("openspec.language") or ""))
    for revision in range(int(ctx.config.get("openspec.max_revisions") or 0) + 1):
        if revision and staging.parent.exists():
            shutil.rmtree(staging.parent)
        with ctx.observer.span("spec-publish-staging", "step", revision=revision):
            result = publish(request, graph, staging, runner=runner)
        found = _author_feedback(result)
        attempts.append({"revision": revision, "status": result.get("status"), "findings": len(found)})
        if result.get("status") in ("published",) or (result.get("status") == "draft" and allow_draft) or not found:
            break
        runner.feedback = found  # the next draft is written knowing what was wrong with this one
    result["authoring_attempts"] = attempts
    if result.get("status") == "draft" and allow_draft:
        # A draft is written to openspec/drafts/<name> for inspection.  It is never approvable: the gate stays blocked, so nothing is copied to
        # openspec/changes and G6/G7 cannot start from it.  The blockers are the reasons it is a draft.
        path = Path(result["path"])
        manifest = result.get("manifest") or {}
        blockers = list(manifest.get("blockers") or (result.get("readiness") or {}).get("blockers") or ["SPEC_DRAFT"])
        return GateOutcome.blocked("SPEC_DRAFT_ONLY", *blockers, output={"draft": True, "draft_path": str(path), "files": sorted(str(f.relative_to(path)) for f in path.rglob("*") if f.is_file()),
                                                                          "draft_blockers": blockers, "manifest": manifest.get("status"), "review": manifest.get("semantic_review"), "cli_validation": result.get("cli_validation"),
                                                                          "validation": result.get("validation")})
    if result.get("status") != "published":
        blockers = list(result.get("blockers") or (result.get("readiness") or {}).get("blockers") or [f"SPEC_{str(result.get('status')).upper()}"])
        coverage = (result.get("readiness") or {}).get("rule_coverage") or {}
        failing = {rid: rec.get("evidence") for rid, rec in coverage.items() if isinstance(rec, dict) and rec.get("outcome") != "pass"}
        return GateOutcome.blocked(*blockers, output={"publish": {k: v for k, v in result.items() if k in ("status", "reason", "run_dir")}, "validation": result.get("validation"),
                                                      "manifest": (result.get("manifest") or {}).get("status"), "failing_rules": failing, "authoring_attempts": result["authoring_attempts"],
                                                      "review": (result.get("manifest") or {}).get("semantic_review"), "cli_validation": result.get("cli_validation")})
    path = Path(result["path"])
    files = {str(f.relative_to(path)): f.read_text(encoding="utf-8") for f in sorted(path.rglob("*")) if f.is_file()}
    manifest = json.loads(files["trace-manifest.json"])
    decision = {"change_name": name, "artifact_hashes": {k: digest(v) for k, v in files.items()}, "trace_manifest": {k: manifest.get(k) for k in ("status", "blockers", "kg_hash", "repo_hash", "semantic_review", "cli_validation")},
                "assertions": len(manifest.get("assertions", []))}
    return GateOutcome.ok({"staged_path": str(path), "change_name": name, "files": sorted(files), "readiness": result["readiness"]["status"], "decision_summary": decision}, decision=decision)


def _author_feedback(result: dict) -> list[str]:
    """What a rejected draft was rejected for, as lines the author can act on (the reviewer's findings, the validation errors)."""
    manifest = result.get("manifest") or {}
    review = manifest.get("semantic_review") or {}
    found = [(f"{f.get('artifact')} {f.get('anchor')}: {f.get('reason') or f.get('message')}" if isinstance(f, dict) else str(f)) for f in (review.get("findings") or [])]
    validation = result.get("validation") or {}
    found += [str(e) for e in (validation.get("errors") or [])][:10]
    return found[:20]


def publish_on_approved(ctx: GateContext, task: dict) -> None:
    """After the SA approves: copy the staged change into the repository and re-validate strictly there."""
    p = _p(ctx)
    out = (ctx.operational.get_task(ctx.run_id, "G5") or {}).get("output") or task.get("output") or {}
    staged, name = Path(out["staged_path"]), out["change_name"]
    repo = Path(p.get("repo_root") or ctx.config.repo_root)
    target = repo / "openspec" / "changes" / name
    if target.exists():
        raise TddAgentError(f"{target} already exists", code="PUBLICATION_NAME_COLLISION")
    shutil.copytree(staged, target)
    manifest_path = target / "trace-manifest.json"
    if manifest_path.is_file():  # who approved what, so a reader of the published bundle can tell an approval of a person from the automatic one
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        manifest["approvals"] = [{k: a.get(k) for k in ("gate", "actor_id", "actor_kind", "role", "approval_grade", "decision_hash", "ts")} for a in ctx.operational.list_approvals(ctx.run_id)]
        manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False), encoding="utf-8")
    check = subprocess.run(["openspec", "validate", name, "--type", "change", "--strict", "--no-interactive"], cwd=repo, capture_output=True, text=True, timeout=120)
    if check.returncode != 0:
        shutil.rmtree(target, ignore_errors=True)
        raise TddAgentError("openspec validate --strict failed after copying the approved change", code="PUBLICATION_VALIDATION_FAILED", details={"output": (check.stdout + check.stderr)[-600:]})
    ctx.observer.event("change_published", change=name, path=str(target))


def _receipts(graph: dict, cr_id: str) -> tuple[list[dict], list[dict]]:
    items = [e for e in graph["elements"] if e.get("class") == "ChangeSpecItem" and (e.get("props") or {}).get("cr_id") == cr_id]
    ids = {e["id"] for e in items}
    return items, [e for e in graph["elements"] if e.get("class") == "TestReceipt" and (e.get("props") or {}).get("cs_id") in ids]


def evidence_gate(ctx: GateContext) -> GateOutcome:
    """G6: every change-spec row must be covered by a passing TestReceipt naming its acceptance criteria."""
    o5 = o5_path.load()
    cr = _cr(ctx)
    cr_id = cr.get("id", "CR")
    graph = head_graph(ctx.knowledge)[1]
    items, receipts = _receipts(graph, cr_id)
    if not items:
        return GateOutcome.blocked("CHANGE_SPEC_ROWS_MISSING")
    table = {"errors": [], "items": [{"id": (i["props"]["table_id"]), **{k: i["props"][k] for k in ("srs_refs", "tdd_refs", "cr_acceptance_refs", "verification")}} for i in items]}
    cs_to_table = {i["id"]: i["props"]["table_id"] for i in items}
    mapped = [{"cs_id": cs_to_table[r["props"]["cs_id"]], "result": r["props"].get("result"), "executed_at": r["props"].get("executed_at"), "cs_version": r["props"].get("cs_version"),
               "cr_version": r["props"].get("cr_version"), "criterion_refs": r["props"].get("criterion_refs", [])} for r in receipts]
    coverage = o5.workflow.validate_change_spec_coverage(table, mapped)
    q8 = ctx.queries().q8_acceptance_coverage(cr_id)
    uncovered = [r["criterion"] for r in q8.rows if r["status"] != "covered"]
    blockers = list(coverage["errors"]) + [f"CRITERION_{r['status'].upper()}:{r['criterion']}" for r in q8.rows if r["status"] != "covered"]
    out = {"coverage": coverage, "criteria": q8.rows, "receipts": [r["id"] for r in receipts]}
    return GateOutcome.pending_evidence(*blockers, output=out) if blockers else GateOutcome.ok(out)


def acceptance_gate(ctx: GateContext) -> GateOutcome:
    cr = _cr(ctx)
    cr_id = cr.get("id", "CR")
    g6 = upstream(ctx, "G6")
    q8 = ctx.queries().q8_acceptance_coverage(cr_id)
    if any(r["status"] != "covered" for r in q8.rows):
        return GateOutcome.blocked("ACCEPTANCE_COVERAGE_INCOMPLETE")
    shared = [r for r in ctx.queries().q9_shared_contract_conflicts().rows if cr_id in r["change_requests"]]
    if shared:
        return GateOutcome.blocked(*[f"SHARED_CONTRACT_CONFLICT:{r['interface']}" for r in shared], output={"shared_contract_conflicts": shared})
    decision = {"cr": cr_id, "criteria": [{"criterion": r["criterion"], "cs_rows": r["cs_rows"], "tests": r["passing_tests"]} for r in q8.rows],
                "stale_refs": [r["ref"] for r in ctx.queries().q6_stale_refs().rows]}
    if decision["stale_refs"]:
        return GateOutcome.blocked(*[f"STALE_REF:{r}" for r in decision["stale_refs"]], output=decision)
    return GateOutcome.ok({"criteria": decision["criteria"], "coverage": g6.get("coverage")}, decision=decision)


def acceptance_on_approved(ctx: GateContext, task: dict) -> None:
    """Record who accepted: the receipt carries the approval grade (``accepted(agent)`` when an agent approved)."""
    approval = ctx.upstream.get("_approval") or task.get("approval") or {}
    cr_id = _cr(ctx).get("id", "CR")
    head, graph = head_graph(ctx.knowledge)
    rid = f"{cr_id}/acceptance"
    el = next((e for e in graph["elements"] if e["id"] == rid), None)
    if el is None:
        raise TddAgentError("acceptance receipt element is missing", code="ACCEPTANCE_RECEIPT_MISSING")
    status = "accepted" if approval.get("approval_grade") == "human" else "accepted(agent)"
    cs = {"id": f"{ctx.run_id}-accept", "base_version": graph["version"], "result_version": f"{graph['version']}+accepted",
          "ops": [{"seq": 1, "op": "modify", "target": rid, "path": "props.status", "before": el["props"]["status"], "after": status},
                  {"seq": 2, "op": "modify", "target": rid, "path": "props.accepted_by", "before": el["props"].get("accepted_by"), "after": approval.get("actor_id")}]}
    ctx.commit_changeset(cs, base_snapshot=head.id)
