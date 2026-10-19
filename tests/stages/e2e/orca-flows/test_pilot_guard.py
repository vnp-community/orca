"""pilot_guard on throwaway repos: the growth Orca is allowed and every change that must stop a run.

  python3 -m unittest tests/stages/e2e/orca-flows/test_pilot_guard.py
"""
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import pilot_guard as pg  # noqa: E402


def sh(cwd: Path, *args: str) -> str:
    done = subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@example.com", *args], cwd=cwd, capture_output=True, text=True)
    assert done.returncode == 0, done.stderr
    return done.stdout


class PilotGuardTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        base = Path(self.tmp.name).resolve()
        self.main = base / "user-repo"
        self.main.mkdir()
        sh(self.main, "init", "-q", "-b", "main")
        (self.main / "a.txt").write_text("a\n")
        sh(self.main, "add", "-A")
        sh(self.main, "commit", "-qm", "init")
        # The pilot is itself a worktree of the user's repo, like .claude/worktrees/tdd-agent-bl-h01-h02.
        self.pilot = base / "user-repo" / ".claude" / "worktrees" / "pilot"
        sh(self.main, "branch", "keep")
        sh(self.main, "worktree", "add", "-q", "-b", "pilot-branch", str(self.pilot))
        (self.main / ".git" / "info" / "exclude").write_text(".claude/\n")
        self.before = pg.snapshot(self.pilot)

    def tearDown(self) -> None:
        self.tmp.cleanup()

    def now(self) -> dict:
        return pg.snapshot(self.pilot)

    def test_an_untouched_repo_has_no_violation_and_no_growth(self) -> None:
        self.assertEqual(pg.compare(self.before, self.now()), ([], []))

    def test_a_task_branch_and_a_neighbour_worktree_are_allowed_growth_but_not_after_cleanup(self) -> None:
        wt = self.pilot.parent / "pilot-task-1"
        sh(self.main, "worktree", "add", "-q", "-b", "task/1", str(wt))
        problems, growth = pg.compare(self.before, self.now())
        self.assertEqual(problems, [])
        self.assertTrue(any("new branch refs/heads/task/1" in g for g in growth), growth)
        self.assertTrue(any("new worktree" in g for g in growth), growth)
        strict, _ = pg.compare(self.before, self.now(), strict=True)
        self.assertTrue(any("left behind" in p for p in strict), strict)
        sh(self.main, "worktree", "remove", "--force", str(wt))
        sh(self.main, "branch", "-D", "task/1")
        self.assertEqual(pg.compare(self.before, self.now(), strict=True), ([], []))

    def test_every_change_to_the_users_work_is_a_violation(self) -> None:
        cases = {
            "an edited file in the pilot worktree": lambda: (self.pilot / "a.txt").write_text("changed\n"),
            "a new untracked file in the main checkout": lambda: (self.main / "new.txt").write_text("x\n"),
            "a commit on the pilot branch": lambda: sh(self.pilot, "commit", "-q", "--allow-empty", "-m", "x"),
            "a tag": lambda: sh(self.main, "tag", "v1"),
            "a stash ref": lambda: (self.pilot / "a.txt").write_text("s\n") or sh(self.pilot, "stash"),
            "a deleted branch": lambda: sh(self.main, "branch", "-D", "keep"),
            "a changed local config": lambda: sh(self.main, "config", "--local", "user.name", "other"),
        }
        for name, change in cases.items():
            with self.subTest(name):
                self.tearDown(), self.setUp()
                change()
                problems, _ = pg.compare(self.before, self.now())
                self.assertTrue(problems, f"{name} must be a violation")

    def test_a_finder_file_is_not_a_change(self) -> None:
        (self.main / ".DS_Store").write_text("noise")
        self.assertEqual(pg.compare(self.before, self.now()), ([], []))

    def test_a_directory_the_agent_may_create_beside_the_repo_can_be_allowed_by_name(self) -> None:
        (self.main.parent / "tmp").mkdir()
        problems, _ = pg.compare(self.before, self.now())
        self.assertTrue(any("new entries next to the repo" in p for p in problems), problems)
        self.assertEqual(pg.compare(self.before, self.now(), allow_neighbours=frozenset({"tmp"}))[0], [])

    def test_a_worktree_outside_the_allowed_place_is_a_violation(self) -> None:
        elsewhere = Path(self.tmp.name) / "somewhere-else"
        sh(self.main, "worktree", "add", "-q", "-b", "stray", str(elsewhere))
        problems, _ = pg.compare(self.before, self.now())
        self.assertTrue(any("outside the allowed place" in p for p in problems), problems)

    def test_a_fetch_moves_remote_tracking_refs_without_being_a_violation(self) -> None:
        remote = Path(self.tmp.name) / "remote.git"
        sh(Path(self.tmp.name), "init", "-q", "--bare", str(remote))
        sh(self.main, "remote", "add", "origin", str(remote))
        sh(self.main, "push", "-q", "origin", "main")
        sh(self.main, "fetch", "-q", "origin")
        before = pg.snapshot(self.pilot)
        sh(self.main, "commit", "-q", "--allow-empty", "-m", "ahead")
        sh(self.main, "push", "-q", "origin", "main")
        sh(self.main, "reset", "-q", "--hard", "HEAD~1")
        sh(self.main, "fetch", "-q", "origin")
        problems, growth = pg.compare(before, pg.snapshot(self.pilot))
        self.assertEqual(problems, [])
        self.assertTrue(any("remote-tracking" in g for g in growth), growth)

    def test_the_snapshot_survives_a_json_round_trip_and_hashes_content_not_only_names(self) -> None:
        (self.pilot / "a.txt").write_text("one\n")
        first = pg.snapshot(self.pilot)
        (self.pilot / "a.txt").write_text("two\n")
        second = pg.snapshot(self.pilot)
        self.assertNotEqual(first["pilot"]["files"], second["pilot"]["files"])
        import json
        self.assertEqual(json.loads(json.dumps(first)), first)

    def test_the_orca_data_of_the_user_may_only_grow_by_the_pilots_own_project(self) -> None:
        before = {"projects": {"p1": {"name": "mine", "repos": []}}, "requests": {"r1": {"number": 1, "status": "completed", "project": "p1", "title": "t"}}}
        now = {"projects": {**before["projects"], "pp": {"name": "pilot", "repos": []}},
               "requests": {**before["requests"], "r2": {"number": 2, "status": "analyzing", "project": "pp", "title": "x"}}}
        self.assertEqual(pg.orca_compare(before, now, {"pp"}, strict=False), [])
        self.assertEqual(len(pg.orca_compare(before, now, set(), strict=False)), 2)
        self.assertEqual(len(pg.orca_compare(before, now, {"pp"}, strict=True)), 2)
        changed = {"projects": before["projects"], "requests": {"r1": {**before["requests"]["r1"], "status": "cancelled"}}}
        self.assertEqual(len(pg.orca_compare(before, changed, set(), strict=False)), 1)
        gone = {"projects": {}, "requests": {}}
        self.assertEqual(len(pg.orca_compare(before, gone, set(), strict=False)), 2)


if __name__ == "__main__":
    unittest.main()
