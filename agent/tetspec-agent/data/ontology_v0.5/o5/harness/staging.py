from __future__ import annotations
from copy import deepcopy
import sys
from pathlib import Path
TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(TOOLS) not in sys.path: sys.path.insert(0, str(TOOLS))
from envelope5 import validate_graph
from transaction5 import apply_changeset


def stage_and_commit(graph: dict, changeset: dict, repair=None, max_attempts: int = 2):
    """Validate a proposal and commit it atomically, optionally repairing it."""
    current = deepcopy(changeset); attempts = []
    for attempt in range(1, max_attempts + 1):
        candidate, report = apply_changeset(graph, current)
        attempts.append({"attempt": attempt, "report": report})
        if report.get("ok"):
            return {"status": "committed", "graph": candidate, "attempts": attempts}
        if not repair: break
        current = repair(current, report)
    return {"status": "rejected", "graph": graph, "attempts": attempts}


def cross_tdd_check(graph: dict) -> dict:
    ids = {x.get("id") for x in graph.get("elements", [])}; errors = []
    owners = {}
    for x in graph.get("elements", []):
        owner = x.get("owner_doc") or x.get("props", {}).get("owner_doc")
        if owner and x.get("id") in owners and owners[x["id"]] != owner: errors.append({"id": x["id"], "code": "MULTIPLE_OWNER"})
        if owner: owners[x["id"]] = owner
    for e in graph.get("edges", []):
        if e.get("src") not in ids or e.get("dst") not in ids: errors.append({"edge": e, "code": "DANGLING_REFERENCE"})
    return {"ok": not errors, "errors": errors, "owners": owners}
