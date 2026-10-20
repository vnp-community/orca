"""Split md into blocks with line numbers: front matter, headings, paragraphs, tables (markdown-it-py)."""
import os
from dataclasses import dataclass, field

import yaml
from markdown_it import MarkdownIt

from ...errors import Diagnostic

_MD = MarkdownIt("commonmark").enable("table")


@dataclass
class Heading:
    level: int
    text: str
    line: int


@dataclass
class Table:
    header: list
    rows: list
    lines: list  # line numbers (1-based) of each row
    line: int


@dataclass
class Block:
    kind: str  # heading | para | table
    line: int
    heading: Heading = None
    text: str = ""
    table: Table = None


@dataclass
class MdDoc:
    name: str
    front: dict = field(default_factory=dict)
    blocks: list = field(default_factory=list)
    diags: list = field(default_factory=list)


def _front(text, name):
    if not text.startswith("---"):
        return {}, text, 0, []
    end = text.find("\n---", 3)
    if end < 0:
        return {}, text, 0, [Diagnostic(name, "1", "MD-FRONT", "front matter has no closing --- line")]
    raw = text[3:end]
    body_start = text.count("\n", 0, end + 4) + 1
    try:
        front = yaml.safe_load(raw) or {}
    except yaml.YAMLError as e:
        return {}, text, 0, [Diagnostic(name, "1", "MD-FRONT", f"invalid front matter YAML: {e}")]
    if not isinstance(front, dict):
        front = {}
    # replace the front matter with blank lines so body line numbers stay the same
    return front, "\n" * body_start + text[text.find("\n", end + 1) + 1:], body_start, []


def parse_md(text, name="<memory>"):
    front, body, _, diags = _front(text, name)
    doc = MdDoc(name, {str(k): v for k, v in front.items()}, [], diags)
    tokens = _MD.parse(body)
    i = 0
    while i < len(tokens):
        t = tokens[i]
        if t.type == "heading_open":
            inline = tokens[i + 1]
            doc.blocks.append(Block("heading", t.map[0] + 1, heading=Heading(int(t.tag[1]), inline.content.strip(), t.map[0] + 1)))
            i += 3
            continue
        if t.type == "paragraph_open":
            doc.blocks.append(Block("para", t.map[0] + 1, text=tokens[i + 1].content))
            i += 3
            continue
        if t.type == "table_open":
            header, rows, lines, row, row_line, in_head = [], [], [], None, 0, False
            j = i + 1
            while tokens[j].type != "table_close":
                tj = tokens[j]
                if tj.type == "thead_open":
                    in_head = True
                elif tj.type == "thead_close":
                    in_head = False
                elif tj.type == "tr_open":
                    row, row_line = [], tj.map[0] + 1 if tj.map else 0
                elif tj.type in ("th_open", "td_open"):
                    row.append(tokens[j + 1].content if tokens[j + 1].type == "inline" else "")
                elif tj.type == "tr_close":
                    if in_head:
                        header = row
                    else:
                        rows.append(row + [""] * (len(header) - len(row)))
                        lines.append(row_line)
                j += 1
            doc.blocks.append(Block("table", t.map[0] + 1, table=Table(header, rows, lines, t.map[0] + 1)))
            i = j + 1
            continue
        i += 1
    return doc


def parse_md_file(path, name=None):
    with open(path, encoding="utf-8") as fh:
        return parse_md(fh.read(), name or os.path.basename(path))
