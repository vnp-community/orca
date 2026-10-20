"""LLM provider contract.

A provider performs one logical completion: it may retry internally (bounded by
``max_attempts``) but it never raises for provider-side problems; it returns a
``LlmResponse`` whose ``status`` is ``ok`` or ``blocked`` with machine-readable
``blockers``.  Output schemas are always validated in-process with
``jsonschema`` regardless of what the underlying CLI offers; a provider never
repairs or fills in values on the model's behalf.
"""
from __future__ import annotations

import abc
import json
import re
import time
from dataclasses import dataclass, field
from hashlib import sha256
from pathlib import Path
from typing import Any

import jsonschema

ROLES = ("author", "critic", "extractor")
BLOCKER_CODES = (
    "PROVIDER_UNAVAILABLE", "PROVIDER_TIMEOUT", "PROVIDER_RATE_LIMITED", "PROVIDER_AUTH_REQUIRED",
    "PROVIDER_OPTION_UNSUPPORTED", "PROVIDER_MODEL_UNSUPPORTED", "PROVIDER_ERROR", "PROVIDER_BAD_OUTPUT", "OUTPUT_SCHEMA_INVALID", "BUDGET_EXCEEDED",
)
READ_ONLY_TOOLS = ("Read", "Grep", "Glob")


class ProviderFailure(Exception):
    """Raised by ``_invoke`` implementations to describe one failed attempt."""

    def __init__(self, code: str, message: str = "", *, retryable: bool = False, diagnostics: str = ""):
        super().__init__(message or code)
        self.code = code
        self.retryable = retryable
        self.diagnostics = diagnostics


@dataclass(frozen=True)
class ProviderCapabilities:
    structured_output: bool
    read_only_sandbox: bool
    tools: tuple[str, ...] = ()


@dataclass
class LlmRequest:
    role: str
    prompt: str
    system: str | None = None
    output_schema: dict | None = None
    timeout_s: int | None = None
    max_attempts: int | None = None
    read_only: bool = True
    cwd: Path | None = None
    allowed_tools: tuple[str, ...] = ()
    metadata: dict = field(default_factory=dict)

    def prompt_hash(self) -> str:
        return sha256(((self.system or "") + "\n---\n" + self.prompt).encode()).hexdigest()

    def schema_hash(self) -> str | None:
        return sha256(json.dumps(self.output_schema, sort_keys=True).encode()).hexdigest() if self.output_schema else None


@dataclass
class RawResult:
    text: str
    structured: Any = None
    usage: dict = field(default_factory=dict)
    model: str | None = None
    effort: str | None = None
    cost_usd: float | None = None
    diagnostics: str = ""


@dataclass
class LlmResponse:
    status: str
    provider: str
    role: str
    text: str = ""
    parsed: Any = None
    blockers: list[str] = field(default_factory=list)
    attempts: list[dict] = field(default_factory=list)
    usage: dict = field(default_factory=dict)
    model: str | None = None
    effort: str | None = None
    cost_usd: float | None = None
    latency_ms: int = 0
    evidence_grade: str = "real"
    prompt_hash: str = ""
    schema_hash: str | None = None
    metadata: dict = field(default_factory=dict)

    @property
    def ok(self) -> bool:
        return self.status == "ok"

    def audit_record(self) -> dict:
        """Everything worth auditing except the payload itself."""
        return {
            "provider": self.provider, "role": self.role, "status": self.status, "blockers": list(self.blockers),
            "model": self.model, "effort": self.effort, "attempts": self.attempts, "usage": self.usage,
            "cost_usd": self.cost_usd, "latency_ms": self.latency_ms, "evidence_grade": self.evidence_grade,
            "prompt_hash": self.prompt_hash, "schema_hash": self.schema_hash,
        }


_FENCE = re.compile(r"^\s*```(?:json)?\s*(.*?)\s*```\s*$", re.S)


def extract_json(text: str) -> Any:
    """Parse JSON from model text, tolerating a surrounding code fence."""
    candidate = text.strip()
    m = _FENCE.match(candidate)
    if m:
        candidate = m.group(1)
    try:
        return json.loads(candidate)
    except json.JSONDecodeError:
        start, end = candidate.find("{"), candidate.rfind("}")
        if start != -1 and end > start:
            return json.loads(candidate[start:end + 1])
        raise


def schema_errors(schema: dict, value: Any, limit: int = 8) -> list[str]:
    validator = jsonschema.Draft202012Validator(schema)
    out = []
    for err in sorted(validator.iter_errors(value), key=lambda e: list(e.absolute_path)):
        out.append(f"{'/'.join(str(p) for p in err.absolute_path) or '$'}: {err.message[:200]}")
        if len(out) >= limit:
            break
    return out


class LlmProvider(abc.ABC):
    name = "provider"
    evidence_grade = "real"
    default_attempts = 2
    default_timeout_s = 900

    def __init__(self, settings: dict | None = None):
        self.settings = dict(settings or {})
        self.default_attempts = int(self.settings.get("max_attempts", self.default_attempts))
        self.default_timeout_s = int(self.settings.get("timeout_seconds", self.default_timeout_s))

    # -- to implement ---------------------------------------------------------
    @abc.abstractmethod
    def capabilities(self) -> ProviderCapabilities: ...

    @abc.abstractmethod
    def _invoke(self, request: LlmRequest, attempt: int) -> RawResult: ...

    def describe(self) -> dict:
        """Effective, secret-free settings (recorded with every acceptance run)."""
        return {"provider": self.name}

    def effective_model(self) -> str | None:
        return self.settings.get("model") or None

    # -- template -------------------------------------------------------------
    def complete(self, request: LlmRequest) -> LlmResponse:
        started = time.monotonic()
        max_attempts = request.max_attempts or self.default_attempts
        attempts: list[dict] = []
        usage: dict = {}
        cost = 0.0
        have_cost = False
        last_code = "PROVIDER_ERROR"
        model = effort = None
        base = dict(provider=self.name, role=request.role, evidence_grade=self.evidence_grade,
                    prompt_hash=request.prompt_hash(), schema_hash=request.schema_hash(), metadata=dict(request.metadata))
        for attempt in range(1, max_attempts + 1):
            t0 = time.monotonic()
            try:
                raw = self._invoke(request, attempt)
            except ProviderFailure as failure:
                attempts.append({"attempt": attempt, "error": failure.code, "message": str(failure)[:300],
                                 "diagnostics": failure.diagnostics[-400:], "elapsed_ms": int((time.monotonic() - t0) * 1000)})
                last_code = failure.code
                if not failure.retryable:
                    break
                continue
            for k, v in (raw.usage or {}).items():
                if isinstance(v, (int, float)):
                    usage[k] = usage.get(k, 0) + v
            if raw.cost_usd is not None:
                cost += raw.cost_usd; have_cost = True
            model, effort = raw.model or model, raw.effort or effort
            parsed = None
            if request.output_schema is not None:
                try:
                    parsed = raw.structured if raw.structured is not None else extract_json(raw.text)
                except (json.JSONDecodeError, ValueError):
                    attempts.append({"attempt": attempt, "error": "OUTPUT_SCHEMA_INVALID", "message": "output is not valid JSON",
                                     "elapsed_ms": int((time.monotonic() - t0) * 1000)})
                    last_code = "OUTPUT_SCHEMA_INVALID"
                    continue
                errors = schema_errors(request.output_schema, parsed)
                if errors:
                    attempts.append({"attempt": attempt, "error": "OUTPUT_SCHEMA_INVALID", "message": "; ".join(errors),
                                     "elapsed_ms": int((time.monotonic() - t0) * 1000)})
                    last_code = "OUTPUT_SCHEMA_INVALID"
                    continue
            attempts.append({"attempt": attempt, "ok": True, "elapsed_ms": int((time.monotonic() - t0) * 1000)})
            return LlmResponse(status="ok", text=raw.text, parsed=parsed, attempts=attempts, usage=usage, model=model or self.effective_model(),
                               effort=effort or self.settings.get("effort"), cost_usd=cost if have_cost else None,
                               latency_ms=int((time.monotonic() - started) * 1000), **base)
        return LlmResponse(status="blocked", blockers=[last_code], attempts=attempts, usage=usage, model=model or self.effective_model(),
                           effort=effort or self.settings.get("effort"), cost_usd=cost if have_cost else None,
                           latency_ms=int((time.monotonic() - started) * 1000), **base)
