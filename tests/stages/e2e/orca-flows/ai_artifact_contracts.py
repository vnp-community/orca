"""Shape checks for what the AI stages produce (Solution, Plan proposal). They apply to real and stubbed AI alike."""
from __future__ import annotations


def check_solution_contract(solution: dict, expected_kind: str) -> None:
    """The Solution contract (CR-REQ-007): kind by request type, 2..4 options with ids opt-N, exactly one recommended and named."""
    assert solution.get("kind") == expected_kind, f"Solution kind is {solution.get('kind')!r}, expected {expected_kind!r}"
    assert solution.get("status") in ("draft", "proposed"), f"a fresh Solution is {solution.get('status')!r}"
    doc = solution.get("options") or {}
    options = doc.get("options") or []
    assert 2 <= len(options) <= 4, f"a change_request Solution needs 2..4 options, got {len(options)}"
    ids = [o.get("id") for o in options]
    assert all(isinstance(i, str) and i.startswith("opt-") for i in ids) and len(set(ids)) == len(ids), f"option ids {ids!r}"
    recommended = [o["id"] for o in options if o.get("recommended")]
    assert len(recommended) == 1, f"exactly one option must be recommended, got {recommended!r}"
    assert (doc.get("recommendation") or {}).get("option_id") == recommended[0], "recommendation.option_id does not name the recommended option"
    for o in options:
        assert (o.get("effort") or {}).get("size") in ("S", "M", "L"), f"option {o['id']} has effort {o.get('effort')!r}"
        assert o.get("title") and o.get("summary") and o.get("approach"), f"option {o['id']} is missing title, summary or approach"


def check_plan_contract(proposal: dict, phased: bool) -> None:
    """A Plan proposal: titled, tasks in the right container for the shape, valid task types, acyclic indices."""
    assert proposal.get("title"), "the Plan has no title"
    tasks = list(proposal.get("tasks") or [])
    phases = list(proposal.get("phases") or [])
    if phased:
        assert phases and not tasks, f"a phased Plan keeps tasks inside phases (phases={len(phases)}, loose tasks={len(tasks)})"
    else:
        assert tasks and not phases, f"an unphased Plan lists tasks directly (tasks={len(tasks)}, phases={len(phases)})"
    for container in ([tasks] if not phased else [ph.get("tasks") or [] for ph in phases]):
        for i, t in enumerate(container):
            assert t.get("title"), f"task {i} has no title"
            assert (t.get("taskType") or "task") in ("task", "bug", "feature", "epic"), f"task {i} has type {t.get('taskType')!r}"
            assert all(0 <= d < len(container) and d != i for d in t.get("dependsOnIndices") or []), f"task {i} has a bad dependency"

