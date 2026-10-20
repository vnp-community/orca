"""The push block exists twice (Python starts the agent with it, TypeScript checks that the agent has it); they must not drift.

  python3 -m unittest tests/stages/e2e/orca-flows/test_agent_push_block.py
"""
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import agent_push_block  # noqa: E402
from agent_push_block import PUSH_BLOCK_ENV, PushBlockedAgent  # noqa: E402

FIXTURE_TS = HERE / "ui-live" / "deepseek-repo-fixture.ts"


class PushBlockTest(unittest.TestCase):
    def test_typescript_constant_matches_python(self) -> None:
        block = re.search(r"PUSH_BLOCK_ENV[^{]*\{(.*?)\n\}", FIXTURE_TS.read_text(), re.S)
        self.assertIsNotNone(block)
        pairs = dict(re.findall(r"(GIT_CONFIG_\w+):\s*'([^']*)'", block.group(1)))
        self.assertEqual(pairs, PUSH_BLOCK_ENV)

    def test_every_push_form_fails_locally(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            run = lambda *a, **k: subprocess.run(a, cwd=repo, capture_output=True, text=True, timeout=20, **k)  # noqa: E731
            run("git", "init", "-q", "-b", "main")
            run("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "x")
            run("git", "remote", "add", "origin", "git@github.com:example/example.git")
            env = {**__import__("os").environ, **PUSH_BLOCK_ENV, "GIT_TERMINAL_PROMPT": "0"}
            for target in ("origin", "git@github.com:example/example.git", "ssh://git@github.com/example/example.git",
                           "https://github.com/example/example.git"):
                done = subprocess.run(["git", "push", "--dry-run", target, "HEAD:refs/heads/probe"], cwd=repo, env=env,
                                      capture_output=True, text=True, timeout=20)
                self.assertNotEqual(done.returncode, 0, target)
                self.assertIn("orca-e2e-push-blocked", done.stderr + done.stdout, target)


class WorkDirTest(unittest.TestCase):
    """ci-cd-kit widens AGENT_WORK_DIR for its run only: start() sets it, restore() hands the captured original back."""

    def _agent(self, work_dir):
        original = agent_push_block.capture_running_agent
        agent_push_block.capture_running_agent = lambda: {"AGENT_WORK_DIR": "/orig", "DEV_SERVER_ID": "dev"}
        try:
            agent = PushBlockedAgent(None, work_dir)
        finally:
            agent_push_block.capture_running_agent = original
        seen = []
        agent._restart = lambda env: seen.append(dict(env))  # type: ignore[method-assign]
        return agent, seen

    def test_start_sets_and_restore_drops_the_work_dir(self) -> None:
        agent, seen = self._agent("/wide")
        agent.start()
        agent.restore()
        self.assertEqual(seen[0]["AGENT_WORK_DIR"], "/wide")
        self.assertEqual(seen[0]["GIT_CONFIG_COUNT"], "4")
        self.assertNotIn("AGENT_WORK_DIR", seen[1])  # _restart's setdefault then puts "/orig" back
        self.assertFalse(any(k.startswith("GIT_CONFIG_") for k in seen[1]))

    def test_without_a_work_dir_nothing_changes(self) -> None:
        agent, seen = self._agent(None)
        agent.start()
        self.assertNotEqual(seen[0].get("AGENT_WORK_DIR"), "/wide")


if __name__ == "__main__":
    unittest.main()
