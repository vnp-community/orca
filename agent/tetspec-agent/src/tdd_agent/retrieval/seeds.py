"""Seed selection by agent intent."""
from __future__ import annotations

from typing import Sequence

from ..store.base import KnowledgeStore
from .closure import SeedSpec
from .queries import QueryService, props, _as_list


def seeds_from_srs_delta(store: KnowledgeStore, delta_id: str, snapshot=None) -> SeedSpec:
    """Change-spec / impact seed: the SRS delta, its requirement deltas and both requirement versions."""
    q = QueryService(store)
    snap = q._snap(snapshot)
    delta = store.get_element(delta_id, snap.id)
    ids = {delta_id} if delta else set()
    if delta:
        cr = props(delta).get("cr_id")
        rds = [e for e in store.by_class("RequirementDelta", snap.id) if props(e).get("change_request_id") == cr]
        ids |= {e["id"] for e in rds}
        ids |= {e["dst"] for e in store.neighbors([e["id"] for e in rds], ["before_requirement", "after_requirement"], "out", snap.id)}
    return SeedSpec("srs_delta", tuple(sorted(ids)), {"delta": delta_id})


def seeds_from_capability(store: KnowledgeStore, capability_id: str, snapshot=None) -> SeedSpec:
    """TDD-generation seed: the capability and the requirement versions it owns."""
    snap = store.get_snapshot(snapshot)
    ids = {capability_id}
    ids |= {e["dst"] for e in store.neighbors([capability_id], ["has_requirement"], "out", snap.id)}
    return SeedSpec("capability", tuple(sorted(ids)), {"capability": capability_id})


def seeds_from_change_spec_item(store: KnowledgeStore, cs_id: str, snapshot=None) -> SeedSpec:
    snap = store.get_snapshot(snapshot)
    cs = store.get_element(cs_id, snap.id)
    ids = {cs_id}
    if cs:
        for field_name in ("srs_refs", "tdd_refs", "cr_acceptance_refs"):
            ids |= {str(r if not isinstance(r, dict) else r.get("id")) for r in _as_list(props(cs).get(field_name)) if r}
    return SeedSpec("change_spec_item", tuple(sorted(i for i in ids if i and i != "None")), {"cs": cs_id})


def seeds_from_prompt_target(target_ids: Sequence[str]) -> SeedSpec:
    return SeedSpec("prompt_target", tuple(sorted(set(target_ids))))
