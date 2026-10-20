import json

from tdd_agent.skills.base import Skill, SkillResult

SCHEMA = {"type": "object", "required": ["verdict", "findings"], "additionalProperties": False, "properties": {
    "verdict": {"enum": ["pass", "fail"]},
    "findings": {"type": "array", "items": {"type": "object", "required": ["artifact", "anchor", "reason"], "additionalProperties": False,
                                            "properties": {"artifact": {"type": "string"}, "anchor": {"type": "string"}, "reason": {"type": "string"}}}}}}


class Impl(Skill):
    def run(self, ctx, payload):
        prompt = ("Independently review the proposed OpenSpec artifacts against ONLY the pinned evidence. Do not edit files or use tools.\n"
                  "Reject unsupported statements or dependencies, invented status codes, lost baseline scenarios or GIVEN conditions, wrong capability deltas, generic tasks without "
                  "deliverable/dependency/acceptance, and mandatory facts that are missing.\n\n"
                  f"## Pinned pack\n{json.dumps(payload['pack'], ensure_ascii=False, sort_keys=True)}\n\n"
                  f"## Artifacts\n{json.dumps(payload['artifacts'], ensure_ascii=False, sort_keys=True)}\n\n"
                  f"## Assertions\n{json.dumps(payload['assertions'], ensure_ascii=False)}\n\n## Proposed decisions\n{json.dumps(payload.get('proposed_decisions', []), ensure_ascii=False)}")
        resp = ctx.llm(prompt, SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        findings, bad = [], []
        for f in resp.parsed["findings"]:
            text = payload["artifacts"].get(f["artifact"])
            anchor_ok = f["anchor"] == "document" or (f["anchor"].startswith("line:") and f["anchor"][5:].isdigit() and text is not None
                                                    and 1 <= int(f["anchor"][5:]) <= len(text.splitlines()))
            (findings if text is not None and anchor_ok else bad).append(f)
        outcome = "fail" if (findings or bad or resp.parsed["verdict"] == "fail") else "pass"
        return SkillResult.done({"verdict": outcome, "findings": findings + bad}, resolved_by="llm", diagnostics={"unanchored_findings": len(bad)})
