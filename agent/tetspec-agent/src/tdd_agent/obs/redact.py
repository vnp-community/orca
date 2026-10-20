"""Secret and payload redaction shared by logs, traces and the LLM audit."""
from __future__ import annotations

import re
from typing import Any

SECRET_KEY = re.compile(r"(token|secret|password|passwd|api[_-]?key|authorization|credential)", re.I)
DEFAULT_PATTERNS = [
    re.compile(r"sk-[A-Za-z0-9_\-]{16,}"),
    re.compile(r"Bearer\s+[A-Za-z0-9._\-]{12,}", re.I),
    re.compile(r"(?i)\b(api[_-]?key|token|secret|password)\s*[=:]\s*[^\s,;\"']{6,}"),
    re.compile(r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}"),
]
MASK = "***"


class Redactor:
    def __init__(self, extra_patterns: list[str] | None = None):
        self.patterns = list(DEFAULT_PATTERNS) + [re.compile(p) for p in (extra_patterns or [])]

    def text(self, value: str) -> str:
        for pattern in self.patterns:
            value = pattern.sub(MASK, value)
        return value

    def data(self, value: Any, key: str = "") -> Any:
        if isinstance(value, dict):
            return {k: self.data(v, str(k)) for k, v in value.items()}
        if isinstance(value, (list, tuple)):
            return [self.data(v, key) for v in value]
        if isinstance(value, str):
            if key and SECRET_KEY.search(key) and not key.lower().endswith("_env"):
                return MASK
            return self.text(value)
        if key and SECRET_KEY.search(key) and value not in (None, False, True, 0, "") and not key.lower().endswith("_env"):
            return MASK
        return value
