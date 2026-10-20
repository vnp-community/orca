from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import srs_edit as S

STR_OR_NULL = {"type": ["string", "null"]}
SCHEMA = {"type": "object", "required": ["edits", "unmatched"], "additionalProperties": False, "properties": {
    "edits": {"type": "array", "items": {"type": "object", "additionalProperties": False,
                                         "required": ["op", "id", "find", "replace", "near_id", "statement", "acceptance_ids", "cr_quote", "rationale"],
                                         "properties": {"op": {"enum": ["modify", "add", "remove"]}, "id": STR_OR_NULL, "find": STR_OR_NULL, "replace": STR_OR_NULL, "near_id": STR_OR_NULL,
                                                        "statement": STR_OR_NULL, "acceptance_ids": {"type": "array", "items": {"type": "string"}}, "cr_quote": {"type": "string"},
                                                        "rationale": {"type": "string"}}}},
    "unmatched": {"type": "array", "items": {"type": "object", "additionalProperties": False, "required": ["cr_quote", "reason"], "properties": {"cr_quote": {"type": "string"}, "reason": {"type": "string"}}}}}}


class Impl(Skill):
    def run(self, ctx, payload):
        cr = payload["cr"]
        rows = payload["requirements"]
        shown = S.candidates(rows, cr["text"])
        criteria = "\n".join(f"- {a}: {cr.get('acceptance_text', {}).get(a, '')}".rstrip(": ") for a in cr.get("acceptance_ids", [])) or "(none listed)"
        listing = "\n".join(f"{r['id']}: {r['statement']}" for r in shown)
        prompt = (f"## Change request\n{cr['text']}\n\n## Acceptance criteria of the CR\n{criteria}\n\n## Requirement rows you may touch ({len(shown)} of {len(rows)})\n{listing}\n\n"
                  "Propose the edits (and anything you cannot map) as the schema requests.")
        resp = ctx.llm(prompt, SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        edits = [dict(e) for e in resp.parsed["edits"]]
        shown_ids = {r["id"] for r in shown}
        for e in edits:  # the model may only touch the rows it was shown
            for key in ("id", "near_id"):
                if e.get(key) and e[key] not in shown_ids and any(r["id"] == e[key] for r in rows):
                    return SkillResult.blocked(f"EDIT_TARGET_NOT_OFFERED:{e[key]}", resolved_by="llm")
        errors = S.verify_edits(edits, cr["text"], rows, list(cr.get("acceptance_ids", [])), set(payload.get("known_ids", [])))
        if errors:
            return SkillResult.blocked(*errors, resolved_by="llm")
        return SkillResult.done({"edits": edits, "unmatched": list(resp.parsed["unmatched"])}, resolved_by="llm")
