"""pilot_diff_report on throwaway repos: branches found, protected paths flagged, the packages of changed Go files named.

  python3 -m unittest tests/stages/e2e/orca-flows/test_pilot_diff_report.py
"""
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import pilot_diff_report as pdr  # noqa: E402


def sh(cwd: Path, *args: str) -> str:
    done = subprocess.run(["git", "-c", "user.name=t", "-c", "user.email=t@example.com", *args], cwd=cwd, capture_output=True, text=True)
    assert done.returncode == 0, done.stderr
    return done.stdout


class DiffReportTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Path(self.tmp.name).resolve() / "repo"
        self.repo.mkdir()
        sh(self.repo, "init", "-q", "-b", "main")
        (self.repo / "go.mod").write_text("module example.com/m\n\ngo 1.21\n")
        (self.repo / "pkg").mkdir()
        (self.repo / "pkg" / "a.go").write_text("package pkg\n")
        sh(self.repo, "add", "-A")
        sh(self.repo, "commit", "-qm", "init")

    def tearDown(self) -> None:
        self.tmp.cleanup()

    def branch(self, name: str, files: dict[str, str]) -> Path:
        wt = self.repo.parent / ("wt-" + name.replace("/", "-"))
        sh(self.repo, "worktree", "add", "-q", "-b", name, str(wt))
        for rel, body in files.items():
            (wt / rel).parent.mkdir(parents=True, exist_ok=True)
            (wt / rel).write_text(body)
        sh(wt, "add", "-A")
        sh(wt, "commit", "-qm", "work")
        return wt

    def test_it_finds_the_task_branches_and_the_files_each_changed(self) -> None:
        self.branch("task/one", {"pkg/b.go": "package pkg\n"})
        self.branch("other/skip", {"x.txt": "x"})
        self.assertEqual(pdr.task_branches(self.repo, "task/"), ["task/one"])
        self.assertEqual(pdr.changed_files(self.repo, "main", "task/one"), ["pkg/b.go"])

    def test_a_protected_path_is_flagged_and_fails_the_report(self) -> None:
        self.branch("task/two", {"backend/services/infrastructure/integration-hub/x.go": "package x\n", "pkg/ok.go": "package pkg\n"})
        text, bad = pdr.report(str(self.repo), "task/", "main", pdr.DEFAULT_PROTECTED, run_tests=False)
        self.assertTrue(bad)
        self.assertIn("PROTECTED PATHS CHANGED", text)
        self.assertIn("integration-hub", text)

    def test_a_clean_branch_passes_and_its_go_package_is_tested_in_its_own_worktree(self) -> None:
        self.branch("task/three", {"pkg/c.go": "package pkg\n", "pkg/c_test.go": 'package pkg\n\nimport "testing"\n\nfunc TestC(t *testing.T) {}\n'})
        text, bad = pdr.report(str(self.repo), "task/", "main", pdr.DEFAULT_PROTECTED, run_tests=True)
        self.assertFalse(bad, text)
        self.assertIn("PASS `.:./pkg`", text)

    def test_a_red_test_is_reported_not_repaired(self) -> None:
        self.branch("task/four", {"pkg/d_test.go": 'package pkg\n\nimport "testing"\n\nfunc TestD(t *testing.T) { t.Fatal("red") }\n'})
        text, bad = pdr.report(str(self.repo), "task/", "main", pdr.DEFAULT_PROTECTED, run_tests=True)
        self.assertTrue(bad)
        self.assertIn("FAIL `.:./pkg`", text)

    def test_a_deleted_go_file_has_no_package_to_test(self) -> None:
        self.assertEqual(pdr.go_packages(["pkg/gone.go", "README.md"], self.repo), [])

    def test_no_branch_is_said_plainly(self) -> None:
        text, bad = pdr.report(str(self.repo), "task/", "main", pdr.DEFAULT_PROTECTED, run_tests=False)
        self.assertFalse(bad)
        self.assertIn("No branch under that prefix", text)


if __name__ == "__main__":
    unittest.main()
