import re

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import facts as F, text as T


_NUM = re.compile(r"(?<![\w.])\d+(?:[.,]\d+)?")


def _definition(el) -> str:
    return " ".join(str(c) for c in el["props"].get("definition", []))


def _numbers_stripped(statement: str) -> str:
    """The wording of a statement with every number and unit removed."""
    return T.normalize_name(T.QUANTITY_RE.sub(" ", _NUM.sub(" ", T.ID_RE.sub(" ", statement))))


def _quantities(statement: str) -> list[dict]:
    """Unit-bearing quantities; a lone bare number (e.g. a batch size) is kept with ``unit = None``."""
    statement = T.ID_RE.sub(" ", statement)  # ids such as AC-4 or FR-12 contain digits that are not quantities
    found = T.quantities(statement)
    if found:
        return found
    bare = [float(n.replace(",", ".")) for n in _NUM.findall(statement)]
    return [{"value": int(b) if b.is_integer() else b, "unit": None, "raw": str(b)} for b in bare[:1]] if len(bare) == 1 else []


def _numbers(statement: str) -> list[str]:
    """Every number in the wording (ids excluded), unit or not: a limit such as ``20.000 component`` carries no recognised unit."""
    return _NUM.findall(T.ID_RE.sub(" ", statement))


def _prose_lines(text: str) -> list[str]:
    return [b.text.strip() for b in T.segment(text) if b.kind in ("paragraph", "list_item") and b.text.strip()]


class Impl(Skill):
    def run(self, ctx, payload):
        b, a = payload["before"], payload["after"]
        fb = F.structured_facts(b["text"], b["source_id"], str(b["revision"]), T.segment(b["text"]), b.get("id_class_map"))
        fa = F.structured_facts(a["text"], a["source_id"], str(a["revision"]), T.segment(a["text"]), a.get("id_class_map"))
        before = {e["id"]: e for e in fb["elements"]}
        after = {e["id"]: e for e in fa["elements"]}
        a_hash = T.sha(a["text"])
        changes = []

        def ref(el, doc):
            line = int(el["provenance"][0]["block"].split(":")[1])
            # the section is addressed by the requirement id (what the change-spec builder matches on); the line is kept as an anchor
            return {"document_id": doc["source_id"], "role": "SRS", "revision": str(doc["revision"]), "content_hash": T.sha(doc["text"]), "section_id": el["id"], "anchor": f"line:{line}"}

        for ident in sorted(after):
            el = after[ident]
            if ident not in before:
                changes.append({"id": ident, "kind": "added", "rationale": f"{ident} is new in revision {a['revision']}", "section_ref": ref(el, a),
                                "after_quote": el["provenance"][0]["quote"], "quantities_after": _quantities(_definition(el))})
            elif before[ident]["provenance"][0]["quote"] != el["provenance"][0]["quote"] and before[ident]["props"].get("definition") != el["props"].get("definition"):
                qb, qa = _quantities(_definition(before[ident])), _quantities(_definition(el))
                same_words = _numbers_stripped(_definition(before[ident])) == _numbers_stripped(_definition(el))
                lb, la = T.limits_in(_definition(before[ident])), T.limits_in(_definition(el))
                caps = [{"unit": u, "before": lb[u], "after": la[u], "direction": "tightened" if la[u] < lb[u] else "loosened"} for u in sorted(set(lb) & set(la)) if lb[u] != la[u]]
                # only the caps changed: with their numbers removed the two statements carry identical numbers
                caps_only = bool(caps) and _numbers(T.strip_limits(_definition(before[ident]))) == _numbers(T.strip_limits(_definition(el)))
                changes.append({"id": ident, "kind": "modified", "rationale": f"{ident} changed between revisions {b['revision']} and {a['revision']}", "section_ref": ref(el, a),
                                "before_quote": before[ident]["provenance"][0]["quote"], "after_quote": el["provenance"][0]["quote"],
                                "quantities_before": qb, "quantities_after": qa, "parameter_only": bool(same_words and _numbers(_definition(before[ident])) != _numbers(_definition(el))),
                                "parameter_changes": caps if caps_only else []})
        for ident in sorted(set(before) - set(after)):
            el = before[ident]
            changes.append({"id": ident, "kind": "removed", "rationale": f"{ident} was removed in revision {a['revision']}", "section_ref": ref(el, b),
                            "before_quote": el["provenance"][0]["quote"]})
        pb, pa = _prose_lines(b["text"]), _prose_lines(a["text"])
        unstructured = sorted(set(pa) ^ set(pb))
        known = {g["id"] for g in fb["gaps"]}
        unmapped = [dict(g, reason=g["reason"] + "; the row is new or changed in this revision, so its change is not part of the structured delta until the prefix is mapped (id_class_map)")
                    for g in fa["gaps"] if g.get("need") == "class_mapping" and g["id"] not in known]
        gaps = unmapped + ([{"id": "GAP-UNSTRUCTURED-SRS-CHANGE", "target": a["source_id"], "need": "structured_delta", "status": "unknown",
                 "reason": "prose lines differ between revisions and have no requirement id; they are not part of the structured delta", "lines": unstructured[:50]}] if unstructured else [])
        doc_ref = {"document_id": a["source_id"], "role": "SRS", "revision": str(a["revision"]), "content_hash": a_hash, "section_id": "delta"}
        out = {"ref": doc_ref, "changes": changes, "conflicts": [], "unstructured": unstructured[:50]}
        if payload.get("cr_ref"):
            out["cr_ref"] = payload["cr_ref"]
        return SkillResult.done(out, gaps=gaps)
