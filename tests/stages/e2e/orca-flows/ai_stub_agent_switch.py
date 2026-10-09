"""Runs the dev-server agent against the fake `claude` for `run_acceptance.py --ai-stub`, then puts the real setup back.

Nothing in the product knows about the stub. The runner only changes the environment of the agent process:

  * ai.complete runs `claude` from the agent's PATH, so a temp directory holding the stub goes first on PATH;
  * the capability probe (`claude auth status`, which decides `aiSources`) looks in a fixed list under $HOME
    (~/.local/bin first), so the agent also gets a throwaway HOME whose ~/.local/bin/claude is the stub.

Both findings are read from agent/src/relay (ai-complete-cli-session.ts, agent-config.ts buildToolPath). POSIX only.
"""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import tempfile
import time
from dataclasses import replace
from pathlib import Path

from orca_session import OrcaSession
from orca_stack_config import REPO_ROOT, StackConfig

START_SCRIPT = REPO_ROOT / "deploy" / "dev" / "scripts" / "start-local-agent.sh"
STUB_SOURCE = Path(__file__).resolve().parent / "fake_claude_cli" / "claude"
AGENT_BUNDLE = REPO_ROOT / "agent" / "out" / "agent.js"
# Variables that tell a restarted agent which machine/repo it serves; the rest of its environment is rebuilt by the script.
PRESERVED_AGENT_ENV = ("AGENT_WORK_DIR", "DEV_SERVER_ID")


class StubAgentError(RuntimeError):
    pass


def _pids_of_agent() -> list[int]:
    out = subprocess.run(["pgrep", "-f", "agent/out/agent.js"], capture_output=True, text=True)
    return sorted(int(p) for p in out.stdout.split() if p.isdigit())


def _process_environment(pid: int) -> dict[str, str]:
    """Environment of a process we own. /proc on Linux; `ps eww` on macOS (values with spaces are cut, which only loses the tail)."""
    proc = Path(f"/proc/{pid}/environ")
    if proc.is_file():
        try:
            return dict(item.split("=", 1) for item in proc.read_bytes().decode(errors="replace").split("\0") if "=" in item)
        except OSError:
            return {}
    out = subprocess.run(["ps", "eww", "-p", str(pid)], capture_output=True, text=True).stdout
    return {m.group(1): m.group(2) for m in re.finditer(r"(?<=\s)([A-Z][A-Z0-9_]*)=(\S*)", out) if m.group(1) in PRESERVED_AGENT_ENV}


def _stdout_path(pid: int) -> str:
    link = Path(f"/proc/{pid}/fd/1")
    if link.exists():
        try:
            return os.readlink(link)
        except OSError:
            return ""
    out = subprocess.run(["lsof", "-a", "-p", str(pid), "-d", "1", "-Fn"], capture_output=True, text=True).stdout
    for line in out.splitlines():
        if line.startswith("n/"):
            return line[1:]
    return ""


def capture_running_agent() -> dict[str, str]:
    """What restore() must reproduce: work dir, dev-server id and log file of the agent running now (empty when none)."""
    for pid in _pids_of_agent():
        env = _process_environment(pid)
        if env.get("DEV_SERVER_ID"):
            found = {k: env[k] for k in PRESERVED_AGENT_ENV if env.get(k)}
            log = _stdout_path(pid)
            if log.startswith("/") and Path(log).is_file():
                found["AGENT_LOG"] = log
            return found
    return {}


class StubbedAgent:
    """Owns the stub's throwaway HOME and the agent process started against it."""

    def __init__(self, cfg: StackConfig) -> None:
        if os.name == "nt":
            raise StubAgentError("--ai-stub needs a POSIX host: the fake claude is a script, and a Windows agent resolves claude.cmd")
        if not AGENT_BUNDLE.is_file():
            raise StubAgentError(f"{AGENT_BUNDLE} is missing: run `node agent/build.mjs` first")
        self.cfg = cfg
        self.current_cfg = cfg
        self.original = capture_running_agent()
        self.home = Path(tempfile.mkdtemp(prefix="orca-fake-claude-home-"))
        self.state_dir = self.home / ".fake-claude"
        self.log = self.home / "agent.log"
        self._install_stub()

    @property
    def calls_file(self) -> Path:
        return self.state_dir / "calls.jsonl"

    def _install_stub(self) -> None:
        bin_dir = self.home / ".local" / "bin"
        bin_dir.mkdir(parents=True)
        STUB_SOURCE.chmod(STUB_SOURCE.stat().st_mode | 0o111)  # a checkout on a filesystem that drops the exec bit
        (bin_dir / "claude").symlink_to(STUB_SOURCE)
        self.state_dir.mkdir()
        real_home = Path(os.path.expanduser("~"))
        for name in (".gitconfig", ".config/git", ".ssh/known_hosts"):  # git identity for repos the agent touches
            src = real_home / name
            if src.exists():
                dst = self.home / name
                dst.parent.mkdir(parents=True, exist_ok=True)
                dst.symlink_to(src)

    def set_logged_in(self, logged_in: bool) -> None:
        (self.state_dir / "logged_in").write_text("1" if logged_in else "0")

    def calls(self) -> list[dict]:
        if not self.calls_file.is_file():
            return []
        return [json.loads(line) for line in self.calls_file.read_text().splitlines() if line.strip()]

    # ── process control ──

    def dev_server_name(self, logged_in: bool) -> str:
        """A dev-server identity per login state. infra-fleet keeps a dev server's capability profile for up to 24h and
        re-probes on attach at most every 5 minutes, so reusing one identity would let the stub's login change go unseen."""
        suffix = os.environ.get("ORCA_E2E_STUB_IDENTITY_SUFFIX", "")  # a new suffix forces a fresh capability probe (see README)
        return f"{self.cfg.dev_server_name}-stub-{'in' if logged_in else 'out'}{suffix}"

    def _run_start_script(self, env: dict[str, str], cfg: StackConfig, approve: bool, wait_seconds: float = 90.0) -> None:
        done = subprocess.run(["bash", str(START_SCRIPT)], env=env, cwd=str(REPO_ROOT), stdin=subprocess.DEVNULL,
                              capture_output=True, text=True, timeout=60)
        if done.returncode != 0:
            raise StubAgentError(f"start-local-agent.sh failed ({done.returncode}): {done.stderr.strip()[:300]}")
        wait_until_connected(cfg, wait_seconds, approve)

    def start(self, logged_in: bool) -> None:
        """(Re)start the agent with the stub first on PATH and as ~/.local/bin/claude. `logged_in` is the stub's login state."""
        self.set_logged_in(logged_in)
        env = dict(os.environ)
        env.update({
            "HOME": str(self.home),
            "PATH": f"{self.home / '.local' / 'bin'}{os.pathsep}{env.get('PATH', '')}",
            "FAKE_CLAUDE_STATE_DIR": str(self.state_dir),
            "FAKE_CLAUDE_LOGGED_IN": "1" if logged_in else "0",
            "AGENT_LOG": str(self.log),
        })
        env.pop("ORCA_AI_CLI_SESSION", None)  # the kill switch must not hide the path under test
        for k in ("ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY"):
            env.pop(k, None)
        if self.original.get("AGENT_WORK_DIR") and not os.environ.get("AGENT_WORK_DIR"):
            env["AGENT_WORK_DIR"] = self.original["AGENT_WORK_DIR"]
        env["DEV_SERVER_ID"] = self.dev_server_name(logged_in)
        self.current_cfg = replace(self.cfg, dev_server_name=env["DEV_SERVER_ID"])
        self._run_start_script(env, self.current_cfg, approve=True)

    def restore(self) -> str:
        """Back to the real setup: the agent as it was (same work dir/dev server/log), real HOME and PATH, no stub anywhere."""
        env = {k: v for k, v in os.environ.items() if not k.startswith("FAKE_CLAUDE_")}
        for k, v in self.original.items():
            env.setdefault(k, v)
        try:
            self._run_start_script(env, self.cfg, approve=False)
            return "agent restored without the stub"
        finally:
            shutil.rmtree(self.home, ignore_errors=True)


def wait_until_connected(cfg: StackConfig, timeout: float, approve: bool) -> None:
    """The dev server is `connected` (and, for a new stub identity, approved by the admin the suites log in as)."""
    deadline = time.time() + timeout
    time.sleep(4)  # let the old connection drop before trusting `connected`
    last = "unknown"
    while time.time() < deadline:
        session = OrcaSession(cfg)
        try:
            if session.login().status == 200:
                for server in session.ws().call("devServer.list", {}):
                    if server.get("name") == cfg.dev_server_name:
                        last = str(server.get("status"))
                        if approve and server.get("approvalStatus") != "approved":
                            session.ws().call("devServer.approve", {"devServerId": server["id"]})
                        if last == "connected":
                            time.sleep(3)  # the capability probe runs on attach; give it a moment before the first AI call
                            return
        except Exception as exc:  # noqa: BLE001 - the gateway may be busy for a moment; keep polling
            last = f"{type(exc).__name__}"
        finally:
            session.close()
        time.sleep(2)
    raise StubAgentError(f"dev server {cfg.dev_server_name!r} did not reconnect within {timeout:.0f}s (last status: {last})")


def describe_stub_for_report(stub: StubbedAgent) -> dict:
    return {
        "mode": "stub",
        "claude": f"fake-claude (tests/stages/e2e/orca-flows/fake_claude_cli/claude), calls recorded: {len(stub.calls())}",
        "note": "AI answers come from a deterministic fixture. This run proves the wiring and contracts, not AI quality.",
    }
