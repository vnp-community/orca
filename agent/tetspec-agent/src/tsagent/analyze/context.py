"""Context pack for one subject (T5.3): the subject node, neighbours 1-2 hops away, the seed nodes to fill, the code allow list.

Stable order (stable hash, so the cache hits). Above `max_chars`, neighbours are dropped by (hop, relation priority), then long
texts are cut; seed nodes are never cut (a subject that is too large is split into several calls by fill).
"""
import json

from ..model import stable_hash

HUBS = {"Document", "ChangeRequest"}
SKIP_RELS = {"describedIn", "affects", "supersedes"}
REL_PRIORITY = {
    "hasAcceptance": 0, "hasField": 0, "hasResponse": 0, "hasTransition": 0, "inMachine": 0, "hasErrorMapping": 0,
    "implements": 1, "allocatedTo": 1, "satisfiedBy": 1, "realizedBy": 1, "invokes": 1, "participatesIn": 1,
    "hasTimeoutPolicy": 1, "hasRetryPolicy": 1, "protectedBy": 1, "hasFallback": 1, "boundedBy": 1, "governs": 1,
    "dependsOn": 2, "integratesVia": 2, "assumes": 2, "blockedBy": 2, "hasRisk": 2, "mitigatedBy": 2, "securedBy": 2,
    "requiresHeader": 2, "usesChannel": 2,
}
VOLATILE = ("sourceFile", "sourceLine")
SEED_FIELDS = {"TestCondition": ("conditionKind", "statement", "expectedEffect", "preState", "trigger"),
               "TestOracle": ("oracleKind", "expected", "tolerance", "measurement"),
               "FaultModel": ("faultKind", "injectionPoint", "timing", "expectedHandling"),
               "Partition": ("parameter", "classKind", "values", "valid"),
               "ScenarioPath": ("pathKind", "steps")}


def node_view(n, max_text=None):
    out = {"code": n.code, "class": n.cls}
    for k in ("label", "status", "testScope", "owner", "description"):
        if n.base.get(k) not in (None, ""):
            out[k] = n.base[k]
    props = {k: v for k, v in sorted(n.props.items()) if k not in VOLATILE}
    if max_text:
        props = {k: (v[:max_text] + "…" if isinstance(v, str) and len(v) > max_text else v) for k, v in props.items()}
    if props:
        out["props"] = props
    return out


def neighbors(pkg, code, depth):
    """[(hop, priority, relation, direction, code)] by BFS, never walking through hub nodes (Document, CR)."""
    seen, out, frontier = {code}, [], [code]
    for hop in range(1, depth + 1):
        nxt = []
        for cur in frontier:
            items = [(rel, "out", t) for rel, ts in pkg.out_edges(cur).items() for t in ts] + \
                    [(rel, "in", s) for rel, ss in pkg.in_edges(cur).items() for s in ss]
            for rel, d, other in sorted(items):
                if rel in SKIP_RELS or other in seen or other not in pkg.nodes:
                    continue
                seen.add(other)
                out.append((hop, REL_PRIORITY.get(rel, 3), rel, d, cur, other))
                if pkg.nodes[other].cls not in HUBS:
                    nxt.append(other)
        frontier = nxt
    return sorted(out)


def seed_view(g, cond):
    v = {"seedCode": cond.code, **{k: cond.attrs.get(k) for k in SEED_FIELDS["TestCondition"] if cond.attrs.get(k) is not None}}
    v["subject"] = g.get(cond.rel("subject")[0]).attrs["tddCode"] if cond.rel("subject") else None
    v["basedOn"] = cond.prov.based_on
    for rel, cls in (("oracle", "TestOracle"), ("faultModel", "FaultModel"), ("partition", "Partition"), ("path", "ScenarioPath")):
        t = g.get(cond.rel(rel)[0]) if cond.rel(rel) else None
        if t is not None:
            v[rel] = {k: t.attrs.get(k) for k in SEED_FIELDS[cls] if t.attrs.get(k) is not None}
    return v


def context_pack(pkg, g, subject, obligations, conditions, depth=2, max_chars=24000, gaps=()):
    """Return the context pack dict. `obligations`, `conditions`: codes of the subject's seed nodes to fill."""
    subj = pkg.get(subject)
    conds = [g.get(c) for c in conditions]
    cond_subjects = sorted({s for c in conds for s in [g.get(c.rel("subject")[0]).attrs["tddCode"]] if s != subject})
    based = sorted({b for c in conds for b in c.prov.based_on} - {subject} - set(cond_subjects))
    pack = {
        "subject": node_view(subj),
        "conditionSubjects": [node_view(pkg.get(c)) for c in cond_subjects if pkg.get(c)],
        "obligations": [{"seedCode": o, "question": g.get(o).attrs["question"]} for o in obligations],
        "conditions": [seed_view(g, c) for c in conds],
        "openGaps": [{"code": x.code, "missing": x.attrs["missing"]} for x in (g.get(c) for c in gaps)],
        "neighbors": [],
        "notes": [],
    }
    fixed = {subject, *cond_subjects}
    nbs = [x for x in neighbors(pkg, subject, depth) if x[-1] not in fixed]
    extra = [b for b in based if b in pkg.nodes and b not in {x[-1] for x in nbs}]
    entries = [{"hop": 1, "rel": "basedOn", "dir": "out", "via": subject, **node_view(pkg.get(b))} for b in extra]
    entries += [{"hop": h, "rel": rel, "dir": d, "via": via, **node_view(pkg.get(o))} for h, _, rel, d, via, o in nbs]
    pack["neighbors"] = entries

    def size():
        return len(json.dumps(pack, ensure_ascii=False, sort_keys=True))
    if size() > max_chars:
        for nb in pack["neighbors"]:
            if "props" in nb:
                nb["props"] = {k: (v[:200] + "…" if isinstance(v, str) and len(v) > 200 else v) for k, v in nb["props"].items()}
        dropped = 0
        while size() > max_chars and pack["neighbors"]:
            pack["neighbors"].pop()
            dropped += 1
        if dropped:
            pack["notes"].append(f"dropped {dropped} distant or less related neighbours to stay within {max_chars} characters")
        else:
            pack["notes"].append("long neighbour texts were cut")
    pack["allowedCodes"] = sorted({subject, *cond_subjects, *based, *(n["code"] for n in pack["neighbors"])})
    pack["hash"] = stable_hash({k: v for k, v in pack.items() if k != "hash"})[:16]
    return pack
