"""Format-neutral document model of the standard TDD.

The builder (rules over the knowledge graph) produces a :class:`Document`; writers turn it into
files.  Every table row carries an ``origin`` and its source references, every element a stable
``anchor`` (the element id), so the same model can be written as Markdown or any later format.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Union

ORIGIN_SOURCE = "nguồn"      # taken from a source document (the element's provenance)
ORIGIN_DERIVED = "suy ra"    # computed by a rule from knowledge-graph content


@dataclass(frozen=True)
class SourceRef:
    doc: str
    revision: str
    block: str = ""

    def label(self) -> str:
        line = self.block.split(":", 1)[1] if self.block.startswith("line:") else self.block
        return f"{self.doc}@{self.revision}" + (f" d.{line}" if line else "")


@dataclass(frozen=True)
class Link:
    """Reference to an element anchor; a writer renders plain text when the anchor does not exist."""
    target: str
    label: str | None = None


Cell = Union[str, Link, list]  # a list is a sequence of str | Link pieces


@dataclass
class Row:
    cells: list[Cell]
    anchor: str | None = None
    origin: str = ORIGIN_SOURCE
    refs: list[SourceRef] = field(default_factory=list)
    label: str | None = None  # reader-facing source label ("SRS §5.1"); None = the writer prints the references


@dataclass
class Table:
    header: list[str]
    rows: list[Row]
    caption: str = ""


@dataclass
class Para:
    text: str


@dataclass
class Bullets:
    items: list[Cell]


@dataclass
class Missing:
    """Information the knowledge graph does not have.  Never filled with invented content."""
    what: str
    reason: str
    owner: str = ""
    pointers: list[tuple[str, str]] = field(default_factory=list)  # (source label, heading) of source sections that look related (not content)


@dataclass
class Note:
    text: str


@dataclass
class UserText:
    """Text a person wrote into a section with an edit request (not read from a source document and not computed by a rule)."""
    markdown: str
    instruction: str = ""
    note_id: str = ""


Block = Union[Para, Bullets, Table, Missing, Note, UserText]


@dataclass
class Section:
    number: str
    title: str
    blocks: list = field(default_factory=list)  # Block | Section
    anchor: str | None = None
    also_anchors: list = field(default_factory=list)  # more element ids that live inside this section (written as invisible anchors)


@dataclass
class Chapter(Section):
    key: str = ""


@dataclass
class Document:
    title: str
    tdd_id: str
    chapters: list[Chapter]
    front: "Section | None" = None  # unnumbered introduction (document information, how to read)
    meta: dict = field(default_factory=dict)
    language: str = "vi"


@dataclass
class OutputFile:
    name: str
    content: str
    media_type: str = "text/markdown"


def walk_blocks(node):
    """Yield (section, block) for every leaf block under a chapter/section."""
    for b in node.blocks:
        if isinstance(b, Section):
            yield from walk_blocks(b)
        else:
            yield node, b
