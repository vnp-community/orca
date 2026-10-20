"""T5.4: layer A LLM fill with FakeLLM (TST-06, TST-07): valid, invented codes, missing evidence, broken JSON, cap reached, breaker open."""
import json
import re

import pytest

from tsagent.analyze import analyze, run_analysis
from tsagent.errors import CircuitOpen, LLMGroupsFailed, LLMPermanent
from tsagent.ingest import ingest
from tsagent.llm import make_client
from tsagent.store import source
from tsagent.store import tso as store_tso
from tsagent.store.db import now

pytestmark = pytest.mark.integration


def pack_of(user):
    m = re.search(r'<data name="context">\n(.*?)\n</data>', user, re.S)
    return json.loads(m.group(1))


def good(system, user, fabricate=False, no_evidence=False):
    p = pack_of(user)
    subj = p["subject"]["code"]
    src = "FR-999" if fabricate else subj
    conds = [{"seedCode": c["seedCode"], "statement": "LLM: " + c["statement"][:200], "expectedEffect": "LLM: " + c["expectedEffect"][:200],
              "basedOn": [subj], "evidence": [] if no_evidence else [{"sourceCode": src, "quote": "trích"}], "confidence": 0.9,
              **({"oracle": {"expected": "LLM oracle", "measurement": "p95 trong 5 phút"}} if c.get("oracle") else {})}
             for c in p["conditions"]]
    return {"schemaVersion": 2, "subject": subj, "obligations": [{"seedCode": o["seedCode"], "question": "LLM hỏi?"} for o in p["obligations"]],
            "conditions": conds,
            "proposedConditions": [{"conditionKind": "concurrency", "statement": "Hai yêu cầu đồng thời", "expectedEffect": "một hiệu ứng",
                                    "oracle": {"oracleKind": "invariant", "expected": "một hiệu ứng", "sourceCode": subj},
                                    "basedOn": [subj], "evidence": [{"sourceCode": subj, "quote": "x"}], "confidence": 0.5}],
            "gaps": [{"missing": "thiếu mã lỗi", "askedQuestion": "Mã lỗi là gì?", "blocks": [c["seedCode"] for c in p["conditions"]][:1]}]}


@pytest.fixture
def pay(cfg, pay_docs):
    pkg, _ = ingest(pay_docs, "PAY", cfg)
    return pkg


def test_valid_output_is_applied_with_provenance(cfg, fake_llm, pay):
    fake_llm.script = good
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg, run_id="r1")
    assert rep.passed, rep.summary()
    c = g.get("TCND-PAY-INT-01-idempotency-replay")
    assert c.attrs["statement"].startswith("LLM:") and c.prov.derivation == "llm"
    assert c.prov.model == "fake:fake-1" and len(c.prov.prompt_hash) == 16 and c.prov.evidence
    assert fr.proposed and g.get(fr.proposed[0]).prov.derivation == "llm"
    assert g.get("TOB-PAY-INT-01-proposal").attrs["origin"] == "llmProposal"
    # a filled condition drops its "content" gap
    assert "TGAP-PAY-INT-01-content-idempotency-payload-mismatch" not in g.entities
    assert any(g.get(x).attrs["missing"] == "thiếu mã lỗi" for x in fr.gaps)
    # the schema is always in the system prompt
    assert '"proposedConditions"' in fake_llm.calls[0]["system"]


def test_fabricated_codes_are_rejected_after_one_repair(cfg, fake_llm, pay):
    fake_llm.script = lambda s, u: good(s, u, fabricate=True)
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed, rep.summary()
    assert fr.rejected and not fr.filled
    repairs = [c for c in fake_llm.calls if "Grounding errors" in c["user"]]
    # exactly one repair turn per attempt; a wrong subject is retried llm_group_retries times
    assert len(repairs) == len(fr.rejected) * (cfg.llm_group_retries + 1)
    assert sorted(fr.retried) == sorted(r["subject"] for r in fr.rejected)
    assert all(not any(ev.source_code == "FR-999" for ev in e.prov.evidence) for e in g.entities.values())
    assert any(c.endswith("llm-invalid") for c in fr.gaps)


def test_missing_evidence_is_rejected(cfg, fake_llm, pay):
    fake_llm.script = lambda s, u: good(s, u, no_evidence=True)
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed and not fr.filled and fr.rejected


def test_broken_json_fails_step_after_retries(cfg, fake_llm, pay):
    # every group returns broken JSON after retries: more than 50 % of groups failed -> the step fails; the step result rides on the error
    fake_llm.script = "không phải JSON"
    with pytest.raises(LLMGroupsFailed, match="LLM_GROUPS_FAILED") as e:
        analyze(pay, "b", make_client(cfg), cfg)
    fr = e.value.result
    assert not fr.filled and fr.skipped and all("schema" in x["reason"] for x in fr.skipped)
    assert len(fr.failed_groups) == e.value.failed == e.value.attempted


def test_budget_exhausted_keeps_seed(cfg, fake_llm, pay):
    cfg.llm_caps = {"analysis": 2, "design": 1, "specification": 1}
    fake_llm.script = good
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed and len(fake_llm.calls) == 2 and any("cap reached" in x["reason"] for x in fr.skipped)


def test_breaker_open_and_fatal(cfg, fake_llm, pay):
    fake_llm.script = CircuitOpen("mở")
    with pytest.raises(LLMGroupsFailed) as e:
        analyze(pay, "b", make_client(cfg), cfg)
    fr = e.value.result
    assert fr.skipped and all("circuitOpen" in x["reason"] for x in fr.skipped)
    fake_llm.script = LLMPermanent("401", fatal=True)
    with pytest.raises(LLMPermanent):
        analyze(pay, "b", make_client(cfg), cfg)


def test_none_mode_uses_rule_templates(cfg, pay):
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed and fr.notes and fr.calls == 0


def test_run_analysis_writes_stage(cfg, fake_llm, pay, tmp_db):
    fake_llm.script = good
    bid = source.save_baseline(tmp_db, pay)["baseline"]
    tmp_db.x("INSERT INTO run(run_id, feature, run_key, mode, status, created_at, updated_at) VALUES ('r1','PAY','k','fake','running',?,?)",
             (now(), now()))
    res = run_analysis(tmp_db, "r1", pay, bid, make_client(cfg), cfg)
    staged = store_tso.stage_load(tmp_db, "r1", "PAY", ["analysis"])
    assert set(staged.entities) == set(res["graph"].entities)
    row = tmp_db.one("SELECT status, llm_calls FROM run_step WHERE run_id='r1' AND step='analysis'")
    assert row["status"] == "done" and row["llm_calls"] > 0
    # can be committed as a SpecVersion (foreign keys, basedOn within the baseline)
    stats = store_tso.commit(tmp_db, "r1", staged, bid)
    assert stats["version"] == 1 and stats["included"] == len(staged.entities)


def test_only_affected_subjects_are_called_others_reused(cfg, fake_llm, pay):
    fake_llm.script = good
    g1, *_ = analyze(pay, "b", make_client(cfg), cfg)
    n1 = len(fake_llm.calls)
    fake_llm.calls.clear()
    cfg.cache_dir = cfg.cache_dir + "-2"  # empty cache: real calls only for affected subjects
    g2, fr, *_ = analyze(pay, "b", make_client(cfg), cfg, only={"INT-01"}, prev=g1)
    assert 0 < len(fake_llm.calls) < n1 and fr.reused
    assert g2.get("TCND-PAY-AC-FR-001-1-functional").attrs["statement"].startswith("LLM:")
