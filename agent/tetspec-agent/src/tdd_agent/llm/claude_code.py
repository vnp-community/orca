"""Claude Code provider (default).

Runs ``claude -p`` non-interactively against the user's existing Claude Code
login.  The flag set was verified against Claude Code 2.1.x (task 2.1): a bare
call loads ~106k tokens of default context, so the provider replaces the system
prompt, disables tools, settings, slash commands, MCP and Chrome integration;
``--bare`` is deliberately not used because it disables the subscription login.
"""
from __future__ import annotations

import json
import os

from .base import (READ_ONLY_TOOLS, LlmProvider, LlmRequest, ProviderCapabilities, ProviderFailure, RawResult)
from .cli import classify_failure, isolated_cwd, require_binary, run, version_of

DEFAULT_SYSTEM = "You are a structured-output function inside an automated pipeline. Reply only with what the caller's schema requests."


class ClaudeCodeProvider(LlmProvider):
    name = "claude_code"

    def __init__(self, settings: dict | None = None):
        super().__init__(settings)
        self.command = self.settings.get("command", "claude")
        self.model = self.settings.get("model") or None
        self.effort = self.settings.get("effort") or None

    def capabilities(self) -> ProviderCapabilities:
        return ProviderCapabilities(structured_output=True, read_only_sandbox=True, tools=READ_ONLY_TOOLS)

    def describe(self) -> dict:
        return {"provider": self.name, "command": self.command, "model": self.model, "effort": self.effort,
                "cli_version": version_of(self.command), "timeout_seconds": self.default_timeout_s, "max_attempts": self.default_attempts}

    def effective_model(self) -> str | None:
        return self.model

    def build_command(self, request: LlmRequest, binary: str) -> list[str]:
        if request.allowed_tools:
            bad = [t for t in request.allowed_tools if t not in READ_ONLY_TOOLS]
            if bad:
                raise ProviderFailure("PROVIDER_OPTION_UNSUPPORTED", f"tools not permitted for read-only use: {bad}", retryable=False)
            tools = ",".join(request.allowed_tools)
        else:
            tools = ""
        system = request.system or self.settings.get("system_prompt") or DEFAULT_SYSTEM
        cmd = [binary, "-p", "--output-format", "json", "--no-session-persistence", "--system-prompt", system,
               "--setting-sources", "", "--disable-slash-commands", "--strict-mcp-config", "--no-chrome",
               "--exclude-dynamic-system-prompt-sections", "--tools", tools]
        if self.model:
            cmd += ["--model", self.model]
        if self.effort:
            cmd += ["--effort", self.effort]
        if request.output_schema is not None:
            cmd += ["--json-schema", json.dumps(request.output_schema)]
        return cmd

    def _invoke(self, request: LlmRequest, attempt: int) -> RawResult:
        binary = require_binary(self.command)
        command = self.build_command(request, binary)
        timeout = request.timeout_s or self.default_timeout_s
        env = dict(os.environ)
        with isolated_cwd(request.cwd) as cwd:
            result = run(command, stdin=request.prompt, cwd=cwd, timeout=timeout, env=env)
        stdout, stderr = result.stdout.strip(), result.stderr.strip()
        payload = None
        if stdout:
            try:
                payload = json.loads(stdout)
            except json.JSONDecodeError:
                payload = None
        if isinstance(payload, dict) and payload.get("is_error"):
            text = f"{payload.get('result', '')} {payload.get('api_error_status', '')} {stderr}"
            failure = classify_failure(text) or ProviderFailure("PROVIDER_ERROR", "model call failed", retryable=True, diagnostics=text)
            failure.diagnostics = failure.diagnostics or text
            raise failure
        if result.returncode != 0:
            text = f"{stdout} {stderr}"
            failure = classify_failure(text) or ProviderFailure("PROVIDER_ERROR", f"exit code {result.returncode}", retryable=True, diagnostics=text)
            raise failure
        if not isinstance(payload, dict):
            raise ProviderFailure("PROVIDER_BAD_OUTPUT", "stdout is not the expected JSON envelope", retryable=True, diagnostics=stdout[:400])
        usage = payload.get("usage") or {}
        flat = {k: v for k, v in usage.items() if isinstance(v, (int, float))}
        model_usage = payload.get("modelUsage") or {}
        return RawResult(text=str(payload.get("result") or ""), structured=payload.get("structured_output"), usage=flat,
                         model=next(iter(model_usage), None) or self.model, effort=self.effort,
                         cost_usd=payload.get("total_cost_usd"), diagnostics=stderr[-400:])
