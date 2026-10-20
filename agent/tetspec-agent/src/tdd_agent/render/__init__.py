"""Standard, reader-facing TDD rendering (rules over the knowledge graph, no language model)."""
from __future__ import annotations

from .builder import RenderInfo, build_document
from .check import check_document
from .writers import get_writer


def render_standard(graph: dict, info: RenderInfo, fmt: str = "md") -> tuple[str, dict]:
    """Render ``graph`` (already restricted to one TDD) to the standard document; returns (text, check report)."""
    writer = get_writer(fmt)
    doc = build_document(graph, info)
    files = writer.write(doc)
    again = writer.write(build_document(graph, info))
    text = files[0].content
    report = check_document(doc, [e["id"] for e in graph.get("elements", [])], text, again[0].content, info.snapshot_hash)
    report["format"] = fmt
    report["files"] = [f.name for f in files]
    return text, report


__all__ = ["RenderInfo", "build_document", "render_standard", "get_writer", "check_document"]
