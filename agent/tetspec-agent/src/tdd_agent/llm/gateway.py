"""Single entry point for every LLM call made by agents and skills.

Responsibilities: map roles to providers (with the optional distinct-critic
rule), enforce ``[llm.budget]`` (default unlimited), wrap each call in a span and
write the audit record.  Provider failures are returned as ``blocked``
responses, never raised.
"""
from __future__ import annotations

import threading
import time
from datetime import datetime, timedelta, timezone
from typing import Any

from ..config import Config
from ..errors import ConfigError
from ..obs.observer import Observer
from .base import LlmProvider, LlmRequest, LlmResponse, ROLES
from .registry import create_provider


class LlmGateway:
    def __init__(self, config: Config, observer: Observer, providers: dict[str, LlmProvider] | None = None):
        self.config = config
        self.observer = observer
        self._providers: dict[str, LlmProvider] = dict(providers or {})
        self._lock = threading.Lock()
        self._run_started: dict[str, float] = {}
        llm = config.section("llm")
        self.default = llm["default"]
        self.budget = llm["budget"]
        self.role_map = {role: (llm["roles"].get(role) or self.default) for role in ROLES}
        for name in set(self.role_map.values()):
            self._provider(name)  # fail early on unknown providers
        if llm["require_distinct_critic"]:
            a, c = self.provider_for("author"), self.provider_for("critic")
            if (a.name, a.effective_model()) == (c.name, c.effective_model()):
                raise ConfigError("llm.require_distinct_critic is set but author and critic resolve to the same provider and model",
                                  code="CRITIC_NOT_DISTINCT")

    def _provider(self, name: str) -> LlmProvider:
        with self._lock:
            if name not in self._providers:
                settings = self.config.section("llm").get(name)
                if settings is None:
                    settings = {}
                self._providers[name] = create_provider(name, settings)
            return self._providers[name]

    def provider_for(self, role: str) -> LlmProvider:
        if role not in self.role_map:
            raise ConfigError(f"unknown llm role {role}", code="CONFIG_UNKNOWN_ROLE")
        return self._provider(self.role_map[role])

    def describe(self) -> dict:
        return {"roles": dict(self.role_map), "providers": {n: self._provider(n).describe() for n in sorted(set(self.role_map.values()))},
                "budget": dict(self.budget)}

    # --- budget -------------------------------------------------------------
    def check_run_quota(self) -> str | None:
        """Return ``BUDGET_EXCEEDED`` when ``max_runs_per_day`` is configured and reached."""
        limit = self.budget["max_runs_per_day"]
        if not limit:
            return None
        since = (datetime.now(timezone.utc) - timedelta(days=1)).isoformat(timespec="microseconds")
        return "BUDGET_EXCEEDED" if self.observer.store.count_runs_since(since) >= limit else None

    def _budget_error(self, run_id: str | None) -> str | None:
        calls, wall = self.budget["max_calls_per_run"], self.budget["max_wall_seconds"]
        if not run_id or not (calls or wall):
            return None
        if calls and len(self.observer.store.list_llm_calls(run_id)) >= calls:
            return "BUDGET_EXCEEDED"
        if wall:
            started = self._run_started.setdefault(run_id, time.monotonic())
            if time.monotonic() - started > wall:
                return "BUDGET_EXCEEDED"
        return None

    # --- call ---------------------------------------------------------------
    def call(self, request: LlmRequest, *, skill: dict | None = None) -> LlmResponse:
        from ..obs import context
        if context.get("llm_forbidden"):
            from ..errors import TddAgentError
            raise TddAgentError(f"LLM call attempted inside logic step {context.get('step')!r}; logic steps must be rule-only", code="LOGIC_STEP_LLM_CALL")
        run_id = context.get("run_id") or request.metadata.get("run_id")
        request.metadata.setdefault("run_id", run_id)
        provider = self.provider_for(request.role)
        with self.observer.span(f"llm:{request.role}", "llm_call", provider=provider.name, role=request.role,
                                model=provider.effective_model()) as span:
            error = self._budget_error(run_id)
            if error:
                response = LlmResponse(status="blocked", provider=provider.name, role=request.role, blockers=[error],
                                       model=provider.effective_model(), evidence_grade=provider.evidence_grade,
                                       prompt_hash=request.prompt_hash(), schema_hash=request.schema_hash())
            else:
                response = provider.complete(request)
            if not response.ok:
                span.block(*response.blockers)
            span.set(attempts=len(response.attempts), latency_ms=response.latency_ms, model=response.model, effort=response.effort,
                     evidence_grade=response.evidence_grade)
            self.observer.llm_call(request, response, skill=skill)
            return response

    def call_json(self, role: str, prompt: str, schema: dict, *, system: str | None = None, skill: dict | None = None,
                  **kwargs: Any) -> LlmResponse:
        return self.call(LlmRequest(role=role, prompt=prompt, system=system, output_schema=schema, **kwargs), skill=skill)
