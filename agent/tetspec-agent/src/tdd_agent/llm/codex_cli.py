"""Codex CLI provider (``codex exec`` in a read-only sandbox)."""
from __future__ import annotations

import json
import os
import re
import tempfile
from pathlib import Path

from .base import LlmProvider, LlmRequest, ProviderCapabilities, ProviderFailure, RawResult
from .cli import classify_failure, isolated_cwd, require_binary, run, version_of


class CodexCliProvider(LlmProvider):
    name = "codex_cli"

    def __init__(self, settings: dict | None = None):
        super().__init__(settings)
        self.command = self.settings.get("command", "codex")
        self.model = self.settings.get("model") or None
        self.isolate = bool(self.settings.get("isolate", True))
        self.last_model: str | None = None  # the model Codex reported it used (it picks one itself when none is configured)

    def capabilities(self) -> ProviderCapabilities:
        return ProviderCapabilities(structured_output=True, read_only_sandbox=True, tools=())

    def describe(self) -> dict:
        return {"provider": self.name, "command": self.command, "model": self.model or self.last_model or "the Codex default", "isolate": self.isolate, "cli_version": version_of(self.command),
                "timeout_seconds": self.default_timeout_s, "max_attempts": self.default_attempts}

    def effective_model(self) -> str | None:
        return self.model or self.last_model

    def build_command(self, binary: str, schema_path: Path | None, output_path: Path) -> list[str]:
        cmd = [binary, "exec"]
        if self.model:
            cmd += ["--model", self.model]
        cmd += ["--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check"]
        if self.isolate:
            cmd += ["--ignore-user-config", "--ignore-rules"]
        if schema_path is not None:
            cmd += ["--output-schema", str(schema_path)]
        cmd += ["-o", str(output_path), "-"]
        return cmd

    def _invoke(self, request: LlmRequest, attempt: int) -> RawResult:
        if request.allowed_tools:
            raise ProviderFailure("PROVIDER_OPTION_UNSUPPORTED", "codex_cli provider does not expose tools", retryable=False)
        binary = require_binary(self.command)
        timeout = request.timeout_s or self.default_timeout_s
        with tempfile.TemporaryDirectory(prefix="tdd-agent-codex-") as work:
            work_dir = Path(work)
            schema_path = None
            if request.output_schema is not None:
                schema_path = work_dir / "schema.json"
                schema_path.write_text(json.dumps(request.output_schema))
            out_path = work_dir / "output.txt"
            prompt = request.prompt if not request.system else f"{request.system}\n\n{request.prompt}"
            with isolated_cwd(request.cwd) as cwd:
                result = run(self.build_command(binary, schema_path, out_path), stdin=prompt, cwd=cwd, timeout=timeout, env=dict(os.environ))
            if result.returncode != 0:
                text = f"{result.stdout} {result.stderr}"
                raise classify_failure(text) or ProviderFailure("PROVIDER_ERROR", f"exit code {result.returncode}", retryable=True, diagnostics=text)
            if not out_path.is_file():
                raise ProviderFailure("PROVIDER_BAD_OUTPUT", "codex produced no output file", retryable=True, diagnostics=result.stderr[-400:])
            text = out_path.read_text(encoding="utf-8")
        model, usage = read_report(result.stderr)
        self.last_model = model or self.last_model
        return RawResult(text=text, model=model or self.model, usage=usage, diagnostics=result.stderr[-400:])


_MODEL_LINE = re.compile(r"^model:\s*(\S+)", re.M)
_TOKENS = re.compile(r"tokens used\s*\n\s*([\d,]+)", re.I)


def read_report(stderr: str) -> tuple[str | None, dict]:
    """The model and the token count that ``codex exec`` prints on stderr (it reports one total, not input and output separately)."""
    model = _MODEL_LINE.search(stderr)
    tokens = _TOKENS.search(stderr)
    return (model.group(1) if model else None), ({"total_tokens": int(tokens.group(1).replace(",", ""))} if tokens else {})
