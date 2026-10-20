"""G0, change-request driven: the SRS the change would produce, derived from the CR and the SRS in force.

Inputs are the CR (prose), the SRS before the change and the knowledge about the SAD and the TDDs; there is no edited SRS.  One model step
reads the CR against the requirement rows and proposes edits (skill ``propose-srs-edits-from-cr``); rules verify them and patch the SRS.
The patched text is registered as the next revision of the SRS (``proposed_from_cr``) so the rest of the flow (SRS delta at G1, impact at G2,
realization at G3, change spec at G4) reads a real before/after pair.  The BA confirms the delta, and its diff, at G1.
"""
from __future__ import annotations

from ..orchestrator.context import GateContext
from ..skills.lib import srs_edit as S, text as T
from ..store.base import SourceDoc
from .common import skills


def derive_srs_from_cr(ctx: GateContext, before: SourceDoc, cr_doc: SourceDoc, cr: dict, class_map: dict | None) -> tuple[SourceDoc | None, dict, list[str]]:
    parsed = S.parse_cr(cr_doc.text)
    acceptance_ids = list(dict.fromkeys(list(cr.get("acceptance") or []) + parsed["acceptance"]))
    rows = S.requirement_rows(before.text, class_map)
    if not rows:
        return None, {}, ["SRS_HAS_NO_REQUIREMENT_ROWS"]
    payload = {"cr": {"text": cr_doc.text, "acceptance_ids": acceptance_ids, "acceptance_text": parsed["acceptance_text"]}, "requirements": rows, "known_ids": sorted(T.ids_in(before.text))}
    res = skills(ctx).run("propose-srs-edits-from-cr", payload)
    if not res.ok:
        return None, {}, list(res.blockers)
    applied = S.apply_edits(before.text, res.output["edits"], rows, parsed["acceptance_text"])
    errors = list(applied.errors) + S.check_applied(applied.text, applied.changes, class_map)
    if errors:
        return None, {"edits": applied.changes}, errors
    after = ctx.register_proposed_srs(before, applied.text, {"proposed_from_cr": cr_doc.id, "cr_hash": cr_doc.content_hash, "base_revision": before.revision})
    report = {"edits": applied.changes, "unmatched": res.output["unmatched"], "proposed_srs": {"id": after.id, "revision": after.revision, "content_hash": after.content_hash},
              "diff": S.diff_lines(before.text, applied.text)[:400], "acceptance_text": parsed["acceptance_text"],
              "verifications": S.verifications_for(applied.changes, parsed["acceptance_text"], acceptance_ids),
              "term_sweep": S.term_sweep(rows, S.renames_in_cr(cr_doc.text), {c["id"] for c in applied.changes})}
    return after, report, []
