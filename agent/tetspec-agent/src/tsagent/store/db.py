"""SQLite connection, initialisation, migrations, per-step transactions (MECH-02, MECH-15).

The connection runs in autocommit mode (isolation_level=None) and opens explicit transactions with BEGIN IMMEDIATE, so one
pipeline step is exactly one write transaction; the LLM runs outside the transaction, so the write lock is held only while writing results.
"""
import contextlib
import glob
import os
import re
import sqlite3
import time

from ..errors import BusyError, TsagentError
from ..ontology import ontology

HERE = os.path.dirname(os.path.abspath(__file__))
SCHEMA_SQL = os.path.join(HERE, "schema.sql")
VIEWS_SQL = os.path.join(os.path.dirname(HERE), "gateway", "views.sql")
MIGRATIONS_DIR = os.path.join(HERE, "migrations")


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _busy(e):
    return isinstance(e, sqlite3.OperationalError) and ("locked" in str(e) or "busy" in str(e))


class Database:
    def __init__(self, path, readonly=False, busy_timeout_ms=5000):
        self.path = path
        self.readonly = readonly
        if path != ":memory:" and not readonly:
            os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
        if readonly:
            if path == ":memory:" or not os.path.exists(path):
                raise TsagentError(f"cannot open the DB read-only: {path} does not exist")
            uri = "file:" + os.path.abspath(path) + "?mode=ro"
            self.conn = sqlite3.connect(uri, uri=True, isolation_level=None, check_same_thread=False)
        else:
            self.conn = sqlite3.connect(path, isolation_level=None, check_same_thread=False)
        self.conn.row_factory = sqlite3.Row
        self.conn.execute(f"PRAGMA busy_timeout = {int(busy_timeout_ms)}")
        self.conn.execute("PRAGMA foreign_keys = ON")
        if not readonly and path != ":memory:":
            self.conn.execute("PRAGMA journal_mode = WAL")
            self.conn.execute("PRAGMA synchronous = NORMAL")

    def close(self):
        self.conn.close()

    # ------------------------------------------------------------------ queries
    def q(self, sql, params=()):
        try:
            return self.conn.execute(sql, params).fetchall()
        except sqlite3.OperationalError as e:
            if _busy(e):
                raise BusyError(f"the DB is being written by another process ({self.path}); retry later or stop `tsagent serve`") from e
            raise

    def one(self, sql, params=()):
        rows = self.q(sql, params)
        return rows[0] if rows else None

    def x(self, sql, params=()):
        try:
            return self.conn.execute(sql, params)
        except sqlite3.OperationalError as e:
            if _busy(e):
                raise BusyError(f"the DB is being written by another process ({self.path}); retry later or stop `tsagent serve`") from e
            raise

    # ------------------------------------------------------------------ transactions
    @contextlib.contextmanager
    def tx(self):
        """One write transaction. An error inside rolls back and is re-raised."""
        if self.conn.in_transaction:
            # nested transaction: use a savepoint so the inner step can roll back on its own
            name = f"sp_{id(object())}"
            self.conn.execute(f"SAVEPOINT {name}")
            try:
                yield self
            except BaseException:
                self.conn.execute(f"ROLLBACK TO {name}")
                self.conn.execute(f"RELEASE {name}")
                raise
            self.conn.execute(f"RELEASE {name}")
            return
        try:
            self.conn.execute("BEGIN IMMEDIATE")
        except sqlite3.OperationalError as e:
            if _busy(e):
                raise BusyError(f"the DB is being written by another process ({self.path}); retry later or stop `tsagent serve`") from e
            raise
        try:
            yield self
        except BaseException:
            self.conn.execute("ROLLBACK")
            raise
        self.conn.execute("COMMIT")

    @contextlib.contextmanager
    def step_tx(self, run_id, step):
        """Transaction of one pipeline step; records the step status in run_step (also on error, in a separate transaction)."""
        started = now()
        try:
            with self.tx():
                self.x("INSERT INTO run_step(run_id, step, status, started_at) VALUES (?, ?, 'running', ?) "
                       "ON CONFLICT(run_id, step) DO UPDATE SET status = 'running', started_at = excluded.started_at, error = NULL",
                       (run_id, step, started))
                yield self
                self.x("UPDATE run_step SET status = 'done', finished_at = ? WHERE run_id = ? AND step = ?", (now(), run_id, step))
        except BaseException as e:
            with self.tx():
                self.x("INSERT INTO run_step(run_id, step, status, started_at, finished_at, error) VALUES (?, ?, 'failed', ?, ?, ?) "
                       "ON CONFLICT(run_id, step) DO UPDATE SET status = 'failed', finished_at = excluded.finished_at, "
                       "error = excluded.error", (run_id, step, started, now(), f"{type(e).__name__}: {e}"[:2000]))
            raise

    def run_script(self, sql):
        for stmt in split_sql(sql):
            self.x(stmt)

    # ------------------------------------------------------------------ schema
    def schema_version(self):
        try:
            row = self.one("SELECT version, yaml_hash FROM schema_version ORDER BY version DESC LIMIT 1")
        except sqlite3.OperationalError:
            return None
        return dict(row) if row else None

    def init_schema(self):
        """Create the schema on an empty DB; migrate an existing DB if needed. Returns the status."""
        cur = self.schema_version()
        o = ontology()
        if cur is None:
            with self.tx():
                self.run_script(_strip_pragmas(open(SCHEMA_SQL, encoding="utf-8").read()))
                self.run_script(open(VIEWS_SQL, encoding="utf-8").read())
                self.x("INSERT INTO schema_version(version, yaml_hash, applied_at) VALUES (?, ?, ?)", (o.version, o.yaml_hash, now()))
            return {"created": True, "version": o.version}
        if cur["version"] == o.version and cur["yaml_hash"] == o.yaml_hash:
            return {"created": False, "version": o.version}
        return self.migrate()

    def migrate(self, migrations_dir=MIGRATIONS_DIR):
        """Run the migration scripts numbered above the current version, in order; recreate views; check foreign keys after each step."""
        o = ontology()
        cur = self.schema_version()
        if cur is None:
            return self.init_schema()
        applied = []
        for path in sorted(glob.glob(os.path.join(migrations_dir, "[0-9][0-9][0-9][0-9]_*.sql"))):
            num = int(re.match(r"(\d{4})_", os.path.basename(path)).group(1))
            if num <= cur["version"]:
                continue
            with self.tx():
                self.run_script(open(path, encoding="utf-8").read())
                bad = self.q("PRAGMA foreign_key_check")
                if bad:
                    raise TsagentError(f"migration {os.path.basename(path)} breaks foreign keys: {[tuple(r) for r in bad[:3]]}")
                self.x("INSERT INTO schema_version(version, yaml_hash, applied_at) VALUES (?, ?, ?)", (num, o.yaml_hash, now()))
            applied.append(os.path.basename(path))
        last = self.schema_version()
        if last["version"] != o.version:
            raise TsagentError(f"DB schema is at version {last['version']} but tso.yaml is {o.version}; missing migration script "
                               f"in {migrations_dir}")
        with self.tx():
            for (name,) in self.q("SELECT name FROM sqlite_master WHERE type = 'view'"):
                self.x(f'DROP VIEW "{name}"')
            self.run_script(open(VIEWS_SQL, encoding="utf-8").read())
            if last["yaml_hash"] != o.yaml_hash:
                self.x("INSERT INTO schema_version(version, yaml_hash, applied_at) VALUES (?, ?, ?)", (o.version, o.yaml_hash, now()))
        return {"created": False, "version": o.version, "applied": applied}


def split_sql(sql):
    """Split a script into complete statements (keeping BEGIN...END trigger bodies). executescript() commits on its own, so it cannot
    be used inside the transaction of a step."""
    out, buf = [], ""
    for line in sql.splitlines(keepends=True):
        if not buf and (not line.strip() or line.strip().startswith("--")):
            continue
        buf += line
        if sqlite3.complete_statement(buf):
            out.append(buf.strip())
            buf = ""
    if buf.strip():
        out.append(buf.strip())
    return out


def _strip_pragmas(sql):
    # PRAGMA foreign_keys has no effect inside a transaction; the connection already enables it
    return "\n".join(ln for ln in sql.splitlines() if not ln.strip().upper().startswith("PRAGMA"))


def open_db(cfg, readonly=False, init=True):
    db = Database(cfg.db, readonly=readonly, busy_timeout_ms=cfg.busy_timeout_ms)
    if init and not readonly:
        db.init_schema()
    return db
