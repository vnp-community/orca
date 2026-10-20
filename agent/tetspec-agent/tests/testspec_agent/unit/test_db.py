"""T1.5: connection, initialisation, migrations, per-step transactions."""
import os
import sqlite3
import threading

import pytest

from tsagent.errors import BusyError, TsagentError
from tsagent.ontology import ontology
from tsagent.store.db import Database

pytestmark = pytest.mark.unit


def test_init_creates_schema_and_is_idempotent(tmp_path):
    db = Database(str(tmp_path / "a.db"))
    assert db.init_schema()["created"] is True
    assert db.init_schema()["created"] is False
    assert db.schema_version()["yaml_hash"] == ontology().yaml_hash
    assert db.one("PRAGMA journal_mode")[0] == "wal"
    names = {r[0] for r in db.q("select name from sqlite_master where type in ('table','view')")}
    assert {"tso_test_case", "src_node", "v_test_case", "v_trace"} <= names


def test_tx_rolls_back_on_error(tmp_path):
    db = Database(str(tmp_path / "a.db"))
    db.init_schema()
    with pytest.raises(RuntimeError):
        with db.tx():
            db.x("insert into run(run_id, feature, run_key, mode, status, created_at, updated_at) values ('r1','F','k','none','Created','t','t')")
            raise RuntimeError("boom")
    assert db.one("select count(*) from run")[0] == 0


def test_step_tx_records_status_even_on_failure(tmp_path):
    db = Database(str(tmp_path / "a.db"))
    db.init_schema()
    with db.tx():
        db.x("insert into run(run_id, feature, run_key, mode, status, created_at, updated_at) values ('r1','F','k','none','Created','t','t')")
    with db.step_tx("r1", "ingest"):
        db.x("update run set status = 'Ingested' where run_id = 'r1'")
    with pytest.raises(ValueError):
        with db.step_tx("r1", "analysis"):
            db.x("update run set status = 'Analyzed' where run_id = 'r1'")
            raise ValueError("llm hỏng")
    steps = {r["step"]: (r["status"], r["error"]) for r in db.q("select * from run_step")}
    assert steps["ingest"] == ("done", None)
    assert steps["analysis"][0] == "failed" and "llm hỏng" in steps["analysis"][1]
    assert db.one("select status from run")[0] == "Ingested"  # the failed step's changes are rolled back


def test_readers_not_blocked_while_writing(tmp_path):
    path = str(tmp_path / "a.db")
    w = Database(path)
    w.init_schema()
    r = Database(path, readonly=True)
    with w.tx():
        w.x("insert into run(run_id, feature, run_key, mode, status, created_at, updated_at) values ('r1','F','k','none','Created','t','t')")
        assert r.one("select count(*) from run")[0] == 0  # readable while writing (WAL), sees the committed state
    assert r.one("select count(*) from run")[0] == 1
    with pytest.raises(sqlite3.OperationalError):
        r.x("insert into run(run_id, feature, run_key, mode, status, created_at, updated_at) values ('r2','F','k','none','Created','t','t')")


def test_second_writer_gets_busy_error(tmp_path):
    path = str(tmp_path / "a.db")
    a = Database(path)
    a.init_schema()
    b = Database(path, busy_timeout_ms=50)
    started, release = threading.Event(), threading.Event()

    def hold():
        with a.tx():
            started.set()
            release.wait(2)

    t = threading.Thread(target=hold)
    t.start()
    started.wait(2)
    try:
        with pytest.raises(BusyError, match="another process"):
            with b.tx():
                pass
    finally:
        release.set()
        t.join()


def test_migrate_applies_scripts_and_detects_missing(tmp_path, monkeypatch):
    db = Database(str(tmp_path / "a.db"))
    db.init_schema()
    mig = tmp_path / "mig"
    mig.mkdir()
    o = ontology()
    # simulate tso.yaml at version 2 without a script: must report it missing
    monkeypatch.setattr(o, "version", 2)
    with pytest.raises(TsagentError, match="missing migration script"):
        db.migrate(str(mig))
    (mig / "0002_add_note.sql").write_text("ALTER TABLE run ADD COLUMN extra TEXT;")
    res = db.migrate(str(mig))
    assert res["applied"] == ["0002_add_note.sql"] and db.schema_version()["version"] == 2
    assert "extra" in [r[1] for r in db.q("PRAGMA table_info(run)")]


def test_readonly_requires_existing_file(tmp_path):
    with pytest.raises(TsagentError):
        Database(str(tmp_path / "none.db"), readonly=True)
    assert not os.path.exists(tmp_path / "none.db")
