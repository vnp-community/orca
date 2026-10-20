"""Writers: Document -> files.  ``md`` is built in; more formats register under the ``tdd_agent.tdd_writers`` entry-point group."""
from __future__ import annotations

import re
from dataclasses import dataclass
from importlib import metadata
from typing import Protocol

from ..errors import ConfigError
from .model import Bullets, Chapter, Document, Link, Missing, Note, OutputFile, Para, Row, Section, Table, UserText
from .template import STRINGS


class TddWriter(Protocol):
    format: str
    extension: str

    def write(self, document: Document) -> list[OutputFile]: ...


def slug(anchor: str) -> str:
    """Stable HTML id for an element id (characters that break markdown/HTML ids are replaced)."""
    s = re.sub(r"[^A-Za-z0-9._:-]", "_", anchor)
    return s


def anchors_of(document: Document) -> dict[str, str]:
    """element id -> unique html id (collisions after sanitizing get a numeric suffix, deterministically)."""
    out: dict[str, str] = {}
    used: set[str] = set()

    def visit(node) -> None:
        if getattr(node, "anchor", None):
            take(node.anchor)
        for a in getattr(node, "also_anchors", []):
            take(a)
        for b in node.blocks:
            if isinstance(b, Section):
                visit(b)
            elif isinstance(b, Table):
                for r in b.rows:
                    if r.anchor:
                        take(r.anchor)

    def take(a: str) -> None:
        if a in out:
            return
        s = slug(a)
        base, k = s, 2
        while s in used:
            s, k = f"{base}-{k}", k + 1
        used.add(s)
        out[a] = s

    for ch in document.chapters:
        visit(ch)
    if document.front is not None:
        visit(document.front)
    return out


class MarkdownWriter:
    format = "md"
    extension = ".md"

    def write(self, document: Document) -> list[OutputFile]:
        self.doc = document
        self.ids = anchors_of(document)
        self.strings = STRINGS[document.language]
        lines: list[str] = []
        m = document.meta
        lines.append(f"# {document.title} — {document.tdd_id}")
        lines.append("")
        lines.append(f"> {self.strings['reader_note']}")
        lines.append("")
        lines.append(f"*Phiên bản {m.get('revision') or '—'} · sinh từ bản dữ liệu `{m.get('snapshot') or '—'}`*")
        lines.append("")
        lines.append(f"## {self.strings['toc']}")
        lines.append("")
        if document.front is not None:
            lines.append(f"- {document.front.title}")
            for sec in (b for b in document.front.blocks if isinstance(b, Section)):
                lines.append(f"  - {sec.number} {sec.title}")
        for ch in document.chapters:
            lines.append(f"- [{ch.number}. {ch.title}](#{self.ids[ch.anchor]})")
            for sec in (b for b in ch.blocks if isinstance(b, Section)):
                lines.append(f"  - {sec.number} {sec.title}")
        lines.append("")
        if document.front is not None:
            lines.append(f"## {document.front.title}")
            lines.append("")
            self.blocks(document.front, lines, 3)
        for ch in document.chapters:
            lines.append(f'<a id="{self.ids[ch.anchor]}"></a>')
            lines.append(f"## {ch.number}. {ch.title}")
            lines.append("")
            self.blocks(ch, lines, 3)
        text = "\n".join(lines).rstrip() + "\n"
        return [OutputFile(f"{document.tdd_id}{self.extension}", text)]

    # ---- blocks
    def blocks(self, node: Section, lines: list[str], level: int) -> None:
        for b in node.blocks:
            if isinstance(b, Section):
                lines.extend(f'<a id="{self.ids[a]}"></a>' for a in ([b.anchor] if b.anchor else []) + list(b.also_anchors))
                lines.append(f"{'#' * min(level, 6)} {b.number} {b.title}")
                lines.append("")
                self.blocks(b, lines, level + 1)
            elif isinstance(b, Para):
                if b.text:
                    lines.append(b.text)
                    lines.append("")
            elif isinstance(b, Note):
                lines.append(f"*{b.text}*")
                lines.append("")
            elif isinstance(b, UserText):
                lines.append(f"*{self.strings['user_text']}*" + (f" “{' '.join(b.instruction.split())[:140]}”" if b.instruction else ""))
                lines.append("")
                lines.extend(b.markdown.strip().splitlines())
                lines.append("")
            elif isinstance(b, Bullets):
                for item in b.items:
                    lines.append(f"- {self.cell(item)}")
                lines.append("")
            elif isinstance(b, Missing):
                lines.extend(self.missing(b))
            elif isinstance(b, Table):
                lines.extend(self.table(b))

    def missing(self, b: Missing) -> list[str]:
        tag = self.strings["missing"]
        out = [f"> **[{tag}] {b.what}.** {b.reason}" + (f" *Cần xác nhận bởi: {b.owner}.*" if b.owner else "")]
        if b.pointers:
            out.append(">")
            out.append("> Mục nguồn có thể liên quan (chỉ nêu tiêu đề; nội dung chưa được trích): " + "; ".join(f"{label} “{t}”" for label, t in b.pointers))
        out.append("")
        return out

    def table(self, t: Table) -> list[str]:
        show_source = any(r.origin == "nguồn" or r.label or r.refs for r in t.rows)
        header = list(t.header) + ([self.strings["origin"]] if show_source else [])
        out = ["| " + " | ".join(header) + " |", "|" + "|".join(["---"] * len(header)) + "|"]
        for r in t.rows:
            cells = [self.cell(c) for c in r.cells]
            if r.anchor and cells:
                cells[0] = f'<a id="{self.ids[r.anchor]}"></a>' + cells[0]
            out.append("| " + " | ".join(cells + ([self.source(r)] if show_source else [])) + " |")
        out.append("")
        return out

    def source(self, r: Row) -> str:
        if r.label:
            return self.esc(r.label)
        if r.refs:
            shown = [x.label() for x in r.refs[:3]]
            more = f" +{len(r.refs) - 3}" if len(r.refs) > 3 else ""
            return ", ".join(shown) + more
        return "tổng hợp"

    # ---- cells
    def cell(self, c) -> str:
        if isinstance(c, list):
            return "".join(self.cell(p) for p in c)
        if isinstance(c, Link):
            label = self.esc(c.label or c.target)
            return f"[{label}](#{self.ids[c.target]})" if c.target in self.ids else label
        return self.esc(str(c))

    @staticmethod
    def esc(s: str) -> str:
        return s.replace("|", "\\|").replace("\r", "").replace("\n", "<br>")


_BUILTIN: dict[str, type] = {"md": MarkdownWriter}


def get_writer(fmt: str) -> TddWriter:
    if fmt in _BUILTIN:
        return _BUILTIN[fmt]()
    for ep in metadata.entry_points(group="tdd_agent.tdd_writers"):
        if ep.name == fmt:
            return ep.load()()
    raise ConfigError(f"unknown TDD output format {fmt!r}; built in: {sorted(_BUILTIN)}", code="RENDER_FORMAT_UNKNOWN")
