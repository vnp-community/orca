"""Builds a throwaway git repo from the LedgerLite finance fixture (tests/stages/e2e/fixtures/ledgerlite-finance).

The e2e suites use it as the project a Request is about. It follows the fixture README exactly:

  copy -> tdd-agent generate -> tdd-agent mcp enable --read-only -> git init + one commit (so task worktrees get .mcp.json)

and can add what the base-branch and analysis-revision scenarios need: a `release/1.x` branch whose tip differs from main, a third
branch the primary checkout can be left on (to prove a worktree does NOT start from the checkout's HEAD), and the one-line bug of
scenario 1 (taken from docs/e2e-scenarios.md, so the patch has a single source of truth). Nothing here uses AI or the network.

  repo = create("brf", release_branch=True, third_branch="feature/other")
  try: ...  finally: repo.cleanup()
"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
FIXTURE_DIR = REPO_ROOT / "tests" / "stages" / "e2e" / "fixtures" / "ledgerlite-finance"
SCENARIOS = FIXTURE_DIR / "docs" / "e2e-scenarios.md"
RELEASE_BRANCH = "release/1.x"
DEFAULT_BRANCH = "main"
_GIT_ENV = {
    "GIT_AUTHOR_NAME": "ORCA e2e", "GIT_AUTHOR_EMAIL": "e2e@orca.invalid",
    "GIT_COMMITTER_NAME": "ORCA e2e", "GIT_COMMITTER_EMAIL": "e2e@orca.invalid",
}
# The fixture is copied without anything a previous run generated.
_COPY_IGNORE = shutil.ignore_patterns(".git", "*.db", "*.db-shm", "*.db-wal", "tdd", "runs", "logs", "staging", "last-generate.json", ".mcp.json")


class FixtureError(RuntimeError):
    pass


def tdd_agent_available() -> bool:
    return shutil.which("tdd-agent") is not None


@dataclass
class LedgerliteRepo:
    path: str
    branches: list[str] = field(default_factory=list)
    knowledge: bool = False
    bug_commit: str = ""
    _owns_dir: bool = True

    @property
    def name(self) -> str:
        return os.path.basename(self.path)

    def git(self, *args: str, check: bool = True) -> str:
        return _run(["git", *args], cwd=self.path, check=check)

    def rev_parse(self, ref: str) -> str:
        return self.git("rev-parse", "--verify", "--quiet", ref + "^{commit}").strip()

    def current_branch(self) -> str:
        return self.git("rev-parse", "--abbrev-ref", "HEAD").strip()

    def go_test(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(["go", "test", "./...", "-count=1"], cwd=self.path, env={**os.environ, "GOWORK": "off"},
                              capture_output=True, text=True, timeout=300)

    def cleanup(self) -> None:
        if not self._owns_dir:
            return
        # Linked worktrees live next to the repo as <repo>-<branch>; remove those this run made, then the repo.
        parent = Path(self.path).parent
        for sibling in parent.glob(Path(self.path).name + "-*"):
            shutil.rmtree(sibling, ignore_errors=True)
        shutil.rmtree(self.path, ignore_errors=True)


def _run(cmd: list[str], cwd: str, check: bool = True, timeout: int = 180) -> str:
    proc = subprocess.run(cmd, cwd=cwd, env={**os.environ, **_GIT_ENV}, capture_output=True, text=True, timeout=timeout)
    if check and proc.returncode != 0:
        raise FixtureError(f"{' '.join(cmd)} failed ({proc.returncode}): {(proc.stderr or proc.stdout).strip()[:500]}")
    return proc.stdout


def bug_patch() -> str:
    """The scenario 1 patch, read from the scenarios document so the two cannot drift."""
    text = SCENARIOS.read_text(encoding="utf-8")
    m = re.search(r"git apply <<'PATCH'\n(.*?)\nPATCH\n", text, re.S)
    if not m:
        raise FixtureError(f"bug patch block not found in {SCENARIOS}")
    return m.group(1) + "\n"


def create(prefix: str, *, knowledge: bool = False, release_branch: bool = False, third_branch: str = "",
           inject_bug: bool = False, verify_baseline: bool = False) -> LedgerliteRepo:
    """Create the repo in a fresh temp directory and return it. Raises FixtureError, leaving no directory behind."""
    if not FIXTURE_DIR.is_dir():
        raise FixtureError(f"fixture not found: {FIXTURE_DIR}")
    if knowledge and not tdd_agent_available():
        raise FixtureError("tdd-agent is not on PATH; install it or create the repo with knowledge=False")
    base = tempfile.mkdtemp(prefix=f"{prefix}-ledgerlite-")
    path = os.path.realpath(os.path.join(base, "ledgerlite"))
    repo = LedgerliteRepo(path=path, _owns_dir=True)
    try:
        shutil.copytree(FIXTURE_DIR, path, ignore=_COPY_IGNORE)
        if knowledge:
            _run(["tdd-agent", "generate"], cwd=path, timeout=300)
            _run(["tdd-agent", "mcp", "enable", "--read-only"], cwd=path, timeout=120)
            repo.knowledge = True
        repo.git("init", "-q", "-b", DEFAULT_BRANCH)
        repo.git("add", "-A")
        repo.git("commit", "-q", "-m", "LedgerLite baseline")
        repo.branches.append(DEFAULT_BRANCH)
        if verify_baseline:
            res = repo.go_test()
            if res.returncode != 0:
                raise FixtureError(f"baseline go test failed:\n{(res.stdout + res.stderr)[-800:]}")
        if inject_bug:
            proc = subprocess.run(["git", "apply"], input=bug_patch(), cwd=path, env={**os.environ, **_GIT_ENV}, capture_output=True, text=True)
            if proc.returncode != 0:
                raise FixtureError(f"bug patch does not apply: {proc.stderr.strip()[:400]}")
            repo.git("commit", "-q", "-am", "Use a rolling 24h window for the daily limit")
            repo.bug_commit = repo.rev_parse("HEAD")
        if release_branch:
            _branch_with_commit(repo, RELEASE_BRANCH, "RELEASE-1.x.md", "Release line 1.x\n")
        if third_branch:
            _branch_with_commit(repo, third_branch, "OTHER.md", f"Work on {third_branch}\n")
            repo.git("checkout", "-q", third_branch)
        return repo
    except BaseException:
        shutil.rmtree(base, ignore_errors=True)
        raise


def _branch_with_commit(repo: LedgerliteRepo, branch: str, filename: str, body: str) -> None:
    """A branch whose tip differs from main, so a worktree's starting commit says which branch it came from."""
    repo.git("checkout", "-q", "-b", branch, DEFAULT_BRANCH)
    Path(repo.path, "docs", filename).write_text(body, encoding="utf-8")
    repo.git("add", "-A")
    repo.git("commit", "-q", "-m", f"Commit on {branch}")
    repo.branches.append(branch)
    repo.git("checkout", "-q", DEFAULT_BRANCH)
