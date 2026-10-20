"""Ambient correlation ids carried through every log line and span."""
from __future__ import annotations

import contextvars
import uuid
from contextlib import contextmanager
from typing import Any, Iterator

_FIELDS = ("run_id", "trace_id", "span_id", "agent", "gate", "step", "llm_forbidden")
_vars = {name: contextvars.ContextVar(f"tdd_agent_{name}", default=None) for name in _FIELDS}


def new_id(nbytes: int = 8) -> str:
    return uuid.uuid4().hex[: nbytes * 2]


def snapshot() -> dict[str, Any]:
    return {name: var.get() for name, var in _vars.items() if var.get() is not None}


def get(name: str) -> Any:
    return _vars[name].get()


@contextmanager
def bind(**values: Any) -> Iterator[None]:
    tokens = []
    for name, value in values.items():
        if name not in _vars:
            raise KeyError(name)
        tokens.append((_vars[name], _vars[name].set(value)))
    try:
        yield
    finally:
        for var, token in reversed(tokens):
            var.reset(token)
