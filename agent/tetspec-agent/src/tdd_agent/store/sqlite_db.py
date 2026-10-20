"""Shared SQLite plumbing: connection, transactions, versioned migrations."""
from __future__ import annotations

import sqlite3
import threading
from contextlib import contextmanager
from pathlib import Path
from typing import Iterator, Sequence

from ..errors import ConfigError

BUSY_TIMEOUT_MS = 30_000


class Database:
    """One connection guarded by a re-entrant lock.

    ``txn()`` takes SQLite's write lock up front (``BEGIN IMMEDIATE``) so two
    processes cannot interleave a read-modify-write of the head snapshot.
    ``read()`` is re-entrant and may be used inside ``txn()``.
    """

    def __init__(self, target: str, migrations: Sequence[str], schema_name: str):
        self.target = target
        self._lock = threading.RLock()
        self._depth = 0
        if target != ":memory:":
            Path(target).parent.mkdir(parents=True, exist_ok=True)
        try:
            self.conn = sqlite3.connect(target, isolation_level=None, check_same_thread=False, timeout=BUSY_TIMEOUT_MS / 1000)
        except sqlite3.Error as exc:
            raise ConfigError(f"cannot open sqlite database {target}: {exc}", code="STORE_OPEN_FAILED") from exc
        self.conn.row_factory = sqlite3.Row
        self.conn.execute(f"PRAGMA busy_timeout={BUSY_TIMEOUT_MS}")
        self.conn.execute("PRAGMA foreign_keys=ON")
        if target != ":memory:":
            self.conn.execute("PRAGMA journal_mode=WAL")
            self.conn.execute("PRAGMA synchronous=NORMAL")
        self._migrate(migrations, schema_name)

    def _migrate(self, migrations: Sequence[str], schema_name: str) -> None:
        with self._lock:
            self.conn.execute("CREATE TABLE IF NOT EXISTS schema_version(name TEXT PRIMARY KEY, version INTEGER NOT NULL)")
            row = self.conn.execute("SELECT version FROM schema_version WHERE name=?", (schema_name,)).fetchone()
            current = row["version"] if row else 0
            if current > len(migrations):
                raise ConfigError(f"database schema {schema_name} v{current} is newer than this runtime (v{len(migrations)})",
                                  code="STORE_SCHEMA_NEWER")
            for version in range(current, len(migrations)):
                self.conn.execute("BEGIN IMMEDIATE")
                try:
                    for statement in [s.strip() for s in migrations[version].split(";") if s.strip()]:
                        self.conn.execute(statement)
                    self.conn.execute("INSERT INTO schema_version(name, version) VALUES(?, ?) "
                                      "ON CONFLICT(name) DO UPDATE SET version=excluded.version", (schema_name, version + 1))
                    self.conn.execute("COMMIT")
                except Exception:
                    self.conn.execute("ROLLBACK")
                    raise

    @contextmanager
    def txn(self) -> Iterator[sqlite3.Connection]:
        with self._lock:
            if self._depth:
                self._depth += 1
                try:
                    yield self.conn
                finally:
                    self._depth -= 1
                return
            self.conn.execute("BEGIN IMMEDIATE")
            self._depth = 1
            try:
                yield self.conn
            except BaseException:
                self.conn.execute("ROLLBACK")
                raise
            else:
                self.conn.execute("COMMIT")
            finally:
                self._depth = 0

    @contextmanager
    def read(self) -> Iterator[sqlite3.Connection]:
        with self._lock:
            yield self.conn

    def close(self) -> None:
        with self._lock:
            try:
                self.conn.close()
            except sqlite3.Error:
                pass


def chunks(values: Sequence, size: int = 400) -> Iterator[Sequence]:
    for i in range(0, len(values), size):
        yield values[i:i + size]
