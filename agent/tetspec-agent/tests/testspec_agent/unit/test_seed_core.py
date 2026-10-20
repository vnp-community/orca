"""T5.2: Core profile seed rules on PAY (md) and MiniPay CR-001 (TST-05, Core part)."""
import collections

import pytest

from tsagent.analyze.seed_core import seed
from tsagent.ingest import ingest

pytestmark = pytest.mark.unit


@pytest.fixture
def pay(cfg, pay_docs):
    pkg, _ = ingest(pay_docs, "PAY", cfg)
    return seed(pkg, "b-pay")


@pytest.fixture
def cr001(cfg, minipay_docs):
    pkg, _ = ingest([minipay_docs["slice"], minipay_docs["solution"], minipay_docs["supplement"]], "ST", cfg)
    return seed(pkg, "b-st")


def kinds(s):
    return collections.Counter(e.attrs["conditionKind"] for e in s.g.of("TestCondition"))


def cond(s, code):
    e = s.g.get(code)
    assert e is not None, code
    return e


def test_expected_counts_pay(pay):
    # expected table (TST-05): update deliberately when the seed rules change
    assert collections.Counter(e.cls for e in pay.g.entities.values()) == {
        "TestCondition": 35, "TestOracle": 29, "TestBasisItem": 26, "TestObligation": 10, "TestGap": 8, "FaultModel": 8,
        "ScenarioPath": 2, "RiskRating": 1}


def test_fr_with_and_without_ac(pay):
    c = cond(pay, "TCND-PAY-AC-FR-001-1-functional")
    assert c.prov.derivation == "stated" and c.rel("obligation") == ["TOB-PAY-FR-001-func"]
    assert pay.g.get(c.rel("oracle")[0]).attrs["oracleKind"] == "acceptanceCriterion"
    gap = pay.g.get("TGAP-PAY-FR-002-no-ac")
    assert gap.rel("blocksObligation") == ["TOB-PAY-FR-002-func"] and gap.attrs["owner"]


def test_integration_rules(pay):
    k = kinds(pay)
    assert k["errorHandling"] >= 7 and k["idempotency"] == 2 and k["concurrency"] == 1
    for key in ("open", "halfopen", "close"):
        cond(pay, f"TCND-PAY-CB-01-stateTransition-{key}")
    fm = pay.g.get(cond(pay, "TCND-PAY-ERR-01-timeout-errorHandling").rel("faultModel")[0])
    assert fm.attrs["injectionPoint"] == "INT-01"
    # worst case above the flow budget: the condition is still generated, figures in the statement
    w = cond(pay, "TCND-PAY-SCN-01-timing-worst-INT-01")
    assert "75000 ms" in w.attrs["statement"] and "30000" in w.attrs["expectedEffect"]
    o = pay.g.get(w.rel("oracle")[0])
    assert o.attrs["oracleKind"] == "threshold" and "xấu nhất" in o.attrs["measurement"]
    cond(pay, "TCND-PAY-SEC-02-access-allow")
    cond(pay, "TCND-PAY-SEC-02-access-deny")


def test_unknown_outcome_fault_is_after_commit(pay):
    ums = [e for e in pay.g.of("TestCondition") if e.rel("faultModel")
           and pay.g.get(e.rel("faultModel")[0]).attrs["faultKind"] == "unknownOutcome"]
    assert ums and all(pay.g.get(e.rel("faultModel")[0]).attrs["timing"] == "afterCommit" for e in ums)


def test_state_machine_and_guards(cr001):
    assert kinds(cr001)["stateTransition"] == 15 and kinds(cr001)["guardViolation"] == 13
    g = cond(cr001, "TCND-ST-TRN-Schedule-1-guardViolation-guard")
    assert "Không chuyển sang PAUSED" in g.attrs["expectedEffect"]


def test_endpoint_fields_and_responses(cr001):
    c = cond(cr001, "TCND-ST-FLD-Create-amount-inputValidation-max")
    part = cr001.g.get(c.rel("partition")[0])
    assert part.attrs["classKind"] == "boundary" and {"value": 50000001, "valid": False} in part.attrs["values"] and part.attrs["valid"] is False
    assert cr001.g.get(c.rel("oracle")[0]).attrs["oracleKind"] == "schema"
    cond(cr001, "TCND-ST-FLD-Create-recurrence-inputValidation-enum")
    r = cond(cr001, "TCND-ST-RSP-Create-201-functional")
    assert r.attrs["expectedEffect"].startswith("HTTP 201")


def test_v1_gaps_are_seeded_without_llm(cr001):
    """BASELINE-V1 marks: G1 concurrent duplicate, G2 crash midway, G3 worst-case budget, G4 throttle, G5 keeps 99 %, G14 access control."""
    k = kinds(cr001)
    assert k["concurrency"] >= 2                                                    # G1
    crash = cond(cr001, "TCND-ST-MECH-RunClaim-resilience-crash-mid")               # G2
    assert cr001.g.get(crash.rel("faultModel")[0]).attrs["timing"] == "afterCommit"
    cond(cr001, "TCND-ST-SCN-ScheduledRun-timing-worst-INT-01")                     # G3
    cond(cr001, "TCND-ST-MECH-SchedulerThrottle-performance-over-limit")            # G4
    o = cr001.g.get(cond(cr001, "TCND-ST-NFR-010-timing-target").rel("oracle")[0])  # G5
    assert "99%" in o.attrs["measurement"]
    eps = {e.attrs["statement"].split(" khi ")[0] for e in cr001.g.of("TestCondition") if e.attrs["conditionKind"] == "access"}
    assert len(eps) == 6                                                            # G14: every endpoint


def test_gaps_only_direct_assumptions(cr001):
    """G13: unconfirmed Assumption/OpenQuestion only blocks subjects that declare it directly; context nodes are not subjects."""
    gaps = [e for e in cr001.g.of("TestGap") if "chưa xác nhận" in e.attrs["missing"]]
    assert not gaps  # in the CR-001 slice, unconfirmed assumptions only hang on Integrations (context)
    assert sum(len(e.rel("blocks")) for e in cr001.g.of("TestGap")) < 30


def test_context_nodes_are_not_subjects(cr001):
    subjects = {cr001.g.get(e.rel("subject")[0]).attrs["tddCode"] for e in cr001.g.of("TestCondition")}
    assert "INT-01" not in subjects and "MOD-API" not in subjects
    assert cr001.g.get("TB-ST-MOD-API").attrs["inScope"] is False  # context Module taking part in the flow


def test_risk_raises_priority(cr001):
    ob = cr001.g.get("TOB-ST-MECH-RunClaim-mechanism")
    assert ob.rel("ratedBy") and ob.attrs["priority"] == "P0"


def test_access_rule_fr_without_ac_gets_allow_and_deny(pay):
    # an FR without acceptance criteria that states an access rule gets the allow/deny pair from the seed (TSO-017),
    # whatever keys the LLM later gives to the conditions it proposes
    from tsagent.analyze.seed_core import ACCESS_RULE_RE
    assert ACCESS_RULE_RE.search("Dữ liệu dự án chỉ hiển thị cho thành viên và vai trò hệ thống")
    assert ACCESS_RULE_RE.search("API token gắn một dự án, scope tối thiểu")
    assert not ACCESS_RULE_RE.search("Hệ thống gửi email nhắc hạn")
    fr = pay.pkg.get("FR-002")
    pay.rule_fr_access(fr, "TOB-PAY-FR-002-func")
    assert cond(pay, "TCND-PAY-FR-002-access-allow").attrs["key"] == "allow"
    assert cond(pay, "TCND-PAY-FR-002-access-deny").attrs["key"] == "deny"
