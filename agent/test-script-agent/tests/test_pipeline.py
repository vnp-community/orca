import json
import subprocess
import sys

import pytest

from tsa import generators, pipeline, tcir, worktree
from tsa.errors import WorktreeError
from tsa.generators import Result

from .conftest import git


def opts(repo, **kw):
    return pipeline.Options(repo=repo, base="main", feature="PAY", **kw)


def test_template_run_writes_scripts_into_a_new_worktree_and_commits(test_repo, pay_bundle):
    out = pipeline.run(opts(test_repo, level=None), pay_bundle, generators.TemplateGenerator())
    root = test_repo.parent / "qa-tests.tsa-worktrees" / "tsa-PAY-v1"
    assert out.branch == "tsa/PAY-v1" and out.worktree == str(root) and out.commit
    assert out.counts["generated"] == 35 and out.counts.get("needs-review", 0) == 0
    assert git(root, "rev-parse", "--abbrev-ref", "HEAD") == "tsa/PAY-v1"
    assert git(test_repo, "status", "--porcelain") == ""                 # the main checkout is untouched
    files = git(root, "show", "--name-only", "--format=", "HEAD").splitlines()
    assert "tests/tsa/pay/assertion-manifest.json" in files and "tests/tsa/pay/tsa-report.md" in files
    assert len([f for f in files if f.endswith(".py")]) == 9               # one file per test set
    manifest = json.loads((root / out.manifest_path).read_text())
    assert len(manifest["assertions"]) == 37 and all(a["status"] == "ok" for a in manifest["assertions"])
    report = (root / out.report_path).read_text()
    assert "đã sinh 35" in report and "văn xuôi" in report
    collected = subprocess.run([sys.executable, "-m", "pytest", "--collect-only", "-q", "tests/tsa"], cwd=root, capture_output=True,
                               text=True)
    assert collected.returncode == 0 and "35 tests collected" in collected.stdout


def test_existing_worktree_needs_reuse(test_repo, pay_bundle):
    small = tcir.Bundle("PAY", 1, None, pay_bundle.cases[:2])
    pipeline.run(opts(test_repo), small, generators.TemplateGenerator())
    with pytest.raises(WorktreeError, match="--reuse"):
        pipeline.run(opts(test_repo), small, generators.TemplateGenerator())
    again = pipeline.run(opts(test_repo, reuse=True), small, generators.TemplateGenerator())
    assert again.commit is None                                              # same content: nothing new to commit


class Scripted:
    """Writes the template, then applies one edit per attempt: lets a test drive the fix loop."""
    name = "scripted"

    def __init__(self, edits):
        self.edits, self.calls = list(edits), []

    def generate(self, req):
        self.calls.append(list(req.feedback))
        path = req.worktree / req.plan.path
        path.parent.mkdir(parents=True, exist_ok=True)
        text = generators.render_template(req)
        edit = self.edits.pop(0) if self.edits else None
        if edit:
            text = edit(text)
        path.write_text(text)
        return Result(True, "scripted")


def one_case(pay_bundle):
    case = next(c for c in pay_bundle.cases if c["expected"][0]["value"] == "FB-01")
    return tcir.Bundle("PAY", 1, None, [case]), case


def test_fix_round_repairs_a_wrong_value(test_repo, pay_bundle):
    bundle, case = one_case(pay_bundle)
    gen = Scripted([lambda t: t.replace("== 'FB-01'", "== 'FB-99'"), None])
    out = pipeline.run(opts(test_repo), bundle, gen)
    (f,) = out.files
    assert f.status == "generated" and f.attempts == 2
    assert any("value-mismatch" in msg for msg in gen.calls[1])


def test_diff_guard_reverts_a_fix_round_that_rewrites_a_verified_assertion(test_repo, pay_bundle):
    bundle, case = one_case(pay_bundle)
    exp = case["expected"][0]["id"]
    broken = lambda t: t.replace(f"# @exp {exp}", f"# @exp {exp}\n    assert 1 == 1")              # noqa: E731  extra assert -> error
    rewrite = lambda t: t.replace("== 'FB-01'", "!= 'FB-01'")                                       # noqa: E731  touches the verified one
    out = pipeline.run(opts(test_repo), bundle, Scripted([broken, rewrite]))
    (f,) = out.files
    assert f.status == "needs-review" and f.attempts == 2
    assert any(x.rule == "diff-guard" for x in f.findings)
    text = (test_repo.parent / "qa-tests.tsa-worktrees" / "tsa-PAY-v1" / f.path).read_text()
    assert "== 'FB-01'" in text and "!= 'FB-01'" not in text                                       # the earlier version is kept


def test_gives_up_after_max_fix_rounds(test_repo, pay_bundle):
    bundle, _ = one_case(pay_bundle)
    always = lambda t: t + "\n\ndef test_extra():\n    assert 1 == 2\n"                              # noqa: E731
    out = pipeline.run(opts(test_repo, max_fix=2), bundle, Scripted([always] * 5))
    (f,) = out.files
    assert f.status == "needs-review" and f.attempts == 3


def test_branch_name_is_safe():
    assert worktree.branch_name("PAY ORDER/x", 3) == "tsa/PAY-ORDER-x-v3"
