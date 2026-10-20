"""T6.1: design rules on CR-001 (TST-08, rule part)."""
import collections

import pytest

from tsagent.analyze.seed_core import seed
from tsagent.design.rules import design, technique_for
from tsagent.ingest import ingest
from tsagent.validate.tso_rules import Ctx, validate

pytestmark = pytest.mark.unit


@pytest.fixture(scope="module")
def cr001(tmp_path_factory):
    import os

    from testspec_agent.conftest import MINIPAY, make_cfg
    cfg = make_cfg(tmp_path_factory.mktemp("d"))
    pkg, _ = ingest([os.path.join(MINIPAY, f) for f in ("cr001-slice.tdd-kg.json", "solution.md", "supplement.md")], "ST", cfg)
    s = seed(pkg, "b")
    design(s.g, pkg)
    return pkg, s.g


def test_every_condition_has_coverage(cr001):
    pkg, g = cr001
    realized = {e.rel("realizes")[0] for e in g.of("CoverageItem")}
    assert {c.code for c in g.of("TestCondition")} <= realized
    rep = validate(Ctx(g, pkg, "b"), "GB", ["TSO-008", "TSO-009", "TSO-010", "TSO-016"])
    assert rep.passed and not any(r.violations for r in rep.results), rep.summary()


def test_levels(cr001):
    _, g = cr001
    lv = collections.Counter(tr.attrs["level"] for tr in g.of("TestRequirement"))
    assert lv["L1"] >= 1 and lv["L3"] >= 3 and lv["L2"] > lv["L3"]
    l1 = [tr for tr in g.of("TestRequirement") if tr.attrs["level"] == "L1"]
    assert all("SCN-ScheduledRun" in tr.code for tr in l1)
    assert g.get("TR-ST-L3-MECH-RunClaim-reliability") is not None


def test_pass_criteria_numbered_and_scope_note(cr001):
    _, g = cr001
    tr = g.get("TR-ST-L2-FR-041-functional")
    pcs = sorted((e for e in g.of("PassCriterion") if e.rel("requirement") == [tr.code]), key=lambda e: e.attrs["ordinal"])
    assert [p.attrs["ordinal"] for p in pcs] == list(range(1, len(tr.rel("groups")) + 1))
    assert {p.rel("condition")[0] for p in pcs} == set(tr.rel("groups"))
    ep = g.get("TR-ST-L2-EP-CreateSchedule-functional")
    assert "security" in ep.attrs["scopeNote"] and not tr.attrs["statement"].startswith("Tạo lịch")


def test_techniques_and_boundary_coverage(cr001):
    _, g = cr001
    c = g.get("TCND-ST-FLD-Create-amount-inputValidation-max")
    assert technique_for(c, g) == "boundaryValue"
    covs = [e for e in g.of("CoverageItem") if e.rel("realizes") == [c.code]]
    assert sorted((e.attrs["value"]["value"], e.attrs["value"]["valid"]) for e in covs) == [(50000000, True), (50000001, False)]
    valid = next(e for e in covs if e.attrs["value"]["valid"])
    assert "được chấp nhận" in valid.attrs["expected"]
    assert technique_for(g.get("TCND-ST-SCN-ScheduledRun-functional-main"), g) == "scenario"
    tds = {e.attrs["technique"] for e in g.of("TestDesign")}
    assert {"boundaryValue", "equivalencePartitioning", "stateTransition", "faultInjection", "accessMatrix", "loadModel"} <= tds


def test_benchmark_for_performance(cr001):
    _, g = cr001
    assert g.get("TR-ST-L2-NFR-013-performance").attrs["verificationMethod"] == "benchmark"
