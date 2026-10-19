"""Real-git side of the base-branch checks (CR-REQ-039): finds the worktrees an Orca run created in the fixture repo and reads
where they started. Nothing here asks Orca: git is the oracle, so a wrong starting commit cannot be hidden by a wrong record.
"""
from __future__ import annotations

import os
import subprocess
from pathlib import Path
from typing import Iterable

from acceptance_context import wait_until
from ledgerlite_fixture import LedgerliteRepo
from proposal_worktrees import Worktree, list_worktrees


def _git(path: str, *args: str) -> str:
    proc = subprocess.run(["git", "-C", path, *args], capture_output=True, text=True, timeout=30)
    if proc.returncode != 0:
        raise AssertionError(f"git {' '.join(args)} failed in {path}: {(proc.stderr or proc.stdout).strip()[:300]}")
    return proc.stdout.strip()


def linked_worktrees(repo: LedgerliteRepo) -> list[Worktree]:
    """Every worktree of the repo except its primary checkout."""
    primary = os.path.realpath(repo.path)
    return [w for w in list_worktrees(Path(repo.path)) if os.path.realpath(w.path) != primary]


def wait_for_worktree(repo: LedgerliteRepo, branches: Iterable[str], timeout: float, what: str) -> Worktree:
    wanted = set(branches)
    return wait_until(lambda: next((w for w in linked_worktrees(repo) if w.branch in wanted), None), timeout=timeout, interval=1.0, what=what)


def head_sha(path: str) -> str:
    return _git(path, "rev-parse", "HEAD")


def branch_of(path: str) -> str:
    return _git(path, "branch", "--show-current")


def is_ancestor(path: str, ancestor: str, descendant: str) -> bool:
    proc = subprocess.run(["git", "-C", path, "merge-base", "--is-ancestor", ancestor, descendant], capture_output=True, text=True, timeout=30)
    if proc.returncode not in (0, 1):
        raise AssertionError(f"git merge-base --is-ancestor failed in {path}: {proc.stderr.strip()[:300]}")
    return proc.returncode == 0


def assert_started_from(wt: Worktree, want_label: str, want_sha: str, not_from: dict[str, str]) -> None:
    """The worktree's HEAD is the chosen branch's tip, or (a real agent may have committed) a descendant of it, and never a
    descendant of a branch it should NOT have started from. `not_from` maps a label to such a branch tip."""
    head = head_sha(wt.path)
    if head != want_sha and not is_ancestor(wt.path, want_sha, head):
        raise AssertionError(f"the worktree {wt.path} (branch {branch_of(wt.path)!r}) is at {head[:10]}, which does not descend from {want_label} "
                             f"{want_sha[:10]}; it started from the wrong commit")
    for label, sha in not_from.items():
        # A commit the chosen branch itself contains (main under release/1.x) proves nothing about where the worktree started.
        if not is_ancestor(wt.path, sha, want_sha) and is_ancestor(wt.path, sha, head):
            raise AssertionError(f"the worktree {wt.path} contains {label} {sha[:10]}: it started from {label}, not from {want_label} {want_sha[:10]}")


def remove_worktrees_and_repo(repo: LedgerliteRepo) -> None:
    """Linked worktrees may live outside the repo directory, so git removes them before the temp repo is deleted."""
    if os.path.isdir(repo.path):
        for wt in linked_worktrees(repo):
            repo.git("worktree", "remove", "--force", wt.path, check=False)
        repo.git("worktree", "prune", check=False)
    repo.cleanup()
    try:
        os.rmdir(os.path.dirname(repo.path))  # the fixture's temp parent; rmdir only removes it when empty
    except OSError:
        pass
