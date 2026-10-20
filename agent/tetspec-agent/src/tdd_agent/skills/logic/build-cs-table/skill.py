import copy

from tdd_agent.core import o5_path
from tdd_agent.errors import TddAgentError
from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib.workflow_io import impact_from_dict


class Impl(Skill):
    def run(self, ctx, payload):
        wf = o5_path.load().workflow
        delta = copy.deepcopy(payload["srs_delta"])
        targets, checks = payload.get("code_targets", {}), payload.get("verifications", {})
        gaps = []
        for change in delta.get("changes", []):
            cid = change["id"]
            if targets.get(cid):
                change["code_target"] = targets[cid]
            else:
                gaps.append({"id": f"GAP-{cid}-CODE-TARGET", "target": cid, "need": "code_target", "status": "missing", "reason": "no code target was resolved from the repository snapshot"})
            if checks.get(cid):
                change["verification"] = checks[cid]
            else:
                gaps.append({"id": f"GAP-{cid}-VERIFICATION", "target": cid, "need": "verification", "status": "missing", "reason": "no scenario or acceptance text could be turned into a verification"})
        try:
            table = wf.build_change_spec(delta, payload["tdd_set"], payload["sad"], impact_from_dict(payload["impact"]), payload.get("cr"))
        except wf.WorkflowError as exc:
            return SkillResult.blocked(*[p for p in str(exc).split(";") if p], gaps=gaps)
        return SkillResult.done(table, gaps=gaps)
