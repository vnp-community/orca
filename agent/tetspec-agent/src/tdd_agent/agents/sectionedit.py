"""Free-form edits of one section of the TDD ("sửa mục 1.3 …", "rewrite section 2.4 …").

The person names a section in words; the model writes the section text from the request and the section's current text (``write-section``); rules check that it adds
no number or id the request did not give.  The text is kept in the knowledge graph as an element of class ``Section`` (id ``NOTE-…``) (provenance: the prompt), so it survives regeneration, is
reviewed like any element, and can be undone.  The renderer writes it into the section, marked as person-written.
"""
from __future__ import annotations

import re

from ..skills.lib import text as T

SECTION_WORDS = r"(?:mục|phần|section|sec|chương|part|§|phụ lục|appendix)"
_NUMBER_REF = re.compile(rf"(?<!\w){SECTION_WORDS}\s*\.?\s*((?:\d+|[A-Z])(?:\.\d+)+)", re.I)
_HEADING = re.compile(r"^(#{3,6})\s+((?:\d+|[A-Z])(?:\.\d+)+)\s+(.+?)\s*$")


def note_id(tdd_id: str, number: str) -> str:
    return f"NOTE-{tdd_id}-{number.replace('.', '-')}"


def sections_of(text: str) -> dict[str, dict]:
    """Sections of a rendered TDD: number -> {title, start, end, level} (end is the line of the next heading of any level)."""
    lines = text.splitlines()
    heads = [(i, _HEADING.match(l)) for i, l in enumerate(lines)]
    heads = [(i, m) for i, m in heads if m]
    out: dict[str, dict] = {}
    for n, (i, m) in enumerate(heads):
        nxt = next((j for j in range(i + 1, len(lines)) if re.match(r"^#{1,6}\s", lines[j]) or lines[j].startswith('<a id="ch-')), len(lines))
        out[m.group(2)] = {"title": m.group(3), "start": i, "end": nxt, "level": len(m.group(1))}
    return out


def section_text(text: str, info: dict) -> str:
    lines = text.splitlines()[info["start"] + 1:info["end"]]
    return "\n".join(l for l in lines if not l.startswith('<a id="')).strip()


def find_section(prompt: str, text: str) -> tuple[str, dict] | tuple[None, str]:
    """The section a prompt names: by number (\"mục 1.3\"), else by exact title (\"mục Phi mục tiêu\").  ``(None, reason)`` when it names none or an unknown one."""
    sections = sections_of(text)
    m = _NUMBER_REF.search(prompt)
    if m:
        number = m.group(1).upper()
        if number in sections:
            return number, sections[number]
        return None, f"the document has no section {number}."
    norm = T.normalize_name(prompt)
    hits = [(n, i) for n, i in sections.items() if len(T.normalize_name(i["title"]).split()) >= 2 and re.search(rf"(?:^| ){re.escape(T.normalize_name(i['title']))}(?: |$)", norm)
            and re.search(rf"(?<!\w){SECTION_WORDS}\b", prompt, re.I)]
    if len(hits) == 1:
        return hits[0][0], hits[0][1]
    if len(hits) > 1:
        return None, "the title matches several sections: " + ", ".join(n for n, _ in hits)
    return None, ""


def merge_note(existing: dict | None, mode: str, markdown: str) -> tuple[str, str]:
    """Adding to a section that already carries a person's text keeps that text; rewriting replaces it."""
    if existing and mode == "append":
        old = existing.get("props") or {}
        return str(old.get("mode") or "append"), (str(old.get("markdown") or "").rstrip() + "\n\n" + markdown.strip()).strip()
    return mode, markdown.strip()
