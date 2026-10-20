"""Evidence closure over a ``KnowledgeStore`` snapshot.

Implements the retrieval steps of the design document: seed by intent, filter
by authority, expand over declared relations to a fixed point (no depth cap),
expand by profile, join evidence, keep negative evidence, apply a budget with
explicit truncation, and sort deterministically so ``resolution_hash`` is
reproducible.  Similarity search (see ``candidates``) is only ever a seed source.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Iterable, Sequence

from ..errors import NotFound
from ..store.base import KnowledgeStore, SnapshotInfo, canonical, digest
from . import expansion
from .policy import default_policies, policy_hash

MAX_BATCH = 2000

# Properties that reference other elements by id.  They are followed forward as
# implicit relations (named ``ref:<field>``) so a ChangeSpecItem pulls in the
# SectionRefs and acceptance criteria it cites even though no edge exists.
REF_FIELDS: dict[str, tuple[str, ...]] = {
    "ChangeSpecItem": ("srs_refs", "tdd_refs", "cr_acceptance_refs"),
    "DesignObligation": ("srs_refs", "cr_acceptance_refs"),
    "ObligationResolution": ("design_refs", "verification_refs"),
    "TestReceipt": ("cs_id", "criterion_refs"),
    "AcceptanceReceipt": ("cr_id", "criterion_refs"),
    "SectionRef": ("document_id",),
}


def referenced_ids(element: dict) -> list[tuple[str, str]]:
    props = element.get("props") or {}
    out: list[tuple[str, str]] = []
    for name in REF_FIELDS.get(element.get("class", ""), ()):
        value = props.get(name, element.get(name))
        for item in value if isinstance(value, (list, tuple)) else [value]:
            ident = item.get("id") if isinstance(item, dict) else item
            if isinstance(ident, str) and ident:
                out.append((name, ident))
    return out


@dataclass(frozen=True)
class SeedSpec:
    kind: str
    ids: tuple[str, ...]
    meta: dict = field(default_factory=dict)


@dataclass
class EvidenceClosure:
    snapshot: str
    snapshot_hash: str
    seeds: list[str]
    seed_kind: str
    profile: str
    budget: int | None
    selected_ids: list[str]
    edges: list[dict]
    reasons: dict[str, dict]
    excluded: list[dict]
    source_refs: list[dict]
    obligations: list[dict]
    gaps: list[dict]
    conflicts: list[dict]
    unresolved: list[dict]
    boundary: list[dict]
    truncated: bool
    missing_ids: list[str]
    mandatory_truncated: list[str]
    policy_hash: str
    resolution_hash: str

    def to_dict(self) -> dict:
        return dict(self.__dict__)

    @property
    def blockers(self) -> list[str]:
        out = []
        if self.mandatory_truncated:
            out.append("TRUNCATED_MANDATORY_NEED")
        if any(c.get("code") == "SOURCE_ROLE_MISMATCH" for c in self.conflicts):
            out.append("SOURCE_ROLE_MISMATCH")
        if any(c.get("code") == "STALE_SOURCE" for c in self.conflicts):
            out.append("STALE_SOURCE")
        return out


def provenance_of(element: dict) -> list[dict]:
    raw = element.get("provenance") or element.get("sources") or element.get("evidence") or []
    return [p for p in raw if isinstance(p, dict)]


class ClosureService:
    def __init__(self, store: KnowledgeStore, policies=None):
        self.store = store
        self.policies = tuple(policies) if policies is not None else default_policies()
        self.policy_map = {p.relation: p for p in self.policies}
        self.policy_digest = policy_hash(self.policies)

    # ------------------------------------------------------------ authority
    def _authority(self, el: dict, role_of: dict[str, str], pins: dict[str, tuple[str, str]]) -> tuple[list[dict], bool]:
        """Return (conflicts, exclude).  Facts from stale or wrong-role sources are never evidence."""
        conflicts: list[dict] = []
        if el.get("class") == "SectionRef":
            props = el.get("props") or {}
            doc, role = props.get("document_id", el.get("document_id")), props.get("role", el.get("role"))
            actual = role_of.get(doc)
            if actual and role and actual != str(role).upper():
                conflicts.append({"code": "SOURCE_ROLE_MISMATCH", "element": el["id"], "document": doc, "declared_role": str(role).upper(), "actual_role": actual})
        for p in provenance_of(el):
            sid = p.get("source_id")
            if not sid:
                continue
            rev, shash = p.get("source_version"), p.get("source_hash")
            if sid in pins:
                pin_rev, pin_hash = pins[sid]
                if (rev is not None and str(rev) != pin_rev) or (shash and shash != pin_hash):
                    conflicts.append({"code": "STALE_SOURCE", "element": el["id"], "document": sid, "element_revision": rev, "pinned_revision": pin_rev})
                continue
            if rev is not None and shash:
                try:
                    doc = self.store.get_source(sid, str(rev))
                except NotFound:
                    continue
                if doc.content_hash != shash:
                    conflicts.append({"code": "STALE_SOURCE", "element": el["id"], "document": sid, "revision": str(rev), "reason": "content hash differs from the registered source"})
        return conflicts, bool(conflicts)

    # ------------------------------------------------------------ resolve
    def resolve(self, seeds: SeedSpec | Sequence[str], *, profile: str = "tdd-generation", budget: int | None = None,
                snapshot: str | int | None = None, mandatory_ids: Iterable[str] = (), pins: dict[str, tuple[str, str]] | None = None) -> EvidenceClosure:
        if not isinstance(seeds, SeedSpec):
            seeds = SeedSpec("ids", tuple(seeds))
        info: SnapshotInfo = self.store.get_snapshot(snapshot)
        pins = {k: (str(v[0]), str(v[1])) for k, v in (pins or {}).items()}
        role_of = {d.id: d.role for d in self.store.list_sources()}
        snap = info.id
        unresolved: list[dict] = []
        conflicts: list[dict] = []
        excluded: dict[str, dict] = {}
        reasons: dict[str, dict] = {}
        depth: dict[str, int] = {}
        elements: dict[str, dict] = {}
        seen_edges: dict[str, dict] = {}
        unsupported_seen: set[str] = set()

        seed_ids = sorted({str(i) for i in seeds.ids if str(i)})
        gaps: list[dict] = []
        for el in self.store.get_elements(seed_ids, snap):
            elements[el["id"]] = el
        for sid in seed_ids:
            if sid not in elements:
                gaps.append({"type": "seed_not_found", "code": "SEED_NOT_FOUND", "element": sid})
                continue
            c, exclude = self._authority(elements[sid], role_of, pins)
            conflicts.extend(c)
            if exclude:
                excluded[sid] = {"id": sid, "reason": [x["code"] for x in c], "depth": 0}
                continue
            depth[sid] = 0
        frontier = sorted(depth)
        while frontier:
            level = depth[frontier[0]]
            edges = self.store.neighbors(frontier, None, "both", snap)
            current = set(frontier)
            candidates: dict[str, tuple[str, dict, str]] = {}
            for e in edges:
                key = canonical(e)
                rel, src, dst = str(e.get("rel", "")), str(e.get("src", "")), str(e.get("dst", ""))
                policy = self.policy_map.get(rel)
                if policy is None:
                    if key not in unsupported_seen:
                        unsupported_seen.add(key)
                        unresolved.append({"type": "unsupported_relation", "src": src, "rel": rel, "dst": dst})
                    continue
                seen_edges[key] = e
                sides = []
                if src in current and policy.direction in ("both", "out"):
                    sides.append((src, dst))
                if dst in current and policy.direction in ("both", "in"):
                    sides.append((dst, src))
                for origin, target in sides:
                    if target in depth or target in excluded or policy.terminal and False:
                        continue
                    prior = candidates.get(target)
                    if prior is None or origin < prior[0]:
                        candidates[target] = (origin, e, rel)
            for origin_id in sorted(current):
                for field_name, ref_id in referenced_ids(elements[origin_id]):
                    if ref_id not in depth and ref_id not in excluded and ref_id != origin_id:
                        prior = candidates.get(ref_id)
                        if prior is None or origin_id < prior[0]:
                            candidates[ref_id] = (origin_id, {"ref": field_name}, f"ref:{field_name}")
            if not candidates:
                break
            found = {el["id"]: el for el in self.store.get_elements(sorted(candidates), snap)}
            nxt: list[str] = []
            for target in sorted(candidates):
                origin, e, rel = candidates[target]
                el = found.get(target)
                if el is None:
                    unresolved.append({"type": "dangling_reference", "src": origin, "rel": rel, "dst": target})
                    continue
                elements[target] = el
                c, exclude = self._authority(el, role_of, pins)
                conflicts.extend(c)
                if exclude:
                    excluded[target] = {"id": target, "reason": [x["code"] for x in c], "depth": level + 1, "via": {"from": origin, "relation": rel}}
                    continue
                depth[target] = level + 1
                reasons[target] = {"from": origin, "relation": rel}
                policy = self.policy_map.get(rel)
                if policy is None or not policy.terminal:
                    nxt.append(target)
            frontier = nxt

        ordered = sorted(depth, key=lambda i: (depth[i], i))
        missing: list[str] = []
        if budget is not None and budget >= 0 and len(ordered) > budget:
            missing, ordered = ordered[budget:], ordered[:budget]
        selected = set(ordered)
        mandatory = sorted(set(mandatory_ids))
        mandatory_truncated = [m for m in mandatory if m in missing]
        gaps.extend({"type": "mandatory_not_reached", "code": "MANDATORY_NOT_IN_CLOSURE", "element": m}
                    for m in mandatory if m not in depth and m not in missing and m not in excluded)
        edges_out, boundary = [], []
        for key, e in seen_edges.items():
            s, d = str(e.get("src")), str(e.get("dst"))
            if s in selected and d in selected:
                edges_out.append(e)
            elif (s in selected) != (d in selected):
                outside = d if s in selected else s
                if outside in missing or outside in elements:
                    boundary.append({"src": s, "rel": e.get("rel"), "dst": d, "reason": "budget" if outside in missing else "excluded_or_out_of_scope"})
        edges_out.sort(key=lambda e: (str(e.get("src")), str(e.get("rel")), str(e.get("dst")), canonical(e)))
        boundary.sort(key=lambda b: (b["src"], str(b["rel"]), b["dst"]))
        selected_elements = [elements[i] for i in ordered]
        if profile != "none":
            gaps.extend(expansion.gaps_for(profile, selected_elements, edges_out))
        obligations = [{"id": e["id"], "status": (e.get("props") or {}).get("status", e.get("status")), "applicability": (e.get("props") or {}).get("applicability", e.get("applicability"))}
                       for e in selected_elements if e.get("class") == "DesignObligation"]
        refs: dict[str, dict] = {}
        for el in selected_elements:
            for p in provenance_of(el):
                if p.get("source_id") is None and p.get("tdd") is None:
                    continue
                ref = {"document": p.get("source_id", p.get("tdd")), "revision": p.get("source_version"), "section": p.get("block") or p.get("section"),
                       "content_hash": p.get("source_hash") or p.get("quote_hash")}
                refs[canonical(ref)] = ref
            if el.get("class") == "SectionRef":
                props = el.get("props") or {}
                ref = {"document": props.get("document_id"), "revision": props.get("revision"), "section": props.get("section_id"), "content_hash": props.get("content_hash")}
                refs[canonical(ref)] = ref
        source_refs = [refs[k] for k in sorted(refs)]
        excluded_list = [excluded[k] for k in sorted(excluded)]
        gaps.sort(key=lambda g: (g.get("code", ""), g.get("element", ""), g.get("field", "")))
        conflicts.sort(key=canonical)
        unresolved.sort(key=canonical)
        resolution_hash = digest({
            "snapshot": info.hash, "seeds": seed_ids, "seed_kind": seeds.kind, "profile": [profile, expansion.PROFILE_VERSION], "budget": budget,
            "policy": self.policy_digest, "selected": ordered, "edges": edges_out, "excluded": excluded_list, "gaps": gaps,
            "conflicts": conflicts, "unresolved": unresolved, "missing": missing, "mandatory": mandatory,
        })
        return EvidenceClosure(
            snapshot=snap, snapshot_hash=info.hash, seeds=seed_ids, seed_kind=seeds.kind, profile=profile, budget=budget,
            selected_ids=ordered, edges=edges_out, reasons={k: reasons[k] for k in ordered if k in reasons}, excluded=excluded_list,
            source_refs=source_refs, obligations=obligations, gaps=gaps, conflicts=conflicts, unresolved=unresolved, boundary=boundary,
            truncated=bool(missing), missing_ids=missing, mandatory_truncated=mandatory_truncated, policy_hash=self.policy_digest,
            resolution_hash=resolution_hash)
