"""T6.2: layer B LLM fill with FakeLLM (TST-08)."""
import json
import re

import pytest

from tsagent.analyze import analyze
from tsagent.design import run_design_step
from tsagent.errors import LLMGroupsFailed
from tsagent.ingest import ingest
from tsagent.llm import make_client

pytestmark = pytest.mark.integration


def pack_of(user):
    return json.loads(re.search(r'<data name="context">\n(.*?)\n</data>', user, re.S).group(1))


def answer(system, user, bad_code=False, bad_value=False):
    p = pack_of(user)
    reqs = []
    for t in p["requirements"]:
        pcs = [{"conditionCode": c["code"], "text": "Đạt khi " + c["oracle"]["expected"][:100]} for c in t["conditions"]]
        if bad_code:
            pcs[0]["conditionCode"] = "TCND-BIA"
        cov = []
        for c in t["coverage"]:
            item = {"seedCode": c["code"], "description": "LLM: " + c["description"][:100]}
            if bad_value and isinstance(c.get("value"), dict):
                item["value"] = 123456789
            cov.append(item)
        reqs.append({"seedCode": t["seedCode"], "statement": "LLM: kiểm " + t["seedCode"], "scopeNote": "LLM: phạm vi " + t["testType"],
                     "passCriteria": pcs, "coverage": cov})
    return {"schemaVersion": 2, "requirements": reqs}


@pytest.fixture
def layer_a(cfg, pay_docs):
    pkg, _ = ingest(pay_docs, "PAY", cfg)
    cfg.llm_steps = ("design",)  # layer A from rule templates, only the design step uses the LLM
    g, *_ = analyze(pkg, "b", make_client(cfg), cfg)
    return pkg, g


def test_valid_fill(cfg, fake_llm, layer_a):
    pkg, g = layer_a
    cfg.llm_steps = ("design",)
    fake_llm.script = answer
    _, fr, rep = run_design_step(g, pkg, "b", make_client(cfg), cfg)
    assert rep.passed, rep.summary()
    tr = next(g.of("TestRequirement"))
    assert tr.attrs["statement"].startswith("LLM:") and tr.prov.derivation == "llm" and tr.prov.evidence
    pc = next(e for e in g.of("PassCriterion") if e.rel("requirement") == [tr.code])
    assert pc.attrs["text"].startswith("Đạt khi") and pc.prov.derivation == "llm"
    assert '"passCriteria"' in fake_llm.calls[0]["system"]


def test_wrong_condition_code_rejected(cfg, fake_llm, layer_a):
    pkg, g = layer_a
    cfg.llm_steps = ("design",)
    fake_llm.script = lambda s, u: answer(s, u, bad_code=True)
    with pytest.raises(LLMGroupsFailed) as e:  # every group ungrounded after retries
        run_design_step(g, pkg, "b", make_client(cfg), cfg)
    fr = e.value.result
    assert fr.rejected and not fr.filled
    assert all(not tr.attrs["statement"].startswith("LLM:") for tr in g.of("TestRequirement"))


def test_value_outside_partition_rejected(cfg, fake_llm, cfg_minipay):
    cfg2, pkg, g = cfg_minipay
    cfg2.llm_steps = ("design",)
    fake_llm.script = lambda s, u: answer(s, u, bad_value=True)
    _, fr, _ = run_design_step(g, pkg, "b", make_client(cfg2), cfg2)
    assert any("outside the Partition" in p for x in fr.rejected for p in x["problems"])


def test_none_mode_templates(cfg, layer_a):
    pkg, g = layer_a
    _, fr, rep = run_design_step(g, pkg, "b", make_client(cfg), cfg)
    assert rep.passed and fr.notes and fr.calls == 0


@pytest.fixture
def cfg_minipay(cfg, minipay_docs):
    pkg, _ = ingest([minipay_docs["slice"], minipay_docs["solution"]], "ST", cfg)
    cfg.llm_steps = ("design",)
    g, *_ = analyze(pkg, "b", make_client(cfg), cfg)
    return cfg, pkg, g
