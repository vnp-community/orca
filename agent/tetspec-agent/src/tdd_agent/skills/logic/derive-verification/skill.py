from tdd_agent.skills.base import Skill, SkillResult


class Impl(Skill):
    def run(self, ctx, payload):
        parts, sources = [], []
        for sc in sorted(payload["scenarios"], key=lambda s: str(s.get("id"))):
            p = sc.get("props", sc)
            given, when, then = (str(p.get(k) or "").strip() for k in ("given", "when", "then"))
            if when and then:
                parts.append(f"{'GIVEN ' + given + ' ' if given else ''}WHEN {when} THEN {then}")
                sources.append(str(sc.get("id")))
        if not parts:
            gap = {"id": f"GAP-{payload['change_id']}-VERIFICATION", "target": payload["change_id"], "need": "verification", "status": "missing",
                   "reason": "no scenario with WHEN and THEN is linked to this change"}
            return SkillResult.done({"verification": None, "sources": []}, gaps=[gap])
        for a in payload.get("acceptance", []):
            sources.append(str(a))
        return SkillResult.done({"verification": "; ".join(parts), "sources": sources})
