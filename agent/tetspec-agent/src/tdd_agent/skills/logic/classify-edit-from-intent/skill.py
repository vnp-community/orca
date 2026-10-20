from tdd_agent.skills.base import Skill, SkillResult

PRESENTATION_OPS = {"format", "reorder"}
PRESENTATION_PATHS = ("presentation.", "layout.", "heading", "order", "format")
SEMANTIC_OPS = {"modify", "add", "remove"}


class Impl(Skill):
    def run(self, ctx, payload):
        intent, upstream = payload["intent"], payload.get("upstream") or {}
        op, path = intent.get("operation"), str(intent.get("path") or "")
        reasons = []
        if op == "clarify" or (op in SEMANTIC_OPS and not path):
            return SkillResult.done({"kind": "clarification", "reasons": ["intent has no operation on a concrete property"]})
        names_a_property = bool(path) and not path.startswith(PRESENTATION_PATHS)  # "name", "props.x" ...; a position such as "before X" is not a property value
        if (op in PRESENTATION_OPS or path.startswith(PRESENTATION_PATHS)) and (path.startswith("props.") or (intent.get("value") is not None and op in PRESENTATION_OPS and names_a_property)):
            # a "presentation" operation that carries a value for a design property is a semantic edit in disguise
            return SkillResult.done({"kind": "semantic", "reasons": [f"operation {op!r} carries a value for design property {path!r}; treated as a semantic edit"]})
        if op in PRESENTATION_OPS or path.startswith(PRESENTATION_PATHS):
            return SkillResult.done({"kind": "presentation", "reasons": [f"operation {op!r} / path {path!r} only affects presentation"]})
        if op in SEMANTIC_OPS:
            key = path[len("props."):] if path.startswith("props.") else path
            current = upstream.get(key)
            value = intent.get("value")
            if upstream.get("locked", {}).get(key) is not None and value is not None and value != upstream["locked"][key]:
                reasons.append(f"{key} is pinned upstream to {upstream['locked'][key]!r} by SRS/SAD and the request asks for {value!r}")
                return SkillResult.done({"kind": "conflict", "reasons": reasons, "upstream_value": upstream["locked"][key]})
            reasons.append(f"{op} on {path!r} changes the design content" + (f" (current {current!r})" if current is not None else ""))
            return SkillResult.done({"kind": "semantic", "reasons": reasons})
        return SkillResult.done({"kind": "clarification", "reasons": [f"unsupported operation {op!r}"]})
