from collections import deque

from tdd_agent.skills.base import Skill, SkillResult

TRACE_RELS = {"traces_to", "satisfies", "has_scenario", "derives_from", "implemented_by", "references_entity", "constrained_by"}
REQ_CLASSES = {"Requirement", "RequirementVersion"}


def _props(el):
    return el.get("props") or {}


def _tdd_of(el, tdd_ids):
    owner = el.get("owner_doc") or _props(el).get("owner_doc")
    if owner in tdd_ids:
        return owner
    for t in el.get("tdds") or _props(el).get("tdds") or []:
        if t in tdd_ids:
            return t
    return None


def _bounds(el):
    p = _props(el)
    rng = p.get("range") or p.get("bounds") or {}
    lo = rng.get("min", p.get("min"))
    hi = rng.get("max", p.get("max"))
    return (lo, hi) if (lo is not None or hi is not None) else None


class Impl(Skill):
    def run(self, ctx, payload):
        hops = payload.get("hops", 3)
        by_id = {e["id"]: e for e in payload["elements"]}
        tdd_ids = {t["id"] for t in payload["tdd_docs"]}
        adj: dict[str, set[str]] = {}
        for e in payload["edges"]:
            if e.get("rel") in TRACE_RELS:
                adj.setdefault(e["src"], set()).add(e["dst"]); adj.setdefault(e["dst"], set()).add(e["src"])
        by_tdd: dict[str, list[dict]] = {t: [] for t in tdd_ids}
        for el in payload["elements"]:
            t = _tdd_of(el, tdd_ids)
            if t:
                by_tdd[t].append(el)
        has_trace = {t: any(any(by_id.get(n, {}).get("class") in REQ_CLASSES for n in adj.get(el["id"], ())) for el in els) for t, els in by_tdd.items()}

        def node_for(change):
            if change["id"] in by_id:
                return change["id"]
            for el in payload["elements"]:
                if _props(el).get("logical_id") == change["id"]:
                    return el["id"]
            return None

        dist_by_change: dict[str, dict[str, int]] = {}
        for c in payload["changes"]:
            root = node_for(c)
            dist: dict[str, int] = {}
            if root:
                dist[root] = 0
                q = deque([root])
                while q:
                    cur = q.popleft()
                    if dist[cur] >= hops:
                        continue
                    for n in adj.get(cur, ()):
                        if n not in dist:
                            dist[n] = dist[cur] + 1; q.append(n)
            dist_by_change[c["id"]] = dist

        tdds, covered_any = [], set()
        for doc in sorted(payload["tdd_docs"], key=lambda d: d["id"]):
            t = doc["id"]
            direct, indirect, kinds, evidence, bound_notes = [], [], {}, [], []
            for c in payload["changes"]:
                dist = dist_by_change[c["id"]]
                near = sorted((max(dist[el["id"]], 1), el["id"]) for el in by_tdd[t] if el["id"] in dist)
                if not near:
                    continue
                d0 = near[0][0]
                (direct if d0 == 1 else indirect).append(c["id"])
                evidence.append({"change": c["id"], "distance": d0, "elements": [i for _, i in near[:5]]})
                kind = c.get("kind", "modified")
                caps = c.get("parameter_changes") or []
                if kind == "modified" and c.get("parameter_only") and caps and d0 == 1:
                    # a cap the design was made for (the baseline statement is part of this TDD): lowering it stays inside that envelope
                    if all(x["direction"] == "tightened" for x in caps):
                        kind = "parameter_within_bounds"
                    else:
                        bound_notes.append({"change": c["id"], "caps": [x for x in caps if x["direction"] == "loosened"]})
                elif kind == "modified" and c.get("parameter_only") and d0 == 1 and c.get("quantities_after"):
                    value = c["quantities_after"][0]["value"]
                    checks = [_bounds(el) for el in by_tdd[t] if el["id"] in dist and dist[el["id"]] == 1]
                    checks = [b for b in checks if b]
                    if checks and all((b[0] is None or value >= b[0]) and (b[1] is None or value <= b[1]) for b in checks):
                        kind = "parameter_within_bounds"
                kinds[c["id"]] = kind
            covered_any.update(direct)
            if direct:
                verdict = "covered"
                reason = None
            elif indirect or not has_trace.get(t):
                verdict = "residue"
                reason = ("no trace edges link this TDD to any requirement, so independence from the delta cannot be proven" if not indirect
                          else f"only indirect trace paths (>1 hop) connect this TDD to {sorted(set(indirect))}")
            else:
                verdict = "no_overlap"
                reason = (f"none of the {len(by_tdd[t])} design element(s) of {t} is within {hops} trace hops of the changed requirement(s) "
                          f"{[c['id'] for c in payload['changes']]}; the TDD has requirement trace edges so the absence of a path is meaningful")
            item = {"id": t, "revision": str(doc.get("revision", "")), "content_hash": doc.get("content_hash", ""), "sections": list(doc.get("sections", [])),
                    "covered_srs_ids": sorted(set(direct)), "affected_srs_ids": [], "evidence": list(doc.get("evidence", []))}
            tdds.append({"id": t, "verdict": verdict, "inventory_item": item, "change_kinds": kinds, "rule_reason": reason,
                         "overlap": evidence, "bound_notes": bound_notes, "residue_changes": sorted(set(indirect) - set(direct)), "element_count": len(by_tdd[t])})
        uncovered = sorted(c["id"] for c in payload["changes"] if c["id"] not in covered_any and c.get("kind") != "removed")
        return SkillResult.done({"tdds": tdds, "delta_uncovered": uncovered})
