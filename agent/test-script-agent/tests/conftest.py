import json
import subprocess
from pathlib import Path

import pytest

from tsa import log, tcir

FIX = Path(__file__).parent / "fixtures"
log.QUIET = True


def git(cwd, *args):
    return subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True).stdout.strip()


@pytest.fixture
def pay_json():
    return json.loads((FIX / "pay-tcir.json").read_text(encoding="utf-8"))


@pytest.fixture
def pay_bundle(pay_json):
    return tcir.Bundle.from_json(pay_json)


@pytest.fixture
def test_repo(tmp_path):
    """A small pytest repository: the test repo TSA writes into (no application code)."""
    root = tmp_path / "qa-tests"
    (root / "tests").mkdir(parents=True)
    (root / "pyproject.toml").write_text("[tool.pytest.ini_options]\ntestpaths = ['tests']\n")
    (root / "tests" / "conftest.py").write_text(
        "import pytest\n\n\n@pytest.fixture\ndef api_client():\n    return object()\n\n\n@pytest.fixture\ndef tsa_driver():\n    return None\n")
    (root / "tests" / "test_health.py").write_text("def test_health(api_client):\n    assert api_client is not None\n")
    git(root, "init", "-q", "-b", "main")
    git(root, "config", "user.email", "qa@example.test")
    git(root, "config", "user.name", "QA")
    git(root, "add", ".")
    git(root, "commit", "-q", "-m", "init")
    return root
