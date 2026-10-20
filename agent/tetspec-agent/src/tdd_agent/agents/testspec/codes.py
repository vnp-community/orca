"""Normalise tdd-agent element codes into valid tsagent codes (SOLUTION section 6.5)."""
from __future__ import annotations

import hashlib
import re
import unicodedata

CODE_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")


def norm_code(raw: str) -> str:
    """Valid codes are kept; a code that must change gets the suffix ``-h`` + 6 hash characters of the original (stable, unique)."""
    raw = str(raw)
    if CODE_RE.match(raw):
        return raw
    s = unicodedata.normalize("NFKD", raw.replace("đ", "d").replace("Đ", "D")).encode("ascii", "ignore").decode()
    s = re.sub(r"[^A-Za-z0-9._-]+", "-", s).strip("-._") or "X"
    if not s[0].isalnum():
        s = "X" + s
    return f"{s}-h{hashlib.sha256(raw.encode('utf-8')).hexdigest()[:6]}"


def norm_feature(raw: str) -> str:
    """Feature code (at most 64 characters, no ``..``)."""
    code = norm_code(raw).replace("..", ".")
    if len(code) > 64:
        code = code[:57] + "-h" + hashlib.sha256(str(raw).encode("utf-8")).hexdigest()[:5]
    return code


class CodeMap:
    """Records the changed codes: {tsagent code: original code}."""

    def __init__(self):
        self.changed: dict[str, str] = {}

    def __call__(self, raw: str) -> str:
        code = norm_code(raw)
        if code != raw:
            self.changed[code] = str(raw)
        return code
