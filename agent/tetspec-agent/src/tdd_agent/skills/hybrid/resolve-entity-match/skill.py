from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["same", "reason"], "properties": {"same": {"type": "boolean"}, "reason": {"type": "string"}}, "additionalProperties": False}


def _label(el):
    return f"{el.get('id')}: {el.get('name') or ''} ({el.get('class')}) {str((el.get('props') or {}).get('definition', ''))[:240]}"


class Impl(Skill):
    def run(self, ctx, payload):
        a, b = payload["a"], payload["b"]
        if a.get("id") and a.get("id") == b.get("id"):
            return SkillResult.done({"same": a.get("class") == b.get("class"), "reason": "identical ids" if a.get("class") == b.get("class") else "same id but different classes"})
        if a.get("class") != b.get("class"):
            return SkillResult.done({"same": False, "reason": "different classes"})
        na, nb = T.normalize_name(a.get("name", "")), T.normalize_name(b.get("name", ""))
        if na and na == nb:
            return SkillResult.done({"same": True, "reason": "identical normalised names"})
        score = T.jaccard(a.get("name", ""), b.get("name", ""))
        if score <= 0.2:
            return SkillResult.done({"same": False, "reason": f"name overlap {score:.2f} is too low to be the same entity"})
        resp = ctx.llm(f"Element A: {_label(a)}\nElement B: {_label(b)}\nAre A and B the same thing in the design?", SCHEMA, pattern="pair:" + "|".join(sorted([T.normalize_name(a.get("name", "")), T.normalize_name(b.get("name", ""))])))
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        if resp.parsed["same"] and score < 0.4:
            # A merge is lossy: without lexical support the pair stays an open conflict.
            return SkillResult.done({"same": None, "reason": f"model said same but name overlap is only {score:.2f}; kept as an unresolved conflict"}, resolved_by="llm",
                                    gaps=[{"id": f"CONFLICT-{a.get('id')}-{b.get('id')}", "target": a.get("id"), "need": "entity_resolution", "status": "conflict", "reason": resp.parsed["reason"]}])
        return SkillResult.done({"same": bool(resp.parsed["same"]), "reason": resp.parsed["reason"]}, resolved_by="llm")
