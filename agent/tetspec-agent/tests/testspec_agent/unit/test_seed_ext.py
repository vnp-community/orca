"""T8.3: Extended seed rules on the module 04 fixture (TST-05, Extended part); a Core package gives the same result."""
import collections
import os

import pytest

from testspec_agent.conftest import MINIPAY
from testspec_agent.helpers.pipeline import build_graph
from tsagent.analyze.seed_core import seed
from tsagent.ingest import ingest
from tsagent.specify import PRE_REVIEW
from tsagent.validate.tso_rules import Ctx, validate

pytestmark = pytest.mark.unit
DOCS = ("solution.md", "supplement.md")


@pytest.fixture(scope="module")
def ext(tmp_path_factory):
    from testspec_agent.conftest import make_cfg
    cfg = make_cfg(tmp_path_factory.mktemp("ext"))
    pkg, _, g, _ = build_graph(cfg, [os.path.join(MINIPAY, f) for f in ("cr001-ext.tdd-kg.json",) + DOCS], "ST")
    return pkg, g


def cond(g, code):
    assert g.get(code) is not None, code
    return g.get(code)


def test_core_package_unchanged_by_extended_code(cfg):
    pkg, _ = ingest([os.path.join(MINIPAY, f) for f in ("cr001-slice.tdd-kg.json",) + DOCS], "ST", cfg)
    codes = set(seed(pkg, "b").g.entities)
    assert not any(c.startswith(("TOB-ST-OBL", "TCND-ST-IDP", "TCND-ST-PD-")) for c in codes)


def test_design_obligations(ext):
    _, g = ext
    obs = {e.code: e for e in g.of("TestObligation") if e.attrs["origin"] == "designObligation"}
    assert set(obs) == {"TOB-ST-OBL-1-design", "TOB-ST-OBL-2-design", "TOB-ST-OBL-3-design"}  # OBL-4 notApplicable
    gap = next(e for e in g.of("TestGap") if "GAP-1" in e.prov.based_on)
    assert gap.rel("blocksObligation") == ["TOB-ST-OBL-1-design"] and gap.attrs["owner"] == "Tech Lead"


def test_operation_invariant_concurrency(ext):
    _, g = ext
    cond(g, "TCND-ST-OP-ExecuteScheduledRun-functional-post-1")
    cond(g, "TCND-ST-OP-ExecuteScheduledRun-guardViolation-pre-1")
    cond(g, "TCND-ST-OP-ExecuteScheduledRun-errorHandling-err-2")
    assert len([c for c in g.of("TestCondition") if c.code.startswith("TCND-ST-INV-OneTransferPerRun-invariant-ep-")]) == 2
    cond(g, "TCND-ST-CP-RunClaim-concurrency-res-1")


def test_error_contract_timing_by_side_effect(ext):
    _, g = ext
    fm = lambda c: g.get(cond(g, c).rel("faultModel")[0]).attrs["timing"]  # noqa: E731
    assert fm("TCND-ST-EC-FraudUnavailable-errorHandling-err") == "before"
    assert fm("TCND-ST-EC-LedgerUnknown-errorHandling-err") == "afterCommit"


def test_idempotency_budget_transaction_permission(ext):
    _, g = ext
    keys = {c.attrs["key"] for c in g.of("TestCondition") if g.get(c.rel("subject")[0]).attrs["tddCode"] == "IDP-ScheduledRun"}
    assert keys == {"replay", "payload-mismatch", "concurrent-dup", "after-retention"}
    t = cond(g, "TCND-ST-TB-ScheduledRun-timing-worst-TB-ScheduledRun")
    assert "1000 ms" in t.attrs["statement"] and "60000" in t.attrs["statement"]
    assert {c.attrs["key"] for c in g.of("TestCondition") if c.attrs["conditionKind"] == "atomicity"} == {"before-commit", "after-commit"}
    pd = [c for c in g.of("TestCondition") if g.get(c.rel("subject")[0]).attrs["tddClass"] == "PermissionDesign"]
    assert len(pd) == 12 and {c.attrs["key"].split("-")[0] for c in pd} == {"allow", "deny"}


def test_extended_gate_and_testcases(ext):
    pkg, g = ext
    rep = validate(Ctx(g, pkg, "b"), "GB", PRE_REVIEW)
    assert rep.passed, rep.summary()
    covered = {g.get(g.get(cv).rel("realizes")[0]).prov.based_on[-1] for tc in g.of("TestCase") for cv in tc.rel("covers")}
    kinds = collections.Counter(g.get(g.get(tc.rel("covers")[0]).rel("realizes")[0]).attrs["conditionKind"] for tc in g.of("TestCase"))
    assert kinds["atomicity"] == 2 and kinds["invariant"] >= 2 and covered
