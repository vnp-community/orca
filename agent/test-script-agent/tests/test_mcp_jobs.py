import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

import anyio
import pytest

from tsa import jobs, worktree
from tsa.mcp_server import Tools
from tsa.runner import run_job

from .conftest import FIX, git

EXT = {"system": "orca", "executionId": "req-1", "stepId": "testscript"}


@pytest.fixture
def project(test_repo):
    """The application repo is also the test repo: an app file next to tests/, TCIR from a file, template generator."""
    (test_repo / "app").mkdir()
    (test_repo / "app" / "service.py").write_text("SECRET_LOGIC = 1\n")
    (test_repo / ".tsa").mkdir()
    shutil.copy(FIX / "pay-tcir.json", test_repo / ".tsa" / "tcir.json")
    (test_repo / ".tsa" / "config.toml").write_text('[tsa]\ngenerator = "template"\ntcir_file = ".tsa/tcir.json"\n')
    git(test_repo, "add", "app")
    git(test_repo, "commit", "-q", "-m", "app")
    return test_repo


def test_start_is_idempotent_and_restarts_after_an_error(project):
    t = Tools(project, spawn=False)
    a = t.tsa_start("PAY", EXT, spec_version=1)
    b = t.tsa_start("PAY", EXT, spec_version=1)
    assert a["job_id"] == b["job_id"] and not a["reused"] and b["reused"] and a["state"] == "queued"
    assert jobs.jobs_dir(project) == project / ".git" / "tsa" / "jobs"
    assert git(project, "status", "--porcelain", "--untracked-files=all").splitlines() == ["?? .tsa/config.toml", "?? .tsa/tcir.json"]
    rerun = t.tsa_start("PAY", {**EXT, "attempt": 2}, spec_version=1)
    assert rerun["job_id"] != a["job_id"] and rerun["reused"] is False
    other = t.tsa_start("PAY", {**EXT, "executionId": "req-2"}, spec_version=1)
    assert other["job_id"] != a["job_id"]
    job = jobs.read(project, a["job_id"])
    job["state"] = "error"
    jobs.write(project, job)
    again = t.tsa_start("PAY", EXT, spec_version=1)
    assert again["job_id"] == a["job_id"] and not again["reused"] and again["attempt"] == 2


def test_start_validates_arguments(project):
    out = Tools(project, spawn=False)
    from tsa.errors import TsaError
    with pytest.raises(TsaError, match="external"):
        out.tsa_start("PAY", {"system": "orca"})
    with pytest.raises(TsaError, match="generator"):
        out.tsa_start("PAY", EXT, generator="gpt")


def test_run_job_uses_a_sparse_worktree_without_the_app_and_commits(project):
    t = Tools(project, spawn=False)
    jid = t.tsa_start("PAY", EXT, spec_version=1, branch="tsa/REQ-1-PAY-v1", base="main")["job_id"]
    assert run_job(project, jid) == 0
    got = t.tsa_get(jid)
    assert got["state"] == "done" and got["branch"] == "tsa/REQ-1-PAY-v1" and got["counts"]["generated"] == 35 and got["commit"]
    wt = got["worktree"]
    assert not os.path.exists(os.path.join(wt, "app", "service.py"))           # the application is not checked out
    assert os.path.exists(os.path.join(wt, "tests", "conftest.py")) and os.path.exists(os.path.join(wt, "pyproject.toml"))
    assert worktree.outside_paths(Path(wt), ["tests"]) == []
    assert (project / "app" / "service.py").exists()                            # the main checkout keeps everything
    changed = git(project, "show", "--name-only", "--format=", "tsa/REQ-1-PAY-v1").splitlines()
    assert changed and all(f.startswith("tests/tsa/pay/") for f in changed)      # the commit only adds test files
    rep = t.tsa_report(jid)
    assert rep["text"].startswith("# TSA: PAY v1") and rep["next"] is None
    script = got["files"][0]["path"]
    page = t.tsa_file(jid, script)
    assert "@pytest.mark.tc(" in page["text"] and "# @exp " in page["text"] and page["total"] == len(page["text"])
    manifest = json.loads(t.tsa_file(jid, got["manifest"])["text"])
    assert len(manifest["assertions"]) == 37
    from tsa.errors import TsaError
    for bad in ("tests/conftest.py", "../app/service.py", "/etc/passwd"):
        with pytest.raises(TsaError) as e:
            t.tsa_file(jid, bad)
        assert e.value.code == "TSA_FILE_NOT_ALLOWED"


def test_main_checkout_is_not_made_sparse(project):
    t = Tools(project, spawn=False)
    jid = t.tsa_start("PAY", EXT, spec_version=1, branch="tsa/REQ-1-PAY-v1", base="main")["job_id"]
    run_job(project, jid)
    proc = subprocess.run(["git", "config", "--get", "core.sparseCheckout"], cwd=project, capture_output=True, text=True)
    assert proc.stdout.strip() in ("", "false")
    git(project, "checkout", "-q", "-b", "later")
    assert (project / "app" / "service.py").exists()


def test_spec_version_mismatch_and_cancel(project):
    t = Tools(project, spawn=False)
    jid = t.tsa_start("PAY", EXT, spec_version=2)["job_id"]
    assert run_job(project, jid) == 1
    got = t.tsa_get(jid)
    assert got["state"] == "error" and got["error"]["code"] == "SPEC_VERSION_CHANGED"
    other = t.tsa_start("PAY", {**EXT, "executionId": "req-9"}, spec_version=1)["job_id"]
    assert t.tsa_cancel(other, "stop run")["state"] == "cancelled"
    assert run_job(project, other) == 0 and t.tsa_get(other)["state"] == "cancelled"


def test_lost_process_is_reported(project):
    t = Tools(project, spawn=False)
    jid = t.tsa_start("PAY", EXT, spec_version=1)["job_id"]
    job = jobs.read(project, jid)
    job["state"], job["pid"] = "running", 2 ** 22 + 12345
    jobs.write(project, job)
    assert t.tsa_get(jid)["error"]["code"] == "TSA_JOB_LOST"


def test_over_mcp_stdio_start_runs_in_the_background(project):
    from mcp import Client, StdioServerParameters
    exe = shutil.which("tsa", path=os.path.dirname(sys.executable))

    async def go():
        params = StdioServerParameters(command=exe, args=["-C", str(project), "mcp", "serve", "--transport", "stdio"],
                                       env={**os.environ})
        async with Client(params) as c:
            names = {t.name for t in (await c.list_tools()).tools}
            assert names == {"tsa_start", "tsa_get", "tsa_report", "tsa_file", "tsa_cancel"}
            res = await c.call_tool("tsa_start", {"feature": "PAY", "spec_version": 1, "branch": "tsa/REQ-7-PAY-v1", "base": "main",
                                                  "external": {**EXT, "executionId": "req-7"}})
            started = json.loads(res.content[0].text)
            for _ in range(200):
                got = json.loads((await c.call_tool("tsa_get", {"job_id": started["job_id"]})).content[0].text)
                if got["state"] in ("done", "error", "cancelled"):
                    break
                await anyio.sleep(0.1)
            bad = await c.call_tool("tsa_get", {"job_id": "tsa-nope"})
            return started, got, bad

    started, got, bad = anyio.run(go)
    assert started["state"] == "queued" and got["state"] == "done", got
    assert got["counts"]["generated"] == 35
    assert bad.is_error and json.loads(bad.content[0].text)["error"]["code"] == "TSA_JOB_NOT_FOUND"


def test_jobs_dir_falls_back_outside_git(tmp_path):
    assert jobs.jobs_dir(tmp_path) == tmp_path / ".tsa" / "jobs"
