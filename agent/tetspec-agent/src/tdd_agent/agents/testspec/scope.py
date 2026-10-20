"""Choose the test-spec scope on the (sub)graph read over MCP (SOLUTION sections 5, 13)."""
from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field

from ...errors import TddAgentError
from ..common import tdd_members

NO_SECTION = ("document", "", None)


@dataclass
class Scope:
    """One feature (= one TDD): elements under test, context elements, CR criteria attached to elements."""
    tdd: str
    targets: list[str]
    context: list[str] = field(default_factory=list)
    acceptance: dict[str, list[dict]] = field(default_factory=dict)  # element id -> [{"id", "text"}]
    unresolved_refs: list[str] = field(default_factory=list)
    unmapped: dict[str, int] = field(default_factory=dict)


def mappable(mapping: dict, cls: str) -> bool:
    return cls in (mapping.get("classes") or {}) or cls in (mapping.get("extended_passthrough") or [])


def _members_of(e: dict) -> set[str]:
    props = e.get("props") or {}
    out = set(e.get("tdds") or props.get("tdds") or [])
    owner = e.get("owner_doc") or props.get("owner_doc")
    if owner:
        out.add(owner)
    return out


def neighbors(graph: dict, start: list[str], mapping: dict, unmapped: dict[str, int] | None = None) -> list[str]:
    """Neighbours up to ``context_depth`` hops (both directions), only through projectable elements; at most ``context_limit`` nodes."""
    depth, limit = int(mapping.get("context_depth", 2)), int(mapping.get("context_limit", 400))
    by_id = {e["id"]: e for e in graph.get("elements", [])}
    adj: dict[str, set[str]] = defaultdict(set)
    for ed in graph.get("edges", []):
        s, d = ed.get("src"), ed.get("dst")
        if s in by_id and d in by_id:
            adj[s].add(d); adj[d].add(s)
    seen, frontier, out = set(start), sorted(set(start)), []
    for _ in range(depth):
        nxt = []
        for x in frontier:
            for y in sorted(adj[x]):
                if y in seen:
                    continue
                seen.add(y)
                cls = by_id[y]["class"]
                if not mappable(mapping, cls):
                    if unmapped is not None:
                        unmapped[cls] = unmapped.get(cls, 0) + 1
                    continue
                if len(out) >= limit:
                    return out
                out.append(y); nxt.append(y)
        frontier = nxt
    return out


def cr_scopes(graph: dict, params: dict, mapping: dict) -> list[Scope]:
    """Scope by change spec: the elements the items' ``srs_refs``/``tdd_refs`` point to, split by TDD."""
    by_id = {e["id"]: e for e in graph.get("elements", [])}
    items = [by_id[x] for x in params.get("cs_elements") or [] if x in by_id]
    if not items:
        raise TddAgentError("no change spec item of this run is in the snapshot", code="TESTSPEC_EMPTY_SCOPE")
    texts = {a["id"]: a.get("text") for a in params.get("acceptance") or []}
    tdds = list(params.get("tdds") or [])
    targets: list[str] = []
    acceptance: dict[str, list[dict]] = defaultdict(list)
    unresolved: list[str] = []
    unmapped: dict[str, int] = {}
    for item in items:
        props = item.get("props") or {}
        refs = list(props.get("srs_refs") or []) + list(props.get("tdd_refs") or [])
        for r in props.get("tdd_refs") or []:
            if r.get("document_id") and r.get("document_id") not in tdds:
                tdds.append(r["document_id"])
        mine = []
        for r in refs:
            sid = r.get("section_id")
            if sid in NO_SECTION:
                continue
            e = by_id.get(sid)
            if e is None:
                unresolved.append(f"{r.get('document_id')}#{sid}")
            elif not mappable(mapping, e["class"]):
                unmapped[e["class"]] = unmapped.get(e["class"], 0) + 1
            elif sid not in mine:
                mine.append(sid)
        for x in mine:
            if x not in targets:
                targets.append(x)
            for ac in props.get("cr_acceptance_refs") or []:
                if by_id[x]["class"] in ("Requirement", "QualityTarget", "BusinessRule") and ac not in [a["id"] for a in acceptance[x]]:
                    acceptance[x].append({"id": ac, "text": texts.get(ac)})
    if not targets:
        raise TddAgentError("the change spec references no element that can be projected", code="TESTSPEC_EMPTY_SCOPE",
                            details={"unresolved_refs": sorted(set(unresolved)), "unmapped": unmapped})
    if not tdds:
        tdds = [params.get("default_tdd") or "TDD"]
    per: dict[str, list[str]] = {t: [] for t in tdds}
    for x in targets:
        owners = [t for t in tdds if t in _members_of(by_id[x])] or [tdds[0]]
        for t in owners:
            per[t].append(x)
    scopes = []
    for t in tdds:
        if not per[t]:
            continue
        um = dict(unmapped)
        ctx = neighbors(graph, per[t], mapping, um)
        scopes.append(Scope(t, sorted(per[t]), ctx, {k: v for k, v in acceptance.items() if k in per[t]},
                            sorted(set(unresolved)), um))
    return scopes


def tdd_scope(graph: dict, tdd: str, mapping: dict, members_ids: list[str] | None = None) -> Scope:
    """Scope by TDD (manual run): every projectable element of the TDD. ``members_ids``: members returned by the MCP server (subgraph)."""
    if members_ids is not None:
        by_id = {e["id"]: e for e in graph.get("elements", [])}
        members = [by_id[i] for i in members_ids if i in by_id]
    else:
        members, _ = tdd_members(graph, tdd)
    if members_ids is None and not members and not any(_members_of(e) & {tdd} for e in graph.get("elements", [])) \
            and not any(e.get("tdds") or (e.get("props") or {}).get("tdds") for e in graph.get("elements", [])):
        members = list(graph.get("elements", []))  # one-TDD KG (`init` project): elements carry no TDD membership
    unmapped: dict[str, int] = {}
    targets = []
    for e in members:
        if mappable(mapping, e["class"]):
            targets.append(e["id"])
        else:
            unmapped[e["class"]] = unmapped.get(e["class"], 0) + 1
    if not targets:
        raise TddAgentError(f"TDD {tdd!r} has no element that can be projected", code="TESTSPEC_EMPTY_SCOPE", details={"unmapped": unmapped})
    targets = sorted(targets)
    return Scope(tdd, targets, neighbors(graph, targets, mapping, unmapped), {}, [], unmapped)
