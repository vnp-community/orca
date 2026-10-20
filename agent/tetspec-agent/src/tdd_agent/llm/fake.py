"""Deterministic provider for tests.

Every response is stamped ``evidence_grade = "fixture"``; acceptance refuses runs
that contain such calls.  The provider is never registered by default.
"""
from __future__ import annotations

import json
from typing import Any, Callable

from .base import LlmProvider, LlmRequest, ProviderCapabilities, ProviderFailure, RawResult

Handler = Callable[[LlmRequest, int], Any]


class FakeProvider(LlmProvider):
    name = "fake"
    evidence_grade = "fixture"

    def __init__(self, handler: Handler | dict | None = None, settings: dict | None = None):
        super().__init__(settings or {"model": "fake-model"})
        self.handler = handler
        self.calls: list[LlmRequest] = []

    def capabilities(self) -> ProviderCapabilities:
        return ProviderCapabilities(structured_output=True, read_only_sandbox=True, tools=())

    def _invoke(self, request: LlmRequest, attempt: int) -> RawResult:
        self.calls.append(request)
        h = self.handler
        if isinstance(h, dict):
            h = h.get(request.role) or h.get("*")
        if h is None:
            raise ProviderFailure("PROVIDER_ERROR", "fake provider has no handler", retryable=False)
        out = h(request, len(self.calls)) if callable(h) else h
        if isinstance(out, ProviderFailure):
            raise out
        if isinstance(out, (dict, list)):
            return RawResult(text=json.dumps(out), structured=out, model="fake-model", usage={"input_tokens": 1, "output_tokens": 1})
        return RawResult(text=str(out), model="fake-model", usage={"input_tokens": 1, "output_tokens": 1})
