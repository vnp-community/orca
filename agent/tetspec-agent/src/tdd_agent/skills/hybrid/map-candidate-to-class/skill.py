import re

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

HEADING_RULES = [
    (r"api|endpoint|interface|giao diện|giao tiếp", "Operation"), (r"business rule|quy tắc|nghiệp vụ", "BusinessRule"),
    (r"nfr|non-functional|quality|performance|hiệu năng|chất lượng", "QualityTarget"), (r"data model|entity|schema|dữ liệu|bảng", "Entity"),
    (r"decision|adr|quyết định", "Decision"), (r"state|lifecycle|trạng thái", "StateMachine"), (r"error|lỗi", "ErrorCode"),
    (r"constraint|ràng buộc", "Constraint"), (r"requirement|yêu cầu", "Requirement"),
]
TEXT_RULES = [(T.METHOD_PATH_RE, "Operation")]
SCHEMA = {"type": "object", "required": ["class", "reason"], "properties": {"class": {"type": ["string", "null"]}, "reason": {"type": "string"}}, "additionalProperties": False}


class Impl(Skill):
    def run(self, ctx, payload):
        cand, allowed = payload["candidate"], payload["allowed_classes"]
        heading = " > ".join(cand.get("heading_path", [])).lower()
        hits: list[str] = []
        for pattern, cls in HEADING_RULES:
            if re.search(pattern, heading, re.I) and cls in allowed and cls not in hits:
                hits.append(cls)
        for regex, cls in TEXT_RULES:
            if regex.search(cand.get("text", "")) and cls in allowed and cls not in hits:
                hits.append(cls)
        if len(hits) == 1:
            return SkillResult.done({"class": hits[0], "shortlist": hits, "reason": "a single heading/keyword rule matched"})
        shortlist = hits or sorted(allowed)[:8]
        if payload.get("rules_only"):
            return SkillResult.done({"class": None, "shortlist": shortlist, "reason": "ambiguous by rule; the model was not consulted (rules_only)"})
        prompt = (f"Candidate line (anchor {cand.get('block')}):\n{cand.get('text')}\nHeading path: {heading or '(none)'}\n"
                  f"Allowed classes: {shortlist}\nChoose the single class this line defines, or null if it defines none of them.")
        resp = ctx.llm(prompt, SCHEMA, pattern="heading:" + (heading or "-")[:80] + "|shortlist:" + ",".join(shortlist))
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        chosen = resp.parsed["class"]
        if chosen is not None and chosen not in shortlist:
            return SkillResult.blocked("CLASS_NOT_IN_SHORTLIST", resolved_by="llm", diagnostics={"chosen": chosen, "shortlist": shortlist})
        gaps = [] if chosen else [{"id": f"GAP-{cand.get('block')}-CLASS", "target": cand.get("block"), "need": "class_mapping", "status": "unknown", "reason": resp.parsed["reason"]}]
        return SkillResult.done({"class": chosen, "shortlist": shortlist, "reason": resp.parsed["reason"]}, resolved_by="llm", gaps=gaps)
