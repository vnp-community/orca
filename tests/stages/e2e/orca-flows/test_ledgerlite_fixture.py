"""Tests of ledgerlite_fixture.py: they build real repos from the real fixture (no mocks) and run real git, go and tdd-agent."""
import os
import shutil
import subprocess
import tempfile
import unittest

import ledgerlite_fixture as lf


@unittest.skipUnless(shutil.which("go") and shutil.which("git"), "needs go and git")
class LedgerliteFixtureTest(unittest.TestCase):
    def test_baseline_is_green_and_branches_have_distinct_tips(self):
        repo = lf.create("t1", release_branch=True, third_branch="feature/other", verify_baseline=True)
        try:
            self.assertEqual(repo.current_branch(), "feature/other")
            shas = {b: repo.rev_parse(b) for b in repo.branches}
            self.assertEqual(len(set(shas.values())), 3, shas)
            self.assertEqual(set(repo.branches), {"main", lf.RELEASE_BRANCH, "feature/other"})
        finally:
            repo.cleanup()

    def test_bug_patch_applies_commits_and_baseline_stays_clean(self):
        repo = lf.create("t2", inject_bug=True)
        try:
            self.assertTrue(repo.bug_commit)
            self.assertEqual(repo.rev_parse("HEAD"), repo.bug_commit)
            with open(f"{repo.path}/internal/ledger/service.go", encoding="utf-8") as fh:
                self.assertIn("now.Add(-24 * time.Hour)", fh.read())
            self.assertEqual(repo.git("status", "--porcelain").strip(), "")
        finally:
            repo.cleanup()

    def test_bug_patch_text_comes_from_the_scenarios_document(self):
        self.assertIn("dayStart := now.Add(-24 * time.Hour)", lf.bug_patch())

    @unittest.skipUnless(lf.tdd_agent_available(), "tdd-agent not installed")
    def test_knowledge_and_mcp_are_generated_and_mcp_file_is_committed(self):
        repo = lf.create("t3", knowledge=True)
        try:
            tracked = repo.git("ls-files").split()
            self.assertIn(".mcp.json", tracked)
            self.assertNotIn(".tdd-agent/knowledge.db", tracked)
            out = subprocess.run(["tdd-agent", "kg", "stats"], cwd=repo.path, capture_output=True, text=True, timeout=120)
            self.assertEqual(out.returncode, 0, out.stderr)
            self.assertIn("67", out.stdout)
        finally:
            repo.cleanup()

    def test_failed_create_leaves_no_directory(self):
        original = lf.bug_patch
        lf.bug_patch = lambda: "not a patch\n"
        before = set(os.listdir(tempfile.gettempdir()))
        try:
            with self.assertRaises(lf.FixtureError):
                lf.create("t4", inject_bug=True)
        finally:
            lf.bug_patch = original
        left = [d for d in set(os.listdir(tempfile.gettempdir())) - before if d.startswith("t4-ledgerlite-")]
        self.assertEqual(left, [])


if __name__ == "__main__":
    unittest.main()
