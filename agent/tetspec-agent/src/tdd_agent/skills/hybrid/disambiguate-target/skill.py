from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["id", "reason"], "properties": {"id": {"type": ["string", "null"]}, "reason": {"type": "string"}}, "additionalProperties": False}


class Impl(Skill):
    def run(self, ctx, payload):
        mention, cands = payload["mention"], payload["candidates"]
        if not cands:
            return SkillResult.done({"id": None, "reason": "no candidate matches the mention; ask the user which element is meant", "considered": []})
        ids = [c for c in cands if c["id"] in T.ids_in(mention + " " + payload.get("context", "")) or c["id"].lower() in (mention + " " + payload.get("context", "")).lower()]
        norm_mention = T.normalize_name(mention + " " + payload.get("context", ""))
        named = [c for c in cands if T.normalize_name(c.get("name", "")) and T.normalize_name(c["name"]) in norm_mention]
        if len(ids) == 1 and any(c["id"] != ids[0]["id"] for c in named) and ids[0] not in named:
            other = next(c for c in named if c["id"] != ids[0]["id"])
            return SkillResult.done({"id": None, "mismatch": True, "considered": [ids[0]["id"], other["id"]],
                                     "reason": f"target mismatch: {ids[0]['id']} is '{ids[0].get('name')}' but the prompt describes '{other.get('name')}' ({other['id']})"})
        if len(ids) == 1:
            return SkillResult.done({"id": ids[0]["id"], "reason": "the prompt names the id explicitly", "considered": [c["id"] for c in cands]})
        norm = T.normalize_name(mention)
        exact = [c for c in cands if T.normalize_name(c.get("name", "")) and T.normalize_name(c["name"]) in norm]
        if len(exact) == 1:
            return SkillResult.done({"id": exact[0]["id"], "reason": "exactly one candidate name occurs in the prompt", "considered": [c["id"] for c in cands]})
        pool = exact or ids or cands
        if len(pool) == 1:
            return SkillResult.done({"id": pool[0]["id"], "reason": "a single candidate remains", "considered": [c["id"] for c in cands]})
        listing = "\n".join(f"- {c['id']}: {c.get('name')} [{c.get('class')}] {str(c.get('summary') or '')[:160]}" for c in pool[:25])
        resp = ctx.llm(f"The user wrote: {mention}\nCandidates:\n{listing}\nWhich element does the user mean? Answer with its id or null if it cannot be decided.", SCHEMA, pattern="mention:" + norm[:80])
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        chosen = resp.parsed["id"]
        if chosen is not None and chosen not in {c["id"] for c in pool[:25]}:
            return SkillResult.blocked("CHOICE_NOT_IN_CANDIDATES", resolved_by="llm")
        return SkillResult.done({"id": chosen, "reason": resp.parsed["reason"], "considered": [c["id"] for c in pool[:25]]}, resolved_by="llm")
