"""Integration T1.4-T1.6: run `origin`, `llm:` front matter, run failure/warning by the group threshold (TC-27, TC-28, TC-30)."""
import json

import pytest

from testspec_agent.integration.test_analyze import good, pack_of
from tsagent.errors import LLMTransient
from tsagent.orchestrator import Orchestrator
from tsagent.store import tso as store_tso

pytestmark = pytest.mark.integration
ORIGIN = {"system": "tdd-agent", "run_id": "run-c1", "snapshot": "snap-1", "snapshot_hash": "h1"}


def _read(path):
    with open(path, encoding="utf-8") as fh:
        return fh.read()


def test_origin_saved_outside_run_key(cfg, tmp_db, pay_docs):
    o = Orchestrator(tmp_db, cfg)
    st = o.start("PAY", pay_docs, origin=ORIGIN)
    assert st["state"] == "done"
    r = tmp_db.one("SELECT params, run_key FROM run WHERE run_id = ?", (st["runId"],))
    assert json.loads(r["params"])["origin"] == ORIGIN
    again = o.start("PAY", pay_docs, origin={**ORIGIN, "run_id": "run-c2"})
    assert again["deduplicated"] and again["runKey"] == r["run_key"]
    # trace back: version -> source baseline -> run -> origin
    row = tmp_db.one("SELECT json_extract(r.params, '$.origin.snapshot') AS snap FROM tso_spec_version v "
                     "JOIN run r ON r.base_baseline_id = v.base_baseline_id WHERE v.feature = 'PAY' ORDER BY r.created_at DESC")
    assert row["snap"] == "snap-1"


def test_none_mode_front_matter_and_note(cfg, tmp_db, pay_docs):
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["result"]["files"]
    for f in st["result"]["files"]:
        text = _read(f)
        assert "\nllm: none\n" in text and '\nllm_model: ""\n' in text
        assert "chưa qua LLM" in text


def test_llm_mode_front_matter(cfg, tmp_db, fake_llm, pay_docs):
    cfg.llm_steps = ("analysis",)
    fake_llm.script = good
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["state"] == "done", st
    text = _read(st["result"]["files"][0])
    assert "\nllm: fake\n" in text and 'llm_model: "fake:fake-1"' in text and "chưa qua LLM" not in text


def test_run_fails_when_most_groups_fail(cfg, tmp_db, fake_llm, pay_docs):
    fake_llm.script = lambda s, u: LLMTransient("quá tải")
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["state"] == "failed" and "LLM_GROUPS_FAILED" in (st["error"] or "")
    assert "groups failed after 2 retries" in st["error"]
    assert not store_tso.versions(tmp_db, "PAY")


def test_run_ok_with_warning_when_few_groups_fail(cfg, tmp_db, fake_llm, pay_docs):
    def script(system, user):
        if pack_of(user)["subject"]["code"] == "INT-01":
            return LLMTransient("quá tải")
        return good(system, user)
    cfg.llm_steps = ("analysis",)
    fake_llm.script = script
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["state"] == "done", st
    assert [w["group"] for w in st["result"]["warnings"]] == ["INT-01"]
    assert any("still failed after retries" in n for n in st["notes"])
