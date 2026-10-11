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

from agent_push_block import PUSH_BLOCK_ENV  # noqa: E402

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


if __name__ == "__main__":
    unittest.main()
