"""The orphan finder reads real git worktrees; these tests build a throwaway repo with one of each kind.

  python3 -m unittest tests/stages/e2e/orca-flows/test_report_orphan_worktrees.py
"""
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from proposal_worktrees import find_orphans, list_proposal_branches, list_worktrees  # noqa: E402


def run(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True, text=True, timeout=30)


class OrphanWorktreesTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name) / "repo"
        self.repo.mkdir()
        run(self.repo, "init", "-q", "-b", "main")
        run(self.repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
        for n in (1, 2, 3, 4):
            run(self.repo, "worktree", "add", "-q", "-b", f"request/{n}-proposal", str(Path(self.tmp.name) / f"wt-{n}"))
        run(self.repo, "worktree", "add", "-q", "-b", "feature/x", str(Path(self.tmp.name) / "wt-feature"))
        run(self.repo, "branch", "request/9-proposal")  # a kept branch with no worktree

    def test_lists_only_proposal_worktrees(self) -> None:
        numbers = sorted(w.request_number for w in list_worktrees(self.repo) if w.request_number)
        self.assertEqual(numbers, [1, 2, 3, 4])

    def test_cancelled_and_unknown_are_orphans_open_and_completed_are_not(self) -> None:
        orphans, held = find_orphans(self.repo, {1: "executing", 2: "cancelled", 3: "completed"})  # 4 is unknown to Orca
        self.assertEqual(sorted(o.worktree.request_number for o in orphans), [2, 4])
        reasons = {o.worktree.request_number: o.reason for o in orphans}
        self.assertIn("cancelled", reasons[2])
        self.assertIn("no such Request", reasons[4])
        self.assertEqual([w.request_number for w in held], [3])

    def test_a_kept_branch_without_a_worktree_is_never_an_orphan(self) -> None:
        orphans, _ = find_orphans(self.repo, {})
        self.assertNotIn(9, [o.worktree.request_number for o in orphans])
        self.assertIn("request/9-proposal", list_proposal_branches(self.repo))

    def test_clean_after_the_orphans_are_removed(self) -> None:
        run(self.repo, "worktree", "remove", "--force", str(Path(self.tmp.name) / "wt-2"))
        run(self.repo, "worktree", "remove", "--force", str(Path(self.tmp.name) / "wt-4"))
        orphans, _ = find_orphans(self.repo, {1: "planning", 2: "cancelled", 3: "awaiting_plan_approval"})
        self.assertEqual(orphans, [])
        self.assertIn("request/2-proposal", list_proposal_branches(self.repo))  # its branch stays


if __name__ == "__main__":
    unittest.main()
