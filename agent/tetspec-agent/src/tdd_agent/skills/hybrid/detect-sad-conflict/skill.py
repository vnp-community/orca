import re

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["suspected"], "properties": {"suspected": {"type": "array", "items": {"type": "object", "required": ["sad_element", "sad_quote", "change_id", "reason"],
          "properties": {"sad_element": {"type": "string"}, "sad_quote": {"type": "string"}, "change_id": {"type": "string"}, "reason": {"type": "string"}}, "additionalProperties": False}}}, "additionalProperties": False}


def _p(el):
    return el.get("props") or {}


def _words(value) -> set[str]:
    return {w for w in T.normalize_name(str(value)).split() if len(w) > 2}


def _tokens(value) -> list[str]:
    return T.normalize_name(str(value)).split()


def _alternatives(raw) -> list[list[str]]:
    """The rejected alternatives a decision lists, each as the words of one whole name.

    Notes in brackets are dropped, the list is split at commas, semicolons and "or"/"hoặc" but never at "/", and an item that is a path or
    a code reference (``app/tms``, ``src/a.py``) is not an alternative at all.
    """
    text = re.sub(r"[(\[{][^)\]}]*[)\]}]", " ", str(raw))
    out = []
    for item in re.split(r"[,;]|\s(?:or|hoặc|hay)\s", text, flags=re.I):
        item = item.strip()
        if not item or re.search(r"\S/\S", item):
            continue
        toks = _tokens(item)
        if toks and (len(toks) > 1 or len(toks[0]) > 3):
            out.append(toks)
    return out


def _contains(quote: list[str], phrase: list[str]) -> bool:
    n = len(phrase)
    return any(quote[i:i + n] == phrase for i in range(len(quote) - n + 1))


class Impl(Skill):
    def run(self, ctx, payload):
        findings = []
        changes = {c["id"]: c for c in payload["changes"]}
        for el in payload["sad_elements"]:
            p = _p(el)
            chosen, alternatives = _words(p.get("chosen", "")), _alternatives(p.get("alternatives", ""))
            for c in changes.values():
                quote = (c.get("after_quote") or "").lower()
                for alt in alternatives:
                    if _contains(_tokens(quote), alt) and not (chosen and chosen <= _words(quote)):
                        findings.append({"code": "SAD_CONFLICT", "origin": "rule", "sad_element": el["id"], "change_id": c["id"],
                                         "message": f"{c['id']} asks for '{' '.join(alt)}', which {el['id']} lists as a rejected alternative to '{p.get('chosen')}'"})
                lim = p.get("limit") if isinstance(p.get("limit"), dict) else {}
                limit = p.get("max") if p.get("max") is not None else lim.get("max")
                unit = p.get("unit") or lim.get("unit")
                if limit is not None:
                    for q in c.get("quantities_after", []):
                        if (unit is None or q["unit"] == unit) and q["value"] > limit:
                            findings.append({"code": "SAD_CONFLICT", "origin": "rule", "sad_element": el["id"], "change_id": c["id"],
                                             "message": f"{c['id']} requires {q['raw']} but {el['id']} caps the value at {limit} {unit or ''}".strip()})
        prose = [e for e in payload["sad_elements"] if e.get("class") in ("Constraint", "Decision") and (_p(e).get("definition") or _p(e).get("rationale"))]
        live = [c for c in payload["changes"] if c.get("kind") != "removed" and c.get("after_quote")]
        resolved_by = "rule"
        if prose and live:
            body = "\n".join(f"- {e['id']}: " + " | ".join(str(x) for x in (_p(e).get("definition") or [_p(e).get("rationale")])) for e in prose[:40])
            asks = "\n".join(f"- {c['id']} ({c['kind']}): {c['after_quote']}" for c in live[:40])
            resp = ctx.llm(f"Architecture constraints (read-only):\n{body}\n\nChanged requirements:\n{asks}\n\nList every changed requirement that contradicts an architecture constraint.", SCHEMA)
            resolved_by = "llm"
            if not resp.ok:
                return SkillResult.blocked(*resp.blockers, resolved_by="llm")
            elements = {e["id"] for e in prose}
            sad_text = payload["sad_source"].get("text", "")
            for s in resp.parsed["suspected"]:
                if s["sad_element"] in elements and s["change_id"] in changes and s["sad_quote"].strip() and s["sad_quote"].strip() in sad_text:
                    findings.append({"code": "SAD_CONFLICT_SUSPECTED", "origin": "llm", "sad_element": s["sad_element"], "change_id": s["change_id"],
                                     "message": s["reason"], "sad_quote": s["sad_quote"].strip()})
        return SkillResult.done({"findings": findings}, resolved_by=resolved_by)
