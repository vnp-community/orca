"""Single entry point that exposes the ``o5`` reference modules.

``o5`` imports its siblings by bare module name (``from ontology5 import load``)
and relies on ``sys.path``; moving or renaming those files would invalidate
existing tests and the hashes stored in historical reports.  This adapter adds
the required directories to ``sys.path`` exactly once and re-exports what the
runtime consumes, so no other module touches ``sys.path`` or imports ``o5``
directly.
"""
from __future__ import annotations

import os
import sys
from functools import lru_cache
from pathlib import Path

from ..errors import ConfigError

_SUBDIRS = ("", "tools", "harness", "edit", "specgen")


def _find_o5_root() -> Path:
    env = os.environ.get("TDD_AGENT_O5_ROOT")
    candidates = [Path(env)] if env else []
    here = Path(__file__).resolve()
    for parent in here.parents:
        candidate = parent / "data" / "ontology_v0.5" / "o5"
        candidates.append(candidate)
    candidates.append(here.parents[1] / "_data" / "ontology_v0.5" / "o5")  # the copy shipped inside an installed package
    for candidate in candidates:
        if (candidate / "workflow.py").is_file() and (candidate / "tools" / "transaction5.py").is_file():
            return candidate.resolve()
    raise ConfigError(
        "ontology o5 runtime not found; reinstall tdd-agent or set TDD_AGENT_O5_ROOT",
        code="O5_NOT_FOUND",
    )


@lru_cache(maxsize=1)
def o5_root() -> Path:
    root = _find_o5_root()
    for sub in _SUBDIRS:
        path = str(root / sub) if sub else str(root)
        if path not in sys.path:
            sys.path.insert(0, path)
    return root


def load():
    """Return a namespace of the o5 functions used by the runtime."""
    o5_root()
    import ontology5, quality5, readiness5, transaction5, envelope5, semantic_resolver, workflow, profiles  # noqa: E402
    import techspec_rules, specialized_checks  # noqa: E402
    from types import SimpleNamespace
    return SimpleNamespace(
        root=o5_root(),
        ontology5=ontology5,
        quality5=quality5,
        readiness5=readiness5,
        transaction5=transaction5,
        envelope5=envelope5,
        semantic_resolver=semantic_resolver,
        workflow=workflow,
        profiles=profiles,
        techspec_rules=techspec_rules,
        specialized_checks=specialized_checks,
        snapshot_hash=transaction5.snapshot_hash,
        apply_changeset=transaction5.apply_changeset,
        validate_graph=envelope5.validate_graph,
        verify_evidence=quality5.verify_evidence,
        fingerprint=quality5.fingerprint,
    )
