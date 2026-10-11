"""Offline tests for report_duplicate_repos.py: python3 -m unittest deploy/dev/scripts/test_report_duplicate_repos.py"""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from report_duplicate_repos import group_duplicates, normalize_repo_location  # noqa: E402

# Keep these cases identical to TestNormalizeRepoLocation in project-service's domain/repo_location_test.go.
CASES = [
    ("/Users/me/code/app", "/Users/me/code/app"),
    ("/Users/me/code/app/", "/Users/me/code/app"),
    ("/Users/me/code/app///", "/Users/me/code/app"),
    ("/Users//me/code///app", "/Users/me/code/app"),
    ("/", "/"),
    ("  /srv/repo/ \n", "/srv/repo"),
    ("/Users/Me/Code", "/Users/Me/Code"),
    ("/a/b/../c", "/a/b/../c"),
    ("/srv/we\\ird", "/srv/we\\ird"),
    ("C:\\Users\\me\\app", "C:\\Users\\me\\app"),
    ("C:\\Users\\me\\app\\", "C:\\Users\\me\\app"),
    ("C:/Users/me/app/", "C:\\Users\\me\\app"),
    ("C:\\Users//me\\\\app", "C:\\Users\\me\\app"),
    ("C:\\", "C:\\"),
    ("C:/", "C:\\"),
    ("c:\\Users\\ME", "c:\\Users\\ME"),
    ("\\\\srv\\share\\x\\", "\\\\srv\\share\\x"),
    ("https://github.com/org/repo/", "https://github.com/org/repo"),
    ("ssh://git@host//org/repo//", "ssh://git@host//org/repo"),
    ("   ", ""),
]


class NormalizeTest(unittest.TestCase):
    def test_cases_match_the_go_implementation(self):
        for raw, want in CASES:
            with self.subTest(raw=raw):
                self.assertEqual(normalize_repo_location(raw), want)


def row(repo_id, project, dev, url):
    return {"repoId": repo_id, "projectId": project, "projectName": project.upper(), "devServerId": dev, "url": url}


class GroupTest(unittest.TestCase):
    def test_same_host_and_spelling_variants_are_grouped(self):
        groups = group_duplicates([row("r1", "p1", "D1", "/x/codex"), row("r2", "p2", "d1", "/x/codex/"), row("r3", "p3", "d1", "/x/other")])
        self.assertEqual(len(groups), 1)
        self.assertEqual(sorted(r["repoId"] for r in groups[0]["repos"]), ["r1", "r2"])

    def test_other_host_or_case_is_not_a_duplicate(self):
        self.assertEqual(group_duplicates([row("r1", "p1", "d1", "/x/a"), row("r2", "p2", "d2", "/x/a"), row("r3", "p3", "d1", "/x/A")]), [])

    def test_blank_paths_and_singletons_are_ignored(self):
        self.assertEqual(group_duplicates([row("r1", "p1", "d1", " "), row("r2", "p2", "d1", "")]), [])


if __name__ == "__main__":
    unittest.main()
