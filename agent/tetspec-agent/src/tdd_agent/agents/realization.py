"""G3: materialise the approved impact plan into actual TDD revisions.

``no_change`` TDDs are reused with proof; ``update``/``add`` TDDs receive the SRS
delta as sourced facts plus open design obligations, are re-rendered and
registered as new revisions.  Design that the sources do not state stays an open
obligation or gap - this step never writes design on its own.
"""
from __future__ import annotations

from ..core import o5_path
from ..orchestrator.context import GateContext
from ..orchestrator.model import GateOutcome
from ..skills.lib.workflow_io import finalize_impact_approval, impact_from_dict, impact_to_dict
from ..skills.steps import logic_step
from ..store.base import digest
from .common import upstream, approval_of, by_role, deep, head_graph, latest_tdd_docs, pinned_sources, render_document, tdd_members, tdd_section_refs
from .generation import Extraction, extract_documents, merge_into_graph, prune_invalid


def _changes_of(entry) -> list[str]:
    for ev in entry.evidence:
        if isinstance(ev, dict) and ev.get("kind") == "changes":
            return list(ev.get("changes", []))
    return []


def realization_gate(ctx: GateContext) -> GateOutcome:
    o5 = o5_path.load()
    wf = o5.workflow
    ob = ctx.observer
    g2 = upstream(ctx, "G2")
    if not g2.get("report") or not approval_of(ctx, "G2"):
        return GateOutcome.blocked("IMPACT_APPROVAL_REQUIRED")
    with logic_step(ob, "orc.gates-approvals-ledger"):
        report, errors = finalize_impact_approval(impact_from_dict(g2["report"]), approval_of(ctx, "G2"))
    if errors:
        return GateOutcome.blocked(*errors)
    delta = upstream(ctx, "G1").get("srs_delta") or g2["srs_delta"]
    intake = upstream(ctx, "G0")
    docs = pinned_sources(ctx)
    srs = intake.get("srs") or {}
    after = next((d for d in docs if d.id == srs.get("id") and d.revision == str(srs.get("after"))), None)
    if after is None:
        return GateOutcome.blocked("SRS_REVISION_NOT_PINNED:after")
    head, graph = head_graph(ctx.knowledge)
    params = ctx.request.get("params", {})
    current = {d.id: d for d in latest_tdd_docs(ctx.knowledge)}
    version = f"{ctx.run_id}-v{(head.seq + 1) if head else 1}"
    changes_by_id = {c["id"]: c for c in delta["changes"]}
    mappings, documents = [], []
    touched_by_tdd: dict[str, list[str]] = {}

    # ---- 1. apply the sourced delta to the KG (one merge, one commit)
    update_ids: dict[str, set[str]] = {}
    for entry in report.entries:
        if entry.action in ("update", "add"):
            update_ids[entry.tdd_id] = set(_changes_of(entry)) & set(changes_by_id)
    wanted = set().union(*update_ids.values()) if update_ids else set()
    extraction = extract_documents(ctx, [after], params, only_ids={c for c in wanted if changes_by_id[c].get("kind") != "removed"} or set(), prose=False) if wanted else Extraction()
    if extraction.blockers:
        return GateOutcome.blocked(*dict.fromkeys(extraction.blockers))
    for el in extraction.elements:
        owners = sorted(t for t, ids in update_ids.items() if el["id"] in ids)
        el["tdds"] = sorted(set(el.get("tdds") or []) | set(owners))
        touched_by_tdd.update({t: touched_by_tdd.get(t, []) + [el["id"]] for t in owners})
    merged, merge_report = merge_into_graph(graph, extraction, version)
    for cid, ch in changes_by_id.items():
        if ch.get("kind") == "removed" and cid in wanted:
            el = next((e for e in merged["elements"] if e["id"] == cid), None)
            if el is not None:
                el.setdefault("props", {}).update(status="removed", removed_in=after.revision)
                for t in [t for t, ids in update_ids.items() if cid in ids]:
                    touched_by_tdd.setdefault(t, []).append(cid)
    for ob_ in wf.derive_obligations(delta, report, "core-tdd"):
        owners = sorted(t for t, ids in update_ids.items() if ob_.id[4:] in ids)
        if not owners:
            continue
        el = {"id": f"{ctx.run_id}/{ob_.id}", "class": "DesignObligation", "name": ob_.question[:80], "origin": "own", "maturity": "sketch", "review_status": "needs_review",
              "owner_doc": owners[0], "tdds": owners, "version_introduced": version,
              "props": {"question": ob_.question, "srs_refs": ob_.srs_refs, "cr_acceptance_refs": ob_.cr_acceptance_refs, "applicability": ob_.applicability, "status": ob_.status, "tdd_id": owners[0]},
              "provenance": [{"kind": "derived", "source_id": ctx.run_id, "source_version": version, "source_hash": digest(ob_.question), "block": f"derivation:{ob_.id}",
                                                "inputs": [ob_.id], "derivation": "derive_obligations(srs_delta, impact)"}]}
        if not any(e["id"] == el["id"] for e in merged["elements"]):
            merged["elements"].append(el)
        for t in owners:
            touched_by_tdd.setdefault(t, []).append(el["id"])
    merged = prune_invalid(merged, extraction)
    # the elements this change brings in are checked like a generated TDD: a contract with an external system needs its failure policy, and so on
    from .generation import _merge_gaps, profile_gap_dicts
    merged["gaps"] = _merge_gaps(merged.get("gaps", []), profile_gap_dicts("spec-change-generation", merged, only={e["id"] for e in extraction.elements}))
    commit = ctx.commit_graph(merged, base_snapshot=head.id if head else None, meta={"flow": "realization", "run": ctx.run_id})
    committed = ctx.knowledge.materialize(commit.snapshot.id)

    # ---- 2. map every impact entry to actual TDD sections
    for entry in sorted(report.entries, key=lambda e: e.tdd_id):
        if entry.action == "no_change":
            doc = current.get(entry.tdd_id)
            if doc is None:
                return GateOutcome.blocked(f"TDD_BASELINE_MISSING:{entry.tdd_id}")
            mappings.append({"tdd_id": entry.tdd_id, "action": "no_change", "before_revision": doc.revision, "after_revision": doc.revision,
                             "actual_sections": list(entry.tdd_refs), "design_reuse_proof": {"rationale": entry.rationale, "evidence": entry.evidence, "srs_refs": entry.srs_refs}})
            continue
        before = current.get(entry.tdd_id)
        with logic_step(ob, "gen.render"):
            text, _ = render_document(ctx, committed, entry.tdd_id, snapshot=commit.snapshot)
        new_doc = ctx.register_generated_tdd(entry.tdd_id, text, {"snapshot": commit.snapshot.id, "run": ctx.run_id})
        documents.append({"id": new_doc.id, "revision": new_doc.revision, "content_hash": new_doc.content_hash})
        out = ctx.config.work_dir / "runs" / ctx.run_id
        out.mkdir(parents=True, exist_ok=True)
        (out / f"{entry.tdd_id}.r{new_doc.revision}.md").write_text(text, encoding="utf-8")
        members, _ = tdd_members(committed, entry.tdd_id)
        in_doc = {m["id"] for m in members}
        planned = {r.get("section_id") for r in entry.tdd_refs if r.get("section_id") != "document" and r.get("section_id") in in_doc}
        sections = tdd_section_refs(new_doc, sorted(planned | set(touched_by_tdd.get(entry.tdd_id, []))))
        sections.insert(0, {"document_id": new_doc.id, "role": "TDD", "revision": new_doc.revision, "content_hash": new_doc.content_hash, "section_id": "document"})
        mappings.append({"tdd_id": entry.tdd_id, "action": entry.action, "before_revision": before.revision if before else None, "after_revision": new_doc.revision,
                         "actual_sections": sections})
    try:
        realization = wf.realize_tdd_set(report, mappings, commit.snapshot.id)
    except wf.WorkflowError as exc:
        return GateOutcome.blocked(*[x for x in str(exc).split(";") if x])
    from dataclasses import asdict
    return GateOutcome.ok({"tdd_set": asdict(realization), "report": impact_to_dict(report), "snapshot": commit.snapshot.id, "snapshot_hash": commit.snapshot.hash,
                           "documents": documents, "merge": merge_report, "srs_delta": delta})
