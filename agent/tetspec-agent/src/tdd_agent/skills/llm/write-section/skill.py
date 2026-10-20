import re

from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["mode", "markdown", "summary"], "additionalProperties": False,
          "properties": {"mode": {"enum": ["replace", "append"]}, "markdown": {"type": "string"}, "summary": {"type": "string"}}}
MAX_CHARS = 8000
_NUMBER = re.compile(r"(?<![\w.])\d+(?:[.,]\d+)?(?![\w])")
_LIST_MARK = re.compile(r"^\s*(?:[-*]\s+)?(?:\(?\d+[.)]\s+)", re.M)


def numbers_of(text: str) -> set[str]:
    without_ids = T.ID_RE.sub(" ", text)   # the digits of FR-AST-13 are checked as an id, not as a number
    return {n.replace(",", ".") for n in _NUMBER.findall(_LIST_MARK.sub("", without_ids))}


_REWRITE = re.compile(r"\b(sửa lại|sửa|viết lại|thay thế|thay đổi|thay|rút gọn|làm gọn|gộp|rewrite|replace|shorten|condense|merge|rephrase)\b", re.I)
_ADD = re.compile(r"\b(thêm|bổ sung|ghi thêm|nối thêm|add|append|also)\b", re.I)


def decide_mode(instruction: str, modelled: str) -> str:
    """The person's verb decides whether text is added or the section rewritten when it is clear; the model only breaks the tie."""
    rewrite, add = bool(_REWRITE.search(instruction)), bool(_ADD.search(instruction))
    if rewrite != add:
        return "replace" if rewrite else "append"
    return modelled


class Impl(Skill):
    def run(self, ctx, payload):
        instruction, section, current = payload["instruction"], payload["section"], payload["current"]
        prompt = (f"Section {section.get('number')} \"{section.get('title')}\" of a technical design document.\n\nCurrent content of the section:\n{current or '(empty)'}\n\n"
                  f"User's request:\n{instruction}\n\nWrite the section text as instructed.")
        resp = ctx.llm(prompt, SCHEMA)
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        out = dict(resp.parsed)
        md = out["markdown"].strip()
        given = f"{instruction}\n{current}"
        errors = []
        if not md:
            errors.append("SECTION_TEXT_EMPTY")
        if len(md) > MAX_CHARS:
            errors.append("SECTION_TEXT_TOO_LONG")
        if any(line.lstrip().startswith("#") for line in md.splitlines()):
            errors.append("SECTION_TEXT_HAS_HEADING")
        extra_numbers = sorted(numbers_of(md) - numbers_of(given))
        if extra_numbers:
            errors.append("SECTION_NUMBER_NOT_IN_REQUEST:" + ",".join(extra_numbers[:5]))
        extra_ids = sorted(set(T.ids_in(md)) - set(T.ids_in(given)))
        if extra_ids:
            errors.append("SECTION_ID_NOT_IN_REQUEST:" + ",".join(extra_ids[:5]))
        if errors:
            return SkillResult.blocked(*errors, resolved_by="llm")
        return SkillResult.done({"mode": decide_mode(instruction, out["mode"]), "markdown": md, "summary": out["summary"].strip()}, resolved_by="llm")
