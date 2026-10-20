"""T7.1: orchestrator: full run with FakeLLM, runKey dedup, stop mid-step then resume without calling the LLM again (TST-15),
dry-run, CR end to end (TST-12), error isolation across features."""
import copy
import json

import pytest

from testspec_agent.helpers.fake_answers import answer, step_of
from tsagent.errors import TsagentError
from tsagent.orchestrator import Orchestrator, run_many
from tsagent.orchestrator.review import baseline_spec, review
from tsagent.store import tso as store_tso

pytestmark = pytest.mark.integration


def test_pay_end_to_end_with_fake_llm(cfg, tmp_db, fake_llm, pay_docs):
    fake_llm.script = answer
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["state"] == "done", st
    assert [s["step"] for s in st["steps"]] == ["ingest", "analysis", "design", "specification", "validate", "commit", "render"]
    assert all(s["status"] == "done" for s in st["steps"]) and len(st["result"]["files"]) == 3
    assert sum(s["llm_calls"] for s in st["steps"]) == len(fake_llm.calls) > 0
    g = store_tso.load_spec(tmp_db, "PAY")
    assert next(g.of("TestCase")).attrs["objective"].startswith("LLM:")
    again = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert again["deduplicated"] and again["runId"] == st["runId"]


@pytest.mark.parametrize("fail_step", ["analysis", "design", "specification", "commit"])
def test_resume_after_failure_without_partial_writes_or_new_llm_calls(cfg, tmp_db, fake_llm, pay_docs, monkeypatch, fail_step):
    fake_llm.script = answer
    real_put, real_commit = store_tso.stage_put, store_tso.commit
    boom = {"armed": True}

    def put(db, run_id, step, entities, replace=True):
        real_put(db, run_id, step, entities, replace)
        if step == fail_step and boom.pop("armed", False):
            raise TsagentError(f"process died at {step}")  # after writing, inside the transaction: must roll back

    def commit(*a, **k):
        if fail_step == "commit" and boom.pop("armed", False):
            raise TsagentError("process died at commit")
        return real_commit(*a, **k)
    monkeypatch.setattr(store_tso, "stage_put", put)
    monkeypatch.setattr(store_tso, "commit", commit)
    o = Orchestrator(tmp_db, cfg)
    st = o.start("PAY", pay_docs)
    assert st["state"] == "failed" and st["status"] == "Failed"
    assert not tmp_db.one("SELECT 1 FROM stage_item WHERE run_id = ? AND step = ?", (st["runId"], fail_step))
    assert not store_tso.versions(tmp_db, "PAY")
    def per_step():
        out = {}
        for c in fake_llm.calls:
            out[step_of(c["system"])] = out.get(step_of(c["system"]), 0) + 1
        return out
    before = per_step()
    st2 = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st2["runId"] == st["runId"] and st2["state"] == "done", st2
    after = per_step()
    # steps already run (even the failed one) do not call the LLM again: finished steps are skipped, the redone step reads the cache
    assert all(after[s] == before[s] for s in before), (before, after)
    assert store_tso.versions(tmp_db, "PAY")[-1]["version"] == 1


def test_dry_run_does_not_commit(cfg, tmp_db, pay_docs):
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs, dry_run=True)
    assert st["state"] == "done" and "commit" not in [s["step"] for s in st["steps"]]
    assert not store_tso.versions(tmp_db, "PAY") and st["result"]["report"]["passed"]


def test_from_step_reruns(cfg, tmp_db, pay_docs):
    o = Orchestrator(tmp_db, cfg)
    st = o.start("PAY", pay_docs)
    st2 = o.start("PAY", pay_docs, from_step="design")
    assert st2["runId"] == st["runId"] and st2["state"] == "done"
    assert store_tso.versions(tmp_db, "PAY")[-1]["version"] == 1  # the Draft is rewritten in place


def write(tmp_path, name, obj):
    p = tmp_path / name
    p.write_text(json.dumps(obj, ensure_ascii=False), encoding="utf-8")
    return str(p)


def test_cr_end_to_end(cfg, tmp_db, tmp_path, minipay_full):
    """TST-12: v1 baseline; a CR edits FR-046 and drops FR-049: v2 in place, unchanged nodes reused, dropped nodes Retired with the CR, regression."""
    o = Orchestrator(tmp_db, cfg)
    minipay_full = copy.deepcopy(minipay_full)
    next(n for n in minipay_full["nodes"] if n["code"] == "DOC-TDD")["status"] = "Baselined"  # T17: the primary TDD must be Baselined
    st = o.start("ST", [write(tmp_path, "v1.json", minipay_full)])
    assert st["state"] == "done", st
    review(tmp_db, "ST", "approved", "qa")
    b1 = baseline_spec(tmp_db, cfg, "ST", "qa")
    assert b1["version"] == 1
    v2 = copy.deepcopy(minipay_full)
    fr = next(n for n in v2["nodes"] if n["code"] == "FR-046")
    fr["props"]["text"] = fr["props"].get("text", "") + " (sửa theo CR-009)"
    v2["nodes"] = [n for n in v2["nodes"] if n["code"] != "FR-049"]
    v2["edges"] = [e for e in v2["edges"] if "FR-049" not in (e[0], e[2])]
    v2["nodes"].append({"code": "CR-009", "class": "ChangeRequest", "props": {"description": "sửa FR-046, bỏ FR-049"}})
    v2["edges"] += [["CR-009", "affects", "FR-046", {}]]
    st2 = o.start("ST", [write(tmp_path, "v2.json", v2)])
    assert st2["state"] == "done", st2
    r = st2["result"]
    assert r["version"] == 2 and r["unchanged"] > 0 and r["modified"] > 0
    assert any("FR-049" in c for c in r["retiring"])
    assert r["regression"], "phải chọn được TC hồi quy"
    hist = store_tso.history(tmp_db, "ST")
    assert [h["version"] for h in hist] == [1, 2] and hist[1]["changedBy"] == ["CR-009"]
    review(tmp_db, "ST", "approved", "qa")
    b2 = baseline_spec(tmp_db, cfg, "ST", "qa")
    assert b2["retired"]
    old = store_tso.load_spec(tmp_db, "ST", 1)
    gone = b2["retired"][0]
    assert old.get(gone) is not None  # the old node is still queryable in version 1
    row = tmp_db.one("SELECT lc.status, lc.retired_by FROM tso_lifecycle lc WHERE lc.tbl = ? AND lc.row_id = ?",
                     old.meta["rows"][gone])
    assert row["status"] == "Retired" and row["retired_by"] == "CR-009"


def test_baseline_blocked_when_primary_tdd_not_baselined(cfg, tmp_db, tmp_path, minipay_full):
    from tsagent.errors import ValidationFailed
    Orchestrator(tmp_db, cfg).start("ST", [write(tmp_path, "v1.json", minipay_full)])
    review(tmp_db, "ST", "approved", "qa")
    with pytest.raises(ValidationFailed, match="DOC-TDD is not Baselined"):
        baseline_spec(tmp_db, cfg, "ST", "qa")


def test_many_features_isolated(cfg, tmp_db, pay_docs, tmp_path):
    bad = tmp_path / "bad.json"
    bad.write_text("{ hỏng", encoding="utf-8")
    out = run_many(tmp_db, cfg, [("BAD", [str(bad)]), ("PAY", pay_docs)])
    assert out["BAD"]["state"] == "failed" and out["PAY"]["state"] == "done"


def test_parallel_llm_same_result_and_faster(cfg, tmp_path, fake_llm, pay_docs):
    """TSAGENT_LLM_PARALLEL: same result as a sequential run (pack, cache, apply order unchanged), less time."""
    import time

    from testspec_agent.conftest import make_cfg
    from tsagent.store.db import Database
    fake_llm.script, fake_llm.delay_s = answer, 0.05
    out = {}
    for par in (1, 4):
        c = make_cfg(tmp_path / f"p{par}", llm_provider="fake", llm_parallel=par)
        db = Database(c.db)
        db.init_schema()
        t = time.perf_counter()
        st = Orchestrator(db, c).start("PAY", pay_docs)
        out[par] = (time.perf_counter() - t, st, {e.code: e.content_hash() for e in store_tso.load_spec(db, "PAY").entities.values()})
        db.close()
    assert out[1][1]["state"] == out[4][1]["state"] == "done"
    assert out[1][2] == out[4][2]                     # identical test KG
    assert out[4][0] < out[1][0] * 0.7                # clearly faster
