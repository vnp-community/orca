from pathlib import Path

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import repo as R, text as T

SCHEMA = {"type": "object", "required": ["index", "reason"], "properties": {"index": {"type": ["integer", "null"]}, "reason": {"type": "string"}}, "additionalProperties": False}


class Impl(Skill):
    def run(self, ctx, payload):
        change, root = payload["change"], Path(payload["repo_root"])
        if not root.is_dir():
            return SkillResult.blocked("REPO_ROOT_MISSING")
        tokens = sorted({t for hint in payload.get("name_hints", []) for t in R.split_identifier(hint)})
        candidates = R.scan(root, [change["id"]], tokens)
        if not candidates:
            gap = {"id": f"GAP-{change['id']}-CODE-TARGET", "target": change["id"], "need": "code_target", "status": "missing",
                   "reason": "no file in the repository snapshot references the id or has a matching symbol name"}
            return SkillResult.done({"code_target": None, "candidates": []}, gaps=[gap])
        literal = [c for c in candidates if c["match"].startswith("literal") and c["symbol"]]
        if len(candidates) == 1 or (len(literal) == 1 and len({(c["path"], c["symbol"]) for c in literal}) == 1):
            pick = literal[0] if literal else candidates[0]
            return SkillResult.done({"code_target": f"{pick['path']}::{pick['symbol'] or '<module>'}", "candidates": candidates[:10]})
        listing = "\n".join(f"{i}. {c['path']}::{c['symbol']} (line {c['line']}, matched by {c['match']})" for i, c in enumerate(candidates[:20]))
        resp = ctx.llm(f"Requirement {change['id']}: {change.get('after_quote') or change.get('rationale') or ''}\nCandidate code locations:\n{listing}\nWhich one implements this requirement? Answer with its number or null.", SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        i = resp.parsed["index"]
        if i is None:
            gap = {"id": f"GAP-{change['id']}-CODE-TARGET", "target": change["id"], "need": "code_target", "status": "missing", "reason": resp.parsed["reason"]}
            return SkillResult.done({"code_target": None, "candidates": candidates[:20]}, resolved_by="llm", gaps=[gap])
        if not 0 <= i < min(len(candidates), 20):
            return SkillResult.blocked("CHOICE_NOT_IN_CANDIDATES", resolved_by="llm")
        pick = candidates[i]
        return SkillResult.done({"code_target": f"{pick['path']}::{pick['symbol'] or '<module>'}", "candidates": candidates[:20]}, resolved_by="llm")
