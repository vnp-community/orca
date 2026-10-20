"""Per-call LLM audit records.

Prompt and response payloads are *never* stored by default: ``none`` stores
nothing beyond hashes already present in the record, ``hash`` is the same plus
output hash, ``full`` stores the payload after redaction and must be enabled
explicitly.
"""
from __future__ import annotations

from hashlib import sha256
from typing import Any

from ..llm.base import LlmRequest, LlmResponse
from ..store.base import OperationalStore, now_iso
from . import context
from .redact import Redactor


def build_record(request: LlmRequest, response: LlmResponse, *, mode: str, redactor: Redactor, skill: dict | None = None) -> dict[str, Any]:
    record = response.audit_record()
    record.update({
        "ts": now_iso(), "run_id": context.get("run_id") or request.metadata.get("run_id"), "trace_id": context.get("trace_id"),
        "span_id": context.get("span_id"), "agent": context.get("agent"), "gate": context.get("gate"), "step": context.get("step"),
        "payload_mode": mode,
    })
    if skill:
        record["skill"] = skill
    record["output_hash"] = sha256((response.text or "").encode()).hexdigest()
    if mode == "full":
        record["payload"] = {"system": redactor.text(request.system or ""), "prompt": redactor.text(request.prompt),
                             "response": redactor.text(response.text or "")}
    return record


def record_call(store: OperationalStore, request: LlmRequest, response: LlmResponse, *, mode: str, redactor: Redactor, skill: dict | None = None) -> dict:
    record = build_record(request, response, mode=mode, redactor=redactor, skill=skill)
    store.add_llm_call(record)
    return record
