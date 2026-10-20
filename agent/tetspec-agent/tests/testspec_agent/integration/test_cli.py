"""T7.4: CLI: init, gen, history, validate, review, render, kg, run, selfcheck; a clear error when the DB is busy."""
import json
import os
import sqlite3

import pytest

from tsagent.cli import main

pytestmark = pytest.mark.integration


@pytest.fixture
def env(cfg, monkeypatch, tmp_path):
    from testspec_agent.conftest import FIX
    for k, v in {"TSAGENT_DB": cfg.db, "TSAGENT_SPECS_ROOT": cfg.specs_root, "TSAGENT_CACHE_DIR": cfg.cache_dir,
                 "TSAGENT_SNAPSHOT_DIR": cfg.snapshot_dir, "TSAGENT_INBOX_DIR": cfg.inbox_dir, "TSAGENT_LLM": "none",
                 "TSAGENT_SOURCE_ROOTS": os.pathsep.join([str(tmp_path), FIX]), "TSAGENT_BUSY_TIMEOUT_MS": "200"}.items():
        monkeypatch.setenv(k, v)
    return cfg


def run(capsys, *args):
    rc = main(list(args))
    out = capsys.readouterr()
    return rc, out.out, out.err


def test_gen_produces_three_documents_and_dedups(env, capsys, minipay_docs):
    rc, out, err = run(capsys, "gen", "--feature", "SCHEDULED-TRANSFER", "--src", minipay_docs["slice"], minipay_docs["solution"])
    st = json.loads(out)
    assert rc == 0 and st["state"] == "done" and len(st["result"]["files"]) == 3
    assert all(os.path.exists(f) for f in st["result"]["files"])
    rc, out, _ = run(capsys, "gen", "--feature", "SCHEDULED-TRANSFER", "--src", minipay_docs["slice"], minipay_docs["solution"])
    assert json.loads(out)["deduplicated"]
    rc, out, _ = run(capsys, "run", "status", st["runId"])
    assert rc == 0 and json.loads(out)["state"] == "done"


def test_review_validate_history_render_kg(env, capsys, pay_docs, tmp_path):
    assert run(capsys, "init")[0] == 0
    assert run(capsys, "gen", "--feature", "PAY", "--src", *pay_docs)[0] == 0
    rc, out, _ = run(capsys, "review", "list", "--feature", "PAY", "--classes", "TestGap")
    assert rc == 0 and all(x["class"] == "TestGap" for x in json.loads(out))
    rc, _, err = run(capsys, "validate", "--feature", "PAY")
    assert rc == 3 and "TSO-007" in err  # an open gap blocks GS
    assert run(capsys, "review", "approve", "--feature", "PAY", "--by", "qa")[0] == 0
    rc, out, _ = run(capsys, "review", "baseline", "--feature", "PAY", "--by", "qa")
    assert rc == 0 and json.loads(out)["version"] == 1
    rc, out, _ = run(capsys, "history", "--feature", "PAY")
    assert json.loads(out)[0]["status"] == "Baselined"
    rc, out, _ = run(capsys, "render", "--feature", "PAY", "--out", "-")
    assert "<!-- 01-test-requirements.md -->" in out and "status: Baselined" in out
    assert run(capsys, "kg", "check")[0] == 0
    rc, out, _ = run(capsys, "kg", "stats")
    assert "PAY" in json.loads(out)["features"]
    snap = str(tmp_path / "s.db")
    assert run(capsys, "kg", "snapshot", "--out", snap)[0] == 0
    rc, out, _ = run(capsys, "kg", "restore", snap, "--into", str(tmp_path / "r.db"))
    assert rc == 0
    exp = str(tmp_path / "e.json")
    assert run(capsys, "kg", "export", "--feature", "PAY", "--out", exp)[0] == 0 and json.load(open(exp))["entities"]
    assert run(capsys, "diff", "--feature", "PAY", "1", "1")[0] == 0


def test_input_error_exit_code(env, capsys, tmp_path):
    p = tmp_path / "bad.json"
    p.write_text("{", encoding="utf-8")
    rc, _, err = run(capsys, "gen", "--feature", "X", "--src", str(p))
    assert rc == 2 and ("I0" in err or "JSON" in err)


def test_busy_db_reports_clearly(env, capsys, pay_docs):
    assert run(capsys, "init")[0] == 0
    con = sqlite3.connect(env.db)
    con.execute("BEGIN EXCLUSIVE")
    try:
        rc, _, err = run(capsys, "gen", "--feature", "PAY", "--src", *pay_docs)
    finally:
        con.rollback()
        con.close()
    assert rc == 4 and "busy" in err


def test_ontology_check_and_selfcheck(env, capsys):
    assert run(capsys, "ontology", "check")[0] == 0
    rc, out, _ = run(capsys, "selfcheck")
    assert rc == 0 and json.loads(out)["problems"] == []
