from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["target_mention", "operation", "path", "value", "unit", "source_ref", "notes"], "additionalProperties": False,
          "properties": {"target_mention": {"type": "string"}, "operation": {"enum": ["modify", "add", "remove", "reorder", "format", "clarify"]},
                         "path": {"type": ["string", "null"]}, "value": {"type": ["string", "number", "boolean", "array", "object", "null"]}, "unit": {"type": ["string", "null"]},
                         "source_ref": {"type": ["string", "null"]}, "notes": {"type": "string"}}}
PATH_PREFIXES = ("props.", "presentation.", "layout.")


class Impl(Skill):
    def run(self, ctx, payload):
        prompt = payload["prompt"]
        known = payload.get("known_paths") or []
        resp = ctx.llm(f"User edit request:\n{prompt}\n\nKnown property paths: {known}\nExtract the structured intent.", SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        intent = dict(resp.parsed)
        path = intent["path"]
        if path is not None and not (path in ("name", "summary", "heading", "order", "format") or path.startswith(PATH_PREFIXES)):
            return SkillResult.blocked("INTENT_PATH_INVALID", resolved_by="llm", diagnostics={"path": path})
        if intent["operation"] in ("modify", "add") and isinstance(intent["value"], (int, float)) and not isinstance(intent["value"], bool):
            if not T.number_supported(prompt, intent["value"]):
                return SkillResult.blocked("INTENT_VALUE_NOT_IN_PROMPT", resolved_by="llm", diagnostics={"value": intent["value"]})
        if intent["unit"] and not any(q["unit"] == T.UNITS.get(intent["unit"].lower(), intent["unit"]) for q in T.quantities(prompt)) and isinstance(intent["value"], (int, float)):
            if intent["unit"] not in prompt:
                intent["unit"] = None  # a unit the prompt never mentioned is dropped, not trusted
        return SkillResult.done({k: intent[k] for k in SCHEMA["required"]}, resolved_by="llm")
