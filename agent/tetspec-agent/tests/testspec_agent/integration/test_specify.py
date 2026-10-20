"""T6.3: layer C (TST-09): TC skeletons, when steps, assertions with oracles, fault injection; rules catch assertions off the oracle,
fault models without injection, secrets; LLM fill with FakeLLM."""
import json
import re

import pytest

from tsagent.analyze import analyze
from tsagent.design import run_design_step
from tsagent.errors import LLMGroupsFailed
from tsagent.ingest import ingest
from tsagent.llm import make_client
from tsagent.specify import PRE_REVIEW, run_specify_step
from tsagent.validate.tso_rules import Ctx, validate

pytestmark = pytest.mark.integration


def build(cfg, docs, feature, steps=()):
    cfg.llm_steps = tuple(steps)
    pkg, _ = ingest(docs, feature, cfg)
    g, *_ = analyze(pkg, "b", make_client(cfg), cfg)
    run_design_step(g, pkg, "b", make_client(cfg), cfg)
    return pkg, g


@pytest.fixture
def cr001(cfg, minipay_docs):
    pkg, g = build(cfg, [minipay_docs["slice"], minipay_docs["solution"], minipay_docs["supplement"]], "ST")
    _, _, rep = run_specify_step(g, pkg, "b", make_client(cfg), cfg)
    return pkg, g, rep


def gs(pkg, g, rules=None):
    return validate(Ctx(g, pkg, "b"), "GB", rules or PRE_REVIEW)


def test_gate_passes_in_none_mode(cr001):
    pkg, g, rep = cr001
    assert rep.passed, rep.summary()
    assert all(r.checked > 0 for r in rep.results if r.rule in ("TSO-011", "TSO-012", "TSO-013", "TSO-014", "TSO-017", "TSO-018"))


def test_testcase_shape(cr001):
    _, g, _ = cr001
    tc = g.get("TC-L3-ST-MECH-RunClaim-resilience-crash-mid-c")
    kinds = [s.attrs["stepKind"] for s in sorted((s for s in g.of("TestStep") if s.rel("testCase") == [tc.code]),
                                                  key=lambda s: s.attrs["ordinal"])]
    assert kinds[-2:] == ["when", "then"] and tc.rel("injects") and tc.rel("inSet")
    asserts = [a for a in g.of("Assertion") if a.rel("testCase") == [tc.code]]
    assert any(a.attrs["assertionKind"] == "count" for a in asserts) and all(a.rel("oracle") for a in asserts)
    amt = g.get("TC-L2-ST-FLD-Create-amount-inputValidation-max-v1")
    a = next(a for a in g.of("Assertion") if a.rel("testCase") == [amt.code])
    assert a.attrs["expected"].startswith("HTTP 422") and amt.rel("needsData")
    stubs = [s for s in g.of("StubSpec")]
    assert stubs and all(s.attrs["targetCode"] != "MECH-RunClaim" for s in stubs)
    assert {e.attrs["name"].split(" (")[0] for e in g.of("TestEnvironment")} >= {"Môi trường service"}


def test_assertion_not_matching_oracle_is_caught(cr001):
    pkg, g, _ = cr001
    a = next(a for a in g.of("Assertion") if a.attrs["expected"].startswith("HTTP 422"))
    a.attrs["expected"] = "HTTP 200: ok"
    assert [v["code"] for v in gs(pkg, g, ["TSO-012"]).result("TSO-012").violations] == [a.code]


def test_fault_model_without_injection_is_caught(cr001):
    pkg, g, _ = cr001
    fi = next(g.of("FaultInjection"))
    del g.entities[fi.code]
    for tc in g.of("TestCase"):
        tc.rels["injects"] = [x for x in tc.rel("injects") if x != fi.code]
    assert gs(pkg, g, ["TSO-013"]).result("TSO-013").violations


def test_secret_is_caught(cr001):
    pkg, g, _ = cr001
    st = next(g.of("TestStep"))
    st.attrs["action"] = "Gửi header Authorization: Bearer sk-ant-api03-ABCDEFGHIJKLMNOPQRSTUV"
    v = gs(pkg, g, ["TSO-018"]).result("TSO-018").violations
    assert v and v[0]["code"] == st.code


def pack_of(user):
    return json.loads(re.search(r'<data name="context">\n(.*?)\n</data>', user, re.S).group(1))


def answer(system, user, secret=False, no_when=False):
    p = pack_of(user)
    out = []
    for t in p["testcases"]:
        when = next(s for s in t["steps"] if s["stepKind"] == "when")
        steps = [{"stepKind": "given", "action": "LLM: chuẩn bị dữ liệu"},
                 {"stepKind": "given" if no_when else "when", "action": "LLM: " + when["action"][:200],
                  "targetCode": when.get("targetCode"), "input": when.get("input")},
                 {"stepKind": "then", "action": "LLM: kiểm kết quả" + (" với password=SuperSecret123" if secret else "")}]
        data = [{"seedCode": d["seedCode"], "example": ("password=SuperSecret123" if secret else
                                                        ("secretRef:key" if d["sensitive"] else str(d["example"])))} for d in t["data"]]
        out.append({"seedCode": t["seedCode"], "objective": "LLM: " + t["objective"][:200], "preconditions": ["LLM: tiền điều kiện"],
                    "steps": steps, "assertions": [{"seedCode": a["seedCode"], "description": "LLM: mô tả"} for a in t["assertions"]],
                    "data": data})
    return {"schemaVersion": 2, "testcases": out}


def test_llm_fill_applied(cfg, fake_llm, pay_docs):
    pkg, g = build(cfg, pay_docs, "PAY", steps=("specification",))
    fake_llm.script = answer
    _, fr, rep = run_specify_step(g, pkg, "b", make_client(cfg), cfg)
    assert rep.passed, rep.summary()
    tc = next(g.of("TestCase"))
    assert tc.attrs["objective"].startswith("LLM:") and tc.prov.derivation == "llm"
    steps = [s for s in g.of("TestStep") if s.rel("testCase") == [tc.code]]
    assert len(steps) == 3 and all(s.prov.derivation == "llm" for s in steps)
    a = next(a for a in g.of("Assertion") if a.rel("testCase") == [tc.code])
    assert a.attrs["description"] == "LLM: mô tả" and not a.attrs["expected"].startswith("LLM")


@pytest.mark.parametrize("kw,msg", [({"secret": True}, "secret patterns"), ({"no_when": True}, "when")])
def test_llm_bad_items_rejected(cfg, fake_llm, pay_docs, kw, msg):
    pkg, g = build(cfg, pay_docs, "PAY", steps=("specification",))
    fake_llm.script = lambda s, u: answer(s, u, **kw)
    with pytest.raises(LLMGroupsFailed) as e:  # every group still wrong after retries
        run_specify_step(g, pkg, "b", make_client(cfg), cfg)
    fr = e.value.result
    assert fr.rejected
    probs = [p for x in fr.rejected for p in x["problems"]]
    assert any(msg in p for p in probs)
    assert all(e.prov.derivation != "llm" for e in g.of("TestStep"))
