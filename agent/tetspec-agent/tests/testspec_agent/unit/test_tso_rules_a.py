"""T5.5: TSO-001..007, one passing and one failing case per rule; gate GA on CR-001."""
import copy

import pytest

from tsagent.analyze.seed_core import seed
from tsagent.ingest import ingest
from tsagent.model import Evidence
from tsagent.validate.tso_rules import Ctx, validate

pytestmark = pytest.mark.unit


@pytest.fixture(scope="module")
def base(tmp_path_factory, pay_docs_mod):
    from testspec_agent.conftest import make_cfg
    cfg = make_cfg(tmp_path_factory.mktemp("c"))
    pkg, _ = ingest(pay_docs_mod, "PAY", cfg)
    return pkg, seed(pkg, "b-pay").g


@pytest.fixture(scope="module")
def pay_docs_mod():
    import os

    from testspec_agent.conftest import PAY
    return [os.path.join(PAY, f) for f in ("urd.md", "srs.md", "tdd.md")]


def run(base, mutate=None, rules=None):
    pkg, g = base
    g = copy.deepcopy(g)
    if mutate:
        mutate(g)
    return validate(Ctx(g, pkg, "b-pay"), "GA", rules)


def viol(rep, rid):
    return rep.result(rid).violations


def test_seed_passes_ga_blocking_rules(base):
    rep = run(base)
    assert rep.passed, rep.summary()
    assert viol(rep, "TSO-007")  # open gap: blocks baseline only, not GA


def first(g, cls):
    return next(g.of(cls))


def test_tso_001(base):
    assert not viol(run(base, rules=["TSO-001"]), "TSO-001")
    rep = run(base, lambda g: setattr(first(g, "TestCondition").prov, "based_on", ["NOPE-1"]), ["TSO-001"])
    assert len(viol(rep, "TSO-001")) == 1


def test_tso_002(base):
    def m(g):
        e = first(g, "TestCondition")
        e.prov.derivation = "llm"
        e.prov.evidence = []
    v = viol(run(base, m, ["TSO-002"]), "TSO-002")
    assert len(v) == 1 and "model" in v[0]["message"] and "evidence" in v[0]["message"]

    def ok(g):
        e = first(g, "TestCondition")
        e.prov.derivation, e.prov.model, e.prov.prompt_hash = "llm", "fake", "abc"
        e.prov.evidence = [Evidence("FR-001", "x")]
    assert not viol(run(base, ok, ["TSO-002"]), "TSO-002")


def test_tso_003(base):
    assert not viol(run(base, rules=["TSO-003"]), "TSO-003")
    rep = run(base, lambda g: first(g, "TestCondition").prov.evidence.append(Evidence("FR-999", "bịa")), ["TSO-003"])
    assert "FR-999" in viol(rep, "TSO-003")[0]["message"]
    rep = run(base, lambda g: first(g, "TestCondition").link("faultModel", "TFM-KHONG-CO"), ["TSO-003"])
    assert viol(rep, "TSO-003")


def test_tso_004(base):
    assert not viol(run(base, rules=["TSO-004"]), "TSO-004")
    rep = run(base, lambda g: g.entities.pop("TOB-PAY-FR-002-func"), ["TSO-004"])
    assert [v["code"] for v in viol(rep, "TSO-004")] == ["FR-002"]


def test_tso_005(base):
    assert not viol(run(base, rules=["TSO-005"]), "TSO-005")
    rep = run(base, lambda g: g.entities.pop("TCND-PAY-CB-01-stateTransition-open"), ["TSO-005"])
    assert [v["code"] for v in viol(rep, "TSO-005")] == ["TCND-PAY-CB-01-stateTransition-open"]


def test_tso_006(base):
    assert not viol(run(base, rules=["TSO-006"]), "TSO-006")

    def m(g):
        c = g.get("TCND-PAY-SCN-01-timing-budget")
        g.get(c.rel("oracle")[0]).attrs.pop("measurement")
        first(g, "TestCondition").rels["oracle"] = []
    assert len(viol(run(base, m, ["TSO-006"]), "TSO-006")) == 2


def test_tso_007(base):
    rep = run(base, lambda g: [g.entities.pop(e.code) for e in list(g.of("TestGap"))], ["TSO-007"])
    assert not viol(rep, "TSO-007")
    rep = run(base, lambda g: first(g, "TestGap").attrs.update(owner=""), ["TSO-007"])
    assert any("owner" in v["message"] for v in viol(rep, "TSO-007"))


def test_gate_ga_on_cr001(cfg, minipay_docs):
    pkg, _ = ingest([minipay_docs["slice"], minipay_docs["solution"], minipay_docs["supplement"]], "ST", cfg)
    rep = validate(Ctx(seed(pkg, "b").g, pkg, "b"), "GA")
    assert rep.passed, rep.summary()
    assert rep.to_dict()["counts"]["TestCondition"] > 100


def test_tso_017_llm_access_on_requirement_counts_functional_as_allow(base):
    """The LLM proposes an access (deny) condition for an FR: the allow case is that FR's functional condition."""
    from tsagent.model import Provenance, TsoEntity

    def m(g):
        g.add(TsoEntity("TestCondition", "TCND-PAY-FR-001-access-p1", {"conditionKind": "access", "statement": "người lạ",
                        "expectedEffect": "403", "key": "p1"},
                        {"subject": ["TB-PAY-FR-001"], "obligation": ["TOB-PAY-FR-001-func"], "oracle": [next(g.of("TestOracle")).code]},
                        Provenance("llm", ["FR-001"])))
    assert not viol(run(base, m, ["TSO-017"]), "TSO-017")
