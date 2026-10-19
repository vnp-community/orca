"""Finds the proposal worktrees of Requests that no longer need one (read-only, no Orca stack needed for the git part).

An OpenSpec plan lives on the local branch `request/<number>-proposal`, in its own worktree while the plan is written. A
cancelled Request's worktree must be gone at once; a completed one is kept for a retention period (the archive step); a
failed run's is kept for the same period. The branch itself always stays, so a branch without a worktree is never an orphan.
"""
from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

PROPOSAL_BRANCH = re.compile(r"^request/(\d+)-proposal$")

# What the janitor does with a Request in this status; anything else (an unknown number) is an orphan too.
OPEN = {
    "classifying", "awaiting_type_confirmation", "awaiting_information", "analyzing", "awaiting_analysis_approval",
    "planning", "awaiting_plan_approval", "executing", "request_backlog",
}


@dataclass(frozen=True)
class Worktree:
    path: str
    branch: str  # short name, "" for a detached head

    @property
    def request_number(self) -> int | None:
        m = PROPOSAL_BRANCH.match(self.branch)
        return int(m.group(1)) if m else None


@dataclass(frozen=True)
class Orphan:
    repo: str
    worktree: Worktree
    reason: str


def git(repo: Path, *args: str) -> str:
    out = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True, timeout=30)
    if out.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed in {repo}: {out.stderr.strip()}")
    return out.stdout


def list_worktrees(repo: Path) -> list[Worktree]:
    """Parses `git worktree list --porcelain` (stable since Git 2.7)."""
    found: list[Worktree] = []
    path, branch = None, ""
    for line in git(repo, "worktree", "list", "--porcelain").splitlines() + [""]:
        if line.startswith("worktree "):
            path, branch = line[len("worktree "):], ""
        elif line.startswith("branch "):
            branch = line[len("branch "):].removeprefix("refs/heads/")
        elif line == "" and path is not None:
            found.append(Worktree(path, branch))
            path = None
    return found


def list_proposal_branches(repo: Path) -> list[str]:
    out = git(repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/request/")
    return [b for b in out.splitlines() if PROPOSAL_BRANCH.match(b)]


def find_orphans(repo: Path, statuses: dict[int, str]) -> tuple[list[Orphan], list[Worktree]]:
    """statuses maps a Request number to its status. Returns (orphans, held): a held worktree belongs to a completed Request
    and waits for its retention; it is reported but is not an error."""
    orphans: list[Orphan] = []
    held: list[Worktree] = []
    for wt in list_worktrees(repo):
        number = wt.request_number
        if number is None:
            continue
        status = statuses.get(number)
        if status in OPEN:
            continue
        if status == "completed":
            held.append(wt)
        elif status is None:
            orphans.append(Orphan(str(repo), wt, "no such Request in Orca"))
        else:
            orphans.append(Orphan(str(repo), wt, f"the Request is {status}"))
    return orphans, held
