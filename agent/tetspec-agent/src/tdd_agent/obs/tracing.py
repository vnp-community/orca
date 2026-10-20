"""Hierarchical spans: run -> gate -> step -> (llm_call | validator | store_commit | retrieval).

Spans are always persisted in the ``OperationalStore``; they are additionally
exported through OpenTelemetry when the SDK is installed *and* an OTLP endpoint
is configured.  A missing SDK never fails a run.
"""
from __future__ import annotations

import logging
import time
from contextlib import contextmanager
from typing import Any, Iterator

from ..store.base import OperationalStore, now_iso
from . import context
from .jsonlog import get_logger, log_event
from .redact import Redactor

KINDS = ("run", "gate", "step", "llm_call", "validator", "store_commit", "retrieval", "tool")
log = get_logger("trace")


class Span:
    def __init__(self, tracer: "Tracer", span_id: str, trace_id: str, name: str, kind: str):
        self._tracer = tracer
        self.span_id, self.trace_id, self.name, self.kind = span_id, trace_id, name, kind
        self.status = "ok"
        self.attributes: dict[str, Any] = {}
        self.blockers: list[str] = []

    def set(self, **attributes: Any) -> None:
        self.attributes.update(self._tracer.redactor.data(attributes))

    def block(self, *codes: str) -> None:
        self.status = "blocked"
        self.blockers.extend(c for c in codes if c not in self.blockers)

    def fail(self, code: str) -> None:
        self.status = "error"
        self.attributes["error_code"] = code


class Tracer:
    def __init__(self, store: OperationalStore, redactor: Redactor | None = None, otlp_endpoint: str = ""):
        self.store = store
        self.redactor = redactor or Redactor()
        self._otel = self._init_otel(otlp_endpoint) if otlp_endpoint else None

    @staticmethod
    def _init_otel(endpoint: str):
        try:
            from opentelemetry import trace
            from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
            from opentelemetry.sdk.resources import Resource
            from opentelemetry.sdk.trace import TracerProvider
            from opentelemetry.sdk.trace.export import BatchSpanProcessor
        except ImportError:
            log_event(log, "otel_unavailable", logging.WARNING, endpoint=endpoint, reason="opentelemetry packages not installed")
            return None
        provider = TracerProvider(resource=Resource.create({"service.name": "tdd-agent"}))
        provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=endpoint)))
        return provider.get_tracer("tdd_agent")

    @contextmanager
    def span(self, name: str, kind: str = "step", **attributes: Any) -> Iterator[Span]:
        if kind not in KINDS:
            raise ValueError(f"unknown span kind {kind}")
        trace_id = context.get("trace_id") or context.new_id(16)
        parent = context.get("span_id")
        span = Span(self, context.new_id(8), trace_id, name, kind)
        span.set(**attributes)
        start = now_iso()
        t0 = time.monotonic()
        run_id, agent, gate, step = (context.get(k) for k in ("run_id", "agent", "gate", "step"))
        self.store.add_span({"span_id": span.span_id, "trace_id": trace_id, "run_id": run_id, "parent_id": parent, "name": name,
                             "kind": kind, "agent": agent, "gate": gate, "step": step, "start": start, "status": "open",
                             "attributes": span.attributes})
        otel_span = self._otel.start_span(name) if self._otel else None
        error: BaseException | None = None
        with context.bind(trace_id=trace_id, span_id=span.span_id):
            log_event(log, "span_start", logging.DEBUG, name=name, kind=kind)
            try:
                yield span
            except BaseException as exc:
                error = exc
                span.status = "error"
                span.attributes["error_code"] = getattr(exc, "code", type(exc).__name__)
                span.attributes["error"] = self.redactor.text(str(exc))[:500]
                raise
            finally:
                duration = int((time.monotonic() - t0) * 1000)
                fields = {"status": span.status, "end": now_iso(), "duration_ms": duration, "attributes": span.attributes}
                if span.blockers:
                    fields["blockers"] = span.blockers
                self.store.end_span(span.span_id, **fields)
                if otel_span is not None:
                    otel_span.set_attribute("tdd.kind", kind)
                    otel_span.set_attribute("tdd.status", span.status)
                    otel_span.end()
                level = logging.ERROR if error else (logging.WARNING if span.status == "blocked" else logging.INFO)
                log_event(log, "span_end", level, name=name, kind=kind, status=span.status, duration_ms=duration, blockers=span.blockers or None)


def render_tree(spans: list[dict], width: int = 100) -> str:
    """Render spans of one trace as an indented tree, including blocked spans."""
    children: dict[str | None, list[dict]] = {}
    ids = {s["span_id"] for s in spans}
    for s in spans:
        parent = s.get("parent_id") if s.get("parent_id") in ids else None
        children.setdefault(parent, []).append(s)
    lines: list[str] = []

    def walk(parent: str | None, depth: int) -> None:
        for s in sorted(children.get(parent, []), key=lambda x: (x.get("start", ""), x["span_id"])):
            dur = s.get("duration_ms")
            tail = f" blockers={','.join(s['blockers'])}" if s.get("blockers") else ""
            code = s.get("attributes", {}).get("error_code")
            tail += f" error={code}" if code else ""
            lines.append(f"{'  ' * depth}{s.get('kind', '?'):<12}{s['name']}  [{s.get('status', '?')}] {'' if dur is None else f'{dur}ms'}{tail}"[:width + 40])
            walk(s["span_id"], depth + 1)

    walk(None, 0)
    return "\n".join(lines)
