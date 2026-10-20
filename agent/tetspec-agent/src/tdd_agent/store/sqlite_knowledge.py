"""SQLite ``KnowledgeStore``.

Elements and edges are stored as versioned rows with a half-open snapshot
interval ``[first_seq, last_seq)``; a row is live at snapshot ``S`` when
``first_seq <= S`` and ``last_seq`` is NULL or ``> S``.  Unchanged rows are
shared by every snapshot that contains them, so a commit stores only the diff.
"""
from __future__ import annotations

import json
from contextlib import contextmanager
from typing import Any, Iterator, Sequence

from .base import (ElementVersion, KnowledgeStore, LiveRow, SnapshotInfo, SourceDoc, canonical, digest)
from .sqlite_db import Database, chunks

MIGRATIONS = [
    """
    CREATE TABLE snapshots(
        seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT UNIQUE NOT NULL, parent_seq INTEGER, hash TEXT NOT NULL,
        graph_version TEXT, graph_meta TEXT NOT NULL, actor TEXT, run_id TEXT, changeset_id TEXT,
        created_at TEXT NOT NULL, meta TEXT NOT NULL, touched TEXT NOT NULL);
    CREATE TABLE elements(
        rowid INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL, first_seq INTEGER NOT NULL, last_seq INTEGER,
        ord INTEGER NOT NULL, class TEXT, content_hash TEXT NOT NULL, data TEXT NOT NULL);
    CREATE INDEX el_id ON elements(id, first_seq);
    CREATE INDEX el_live ON elements(last_seq, class);
    CREATE TABLE edges(
        rowid INTEGER PRIMARY KEY AUTOINCREMENT, first_seq INTEGER NOT NULL, last_seq INTEGER, ord INTEGER NOT NULL,
        src TEXT, rel TEXT, dst TEXT, data TEXT NOT NULL);
    CREATE INDEX ed_src ON edges(src, rel);
    CREATE INDEX ed_dst ON edges(dst, rel);
    CREATE INDEX ed_live ON edges(last_seq);
    CREATE TABLE sources(
        id TEXT NOT NULL, revision TEXT NOT NULL, role TEXT NOT NULL, content_hash TEXT NOT NULL, text TEXT NOT NULL,
        meta TEXT NOT NULL, registered_at TEXT NOT NULL, ord INTEGER NOT NULL, PRIMARY KEY(id, revision));
    CREATE INDEX src_role ON sources(role)
    """,
]

LIVE = "first_seq <= :s AND (last_seq IS NULL OR last_seq > :s)"


def _snapshot_from_row(r: Any, parent_id: str | None) -> SnapshotInfo:
    return SnapshotInfo(
        id=r["id"], seq=r["seq"], parent=parent_id, hash=r["hash"], graph_version=r["graph_version"],
        actor=r["actor"], run_id=r["run_id"], changeset_id=r["changeset_id"], created_at=r["created_at"],
        graph_meta=json.loads(r["graph_meta"]), meta=json.loads(r["meta"]), touched=tuple(json.loads(r["touched"])))


FTS_VERSION = 1   # version of the index layout, kept in schema_version under "knowledge_fts"
FTS_TABLES = (
    "CREATE VIRTUAL TABLE IF NOT EXISTS element_fts USING fts5(id, name, body, content='', tokenize='unicode61 remove_diacritics 2')",
    "CREATE VIRTUAL TABLE IF NOT EXISTS element_tri USING fts5(id, content='', tokenize='trigram')",
)


class SqliteKnowledgeStore(KnowledgeStore):
    def __init__(self, path: str):
        self.db = Database(path, MIGRATIONS, "knowledge")
        self.supports_search = self._fts_available()
        if self.supports_search:
            self._ensure_index()

    # --- full-text index: derived data, rebuilt from the element rows, never a source of truth -------
    def _fts_available(self) -> bool:
        options = {r[0] for r in self.db.conn.execute("PRAGMA compile_options")}
        return "ENABLE_FTS5" in options

    def _ensure_index(self) -> None:
        """Create the index when the database has none (a database made before it existed) and index every element row it does not hold yet."""
        with self.db.txn() as c:
            for statement in FTS_TABLES:
                c.execute(statement)
            missing = [r["rowid"] for r in c.execute("SELECT rowid FROM elements WHERE rowid NOT IN (SELECT rowid FROM element_fts)")]
            if missing:
                self._index_rows(c, missing)
            c.execute("INSERT INTO schema_version(name, version) VALUES('knowledge_fts', ?) ON CONFLICT(name) DO UPDATE SET version=excluded.version", (FTS_VERSION,))

    def _index_rows(self, c, rowids: Sequence[int]) -> None:
        from .search import element_text
        for part in chunks(list(rowids)):
            for r in c.execute(f"SELECT rowid, data FROM elements WHERE rowid IN ({','.join('?' * len(part))})", tuple(part)).fetchall():
                el = json.loads(r["data"])
                eid, name, body = element_text(el)
                c.execute("INSERT INTO element_fts(rowid, id, name, body) VALUES(?,?,?,?)", (r["rowid"], eid, name, body))
                c.execute("INSERT INTO element_tri(rowid, id) VALUES(?,?)", (r["rowid"], eid))

    def reindex(self) -> dict:
        """Drop the index and build it again from the element rows (the knowledge graph itself is untouched: no snapshot or hash changes)."""
        from .search import require_search
        require_search(self)
        with self.db.txn() as c:
            c.execute("DROP TABLE IF EXISTS element_fts")
            c.execute("DROP TABLE IF EXISTS element_tri")
            for statement in FTS_TABLES:
                c.execute(statement)
            self._index_rows(c, [r["rowid"] for r in c.execute("SELECT rowid FROM elements")])
        return self.index_check()

    def index_check(self) -> dict:
        """Does the index hold exactly the element rows?  (``missing``: rows without an entry; ``extra``: entries without a row.)"""
        from .search import require_search
        require_search(self)
        c = self.db.conn
        rows = {r[0] for r in c.execute("SELECT rowid FROM elements")}
        indexed = {r[0] for r in c.execute("SELECT rowid FROM element_fts")}
        tri = {r[0] for r in c.execute("SELECT rowid FROM element_tri")}
        return {"rows": len(rows), "indexed": len(indexed), "missing": sorted(rows - indexed)[:20], "extra": sorted(indexed - rows)[:20], "trigram_matches": tri == indexed,
                "ok": rows == indexed and tri == indexed}

    def _search(self, seq: int, query: str, tokens: list[str], classes: tuple[str, ...], limit: int) -> list:
        from .search import Candidate, fold
        c = self.db.conn
        live = "e.first_seq <= ? AND (e.last_seq IS NULL OR e.last_seq > ?)"
        by_class = f" AND e.class IN ({','.join('?' * len(classes))})" if classes else ""
        found: dict[str, Candidate] = {}
        compact = "".join(fold(query).split())
        if len(compact) >= 3:   # an id fragment ("AST-13") by trigram; the query is quoted whole, so FTS syntax in it is just text
            quoted = '"' + compact.replace('"', '""') + '"'
            sql = (f"SELECT e.data FROM element_tri JOIN elements e ON e.rowid = element_tri.rowid WHERE element_tri MATCH ? AND {live}{by_class} ORDER BY e.id LIMIT {int(limit)}")
            for r in c.execute(sql, (quoted, seq, seq, *classes)).fetchall():
                el = json.loads(r["data"])
                found[el["id"]] = Candidate(el, 100.0, ("id",))
        if tokens:
            parts = [f'"{t}"' for t in tokens]   # tokens are letters and digits only: nothing of the query can be read as FTS syntax
            if len(tokens[-1]) >= 2:
                parts[-1] += "*"
            # any word, ranked by bm25: an element that has every word scores above one that has some, without losing the one whose wording differs a little
            sql = ("SELECT e.data AS data, -bm25(element_fts, 10.0, 6.0, 1.0) AS score FROM element_fts JOIN elements e ON e.rowid = element_fts.rowid "
                   f"WHERE element_fts MATCH ? AND {live}{by_class} ORDER BY score DESC, e.id LIMIT {int(limit)}")
            for r in c.execute(sql, (" OR ".join(parts), seq, seq, *classes)).fetchall():
                el = json.loads(r["data"])
                found.setdefault(el["id"], Candidate(el, float(r["score"]), ("text",)))
        return sorted(found.values(), key=lambda x: (-x.score, x.element["id"]))[:limit]

    # --- context managers -----------------------------------------------------
    @contextmanager
    def _txn(self) -> Iterator[None]:
        with self.db.txn():
            yield

    @contextmanager
    def _read(self) -> Iterator[None]:
        with self.db.read():
            yield

    # --- snapshots ------------------------------------------------------------
    def _snapshot(self, ref: str | int | None) -> SnapshotInfo | None:
        c = self.db.conn
        if ref is None:
            row = c.execute("SELECT * FROM snapshots ORDER BY seq DESC LIMIT 1").fetchone()
        elif isinstance(ref, int):
            row = c.execute("SELECT * FROM snapshots WHERE seq=?", (ref,)).fetchone()
        else:
            row = c.execute("SELECT * FROM snapshots WHERE id=?", (ref,)).fetchone()
        if row is None:
            return None
        parent = None
        if row["parent_seq"] is not None:
            p = c.execute("SELECT id FROM snapshots WHERE seq=?", (row["parent_seq"],)).fetchone()
            parent = p["id"] if p else None
        return _snapshot_from_row(row, parent)

    def _all_snapshots(self) -> list[SnapshotInfo]:
        rows = self.db.conn.execute("SELECT * FROM snapshots ORDER BY seq").fetchall()
        ids = {r["seq"]: r["id"] for r in rows}
        return [_snapshot_from_row(r, ids.get(r["parent_seq"])) for r in rows]

    def _write_snapshot(self, info: SnapshotInfo, close_elements: list[Any], open_elements: list[tuple[dict, int]],
                        close_edges: list[Any], open_edges: list[tuple[dict, int]]) -> None:
        c = self.db.conn
        parent_seq = info.seq - 1 if info.parent else None
        c.execute(
            "INSERT INTO snapshots(seq,id,parent_seq,hash,graph_version,graph_meta,actor,run_id,changeset_id,created_at,meta,touched) "
            "VALUES(?,?,?,?,?,?,?,?,?,?,?,?)",
            (info.seq, info.id, parent_seq, info.hash, info.graph_version, canonical(info.graph_meta), info.actor, info.run_id,
             info.changeset_id, info.created_at, canonical(info.meta), json.dumps(list(info.touched))))
        for part in chunks(close_elements):
            c.execute(f"UPDATE elements SET last_seq=? WHERE rowid IN ({','.join('?' * len(part))})", (info.seq, *part))
        for part in chunks(close_edges):
            c.execute(f"UPDATE edges SET last_seq=? WHERE rowid IN ({','.join('?' * len(part))})", (info.seq, *part))
        new_rows = []
        for el, o in open_elements:
            cur = c.execute("INSERT INTO elements(id,first_seq,last_seq,ord,class,content_hash,data) VALUES(?,?,NULL,?,?,?,?)",
                            (el["id"], info.seq, o, el.get("class"), digest(el), canonical(el)))
            new_rows.append(cur.lastrowid)
        if self.supports_search and new_rows:
            self._index_rows(c, new_rows)   # same transaction: a rejected commit leaves nothing indexed
        c.executemany(
            "INSERT INTO edges(first_seq,last_seq,ord,src,rel,dst,data) VALUES(?,NULL,?,?,?,?,?)",
            [(info.seq, o, e.get("src"), e.get("rel"), e.get("dst"), canonical(e)) for e, o in open_edges])

    # --- reads ----------------------------------------------------------------
    def _live_elements(self, seq: int) -> list[LiveRow]:
        rows = self.db.conn.execute(f"SELECT rowid, ord, data FROM elements WHERE {LIVE} ORDER BY ord", {"s": seq}).fetchall()
        return [LiveRow(r["rowid"], r["ord"], json.loads(r["data"])) for r in rows]

    def _live_edges(self, seq: int) -> list[LiveRow]:
        rows = self.db.conn.execute(f"SELECT rowid, ord, data FROM edges WHERE {LIVE} ORDER BY ord", {"s": seq}).fetchall()
        return [LiveRow(r["rowid"], r["ord"], json.loads(r["data"])) for r in rows]

    def _element_row(self, seq: int, element_id: str) -> dict | None:
        r = self.db.conn.execute(f"SELECT data FROM elements WHERE id=:i AND {LIVE}", {"i": element_id, "s": seq}).fetchone()
        return json.loads(r["data"]) if r else None

    def _elements_by_class(self, seq: int, cls: str) -> list[dict]:
        rows = self.db.conn.execute(f"SELECT data FROM elements WHERE class=:c AND {LIVE} ORDER BY ord", {"c": cls, "s": seq}).fetchall()
        return [json.loads(r["data"]) for r in rows]

    def _elements_by_ids(self, seq: int, ids: Sequence[str]) -> list[dict]:
        found: dict[str, dict] = {}
        order = {i: n for n, i in enumerate(ids)}
        for part in chunks(list(ids)):
            q = f"SELECT id, data FROM elements WHERE id IN ({','.join('?' * len(part))}) AND first_seq <= ? AND (last_seq IS NULL OR last_seq > ?)"
            for r in self.db.conn.execute(q, (*part, seq, seq)):
                found[r["id"]] = json.loads(r["data"])
        return [found[i] for i in sorted(found, key=order.get)]

    def _edges_touching(self, seq: int, ids: Sequence[str], relations: Sequence[str] | None, direction: str) -> list[dict]:
        out: dict[int, tuple[int, dict]] = {}
        rel_sql = ""
        rel_args: tuple = ()
        if relations is not None:
            if not relations:
                return []
            rel_sql = f" AND rel IN ({','.join('?' * len(relations))})"
            rel_args = tuple(relations)
        sides = {"out": ("src",), "in": ("dst",), "both": ("src", "dst")}[direction]
        for side in sides:
            for part in chunks(list(ids)):
                q = (f"SELECT rowid, ord, data FROM edges WHERE {side} IN ({','.join('?' * len(part))})"
                     f" AND first_seq <= ? AND (last_seq IS NULL OR last_seq > ?){rel_sql}")
                for r in self.db.conn.execute(q, (*part, seq, seq, *rel_args)):
                    out[r["rowid"]] = (r["ord"], json.loads(r["data"]))
        return [d for _, d in sorted(out.values(), key=lambda t: t[0])]

    def _element_versions(self, element_id: str) -> list[ElementVersion]:
        c = self.db.conn
        names = {r["seq"]: r["id"] for r in c.execute("SELECT seq, id FROM snapshots")}
        rows = c.execute("SELECT first_seq, last_seq, content_hash, data FROM elements WHERE id=? ORDER BY first_seq", (element_id,)).fetchall()
        return [ElementVersion(element_id, names[r["first_seq"]], names.get(r["last_seq"]), r["content_hash"], json.loads(r["data"])) for r in rows]

    # --- sources --------------------------------------------------------------
    def _put_source(self, doc: SourceDoc) -> SourceDoc:
        from .base import now_iso
        c = self.db.conn
        n = c.execute("SELECT COALESCE(MAX(ord), 0) + 1 FROM sources").fetchone()[0]
        stamped = SourceDoc(doc.id, doc.revision, doc.role, doc.text, doc.content_hash, doc.meta, now_iso())
        c.execute("INSERT INTO sources(id,revision,role,content_hash,text,meta,registered_at,ord) VALUES(?,?,?,?,?,?,?,?)",
                  (stamped.id, stamped.revision, stamped.role, stamped.content_hash, stamped.text, canonical(stamped.meta), stamped.registered_at, n))
        return stamped

    @staticmethod
    def _doc(r: Any) -> SourceDoc:
        return SourceDoc(r["id"], r["revision"], r["role"], r["text"], r["content_hash"], json.loads(r["meta"]), r["registered_at"])

    def _get_source(self, id: str, revision: str | None) -> SourceDoc | None:
        c = self.db.conn
        if revision is None:
            r = c.execute("SELECT * FROM sources WHERE id=? ORDER BY ord DESC LIMIT 1", (id,)).fetchone()
        else:
            r = c.execute("SELECT * FROM sources WHERE id=? AND revision=?", (id, revision)).fetchone()
        return self._doc(r) if r else None

    def _list_sources(self, role: str | None) -> list[SourceDoc]:
        c = self.db.conn
        rows = c.execute("SELECT * FROM sources ORDER BY ord").fetchall() if role is None else \
            c.execute("SELECT * FROM sources WHERE role=? ORDER BY ord", (role,)).fetchall()
        return [self._doc(r) for r in rows]

    def close(self) -> None:
        self.db.close()
