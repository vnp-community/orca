"""The parts of run_cli_flow.py that need no stack: reading the shape of tasks.md and the repo the flow builds its fixture from.

  python3 -m unittest tests/stages/e2e/orca-flows/test_run_cli_flow.py
"""
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import run_cli_flow as flow  # noqa: E402

TASKS = """prose
<!-- orca:begin plan request=REQ-1 schema=1 -->
## PH-1 x
- [ ] T1.1 one [type=feature]
  Serves: FR-1
  Code: create `a.go`
  Verify: it works via `go test`
  Done when: y
- [x] T1.2 two [type=feature]
  Serves: FR-1
  Code: modify `b.go`
<!-- orca:end plan -->
"""


class FlowHelpersTest(unittest.TestCase):
    def test_tasks_md_shape_counts_tasks_and_the_ones_with_all_four_lines(self) -> None:
        self.assertEqual(flow.tasks_md_shape(TASKS), (2, 1))
        self.assertEqual(flow.tasks_md_shape("no region at all"), (0, 0))

    def test_the_fixture_is_a_small_go_service_that_tests_green_on_its_own(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            for name in ("go.mod", "main.go", "main_test.go"):
                shutil.copy(flow.FIXTURE / "src" / name, Path(tmp) / name)
            done = subprocess.run(["go", "test", "./..."], cwd=tmp, capture_output=True, text=True, env={**__import__("os").environ, "GOWORK": "off"})
            self.assertEqual(done.returncode, 0, done.stdout + done.stderr)

    def test_the_criteria_the_flow_answers_with_are_the_ones_it_later_looks_for(self) -> None:
        self.assertEqual(len(flow.CRITERIA), 3)
        self.assertIn("/healthz", flow.REQUEST_BODY)

    def test_the_report_remembers_what_failed(self) -> None:
        rep = flow.Report()
        self.assertTrue(rep.check("fine", True))
        self.assertFalse(rep.check("broken", False, "why"))
        self.assertEqual(rep.failed, ["broken"])


if __name__ == "__main__":
    unittest.main()
