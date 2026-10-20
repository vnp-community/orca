from tdd_agent.skills.base import Skill, SkillResult

SCHEMA = {"type": "object", "required": ["id", "reason"], "additionalProperties": False, "properties": {"id": {"type": ["string", "null"]}, "reason": {"type": "string"}}}


class Impl(Skill):
    def run(self, ctx, payload):
        change, candidates = payload["change"], payload["candidates"]
        listing = "\n".join(f"- {c['id']} [{c.get('class')}] {c.get('name')}: {str(c.get('summary') or '')[:200]}" for c in candidates)
        resp = ctx.llm(f"Changed requirement {change['id']}: {change.get('statement', '')}\n\nCandidate design elements:\n{listing}\n\nWhich one realizes it?", SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        out = resp.parsed
        if out["id"] is not None and out["id"] not in {c["id"] for c in candidates}:
            return SkillResult.blocked("SATISFIER_NOT_A_CANDIDATE", resolved_by="llm", diagnostics={"id": out["id"]})
        if not out["reason"].strip():
            return SkillResult.blocked("SATISFIER_REASON_MISSING", resolved_by="llm")
        return SkillResult.done({"id": out["id"], "reason": out["reason"].strip()}, resolved_by="llm")
