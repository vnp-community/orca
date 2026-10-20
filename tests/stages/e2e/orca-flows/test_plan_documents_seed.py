"""Self-test of the Plan documents seed: python3 -m unittest tests/stages/e2e/orca-flows/test_plan_documents_seed.py
Covers what needs no docker: the planted files, their index and the final JSON line contract the UI spec reads."""
import hashlib
import re
import unittest

import plan_documents_seed as seed

SECRET_SHAPE = re.compile(r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b")  # common/secretscan KindAWSAccessKey
OMITTED = re.compile(r"^<omitted size=(\d+) sha256=([0-9a-f]{64})>$")  # domain.omittedMarkerPattern


class PlanDocumentsSeed(unittest.TestCase):
    def test_the_change_has_the_four_groups_of_documents_and_the_machine_region(self):
        files = seed.bundle(1, "REQ-9")
        self.assertTrue({"proposal.md", "design.md", "tasks.md", "specs/ledger/spec.md"} <= set(files))
        self.assertIn("<!-- orca:begin plan request=REQ-9 schema=1 -->", files["tasks.md"])
        self.assertIn("#### Scenario: AC-1", files["specs/ledger/spec.md"])

    def test_one_file_is_only_an_omitted_marker_the_service_parses(self):
        marker = seed.bundle(1, "REQ-9")[seed.OMITTED_PATH]
        m = OMITTED.match(marker)
        self.assertIsNotNone(m, marker)
        self.assertEqual(int(m.group(1)), seed.OMITTED_SIZE)

    def test_the_planted_secret_is_a_shape_the_service_masks_and_sits_unmasked_in_the_design(self):
        self.assertRegex(seed.FAKE_SECRET, SECRET_SHAPE)
        self.assertIn(seed.FAKE_SECRET, seed.bundle(1, "REQ-9")["design.md"])
        self.assertIn(seed.FAKE_SECRET, seed.bundle(2, "REQ-9")["design.md"])

    def test_versions_differ_in_tasks_and_design_only(self):
        a, b = seed.bundle(1, "REQ-9"), seed.bundle(2, "REQ-9")
        self.assertEqual({p for p in a if a[p] != b[p]}, {"proposal.md", "design.md", "tasks.md"})
        self.assertEqual(a["specs/ledger/spec.md"], b["specs/ledger/spec.md"])

    def test_the_index_is_sorted_and_marks_the_omitted_file_with_the_marker_size(self):
        index = seed.file_index(seed.bundle(2, "REQ-9"))
        self.assertEqual([f["path"] for f in index], sorted(f["path"] for f in index))
        omitted = [f for f in index if f["omitted"]]
        self.assertEqual([(f["path"], f["size"], f["sha256"]) for f in omitted], [(seed.OMITTED_PATH, seed.OMITTED_SIZE, seed.OMITTED_SHA)])
        design = next(f for f in index if f["path"] == "design.md")
        self.assertEqual(design["sha256"], hashlib.sha256(seed.bundle(2, "REQ-9")["design.md"].encode()).hexdigest())

    def test_the_bundle_digest_changes_with_a_version(self):
        self.assertNotEqual(seed.bundle_digest(seed.bundle(1, "REQ-9")), seed.bundle_digest(seed.bundle(2, "REQ-9")))


if __name__ == "__main__":
    unittest.main()
