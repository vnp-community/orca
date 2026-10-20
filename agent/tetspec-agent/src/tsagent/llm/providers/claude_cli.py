"""Provider `claude-cli`: runs Claude Code in print mode with the machine's existing login (no API key needed).

`--bare` is not used because it disables OAuth login. No tools, no MCP, no saved session, runs in an empty folder so
CLAUDE.md or project settings never leak into the context. The prompt goes through stdin (no argument length limit).
"""
import json
import os
import shutil
import subprocess
import tempfile

from ...errors import LLMPermanent, LLMTransient, LLMUnknownOutcome
from ..policy import CircuitBreaker, RetryPolicy, call_with_retry
from .http_base import Provider

ARG_LIMIT = 64 * 1024  # a larger system prompt is written to a file and passed with --system-prompt-file
LOGIN_HINTS = ("not logged in", "please run /login", "invalid api key", "authentication", "oauth token")


class ClaudeCliProvider(Provider):
    name = "claude-cli"

    def __init__(self, model="", binary="", timeout_s=600, policy=None, breaker=None, run=subprocess.run, sleep=None):
        self.model = model
        self.binary = binary or shutil.which("claude") or "claude"
        self.timeout_s = timeout_s
        self.policy = policy or RetryPolicy(max_attempts=3)
        self.breaker = breaker or CircuitBreaker()
        self._run = run
        self._sleep = sleep

    @property
    def tag(self):
        return f"{self.name}:{self.model or 'default'}"

    def argv(self, system, workdir):
        cmd = [self.binary, "-p", "--output-format", "json", "--tools", "", "--strict-mcp-config",
               "--no-session-persistence", "--disable-slash-commands"]
        if len(system.encode("utf-8")) > ARG_LIMIT:
            path = os.path.join(workdir, "system-prompt.txt")
            with open(path, "w", encoding="utf-8") as fh:
                fh.write(system)
            cmd += ["--system-prompt-file", path]
        else:
            cmd += ["--system-prompt", system]
        if self.model:
            cmd += ["--model", self.model]
        return cmd

    @staticmethod
    def classify(proc):
        """CompletedProcess -> result text, or raise an error following the ErrorMapping."""
        out = (proc.stdout or "").strip()
        err = (proc.stderr or "").strip()
        data = None
        if out:
            try:
                data = json.loads(out.splitlines()[-1] if not out.startswith("{") else out)
            except ValueError:
                data = None
        if isinstance(data, dict) and not data.get("is_error") and proc.returncode == 0:
            res = data.get("result")
            if not isinstance(res, str):
                raise LLMPermanent("claude returned JSON without a result field")
            return res
        status = data.get("api_error_status") if isinstance(data, dict) else None
        msg = (data.get("result") if isinstance(data, dict) else None) or err or out or f"exit code {proc.returncode}"
        msg = str(msg)[:300]
        low = msg.lower()
        if status in (401, 403) or any(h in low for h in LOGIN_HINTS):
            raise LLMPermanent(f"claude is not logged in or not authorised: {msg}", fatal=True)
        if status == 429 or (isinstance(status, int) and status >= 500) or "overloaded" in low or "rate limit" in low:
            raise LLMTransient(f"claude: {msg}")
        raise LLMPermanent(f"claude: {msg}")

    def _once(self, system, user):
        with tempfile.TemporaryDirectory(prefix="tsagent-claude-") as wd:
            try:
                proc = self._run(self.argv(system, wd), input=user, capture_output=True, text=True, cwd=wd,
                                 timeout=self.timeout_s, env={**os.environ, "CLAUDE_CODE_DISABLE_TERMINAL_TITLE": "1"})
            except subprocess.TimeoutExpired:
                raise LLMUnknownOutcome(f"claude exceeded {self.timeout_s}s") from None
            except FileNotFoundError:
                raise LLMPermanent(f"claude command not found ({self.binary}); set CLAUDE_BIN", fatal=True) from None
        return self.classify(proc)

    def send(self, system, user):
        kw = {"sleep": self._sleep} if self._sleep else {}
        return call_with_retry(lambda: self._once(system, user), self.policy, self.breaker, **kw)
