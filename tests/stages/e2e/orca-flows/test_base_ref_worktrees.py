"""Tests of base_ref_worktrees.py and the error parsing of base_ref_stack.py against real git repos built from the LedgerLite fixture.

The suite's proof (BRF-06/07) is only as good as these checks: each negative case here is the bug the suite must catch."""
import os
import shutil
import tempfile
import unittest

import base_ref_stack as stack
import base_ref_worktrees as gitwt
import ledgerlite_fixture as lf
from ws_rpc_client import RpcError

THIRD = "feature/other"


@unittest.skipUnless(shutil.which("git"), "needs git")
class StartingCommitTest(unittest.TestCase):
    def setUp(self):
        self.repo = lf.create("t-brf", release_branch=True, third_branch=THIRD)
        self.addCleanup(gitwt.remove_worktrees_and_repo, self.repo)
        self.tips = {b: self.repo.rev_parse(b) for b in (lf.DEFAULT_BRANCH, lf.RELEASE_BRANCH, THIRD)}
        self.outside = tempfile.mkdtemp(prefix="t-brf-wt-")  # worktrees live outside the repo, like the agent's
        self.addCleanup(shutil.rmtree, self.outside, True)

    def add(self, branch: str, start: str) -> gitwt.Worktree:
        self.repo.git("worktree", "add", "-q", "-b", branch, os.path.join(self.outside, branch.replace("/", "-")), start)
        return gitwt.wait_for_worktree(self.repo, {branch}, 5, "worktree")

    def not_from(self, include_release: bool) -> dict:
        out = {"third": self.tips[THIRD]}
        if include_release:
            out["release"] = self.tips[lf.RELEASE_BRANCH]
        return out

    def test_primary_checkout_is_not_a_linked_worktree_and_is_on_the_third_branch(self):
        self.assertEqual(self.repo.current_branch(), THIRD)
        self.assertEqual(gitwt.linked_worktrees(self.repo), [])

    def test_release_worktree_passes_and_main_is_not_counted_against_it(self):
        wt = self.add("task/r1", lf.RELEASE_BRANCH)
        self.assertEqual(gitwt.head_sha(wt.path), self.tips[lf.RELEASE_BRANCH])
        self.assertEqual(gitwt.branch_of(wt.path), "task/r1")
        gitwt.assert_started_from(wt, "release", self.tips[lf.RELEASE_BRANCH], {"third": self.tips[THIRD], "main": self.tips[lf.DEFAULT_BRANCH]})

    def test_default_worktree_passes_and_rejects_a_release_or_third_start(self):
        wt = self.add("task/d1", lf.DEFAULT_BRANCH)
        gitwt.assert_started_from(wt, "main", self.tips[lf.DEFAULT_BRANCH], self.not_from(True))
        for wrong, branch in ((lf.RELEASE_BRANCH, "task/d2"), (THIRD, "task/d3")):
            bad = self.add(branch, wrong)
            with self.assertRaises(AssertionError, msg=f"a worktree started from {wrong} passed as a main start"):
                gitwt.assert_started_from(bad, "main", self.tips[lf.DEFAULT_BRANCH], self.not_from(True))

    def test_release_expectation_rejects_main_and_third_starts(self):
        for wrong, branch in ((lf.DEFAULT_BRANCH, "task/x1"), (THIRD, "task/x2")):
            bad = self.add(branch, wrong)
            with self.assertRaises(AssertionError, msg=f"a worktree started from {wrong} passed as a release start"):
                gitwt.assert_started_from(bad, "release", self.tips[lf.RELEASE_BRANCH], self.not_from(False))

    def test_a_commit_made_by_the_agent_on_top_of_the_right_start_still_passes(self):
        wt = self.add("task/c1", lf.RELEASE_BRANCH)
        with open(os.path.join(wt.path, "NEW.md"), "w", encoding="utf-8") as fh:
            fh.write("agent work\n")
        for args in (("add", "-A"), ("-c", "user.name=a", "-c", "user.email=a@b.c", "commit", "-q", "-m", "work")):
            gitwt._git(wt.path, *args)
        self.assertNotEqual(gitwt.head_sha(wt.path), self.tips[lf.RELEASE_BRANCH])
        gitwt.assert_started_from(wt, "release", self.tips[lf.RELEASE_BRANCH], self.not_from(False))

    def test_cleanup_removes_worktrees_outside_the_repo_and_the_repo(self):
        wt = self.add("task/z1", lf.DEFAULT_BRANCH)
        gitwt.remove_worktrees_and_repo(self.repo)
        self.assertFalse(os.path.exists(wt.path))
        self.assertFalse(os.path.exists(self.repo.path))
        gitwt.remove_worktrees_and_repo(self.repo)  # idempotent: the suite's cleanup may run after a failed setup


class ErrorParsingTest(unittest.TestCase):
    def test_trailer_is_read_from_the_end_of_the_message(self):
        exc = RpcError('REQUEST_BASE_REF_NOT_FOUND: base ref "x" does not exist in the repository {"suggestions":["main","release/1.x"]}')
        self.assertEqual(stack.error_trailer(exc), {"suggestions": ["main", "release/1.x"]})
        self.assertEqual(stack.error_trailer(RpcError("REQUEST_BASE_REF_LOCKED: base ref can no longer change (reason: task) {\"reason\":\"task\"}")), {"reason": "task"})

    def test_message_without_a_trailer_has_no_details(self):
        self.assertEqual(stack.error_trailer(RpcError("REQUEST_BASE_REF_INVALID: base ref must not start with '-'")), {})

    def test_expect_error_checks_the_code_and_a_silent_success(self):
        def refuse():
            raise RpcError("REQUEST_BASE_REF_INVALID: base ref must not contain '..'")
        self.assertIn("INVALID", stack.expect_error("REQUEST_BASE_REF_INVALID", refuse).message)
        with self.assertRaises(AssertionError):
            stack.expect_error("REQUEST_BASE_REF_LOCKED", refuse)
        with self.assertRaises(AssertionError):
            stack.expect_error("REQUEST_BASE_REF_LOCKED", lambda: None)


if __name__ == "__main__":
    unittest.main()
