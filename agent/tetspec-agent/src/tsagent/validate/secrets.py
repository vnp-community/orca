"""Scan for secret patterns in test KG content and md (MECH-06, TSO-018). Sensitive data may only be written as `secretRef:<name>`."""
import re

PATTERNS = [
    ("anthropicKey", re.compile(r"sk-ant-[A-Za-z0-9_\-]{8,}")),
    ("openaiKey", re.compile(r"\bsk-[A-Za-z0-9]{20,}")),
    ("awsKey", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("githubToken", re.compile(r"\bgh[pousr]_[A-Za-z0-9]{30,}")),
    ("jwt", re.compile(r"\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}")),
    ("privateKey", re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----")),
    ("bearer", re.compile(r"(?i)\bbearer\s+(?!<|\{|secretRef)[A-Za-z0-9._\-]{20,}")),
    ("assignment", re.compile(r"(?i)\b(?:api[_-]?key|password|passwd|secret|token)\s*[=:]\s*['\"]?(?!secretRef:|<|\{|\*)[^\s'\",;<>]{8,}")),
]
SECRET_REF = re.compile(r"^secretRef:[A-Za-z0-9_.\-/]+$")


def find_secrets(text):
    """Names of the patterns matching `text`."""
    s = str(text or "")
    return [name for name, p in PATTERNS if p.search(s)]


def walk_strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for v in value.values():
            yield from walk_strings(v)
    elif isinstance(value, (list, tuple)):
        for v in value:
            yield from walk_strings(v)


def scan_entity(e):
    hits = []
    for v in walk_strings(e.attrs):
        hits += find_secrets(v)
    return sorted(set(hits))
