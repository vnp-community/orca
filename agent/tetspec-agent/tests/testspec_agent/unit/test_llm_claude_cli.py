"""T4.2: claude-cli provider: call flags, error classification, a fake `claude` file run for real with a large prompt over stdin."""
import json
import os
import subprocess
import sys

import pytest

from tsagent.errors import LLMPermanent, LLMTransient, LLMUnknownOutcome
from tsagent.llm.policy import RetryPolicy
from tsagent.llm.providers.claude_cli import ClaudeCliProvider

pytestmark = pytest.mark.unit


def done(payload, rc=0, stderr=""):
    return subprocess.CompletedProcess([], rc, json.dumps(payload) if isinstance(payload, dict) else payload, stderr)


def prov(results, **kw):
    seen = []

    def run(cmd, **k):
        seen.append((cmd, k))
        r = results[min(len(seen) - 1, len(results) - 1)]
        if isinstance(r, Exception):
            raise r
        return r
    p = ClaudeCliProvider(binary="claude", run=run, sleep=lambda s: None, policy=RetryPolicy(max_attempts=3, base_s=0), **kw)
    return p, seen


def test_flags_stdin_and_empty_cwd():
    p, seen = prov([done({"type": "result", "is_error": False, "result": "{}"})], model="sonnet")
    assert p.send("SYS", "USER") == "{}"
    cmd, k = seen[0]
    for flag in ("-p", "--output-format", "--strict-mcp-config", "--no-session-persistence", "--disable-slash-commands"):
        assert flag in cmd
    assert cmd[cmd.index("--tools") + 1] == "" and cmd[cmd.index("--system-prompt") + 1] == "SYS"
    assert "--bare" not in cmd and cmd[cmd.index("--model") + 1] == "sonnet"
    assert k["input"] == "USER" and os.path.basename(k["cwd"]).startswith("tsagent-claude-")


def test_large_system_prompt_goes_to_file():
    p, seen = prov([done({"is_error": False, "result": "{}"})])
    p.send("x" * 70000, "u")
    assert "--system-prompt-file" in seen[0][0] and "--system-prompt" not in seen[0][0]


@pytest.mark.parametrize("res,exc,fatal", [
    (done({"is_error": True, "api_error_status": 401, "result": "Invalid API key"}, 1), LLMPermanent, True),
    (done("", 1, "Not logged in · Please run /login"), LLMPermanent, True),
    (done({"is_error": True, "api_error_status": 400, "result": "bad"}, 1), LLMPermanent, False),
])
def test_permanent_errors(res, exc, fatal):
    p, seen = prov([res])
    with pytest.raises(exc) as e:
        p.send("s", "u")
    assert e.value.fatal is fatal and len(seen) == 1


def test_transient_retried():
    p, seen = prov([done({"is_error": True, "api_error_status": 529, "result": "Overloaded"}, 1),
                    done({"is_error": False, "result": "ok"})])
    assert p.send("s", "u") == "ok" and len(seen) == 2


def test_transient_exhausted():
    p, _ = prov([done({"is_error": True, "api_error_status": 429, "result": "rate limit"}, 1)])
    with pytest.raises(LLMTransient):
        p.send("s", "u")


def test_timeout_is_unknown_outcome():
    p, seen = prov([subprocess.TimeoutExpired("claude", 1)])
    with pytest.raises(LLMUnknownOutcome):
        p.send("s", "u")
    assert len(seen) == 3


def test_missing_binary_is_fatal():
    p = ClaudeCliProvider(binary="/nonexistent/claude")
    with pytest.raises(LLMPermanent) as e:
        p.send("s", "u")
    assert e.value.fatal


def test_real_fake_binary_with_large_stdin(tmp_path):
    script = tmp_path / "claude"
    script.write_text(f"#!{sys.executable}\nimport json,sys,os\ndata=sys.stdin.read()\n"
                      "print(json.dumps({'type':'result','is_error':False,'result':json.dumps({'n':len(data),"
                      "'cwd_empty':len([f for f in os.listdir('.') if f!='system-prompt.txt'])==0})}))\n")
    script.chmod(0o755)
    p = ClaudeCliProvider(binary=str(script), timeout_s=30)
    out = json.loads(p.send("s", "é" * 300000))
    assert out == {"n": 300000, "cwd_empty": True}


@pytest.mark.llm_real
def test_real_claude_logged_in():
    p = ClaudeCliProvider(timeout_s=180)
    out = p.send("Trả lời đúng một đối tượng JSON.", 'Trả về {"ok": true}')
    assert "ok" in out
