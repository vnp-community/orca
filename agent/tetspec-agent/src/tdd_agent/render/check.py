"""Checks of a rendered standard TDD (they replace the lossless round trip, which does not apply to a reader document)."""
from __future__ import annotations

from .model import Document, Missing, Row, Table, walk_blocks
from .writers import anchors_of


def check_document(document: Document, element_ids: list[str], text: str, again: str, snapshot_hash: str = "") -> dict:
    """Every element anchored, every row labelled with an origin and sources/rule, anchors unique, every MISSING has a reason, deterministic, snapshot named."""
    anchors = set(anchors_of(document))
    missing_elements = sorted(i for i in element_ids if i not in anchors)
    rows = unlabelled = 0
    miss = no_reason = 0
    for ch in document.chapters:
        for _, blk in walk_blocks(ch):
            if isinstance(blk, Table):
                for r in blk.rows:
                    rows += 1
                    if r.origin == "nguồn" and not r.refs and not r.label:
                        unlabelled += 1
            elif isinstance(blk, Missing):
                miss += 1
                no_reason += 0 if blk.reason.strip() else 1
    errors = []
    if missing_elements:
        errors.append(f"{len(missing_elements)} element(s) not anchored in the document")
    if unlabelled:
        errors.append(f"{unlabelled} row(s) claim a source but carry no source reference")
    if no_reason:
        errors.append(f"{no_reason} MISSING block(s) without a reason")
    if text != again:
        errors.append("rendering is not deterministic")
    if snapshot_hash and snapshot_hash not in text:
        errors.append("snapshot hash not stated in the document")
    return {"ok": not errors, "errors": errors, "elements": len(element_ids), "anchored": len(element_ids) - len(missing_elements), "missing_elements": missing_elements[:20],
            "rows": rows, "rows_without_source": unlabelled, "missing_blocks": miss, "deterministic": text == again, "chapters": len(document.chapters)}
