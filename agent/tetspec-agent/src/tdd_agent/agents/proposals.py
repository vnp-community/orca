"""What a change request left out, proposed by the model and marked as proposed (never as given).

A person who runs the flow without anyone to ask (automatic mode) still gets a spec: acceptance criteria written from the sentences of the CR, the existing design element that
realizes a changed requirement, and, when the repository has no code to point at, the component as the place of the change.  Every proposal quotes or names what it comes from,
passes the rules of its skill, and is listed in ``review.md`` and in the change-spec table's output so a person can overrule it.
"""
from __future__ import annotations

from ..skills.lib import text as T
from .common import skills

REALIZER_CLASSES = ("Component", "Module", "Operation", "Job", "Algorithm", "Interface", "Flow", "Event")
MAX_CANDIDATES = 25


def acceptance(ctx, cr_text: str, changes: list[dict]) -> tuple[list[dict], list[str]]:
    """Criteria for the changed requirements: ``[{id, change_id, given, when, then, cr_quote}]`` and the blockers."""
    res = skills(ctx).run("propose-acceptance-from-cr", {"cr_text": cr_text, "changes": changes})
    if not res.ok:
        return [], list(res.blockers)
    out = []
    for n, c in enumerate(res.output["criteria"], 1):
        out.append({"id": f"AC-PROPOSED-{n}", **c})
    return out, []


def gwt(c: dict) -> str:
    return f"GIVEN {c['given']} WHEN {c['when']} THEN {c['then']}"


def satisfier(ctx, graph: dict, change: dict) -> dict | None:
    """The existing element proposed as realizing ``change`` (``{id, name, class, reason}``), or ``None`` when none is clearly responsible."""
    statement = str(change.get("statement") or "")
    pool = [e for e in graph.get("elements", []) if e.get("class") in REALIZER_CLASSES and not str(e["id"]).startswith(("NOTE-", "FACT-"))]
    def brief(e):
        p = e.get("props") or {}
        d = p.get("definition")
        return {"id": e["id"], "class": e.get("class"), "name": e.get("name"), "summary": " ".join(str(x) for x in d) if isinstance(d, list) else str(p.get("responsibility") or p.get("summary") or p.get("statement") or "")}
    ranked = sorted(pool, key=lambda e: (-T.jaccard(statement, f"{e.get('name', '')} {brief(e)['summary']}"), e["id"]))
    components = [e for e in pool if e.get("class") == "Component"][:12]
    chosen = list({e["id"]: e for e in components + ranked[:MAX_CANDIDATES]}.values())[:MAX_CANDIDATES]
    if not chosen:
        return None
    res = skills(ctx).run("propose-satisfying-element", {"change": {"id": change["id"], "statement": statement}, "candidates": [brief(e) for e in chosen]})
    if not res.ok or not res.output.get("id"):
        return None
    el = next(e for e in chosen if e["id"] == res.output["id"])
    return {"id": el["id"], "name": el.get("name"), "class": el.get("class"), "reason": res.output["reason"]}


def place_of_change(element: dict) -> str:
    """Where a change goes when the repository has no code to point at: the element that realizes the requirement, said as what it is."""
    return f"{element.get('name') or element['id']} ({element['id']}): the file and function are not identified yet"
