"""Provider registry; third-party providers register via the
``tdd_agent.llm_providers`` entry-point group or ``register_provider``."""
from __future__ import annotations

from importlib import metadata
from typing import Callable

from ..errors import ConfigError
from .base import LlmProvider

Factory = Callable[[dict], LlmProvider]
_REGISTRY: dict[str, Factory] = {}


def register_provider(name: str, factory: Factory) -> None:
    _REGISTRY[name] = factory


def _load() -> None:
    from .claude_code import ClaudeCodeProvider
    from .codex_cli import CodexCliProvider
    _REGISTRY.setdefault("claude_code", lambda s: ClaudeCodeProvider(s))
    _REGISTRY.setdefault("codex_cli", lambda s: CodexCliProvider(s))
    try:
        for ep in metadata.entry_points(group="tdd_agent.llm_providers"):
            if ep.name not in _REGISTRY:
                _REGISTRY[ep.name] = ep.load()
    except Exception as exc:  # pragma: no cover - plugin failure surfaces as configuration error
        raise ConfigError(f"provider plugin failed to load: {exc}", code="PROVIDER_PLUGIN_FAILED") from exc


def available() -> list[str]:
    _load()
    return sorted(_REGISTRY)


def create_provider(name: str, settings: dict | None = None) -> LlmProvider:
    _load()
    if name not in _REGISTRY:
        raise ConfigError(f"unknown llm provider {name!r} (available: {sorted(_REGISTRY)})", code="PROVIDER_UNKNOWN")
    return _REGISTRY[name](dict(settings or {}))
