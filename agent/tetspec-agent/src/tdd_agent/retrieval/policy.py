"""Relation traversal policy: which declared relations the closure may follow."""
from __future__ import annotations

from dataclasses import asdict
from functools import lru_cache

from ..core import o5_path
from ..store.base import digest


@lru_cache(maxsize=1)
def _relation_names() -> tuple[str, ...]:
    o5 = o5_path.load()
    manifest = o5.ontology5.load()
    names = set(manifest.get("relations", {}))
    names |= set(o5.ontology5.legacy_relation_specs())
    return tuple(sorted(names))


def registered_relations() -> frozenset[str]:
    """Relations declared by the active ontology registry (canonical + legacy)."""
    return frozenset(_relation_names())


def default_policies():
    """o5's reviewed policy set plus every relation the registry declares.

    Relations absent from this set are *not* silently skipped by the closure:
    they surface as ``unresolved`` entries so a missing relation is visible.
    """
    o5 = o5_path.load()
    sr = o5.semantic_resolver
    known = {p.relation: p for p in sr.DEFAULT_POLICIES}
    for name in registered_relations():
        known.setdefault(name, sr.RelationPolicy(name, "both", "dependency"))
    return tuple(known[k] for k in sorted(known))


def policy_hash(policies) -> str:
    return digest([asdict(p) for p in policies])


def reset_cache() -> None:  # used by tests that change the registry
    _relation_names.cache_clear()
