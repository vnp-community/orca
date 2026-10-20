"""MOD-Delta: compare two source baselines, the affected set, the implicit CR (FR-003)."""
from collections import deque
from dataclasses import dataclass, field

from ..model import SrcNode, stable_hash
from ..store import source

SKIP_CLASSES = {"Document", "ChangeRequest"}
SKIP_RELS = {"describedIn", "supersedes"}
TESTABLE = {"FunctionalRequirement", "NonFunctionalRequirement", "UserRequirement", "Module", "Integration", "Scenario",
            "StateMachine", "Transition", "Endpoint", "Field", "Response", "AcceptanceCriterion"}


@dataclass
class Delta:
    added: set = field(default_factory=set)
    modified: set = field(default_factory=set)
    removed: set = field(default_factory=set)
    renamed: dict = field(default_factory=dict)   # new code -> old code (supersede on the TDD side)
    declared: dict = field(default_factory=dict)  # CR -> {affects, removes}

    @property
    def changed(self):
        return self.added | self.modified | self.removed

    def to_dict(self):
        return {"added": sorted(self.added), "modified": sorted(self.modified), "removed": sorted(self.removed),
                "renamed": dict(sorted(self.renamed.items())), "declared": self.declared}


def diff(prev, new):
    """Delta by per-node content hash. `prev=None` means every node is new. A supersede pair renaming a code counts as a change."""
    d = Delta()
    pn = {c: n for c, n in (prev.nodes.items() if prev else []) if n.cls not in SKIP_CLASSES}
    nn = {c: n for c, n in new.nodes.items() if n.cls not in SKIP_CLASSES}
    renamed = {s["new"]: s["old"] for s in new.supersede if s["old"] in pn and s["new"] in nn}
    for c, n in nn.items():
        old = renamed.get(c, c)
        if old not in pn:
            d.added.add(c)
        elif pn[old].content_hash() != n.content_hash() or old != c:
            d.modified.add(c)
    d.removed = set(pn) - set(nn) - set(renamed.values())
    d.renamed = renamed
    if prev:
        pe = {(e.src, e.rel, e.dst) for e in prev.edges if e.rel not in SKIP_RELS}
        ne = {(e.src, e.rel, e.dst) for e in new.edges if e.rel not in SKIP_RELS}
        for s, _, _t in pe ^ ne:  # a changed relation is also a change of the source node
            if s in nn and s not in d.added:
                d.modified.add(s)
    for n in new.of("ChangeRequest"):
        old = prev.get(n.code) if prev else None
        # a target removed from the source drops its affects edge: not a re-declared CR
        old_affects = {x.code for x in prev.targets(n.code, "affects")} & set(new.nodes) if old is not None else None
        if old is not None and old.content_hash() == n.content_hash() and \
                old_affects == {x.code for x in new.targets(n.code, "affects")}:
            continue  # the CR was already in the previous baseline, not declared by this change
        d.declared[n.code] = {"affects": sorted(x.code for x in new.targets(n.code, "affects")),
                              "removes": sorted(set(n.props.get("removes", [])) | {x.code for x in new.targets(n.code, "removes")})}
    return d


def impact(new, prev, changed, depth=3):
    """Codes of testable classes within `depth` hops of the changed nodes (undirected graph of both baselines)."""
    adj = {}
    for pkg in (new, prev):
        if pkg is None:
            continue
        for e in pkg.edges:
            if e.rel in SKIP_RELS:
                continue
            adj.setdefault(e.src, set()).add(e.dst)
            adj.setdefault(e.dst, set()).add(e.src)
    seen = {c: 0 for c in changed}
    q = deque(sorted(changed))
    while q:
        cur = q.popleft()
        if seen[cur] >= depth:
            continue
        for nb in sorted(adj.get(cur, ())):
            if nb not in seen:
                seen[nb] = seen[cur] + 1
                q.append(nb)
    out = set()
    for c in seen:
        n = new.get(c) or (prev.get(c) if prev else None)
        if n is not None and n.cls in TESTABLE:
            out.add(c)
    return out


def implicit_cr(new, delta):
    """Implicit CR when the source changed without a declared CR; returns the `derived` entry for save_baseline, or None."""
    if not delta.changed or delta.declared:
        return None
    code = "CR-AUTO-" + stable_hash(delta.to_dict())[:10]
    # the description is document content (Vietnamese): it appears in the version history of the test specs
    node = SrcNode(code, "ChangeRequest", {"status": "Baselined"},
                   {"description": "CR ngầm định: suy ra từ khác biệt hai baseline nguồn", "changeKind": "implicit",
                    "removes": sorted(delta.removed)})
    edges = [[code, "affects", c] for c in sorted(delta.added | delta.modified) if c in new.nodes]
    return {"node": node, "edges": edges, "reason": "source changed without a CR"}


def previous_baseline(db, feature, baseline_id):
    """The baseline right before `baseline_id` in load order (correct even when the new baseline was saved earlier with `kg ingest`)."""
    ids = [r["id"] for r in source.list_baselines(db, feature)]
    if baseline_id in ids:
        i = ids.index(baseline_id)
        return ids[i - 1] if i > 0 else None
    return ids[-1] if ids else None


def prepare_change(db, pkg):
    """Save the baseline (or reuse it) and compute the delta to the previous one; the implicit CR is stored with the baseline."""
    bid = pkg.baseline_id or source.baseline_id_of(pkg)
    prev_id = previous_baseline(db, pkg.feature, bid)
    prev = source.load_source(db, pkg.feature, prev_id) if prev_id else None
    d = diff(prev, pkg)
    derived = [x for x in [implicit_cr(pkg, d)] if x]
    saved = source.save_baseline(db, pkg, derived=derived)
    cur = source.load_source(db, pkg.feature, saved["baseline"])
    # the implicit CR (just written, or written by an earlier run on the same baseline) is always in the CR list
    for n in cur.of("ChangeRequest"):
        if n.code not in d.declared and n.origin == "derived":
            d.declared[n.code] = {"affects": sorted(x.code for x in cur.targets(n.code, "affects")),
                                  "removes": sorted(n.props.get("removes", []))}
    return {"baseline": saved["baseline"], "reused": saved["reused"], "prev": prev_id, "delta": d, "source": cur, "previous": prev,
            "crs": sorted(d.declared)}
