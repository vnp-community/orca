from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T


class Impl(Skill):
    def run(self, ctx, payload):
        blocks = T.segment(payload["text"])
        counts: dict[str, int] = {}
        for b in blocks:
            counts[b.kind] = counts.get(b.kind, 0) + 1
        return SkillResult.done({"source_id": payload["source_id"], "revision": str(payload["revision"]), "role": payload["role"].upper(),
                                 "source_hash": T.sha(payload["text"]), "blocks": [b.as_dict() for b in blocks], "counts": counts})
