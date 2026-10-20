"""Facade used by every component: logging, spans, metrics and LLM audit."""
from __future__ import annotations

import logging
from contextlib import contextmanager
from typing import Any, Iterator

from ..config import Config
from ..llm.base import LlmRequest, LlmResponse
from ..store.base import OperationalStore
from ..store.memory import MemoryOperationalStore
from . import context
from .jsonlog import configure_logging, get_logger, log_event
from .llm_audit import record_call
from .metrics import Metrics
from .redact import Redactor
from .tracing import Span, Tracer


class Observer:
    def __init__(self, store: OperationalStore, *, payload_mode: str = "none", redact_patterns: list[str] | None = None,
                 otlp_endpoint: str = ""):
        self.store = store
        self.payload_mode = payload_mode
        self.redactor = Redactor(redact_patterns)
        self.tracer = Tracer(store, self.redactor, otlp_endpoint)
        self.metrics = Metrics(store)
        self.log = get_logger("runtime")

    @classmethod
    def from_config(cls, config: Config, store: OperationalStore, *, configure: bool = True) -> "Observer":
        obs = config.section("obs")
        if configure:
            log_file = config.path("obs.log_file") if obs.get("log_file") else None
            configure_logging(obs["log_level"], log_file, obs["log_stderr"], obs["redact_patterns"])
        return cls(store, payload_mode=obs["log_payloads"], redact_patterns=obs["redact_patterns"], otlp_endpoint=obs["otlp_endpoint"])

    @classmethod
    def in_memory(cls) -> "Observer":
        return cls(MemoryOperationalStore())

    # --- logging / spans ----------------------------------------------------
    def event(self, name: str, level: int = logging.INFO, **fields: Any) -> None:
        log_event(self.log, name, level, **fields)

    def span(self, name: str, kind: str = "step", **attributes: Any):
        return self.tracer.span(name, kind, **attributes)

    @contextmanager
    def bind(self, **values: Any) -> Iterator[None]:
        with context.bind(**values):
            yield

    # --- llm ----------------------------------------------------------------
    def llm_call(self, request: LlmRequest, response: LlmResponse, skill: dict | None = None) -> dict:
        record = record_call(self.store, request, response, mode=self.payload_mode, redactor=self.redactor, skill=skill)
        self.metrics.llm_call(response.provider, response.role, response.status, response.latency_ms, response.usage, len(response.attempts))
        level = logging.INFO if response.ok else logging.WARNING
        self.event("llm_call", level, provider=response.provider, role=response.role, status=response.status, model=response.model,
                   blockers=response.blockers or None, latency_ms=response.latency_ms, attempts=len(response.attempts),
                   evidence_grade=response.evidence_grade)
        return record
