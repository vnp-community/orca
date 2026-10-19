"""review_debt.py against a fake tdd-agent, so no knowledge graph is needed.

  python3 -m unittest specs/backend-go/crs/v8/tdd-agent-integration/evidence/test_review_debt.py
"""
import json
import os
import stat
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import review_debt  # noqa: E402

STATUS = {"knowledge": {"snapshot": "snap-000001", "elements": 10, "edges": 3, "gaps": 1, "conflicts": 0}}


def fake_agent(directory: Path, status: dict, pending: list, fail: bool = False) -> str:
    path = directory / "tdd-agent"
    path.write_text(
        "#!/usr/bin/env python3\nimport json, sys\n"
        f"if {fail!r}:\n    sys.stderr.write('boom'); sys.exit(2)\n"
        f"status = {json.dumps(status)}\npending = {json.dumps(pending)}\n"
        "print(json.dumps(pending if 'review' in sys.argv else status))\n"
    )
    path.chmod(path.stat().st_mode | stat.S_IEXEC)
    return str(path)


def pending(*pairs):
    return [{"id": f"E{i}", "class": c, "owner_doc": d, "review_status": "needs_review"} for i, (c, d) in enumerate(pairs)]


class ReviewDebtTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)

    def test_counts_elements_waiting_by_class_and_document(self) -> None:
        agent = fake_agent(self.dir, STATUS, pending(("Requirement", "URD"), ("Requirement", "SRS"), ("BusinessRule", "SRS")))
        snap = review_debt.take_snapshot("/repo", agent)
        self.assertEqual((snap["elements"], snap["needs_review"], snap["reviewed"]), (10, 3, 7))
        self.assertEqual(snap["by_class"], {"BusinessRule": 1, "Requirement": 2})
        self.assertEqual(snap["by_doc"], {"SRS": 2, "URD": 1})
        self.assertIn("3 of 10 element(s) wait for review", review_debt.render(snap))

    def test_compare_says_how_many_went_back_to_review_and_where(self) -> None:
        before = review_debt.take_snapshot("/repo", fake_agent(self.dir, STATUS, pending(("Requirement", "URD"))))
        after = review_debt.take_snapshot("/repo", fake_agent(self.dir, STATUS, pending(("Requirement", "URD"), ("BusinessRule", "SRS"), ("BusinessRule", "SRS"))))
        c = review_debt.compare(before, after)
        self.assertEqual(c["needs_review"], [1, 3])
        self.assertEqual(c["added_to_review"], 2)
        self.assertEqual(c["by_class"], {"BusinessRule": 2})
        text = review_debt.render_compare(c)
        self.assertIn("needs_review 1 -> 3  (+2)", text)
        self.assertIn("2 element(s) went back to review", text)

    def test_a_reviewed_graph_has_no_debt(self) -> None:
        snap = review_debt.take_snapshot("/repo", fake_agent(self.dir, STATUS, []))
        self.assertEqual(snap["needs_review"], 0)
        self.assertEqual(review_debt.compare(snap, snap)["added_to_review"], 0)

    def test_a_failing_tdd_agent_is_an_error_not_a_zero(self) -> None:
        agent = fake_agent(self.dir, STATUS, [], fail=True)
        with self.assertRaises(RuntimeError):
            review_debt.take_snapshot("/repo", agent)
        out = self.dir / "s.json"
        self.assertEqual(review_debt.main(["--tdd-agent", agent, "snapshot", "--repo", "/repo", "--out", str(out)]), 1)
        self.assertFalse(out.exists())

    def test_snapshot_then_compare_through_the_command_line(self) -> None:
        a = fake_agent(self.dir, STATUS, pending(("Requirement", "URD")))
        before, after = self.dir / "before.json", self.dir / "after.json"
        self.assertEqual(review_debt.main(["--tdd-agent", a, "snapshot", "--repo", "/repo", "--out", str(before)]), 0)
        a = fake_agent(self.dir, STATUS, pending(("Requirement", "URD"), ("Requirement", "URD")))
        self.assertEqual(review_debt.main(["--tdd-agent", a, "snapshot", "--repo", "/repo", "--out", str(after)]), 0)
        self.assertEqual(review_debt.main(["compare", str(before), str(after)]), 0)
        self.assertEqual(json.loads(after.read_text())["needs_review"], 2)


if __name__ == "__main__":
    unittest.main()
