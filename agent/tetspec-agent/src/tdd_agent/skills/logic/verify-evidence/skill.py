from tdd_agent.core import o5_path
from tdd_agent.skills.base import Skill, SkillResult


class Impl(Skill):
    def run(self, ctx, payload):
        o5 = o5_path.load()
        sources = [{"id": s["id"], "version": str(s.get("version", s.get("revision"))), "content": s["content"] if "content" in s else s["text"]} for s in payload["sources"]]
        results = [{"index": i, "errors": o5.verify_evidence(r, sources)} for i, r in enumerate(payload["records"])]
        return SkillResult.done({"ok": all(not r["errors"] for r in results), "results": results})
