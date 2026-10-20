from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T


class Impl(Skill):
    def run(self, ctx, payload):
        covered = set(payload.get("covered_lines", []))
        spans = []
        for b in T.prose_candidates(T.segment(payload["text"])):
            if b.line in covered:
                continue
            spans.append({"block": b.id, "line": b.line, "text": b.text, "heading_path": b.heading_path,
                          "quantities": T.quantities(b.text), "ids": T.ids_in(b.text)})
        return SkillResult.done({"spans": spans})
