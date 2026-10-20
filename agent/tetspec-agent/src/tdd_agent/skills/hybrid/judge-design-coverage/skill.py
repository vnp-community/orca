from tdd_agent.skills.base import Skill, SkillResult

SCHEMA = {"type": "object", "required": ["covered", "cited", "reason"],
          "properties": {"covered": {"type": "boolean"}, "cited": {"type": "array", "items": {"type": "string"}}, "reason": {"type": "string"}}, "additionalProperties": False}
MAX_DISTANCE = 3


def _brief(s):
    p = s.get("props") or {}
    text = p.get("statement") or p.get("summary") or p.get("definition") or s.get("summary") or ""
    return f"- {s['id']} [{s.get('class')}] {s.get('name') or ''}: {str(text)[:300]}"


class Impl(Skill):
    def run(self, ctx, payload):
        change, sections = payload["change"], payload["sections"]
        if not sections:
            return SkillResult.done({"decision": "update", "cited": [], "reason": "the TDD has no design elements to rely on"})
        prompt = (f"Changed requirement {change['id']} ({change.get('kind')}): {change.get('after_quote') or change.get('before_quote') or ''}\n"
                  f"Existing design of {payload['tdd_id']}:\n" + "\n".join(_brief(s) for s in sections[:60]) +
                  "\nDoes the existing design already satisfy the changed requirement without any edit? Cite the element ids that prove it.")
        resp = ctx.llm(prompt, SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        ids = {s["id"] for s in sections}
        dist = payload.get("distance_by_id") or {}
        cited = list(dict.fromkeys(resp.parsed["cited"]))
        verified = bool(resp.parsed["covered"]) and bool(cited) and all(c in ids for c in cited) and all(dist.get(c, MAX_DISTANCE) <= MAX_DISTANCE for c in cited)
        if resp.parsed["covered"] and not verified:
            reason = "citations were missing, unknown or not within trace distance; treated as needing an update"
        else:
            reason = resp.parsed["reason"]
        return SkillResult.done({"decision": "no_change" if verified else "update", "cited": cited if verified else [], "reason": reason}, resolved_by="llm")
