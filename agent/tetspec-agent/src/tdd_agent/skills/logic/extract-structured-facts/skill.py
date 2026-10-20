from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import facts as F, text as T


class Impl(Skill):
    def run(self, ctx, payload):
        blocks = T.segment(payload["text"])
        out = F.structured_facts(payload["text"], payload["source_id"], str(payload["revision"]), blocks, payload.get("id_class_map"))
        return SkillResult.done(out, gaps=out["gaps"])
