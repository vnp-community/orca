"""Structured JSON logging (one event per line)."""
from __future__ import annotations

import json
import logging
import logging.handlers
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from . import context
from .redact import Redactor

LOGGER_NAME = "tdd_agent"
_STD = set(logging.LogRecord("x", 0, "", 0, "", (), None).__dict__) | {"message", "asctime", "fields"}


class JsonFormatter(logging.Formatter):
    def __init__(self, redactor: Redactor | None = None):
        super().__init__()
        self.redactor = redactor or Redactor()

    def format(self, record: logging.LogRecord) -> str:
        event: dict[str, Any] = {
            "ts": datetime.fromtimestamp(record.created, timezone.utc).isoformat(timespec="milliseconds"),
            "level": record.levelname,
            "logger": record.name,
            "event": record.getMessage(),
        }
        event.update(context.snapshot())
        fields = getattr(record, "fields", None)
        if fields:
            event.update(self.redactor.data(fields))
        if record.exc_info:
            event["exception"] = self.redactor.text(self.formatException(record.exc_info))
        return json.dumps(event, ensure_ascii=False, default=str)


def configure_logging(level: str = "INFO", log_file: str | Path | None = None, to_stderr: bool = False,
                      extra_patterns: list[str] | None = None) -> logging.Logger:
    logger = logging.getLogger(LOGGER_NAME)
    logger.setLevel(getattr(logging, level.upper(), logging.INFO))
    logger.propagate = False
    for handler in list(logger.handlers):
        logger.removeHandler(handler)
        handler.close()
    formatter = JsonFormatter(Redactor(extra_patterns))
    if log_file:
        path = Path(log_file)
        path.parent.mkdir(parents=True, exist_ok=True)
        handler: logging.Handler = logging.handlers.RotatingFileHandler(path, maxBytes=10_000_000, backupCount=5, encoding="utf-8")
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    if to_stderr or not logger.handlers:
        stream = logging.StreamHandler(sys.stderr)
        stream.setFormatter(formatter)
        logger.addHandler(stream)
    return logger


def get_logger(name: str = "") -> logging.Logger:
    return logging.getLogger(f"{LOGGER_NAME}.{name}" if name else LOGGER_NAME)


def log_event(logger: logging.Logger, event: str, level: int = logging.INFO, **fields: Any) -> None:
    logger.log(level, event, extra={"fields": fields})
