"""Store registry.

Backends register a factory per URL scheme.  ``sqlite:`` and ``memory:`` ship
with the runtime; further backends are discovered through the
``tdd_agent.stores`` entry-point group and must pass ``store.contract``.
"""
from __future__ import annotations

from importlib import metadata
from typing import Callable

from ..errors import ConfigError
from .base import (CommitResult, ElementVersion, KnowledgeStore, OperationalStore, RunRecord, SnapshotInfo, SourceDoc)

KnowledgeFactory = Callable[[str], KnowledgeStore]
OperationalFactory = Callable[[str], OperationalStore]

_KNOWLEDGE: dict[str, KnowledgeFactory] = {}
_OPERATIONAL: dict[str, OperationalFactory] = {}


def register_knowledge_backend(scheme: str, factory: KnowledgeFactory) -> None:
    _KNOWLEDGE[scheme] = factory


def register_operational_backend(scheme: str, factory: OperationalFactory) -> None:
    _OPERATIONAL[scheme] = factory


def _builtin() -> None:
    from .memory import MemoryKnowledgeStore, MemoryOperationalStore
    from .sqlite_knowledge import SqliteKnowledgeStore
    from .sqlite_operational import SqliteOperationalStore

    def sqlite_target(url: str) -> str:
        rest = url.split(":", 1)[1]
        return ":memory:" if rest in (":memory:", "") else rest

    _KNOWLEDGE.setdefault("sqlite", lambda url: SqliteKnowledgeStore(sqlite_target(url)))
    _OPERATIONAL.setdefault("sqlite", lambda url: SqliteOperationalStore(sqlite_target(url)))
    _KNOWLEDGE.setdefault("memory", lambda url: MemoryKnowledgeStore())
    _OPERATIONAL.setdefault("memory", lambda url: MemoryOperationalStore())


def _plugins() -> None:
    try:
        eps = metadata.entry_points(group="tdd_agent.stores")
    except Exception:  # pragma: no cover - metadata backend failure
        return
    for ep in eps:
        try:
            ep.load()()  # the plugin registers its own schemes
        except Exception as exc:
            raise ConfigError(f"store plugin {ep.name} failed to load: {exc}", code="STORE_PLUGIN_FAILED") from exc


def _scheme(url: str) -> str:
    if ":" not in url:
        raise ConfigError(f"store url needs a scheme, e.g. sqlite:path (got {url!r})", code="STORE_URL_INVALID")
    return url.split(":", 1)[0]


def open_knowledge_store(url: str) -> KnowledgeStore:
    _builtin(); _plugins()
    scheme = _scheme(url)
    if scheme not in _KNOWLEDGE:
        raise ConfigError(f"no knowledge store backend for scheme {scheme!r} (have {sorted(_KNOWLEDGE)})", code="STORE_BACKEND_UNKNOWN")
    return _KNOWLEDGE[scheme](url)


def open_operational_store(url: str) -> OperationalStore:
    _builtin(); _plugins()
    scheme = _scheme(url)
    if scheme not in _OPERATIONAL:
        raise ConfigError(f"no operational store backend for scheme {scheme!r} (have {sorted(_OPERATIONAL)})", code="STORE_BACKEND_UNKNOWN")
    return _OPERATIONAL[scheme](url)


__all__ = [
    "CommitResult", "ElementVersion", "KnowledgeStore", "OperationalStore", "RunRecord", "SnapshotInfo", "SourceDoc",
    "open_knowledge_store", "open_operational_store", "register_knowledge_backend", "register_operational_backend",
]
