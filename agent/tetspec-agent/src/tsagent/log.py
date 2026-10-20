"""JSON logs carrying runId, step and node code (MECH-10 of v1, kept in v2); secret patterns are masked before writing (MECH-06)."""
import json
import logging
import re
import time

SECRET_PATTERNS = [
    re.compile(r"sk-ant-[A-Za-z0-9_\-]{8,}"),
    re.compile(r"sk-[A-Za-z0-9]{20,}"),
    re.compile(r"(?i)(bearer\s+)[A-Za-z0-9._\-]{8,}"),
    re.compile(r"(?i)((?:api[_-]?key|token|password|secret)\s*[=:]\s*)['\"]?[^\s'\",;]{6,}"),
]


def mask_secrets(text):
    s = str(text)
    for p in SECRET_PATTERNS:
        s = p.sub(lambda m: (m.group(1) if m.groups() else "") + "***", s)
    return s


class JsonFormatter(logging.Formatter):
    def format(self, record):
        out = {"ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(record.created)), "level": record.levelname,
               "logger": record.name, "msg": mask_secrets(record.getMessage())}
        for k in ("runId", "step", "code", "feature"):
            v = getattr(record, k, None)
            if v is not None:
                out[k] = v
        if record.exc_info:
            out["exc"] = mask_secrets(self.formatException(record.exc_info))
        return json.dumps(out, ensure_ascii=False)


def get_logger(name="tsagent", **context):
    """Logger bound to a context: get_logger(runId=..., step=...)."""
    base = logging.getLogger(name)
    return logging.LoggerAdapter(base, context) if context else base


def setup(level="INFO", stream=None):
    h = logging.StreamHandler(stream)
    h.setFormatter(JsonFormatter())
    root = logging.getLogger("tsagent")
    root.handlers[:] = [h]
    root.setLevel(level)
    root.propagate = False
    return root
