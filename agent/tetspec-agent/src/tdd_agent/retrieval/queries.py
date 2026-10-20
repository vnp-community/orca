"""Q1-Q9: typed queries over a ``KnowledgeStore`` snapshot.

The design document's pseudo-Cypher is documentation; these functions are the
contract.  Every result is a ``QueryResult`` of one of four kinds: ``input``
(feeds an agent), ``valid`` (confirms validity), ``gap`` (something is missing)
or ``drift`` (something diverged and needs a rebase).  A query that depends on a
relation which the ontology registry does not declare fails with
``RelationNotRegistered`` instead of returning an empty (false-clean) answer.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Iterable

from ..errors import NotFound, RelationNotRegistered, SourceRoleError
from ..store.base import KnowledgeStore
from .closure import provenance_of
from .policy import registered_relations

DESIGN_CLASSES = ("DesignUnit", "Operation", "Interface", "Entity", "Flow", "QualityTarget")
UNRESOLVED = {"open", "unresolved", "unknown", "conflict", "proposed", "draft"}
RESOLVED = {"satisfied", "verified", "accepted", "resolved", "not_applicable"}
# An element two open change requests may not both change: an interface contract or the requirement that states it.
SHARED_CONTRACT_CLASSES = ("Interface", "Operation", "Event", "Requirement")


@dataclass
class QueryResult:
    name: str
    kind: str
    rows: list[dict]
    snapshot: str
    meta: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {"name": self.name, "kind": self.kind, "snapshot": self.snapshot, "count": len(self.rows), "rows": self.rows, "meta": self.meta}


def props(el: dict) -> dict:
    merged = dict(el.get("props") or {})
    for k, v in el.items():
        if k not in ("props",) and k not in merged:
            merged[k] = v
    return merged


def _as_list(value: Any) -> list:
    if value is None:
        return []
    return list(value) if isinstance(value, (list, tuple, set)) else [value]


class QueryService:
    def __init__(self, store: KnowledgeStore, registry: Iterable[str] | None = None):
        self.store = store
        self._registry = frozenset(registry) if registry is not None else None

    # ----------------------------------------------------------------- helpers
    def _snap(self, snapshot: str | int | None):
        return self.store.get_snapshot(snapshot)

    def _require(self, *relations: str) -> None:
        known = self._registry if self._registry is not None else registered_relations()
        missing = [r for r in relations if r not in known]
        if missing:
            raise RelationNotRegistered(f"relation(s) not declared in the ontology registry: {missing}",
                                        details={"missing": missing})

    def _el(self, ident: str, snap: str) -> dict | None:
        return self.store.get_element(ident, snap)

    def _out(self, ids, rel, snap):
        return [e for e in self.store.neighbors(list(ids), [rel], "out", snap)]

    def _section(self, ref: Any, snap: str) -> dict:
        """Normalize a SectionRef given as an id or as an inline dict."""
        if isinstance(ref, dict):
            return dict(ref)
        el = self._el(str(ref), snap)
        if el is None:
            return {"id": str(ref), "missing": True}
        p = props(el)
        return {"id": el["id"], "document_id": p.get("document_id"), "role": p.get("role"), "revision": p.get("revision"),
                "content_hash": p.get("content_hash"), "section_id": p.get("section_id")}

    def _document_state(self, doc_id: str, snap: str) -> dict | None:
        """Current revision/hash/role of a document from the graph, else from registered sources."""
        el = self._el(doc_id, snap)
        if el is not None and el.get("class") == "DocumentRef":
            p = props(el)
            return {"revision": str(p.get("revision")), "content_hash": p.get("content_hash"), "role": str(p.get("role", "")).upper()}
        try:
            src = self.store.get_source(doc_id)
        except NotFound:
            return None
        return {"revision": src.revision, "content_hash": src.content_hash, "role": src.role}

    # ----------------------------------------------------------------- Q1
    def q1_candidate_tdds(self, delta_id: str, snapshot=None, depth: int = 3, design_classes: Iterable[str] = DESIGN_CLASSES) -> QueryResult:
        """Candidate design elements touched by an SRS delta.  A candidate list only: the
        impact agent must still scan the whole inventory (``no_change`` needs evidence)."""
        snap = self._snap(snapshot)
        delta = self._el(delta_id, snap.id)
        if delta is None:
            raise NotFound(f"SrsDelta not found: {delta_id}", code="DELTA_NOT_FOUND")
        cr = props(delta).get("cr_id")
        deltas = [e for e in self.store.by_class("RequirementDelta", snap.id) if props(e).get("change_request_id") == cr]
        reqs: set[str] = set()
        for e in self.store.neighbors([d["id"] for d in deltas], ["before_requirement", "after_requirement"], "out", snap.id):
            reqs.add(e["dst"])
        design = set(design_classes)
        reach: dict[str, set[str]] = {}
        for req in sorted(reqs):
            seen, frontier = {req}, [req]
            for _ in range(depth):
                nxt = []
                for e in self.store.neighbors(frontier, ["traces_to", "satisfies", "has_scenario"], "both", snap.id):
                    for end in (e["src"], e["dst"]):
                        if end not in seen:
                            seen.add(end); nxt.append(end)
                frontier = nxt
            for el in self.store.get_elements(sorted(seen - {req}), snap.id):
                if el.get("class") in design:
                    reach.setdefault(el["id"], set()).add(req)
        rows = []
        for el in self.store.get_elements(sorted(reach), snap.id):
            p = props(el)
            rows.append({"id": el["id"], "class": el.get("class"), "revision": p.get("revision"), "owner_doc": p.get("owner_doc"),
                         "via_requirements": sorted(reach[el["id"]]), "source_refs": provenance_of(el)})
        rows.sort(key=lambda r: (r["class"] or "", r["id"]))
        return QueryResult("Q1", "input", rows, snap.id, {"candidate_only": True, "delta": delta_id, "requirements": sorted(reqs)})

    # ----------------------------------------------------------------- Q2
    def q2_unresolved_obligations(self, tdd_id: str | None = None, snapshot=None) -> QueryResult:
        snap = self._snap(snapshot)
        rows = []
        for ob in self.store.by_class("DesignObligation", snap.id):
            p = props(ob)
            if tdd_id is not None and p.get("tdd_id") != tdd_id:
                continue
            res = [r for r in self.store.get_elements([e["dst"] for e in self._out([ob["id"]], "has_obligation_resolution", snap.id)], snap.id)]
            statuses = [str(props(r).get("status")) for r in res]
            applicability = p.get("applicability", "applicable")
            status = str(p.get("status", "open"))
            if applicability == "not_applicable" and not res:
                state = "unresolved"  # N/A without evidence never counts as resolved
            elif res:
                state = "resolved" if all(s in RESOLVED for s in statuses) else "unresolved"
            else:
                state = "unresolved" if status in UNRESOLVED or applicability in ("applicable", "unknown") else "resolved"
            if state == "unresolved":
                rows.append({"id": ob["id"], "question": p.get("question"), "applicability": applicability, "status": status,
                             "resolution_statuses": statuses, "evidence_refs": [r2 for r in res for r2 in _as_list(props(r).get("design_refs"))]})
        return QueryResult("Q2", "gap", rows, snap.id, {"tdd_id": tdd_id})

    # ----------------------------------------------------------------- Q3
    def q3_context_pack(self, cs_id: str, snapshot=None, dependency_depth: int = 4) -> QueryResult:
        snap = self._snap(snapshot)
        cs = self._el(cs_id, snap.id)
        if cs is None or cs.get("class") != "ChangeSpecItem":
            raise NotFound(f"ChangeSpecItem not found: {cs_id}", code="CS_NOT_FOUND")
        p = props(cs)
        srs = [self._section(r, snap.id) for r in _as_list(p.get("srs_refs"))]
        tdd = [self._section(r, snap.id) for r in _as_list(p.get("tdd_refs"))]
        errors = []
        for label, refs, role in (("srs_refs", srs, "SRS"), ("tdd_refs", tdd, "TDD")):
            for r in refs:
                if r.get("missing"):
                    errors.append({"code": "SECTION_REF_MISSING", "ref": r["id"], "field": label})
                elif str(r.get("role", "")).upper() != role:
                    errors.append({"code": "SOURCE_ROLE_MISMATCH", "ref": r.get("id"), "field": label, "expected": role, "actual": r.get("role")})
                else:
                    state = self._document_state(str(r.get("document_id")), snap.id)
                    if state and state["role"] and state["role"] != role:
                        errors.append({"code": "SOURCE_ROLE_MISMATCH", "ref": r.get("id"), "field": label, "expected": role, "actual": state["role"], "document": r.get("document_id")})
        if errors and any(e["code"] == "SOURCE_ROLE_MISMATCH" for e in errors):
            raise SourceRoleError("change spec item references a section whose document has the wrong role", details={"errors": errors})
        seen, frontier = {r["id"] for r in tdd if r.get("id")}, [r["id"] for r in tdd if r.get("id")]
        origin = set(frontier)
        for _ in range(dependency_depth):
            nxt = []
            for e in self.store.neighbors(frontier, ["traces_to", "depends_on"], "out", snap.id):
                if e["dst"] not in seen:
                    seen.add(e["dst"]); nxt.append(e["dst"])
            frontier = nxt
        deps = self.store.get_elements(sorted(seen - origin), snap.id)
        criteria = [self._el(str(c), snap.id) or {"id": str(c), "missing": True} for c in _as_list(p.get("cr_acceptance_refs"))]
        row = {"cs": cs, "srs": srs, "tdd": tdd, "acceptance": criteria, "dependencies": deps,
               "srs_documents": sorted({str(r.get("document_id")) + "@" + str(r.get("revision")) for r in srs}),
               "tdd_documents": sorted({str(r.get("document_id")) + "@" + str(r.get("revision")) for r in tdd}), "errors": errors}
        return QueryResult("Q3", "input", [row], snap.id, {"cs": cs_id})

    # ----------------------------------------------------------------- Q4
    def q4_missing_failure_policy(self, snapshot=None) -> QueryResult:
        snap = self._snap(snapshot)
        rows = []
        for op in self.store.by_class("Operation", snap.id):
            p = props(op)
            uses = self.store.neighbors([op["id"]], ["uses", "exposes", "consumes", "publishes"], "out", snap.id)
            ifaces = {i["id"]: i for i in self.store.get_elements([e["dst"] for e in uses], snap.id)}
            external = [i for i in ifaces.values() if props(i).get("external")]
            if not (p.get("integration") or external):
                continue
            policies = [e for e in self.store.neighbors([op["id"]], ["governed_by"], "out", snap.id)]
            pol_els = self.store.get_elements([e["dst"] for e in policies], snap.id)
            found = any(x.get("class") == "FailurePolicy" for x in pol_els)
            rows.append({"operation": op["id"], "interfaces": sorted(i["id"] for i in (external or ifaces.values())),
                         "failure_policy": "found" if found else "missing"})
        missing = [r for r in rows if r["failure_policy"] == "missing"]
        return QueryResult("Q4", "gap", missing, snap.id, {"integration_operations": len(rows)})

    # ----------------------------------------------------------------- Q5
    def q5_incomplete_migrations(self, snapshot=None) -> QueryResult:
        snap = self._snap(snapshot)
        staged = {e["src"] for e in self.store.neighbors([m["id"] for m in self.store.by_class("Migration", snap.id)], ["has_migration_stage"], "out", snap.id)}
        rows = []
        for m in self.store.by_class("Migration", snap.id):
            p = props(m)
            missing = [f for f in ("rollback", "backfill") if p.get(f) in (None, "", [], {})]
            if p.get("stages") in (None, "", [], {}) and m["id"] not in staged:
                missing.append("stages")
            if missing:
                rows.append({"id": m["id"], "from_schema": p.get("from_schema"), "to_schema": p.get("to_schema"), "lock": p.get("lock"),
                             "compatibility": p.get("compatibility"), "missing": missing})
        return QueryResult("Q5", "gap", rows, snap.id)

    # ----------------------------------------------------------------- Q6
    def q6_stale_refs(self, snapshot=None) -> QueryResult:
        snap = self._snap(snapshot)
        rows = []
        for ref in self.store.by_class("SectionRef", snap.id):
            p = props(ref)
            state = self._document_state(str(p.get("document_id")), snap.id)
            if state is None:
                rows.append({"ref": ref["id"], "document": p.get("document_id"), "status": "unresolved", "reason": "DOCUMENT_UNKNOWN"})
                continue
            reasons = []
            if state["content_hash"] and p.get("content_hash") and p["content_hash"] != state["content_hash"]:
                reasons.append("CONTENT_HASH_DIFFERS")
            if p.get("revision") is not None and str(p["revision"]) != state["revision"]:
                reasons.append("REVISION_DIFFERS")
            if reasons:
                rows.append({"ref": ref["id"], "document": p.get("document_id"), "ref_revision": p.get("revision"),
                             "source_revision": state["revision"], "status": "stale", "reasons": reasons})
        return QueryResult("Q6", "drift", rows, snap.id)

    # ----------------------------------------------------------------- Q7
    def q7_no_change_validity(self, report_id: str, snapshot=None) -> QueryResult:
        snap = self._snap(snapshot)
        report = self._el(report_id, snap.id)
        if report is None:
            raise NotFound(f"ImpactReport not found: {report_id}", code="IMPACT_REPORT_NOT_FOUND")
        rows = []
        for entry in _as_list(props(report).get("entries")):
            if entry.get("action") != "no_change":
                continue
            errors = []
            if not (entry.get("rationale") or entry.get("reason")):
                errors.append("NO_CHANGE_REASON_MISSING")
            if not entry.get("srs_refs"):
                errors.append("NO_CHANGE_SRS_REFS_MISSING")
            tdd_refs = _as_list(entry.get("tdd_refs"))
            if not tdd_refs:
                errors.append("NO_CHANGE_TDD_REF_MISSING")
            for ref in tdd_refs:
                r = self._section(ref, snap.id) if not isinstance(ref, dict) else ref
                state = self._document_state(str(r.get("document_id", entry.get("tdd_id"))), snap.id)
                if state is None:
                    errors.append("NO_CHANGE_TDD_UNKNOWN")
                elif r.get("revision") is not None and str(r["revision"]) != state["revision"]:
                    errors.append("NO_CHANGE_REVISION_STALE")
            rows.append({"tdd_id": entry.get("tdd_id"), "valid": not errors, "errors": sorted(set(errors)),
                         "reason": entry.get("rationale") or entry.get("reason"), "srs_refs": entry.get("srs_refs")})
        return QueryResult("Q7", "valid", rows, snap.id, {"report": report_id, "all_valid": all(r["valid"] for r in rows)})

    # ----------------------------------------------------------------- Q8
    def q8_acceptance_coverage(self, cr_id: str, snapshot=None) -> QueryResult:
        self._require("implements", "verifies")
        snap = self._snap(snapshot)
        criteria: dict[str, dict] = {}
        for ar in self.store.by_class("AcceptanceReceipt", snap.id):
            p = props(ar)
            if p.get("cr_id") != cr_id:
                continue
            for c in _as_list(p.get("criterion_refs")):
                criteria.setdefault(str(c), {"receipt": ar["id"], "receipt_status": p.get("status")})
        cs_by_criterion: dict[str, list[str]] = {c: [] for c in criteria}
        for cs in self.store.by_class("ChangeSpecItem", snap.id):
            for c in _as_list(props(cs).get("cr_acceptance_refs")):
                if str(c) in cs_by_criterion:
                    cs_by_criterion[str(c)].append(cs["id"])
        tests = self.store.by_class("TestReceipt", snap.id)
        rows = []
        for criterion, info in sorted(criteria.items()):
            cs_ids = sorted(set(cs_by_criterion[criterion]))
            work = sorted({e["src"] for e in self.store.neighbors(cs_ids, ["implements"], "in", snap.id)}) if cs_ids else []
            verified = {e["src"] for e in self.store.neighbors(work, ["verifies"], "in", snap.id)} if work else set()
            # A receipt covers a criterion only if it names that criterion; the verifies/cs links narrow
            # which receipts are eligible but never extend coverage to criteria the receipt does not cite.
            related = [t for t in tests if (t["id"] in verified or props(t).get("cs_id") in cs_ids)
                       and criterion in map(str, _as_list(props(t).get("criterion_refs")))]
            passing = [t["id"] for t in related if str(props(t).get("result")).lower() in ("pass", "passed")]
            status = "covered" if passing else ("failing" if related else "uncovered")
            rows.append({"criterion": criterion, "cs_rows": cs_ids, "work_items": work, "tests": sorted(t["id"] for t in related),
                         "passing_tests": sorted(passing), "status": status, **info})
        return QueryResult("Q8", "gap", rows, snap.id, {"cr": cr_id, "uncovered": [r["criterion"] for r in rows if r["status"] != "covered"]})

    # ----------------------------------------------------------------- Q9
    def q9_shared_contract_conflicts(self, snapshot=None, accepted=("accepted",)) -> QueryResult:
        snap = self._snap(snapshot)
        chain = [("has_srs_delta", None), ("has_impact_report", None), ("realizes_tdd_set", None), ("has_change_spec_item", None)]
        touches: dict[str, dict[str, set[str]]] = {}
        for cr in self.store.by_class("ChangeRequest", snap.id):
            status = str(props(cr).get("status", "open")).lower()
            if status in accepted:
                continue
            frontier = [cr["id"]]
            for rel, _ in chain:
                frontier = sorted({e["dst"] for e in self._out(frontier, rel, snap.id)})
                if not frontier:
                    break
            if not frontier:
                continue
            for e in self._out(frontier, "traces_to", snap.id):
                target = self._el(e["dst"], snap.id)
                if target is not None and target.get("class") in SHARED_CONTRACT_CLASSES:
                    touches.setdefault(target["id"], {}).setdefault(cr["id"], set()).add(e["src"])
        rows = [{"interface": i, "change_requests": sorted(crs), "change_spec_items": {c: sorted(v) for c, v in sorted(crs.items())}}
                for i, crs in sorted(touches.items()) if len(crs) > 1]
        return QueryResult("Q9", "drift", rows, snap.id, {"action": "mark mappings stale and rebase; never last-write-wins"})

    # ----------------------------------------------------------------- dispatch
    NAMES = {"Q1": "q1_candidate_tdds", "Q2": "q2_unresolved_obligations", "Q3": "q3_context_pack", "Q4": "q4_missing_failure_policy",
             "Q5": "q5_incomplete_migrations", "Q6": "q6_stale_refs", "Q7": "q7_no_change_validity", "Q8": "q8_acceptance_coverage",
             "Q9": "q9_shared_contract_conflicts"}

    def run(self, name: str, **kwargs: Any) -> QueryResult:
        key = name.upper()
        if key not in self.NAMES:
            raise NotFound(f"unknown query {name}; expected one of {sorted(self.NAMES)}", code="QUERY_UNKNOWN")
        fn: Callable[..., QueryResult] = getattr(self, self.NAMES[key])
        return fn(**kwargs)
