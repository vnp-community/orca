import re

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["criteria"], "additionalProperties": False, "properties": {"criteria": {"type": "array", "items": {
    "type": "object", "required": ["change_id", "given", "when", "then", "cr_quote"], "additionalProperties": False,
    "properties": {"change_id": {"type": "string"}, "given": {"type": "string"}, "when": {"type": "string"}, "then": {"type": "string"}, "cr_quote": {"type": "string"}}}}}}
_NUMBER = re.compile(r"(?<![\w.])\d+(?:[.,]\d+)?(?![\w])")


def _squash(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip().lower()


def _numbers(text: str) -> set[str]:
    return {n.replace(",", ".") for n in _NUMBER.findall(T.ID_RE.sub(" ", text))}


class Impl(Skill):
    def run(self, ctx, payload):
        cr, changes = payload["cr_text"], payload["changes"]
        listing = "\n".join(f"- {c['id']} ({c['kind']}): {c.get('statement', '')}" for c in changes)
        resp = ctx.llm(f"Change request:\n{cr}\n\nChanged requirements:\n{listing}\n\nPropose the acceptance criteria.", SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        known = {c["id"] for c in changes}
        given_text = cr + "\n" + "\n".join(c.get("statement", "") for c in changes)
        ok_numbers, ok_ids = _numbers(given_text), set(T.ids_in(given_text))
        flat = _squash(cr)
        kept, errors = [], []
        for i, c in enumerate(resp.parsed["criteria"], 1):
            text = " ".join(c[k] for k in ("given", "when", "then"))
            if c["change_id"] not in known:
                errors.append(f"CRITERION_CHANGE_UNKNOWN:#{i}")
            elif not c["cr_quote"].strip() or _squash(c["cr_quote"]) not in flat:
                errors.append(f"CRITERION_QUOTE_NOT_IN_CR:#{i}")
            elif not all(c[k].strip() for k in ("given", "when", "then")):
                errors.append(f"CRITERION_INCOMPLETE:#{i}")
            elif _numbers(text) - ok_numbers:
                errors.append(f"CRITERION_NUMBER_NOT_IN_CR:#{i}")
            elif set(T.ids_in(text)) - ok_ids:
                errors.append(f"CRITERION_ID_NOT_IN_CR:#{i}")
            else:
                kept.append(c)
        if errors:
            return SkillResult.blocked(*errors, resolved_by="llm")
        return SkillResult.done({"criteria": kept}, resolved_by="llm")
