"""Shared subprocess helpers for CLI-backed providers."""
from __future__ import annotations

import re
import shutil
import subprocess
import tempfile
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator, Sequence

from .base import ProviderFailure

_RATE = re.compile(r"rate[ _-]?limit|usage limit|limit (?:reached|exceeded)|too many requests|\b429\b|quota", re.I)
_AUTH = re.compile(r"not logged in|please run /?login|\blogin required\b|invalid api key|authentication (?:failed|required|error)|unauthorized|\b401\b", re.I)
_OPTION = re.compile(r"unknown (?:option|argument|flag)|unrecognized (?:option|argument|arguments)|invalid value|invalid (?:model|effort)|"
                     r"(?:model|effort)[^.\n]{0,40}(?:not (?:found|supported|available)|unsupported|invalid)|unexpected argument|"
                     r"issue with the selected model", re.I)
_MODEL = re.compile(r"The '[^']+' model is not supported|model is not supported when using|model[^.\n]{0,40}does not exist or you do not have access", re.I)
_OVERLOAD = re.compile(r"overloaded|\b529\b|\b503\b|temporarily unavailable|connection (?:reset|error)", re.I)


@dataclass
class CliResult:
    returncode: int
    stdout: str
    stderr: str


def classify_failure(text: str) -> ProviderFailure | None:
    """Map CLI/model error text to a provider failure; ``None`` when unrecognized."""
    if _RATE.search(text):
        return ProviderFailure("PROVIDER_RATE_LIMITED", "provider usage or rate limit reached", retryable=False, diagnostics=text)
    if _AUTH.search(text):
        return ProviderFailure("PROVIDER_AUTH_REQUIRED", "provider is not authenticated", retryable=False, diagnostics=text)
    if _MODEL.search(text):
        found = re.search(r"The '([^']+)' model", text)
        return ProviderFailure("PROVIDER_MODEL_UNSUPPORTED", f"the model{' ' + repr(found.group(1)) if found else ''} cannot be used with this account: set [llm.<provider>] model to one that can", retryable=False, diagnostics=text)
    if _OPTION.search(text):
        return ProviderFailure("PROVIDER_OPTION_UNSUPPORTED", "provider rejected a configured option", retryable=False, diagnostics=text)
    if _OVERLOAD.search(text):
        return ProviderFailure("PROVIDER_ERROR", "provider temporarily unavailable", retryable=True, diagnostics=text)
    return None


def require_binary(command: str) -> str:
    path = shutil.which(command)
    if not path:
        raise ProviderFailure("PROVIDER_UNAVAILABLE", f"command not found on PATH: {command}", retryable=False)
    return path


@contextmanager
def isolated_cwd(requested: Path | None) -> Iterator[Path]:
    if requested is not None:
        requested.mkdir(parents=True, exist_ok=True)
        yield requested
        return
    with tempfile.TemporaryDirectory(prefix="tdd-agent-llm-") as folder:
        yield Path(folder)


def run(command: Sequence[str], *, stdin: str, cwd: Path, timeout: int, env: dict | None = None) -> CliResult:
    try:
        done = subprocess.run(list(command), input=stdin, cwd=str(cwd), capture_output=True, text=True, timeout=timeout, env=env)
    except subprocess.TimeoutExpired as exc:
        raise ProviderFailure("PROVIDER_TIMEOUT", f"command exceeded {timeout}s", retryable=True,
                              diagnostics=(exc.stderr.decode(errors="replace") if isinstance(exc.stderr, bytes) else (exc.stderr or ""))) from exc
    except OSError as exc:
        raise ProviderFailure("PROVIDER_UNAVAILABLE", f"cannot execute {command[0]}: {exc}", retryable=False) from exc
    return CliResult(done.returncode, done.stdout, done.stderr)


def version_of(command: str, timeout: int = 15) -> str | None:
    path = shutil.which(command)
    if not path:
        return None
    try:
        out = subprocess.run([path, "--version"], capture_output=True, text=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return (out.stdout or out.stderr).strip().splitlines()[0] if (out.stdout or out.stderr).strip() else None
