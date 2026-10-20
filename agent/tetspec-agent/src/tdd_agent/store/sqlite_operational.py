"""SQLite ``OperationalStore`` (runs, ledger, approvals, spans, LLM audit, metrics)."""
from __future__ import annotations

import json
from typing import Any

from ..errors import NotFound
from .base import OperationalStore, RunRecord, canonical, now_iso
from .sqlite_db import Database

MIGRATIONS = [
    """
    CREATE TABLE runs(
        run_id TEXT PRIMARY KEY, flow TEXT NOT NULL, status TEXT NOT NULL, state TEXT NOT NULL, request TEXT NOT NULL,
        result TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, trace_id TEXT NOT NULL, parent_run_id TEXT);
    CREATE INDEX runs_flow ON runs(flow, created_at);
    CREATE INDEX runs_status ON runs(status, created_at);
    CREATE TABLE transitions(
        id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT NOT NULL, ts TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX tr_run ON transitions(run_id, id);
    CREATE TABLE tasks(run_id TEXT NOT NULL, task_id TEXT NOT NULL, data TEXT NOT NULL, PRIMARY KEY(run_id, task_id));
    CREATE TABLE approvals(id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, ts TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX ap_run ON approvals(run_id);
    CREATE TABLE receipts(
        id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT NOT NULL, actor_id TEXT NOT NULL, decision_hash TEXT NOT NULL,
        ts TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX rc_key ON receipts(run_id, actor_id, decision_hash);
    CREATE TABLE audit(id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, kind TEXT NOT NULL, ts TEXT NOT NULL, payload TEXT NOT NULL);
    CREATE INDEX au_run ON audit(run_id, kind);
    CREATE TABLE spans(
        span_id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, run_id TEXT, parent_id TEXT, start TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX sp_trace ON spans(trace_id, start);
    CREATE INDEX sp_run ON spans(run_id, start);
    CREATE TABLE llm_calls(id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, ts TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX llm_run ON llm_calls(run_id, id);
    CREATE TABLE metrics(name TEXT NOT NULL, labels TEXT NOT NULL, value REAL NOT NULL, PRIMARY KEY(name, labels))
    """,
]


def _run(r: Any) -> RunRecord:
    return RunRecord(run_id=r["run_id"], flow=r["flow"], status=r["status"], state=r["state"], request=json.loads(r["request"]),
                     result=json.loads(r["result"]), created_at=r["created_at"], updated_at=r["updated_at"],
                     trace_id=r["trace_id"], parent_run_id=r["parent_run_id"])


class SqliteOperationalStore(OperationalStore):
    def __init__(self, path: str):
        self.db = Database(path, MIGRATIONS, "operational")

    # runs -------------------------------------------------------------------
    def create_run(self, run: RunRecord) -> RunRecord:
        with self.db.txn() as c:
            c.execute("INSERT INTO runs(run_id,flow,status,state,request,result,created_at,updated_at,trace_id,parent_run_id) VALUES(?,?,?,?,?,?,?,?,?,?)",
                      (run.run_id, run.flow, run.status, run.state, canonical(run.request), canonical(run.result),
                       run.created_at, run.updated_at, run.trace_id, run.parent_run_id))
        return run

    def update_run(self, run_id: str, **fields: Any) -> RunRecord:
        allowed = {"status", "state", "request", "result", "trace_id", "parent_run_id", "flow"}
        bad = set(fields) - allowed
        if bad:
            raise ValueError(f"unknown run fields {sorted(bad)}")
        with self.db.txn() as c:
            row = c.execute("SELECT * FROM runs WHERE run_id=?", (run_id,)).fetchone()
            if row is None:
                raise NotFound(f"run not found: {run_id}", code="RUN_NOT_FOUND")
            sets, args = ["updated_at=?"], [now_iso()]
            for k, v in fields.items():
                sets.append(f"{k}=?")
                args.append(canonical(v) if k in ("request", "result") else v)
            c.execute(f"UPDATE runs SET {', '.join(sets)} WHERE run_id=?", (*args, run_id))
            return _run(c.execute("SELECT * FROM runs WHERE run_id=?", (run_id,)).fetchone())

    def get_run(self, run_id: str) -> RunRecord:
        with self.db.read() as c:
            row = c.execute("SELECT * FROM runs WHERE run_id=?", (run_id,)).fetchone()
        if row is None:
            raise NotFound(f"run not found: {run_id}", code="RUN_NOT_FOUND")
        return _run(row)

    def list_runs(self, *, flow: str | None = None, status: str | None = None, limit: int = 50, offset: int = 0) -> list[RunRecord]:
        where, args = [], []
        if flow: where.append("flow=?"); args.append(flow)
        if status: where.append("status=?"); args.append(status)
        sql = "SELECT * FROM runs" + (" WHERE " + " AND ".join(where) if where else "") + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
        with self.db.read() as c:
            return [_run(r) for r in c.execute(sql, (*args, limit, offset))]

    def count_runs_since(self, iso_timestamp: str) -> int:
        with self.db.read() as c:
            return c.execute("SELECT COUNT(*) FROM runs WHERE created_at >= ?", (iso_timestamp,)).fetchone()[0]

    # transitions ------------------------------------------------------------
    def add_transition(self, run_id: str, record: dict) -> dict:
        ts = record.get("ts") or now_iso()
        with self.db.txn() as c:
            n = c.execute("SELECT COUNT(*) FROM transitions WHERE run_id=?", (run_id,)).fetchone()[0] + 1
            rec = dict(record, run_id=run_id, ts=ts, n=n)
            c.execute("INSERT INTO transitions(run_id,ts,data) VALUES(?,?,?)", (run_id, ts, canonical(rec)))
        return rec

    def list_transitions(self, run_id: str) -> list[dict]:
        with self.db.read() as c:
            return [json.loads(r["data"]) for r in c.execute("SELECT data FROM transitions WHERE run_id=? ORDER BY id", (run_id,))]

    # ledger -----------------------------------------------------------------
    def get_task(self, run_id: str, task_id: str) -> dict | None:
        with self.db.read() as c:
            r = c.execute("SELECT data FROM tasks WHERE run_id=? AND task_id=?", (run_id, task_id)).fetchone()
        return json.loads(r["data"]) if r else None

    def put_task(self, run_id: str, task_id: str, record: dict) -> None:
        with self.db.txn() as c:
            c.execute("INSERT INTO tasks(run_id,task_id,data) VALUES(?,?,?) ON CONFLICT(run_id,task_id) DO UPDATE SET data=excluded.data",
                      (run_id, task_id, canonical(record)))

    def list_tasks(self, run_id: str) -> list[dict]:
        with self.db.read() as c:
            return [dict(json.loads(r["data"]), task_id=r["task_id"]) for r in c.execute("SELECT task_id, data FROM tasks WHERE run_id=? ORDER BY task_id", (run_id,))]

    # approvals --------------------------------------------------------------
    def add_approval(self, record: dict) -> dict:
        rec = dict(record, ts=record.get("ts") or now_iso())
        with self.db.txn() as c:
            c.execute("INSERT INTO approvals(run_id,ts,data) VALUES(?,?,?)", (rec.get("run_id"), rec["ts"], canonical(rec)))
        return rec

    def list_approvals(self, run_id: str) -> list[dict]:
        with self.db.read() as c:
            return [json.loads(r["data"]) for r in c.execute("SELECT data FROM approvals WHERE run_id=? ORDER BY id", (run_id,))]

    def add_receipt(self, record: dict) -> dict:
        rec = dict(record, ts=record.get("ts") or now_iso())
        with self.db.txn() as c:
            c.execute("INSERT INTO receipts(run_id,actor_id,decision_hash,ts,data) VALUES(?,?,?,?,?)",
                      (rec["run_id"], rec["actor_id"], rec["decision_hash"], rec["ts"], canonical(rec)))
        return rec

    def find_receipt(self, run_id: str, actor_id: str, decision_hash: str) -> dict | None:
        with self.db.read() as c:
            r = c.execute("SELECT data FROM receipts WHERE run_id=? AND actor_id=? AND decision_hash=? ORDER BY id DESC LIMIT 1",
                          (run_id, actor_id, decision_hash)).fetchone()
        return json.loads(r["data"]) if r else None

    def add_audit(self, kind: str, payload: dict, run_id: str | None = None) -> dict:
        ts = now_iso()
        with self.db.txn() as c:
            c.execute("INSERT INTO audit(run_id,kind,ts,payload) VALUES(?,?,?,?)", (run_id, kind, ts, canonical(payload)))
        return {"kind": kind, "run_id": run_id, "payload": payload, "ts": ts}

    def list_audit(self, *, run_id: str | None = None, kind: str | None = None, limit: int = 200) -> list[dict]:
        where, args = [], []
        if run_id: where.append("run_id=?"); args.append(run_id)
        if kind: where.append("kind=?"); args.append(kind)
        sql = "SELECT * FROM (SELECT * FROM audit" + (" WHERE " + " AND ".join(where) if where else "") + " ORDER BY id DESC LIMIT ?) ORDER BY id"
        with self.db.read() as c:
            return [{"kind": r["kind"], "run_id": r["run_id"], "payload": json.loads(r["payload"]), "ts": r["ts"]} for r in c.execute(sql, (*args, limit))]

    # spans / llm calls / metrics ---------------------------------------------
    def add_span(self, span: dict) -> None:
        with self.db.txn() as c:
            c.execute("INSERT OR REPLACE INTO spans(span_id,trace_id,run_id,parent_id,start,data) VALUES(?,?,?,?,?,?)",
                      (span["span_id"], span["trace_id"], span.get("run_id"), span.get("parent_id"), span.get("start", now_iso()), canonical(span)))

    def end_span(self, span_id: str, **fields: Any) -> None:
        with self.db.txn() as c:
            r = c.execute("SELECT data FROM spans WHERE span_id=?", (span_id,)).fetchone()
            if r is None:
                return
            data = json.loads(r["data"]); data.update(fields)
            c.execute("UPDATE spans SET data=? WHERE span_id=?", (canonical(data), span_id))

    def list_spans(self, *, trace_id: str | None = None, run_id: str | None = None) -> list[dict]:
        where, args = [], []
        if trace_id: where.append("trace_id=?"); args.append(trace_id)
        if run_id: where.append("run_id=?"); args.append(run_id)
        sql = "SELECT data FROM spans" + (" WHERE " + " AND ".join(where) if where else "") + " ORDER BY start, span_id"
        with self.db.read() as c:
            return [json.loads(r["data"]) for r in c.execute(sql, args)]

    def add_llm_call(self, record: dict) -> None:
        with self.db.txn() as c:
            c.execute("INSERT INTO llm_calls(run_id,ts,data) VALUES(?,?,?)", (record.get("run_id"), record.get("ts") or now_iso(), canonical(record)))

    def list_llm_calls(self, run_id: str | None = None, limit: int = 1000) -> list[dict]:
        sql = "SELECT * FROM (SELECT * FROM llm_calls" + (" WHERE run_id=?" if run_id else "") + " ORDER BY id DESC LIMIT ?) ORDER BY id"
        args = (run_id, limit) if run_id else (limit,)
        with self.db.read() as c:
            return [json.loads(r["data"]) for r in c.execute(sql, args)]

    def incr_metric(self, name: str, labels: dict[str, str] | None = None, value: float = 1.0) -> None:
        with self.db.txn() as c:
            c.execute("INSERT INTO metrics(name,labels,value) VALUES(?,?,?) ON CONFLICT(name,labels) DO UPDATE SET value=value+excluded.value",
                      (name, canonical(labels or {}), value))

    def list_metrics(self, name: str | None = None) -> list[dict]:
        sql = "SELECT * FROM metrics" + (" WHERE name=?" if name else "") + " ORDER BY name, labels"
        with self.db.read() as c:
            return [{"name": r["name"], "labels": json.loads(r["labels"]), "value": r["value"]} for r in c.execute(sql, (name,) if name else ())]

    def close(self) -> None:
        self.db.close()
