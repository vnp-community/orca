"""Integrity, snapshot, restore, JSON export, statistics (FR-013, FR-016; MECH-08). Every integrity error names its feature."""
import hashlib
import json
import os
import sqlite3

from ..errors import TsagentError
from ..ontology import ontology
from .db import Database
from .lifecycle import graph_hash
from .source import load_source
from .tso import entity_to_payload, load_spec, payload_to_entity, versions


def _features(db):
    return sorted({r[0] for r in db.q("SELECT feature FROM tso_spec_version UNION SELECT feature FROM src_baseline")})


def check(db, feature=None):
    """Return the list of problems {feature, rule, message}. `feature=None` checks every feature."""
    o = ontology()
    problems = []
    feats = [feature] if feature else _features(db)
    fk = db.q("PRAGMA foreign_key_check")
    for r in fk:
        problems.append({"feature": None, "rule": "FK", "message": f"broken foreign key in {r[0]} row {r[1]} -> {r[2]}"})
    shapes = json.load(open(os.path.join(os.path.dirname(os.path.dirname(__file__)), "validate", "shape_rules.json"), encoding="utf-8"))["rules"]
    for f in feats:
        for b in db.q("SELECT id, content_hash FROM src_baseline WHERE feature = ?", (f,)):
            if load_source(db, f, b["id"], include_derived=False).content_hash() != b["content_hash"]:
                problems.append({"feature": f, "rule": "R17", "message": f"source graph {b['id']} changed after loading"})
        for sv in versions(db, f):
            if sv["status"] in ("Baselined", "Superseded") and sv["graph_hash"] and graph_hash(db, sv["id"]) != sv["graph_hash"]:
                problems.append({"feature": f, "rule": "T16", "message": f"version {sv['version']} is baselined but its content changed"})
            for it in db.q("SELECT n.code, n.cls, i.tbl FROM tso_spec_version_item i JOIN tso_node n ON n.id = i.node_id "
                           "WHERE i.spec_version_id = ?", (sv["id"],)):
                if o.cls(it["cls"]).table != it["tbl"]:
                    problems.append({"feature": f, "rule": "T19", "message": f"{it['code']}: class {it['cls']} sits in table {it['tbl']}"})
        latest = versions(db, f)
        if latest:
            g = load_spec(db, f)
            for rule in shapes:
                for e in g.of(rule["class"]):
                    if "relation" in rule and len(e.rel(rule["relation"])) < rule["min"]:
                        problems.append({"feature": f, "rule": "SHAPE", "message": f"{e.code}.{rule['relation']} needs at least {rule['min']}"})
            got = [v["version"] for v in latest]
            if got != list(range(1, len(got) + 1)):
                problems.append({"feature": f, "rule": "T19", "message": f"versions are not contiguous: {got}"})
    for c in o.classes.values():
        for r in db.q(f"SELECT supersedes_id, COUNT(*) AS n FROM {c.table} WHERE supersedes_id IS NOT NULL GROUP BY supersedes_id HAVING n > 1"):
            feat = db.one(f"SELECT n.feature FROM {c.table} t JOIN tso_node n ON n.id = t.node_id WHERE t.id = ?", (r["supersedes_id"],))
            if feature is None or (feat and feat[0] == feature):
                problems.append({"feature": feat[0] if feat else None, "rule": "T19",
                                 "message": f"{c.name}: row {r['supersedes_id']} has {r['n']} successors (supersedes chain is not linear)"})
    for r in db.q("SELECT tbl, row_id FROM tso_lifecycle WHERE status = 'Retired' AND retired_by IS NULL"):
        feat = db.one(f"SELECT n.feature FROM {r['tbl']} t JOIN tso_node n ON n.id = t.node_id WHERE t.id = ?", (r["row_id"],))
        if feature is None or (feat and feat[0] == feature):
            problems.append({"feature": feat[0] if feat else None, "rule": "T20", "message": f"{r['tbl']}#{r['row_id']} Retired without retiredBy"})
    return problems


def _sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def snapshot(db, path):
    os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
    if os.path.exists(path):
        raise TsagentError(f"{path} already exists; the snapshot is not overwritten")
    dest = sqlite3.connect(path)
    try:
        db.conn.backup(dest)
    finally:
        dest.close()
    digest = _sha(path)
    with open(path + ".sha256", "w") as fh:
        fh.write(digest + "\n")
    return {"path": path, "sha256": digest, "bytes": os.path.getsize(path)}


def restore(path, into):
    """Restore into an empty file and check integrity before returning (SCN-06)."""
    try:
        want = open(path + ".sha256").read().strip()
    except OSError:
        raise TsagentError(f"missing {path}.sha256; a snapshot without a hash is not restored") from None
    if _sha(path) != want:
        raise TsagentError("snapshot hash mismatch; restore refused")
    if os.path.exists(into) and os.path.getsize(into) > 0:
        raise TsagentError(f"{into} is not empty; restore into an empty file, then move it")
    src = sqlite3.connect("file:" + os.path.abspath(path) + "?mode=ro", uri=True)
    dest = sqlite3.connect(into)
    try:
        src.backup(dest)
    finally:
        src.close()
        dest.close()
    db = Database(into)
    problems = check(db)
    if problems:
        db.close()
        raise TsagentError("restored, but the integrity check failed: " + "; ".join(p["message"] for p in problems[:3]))
    return db


def export_json(db, feature, version=None):
    g = load_spec(db, feature, version)
    return {"format": "tso-export/v1", "feature": feature, "ontologyHash": ontology().yaml_hash, "meta": g.meta,
            "regression": g.regression, "entities": [entity_to_payload(e) for e in g.entities.values()]}


def graph_from_export(obj):
    from ..model import TsoGraph
    g = TsoGraph(obj["feature"])
    for p in obj["entities"]:
        g.add(payload_to_entity(p))
    g.regression = list(obj.get("regression", []))
    g.meta.update(obj.get("meta", {}))
    return g


def stats(db):
    o = ontology()
    out = {"tables": {}, "features": {}}
    for c in o.classes.values():
        out["tables"][c.table] = db.one(f"SELECT COUNT(*) FROM {c.table}")[0]
    for t in ("src_baseline", "src_node", "src_edge", "tso_node", "tso_spec_version", "run"):
        out["tables"][t] = db.one(f"SELECT COUNT(*) FROM {t}")[0]
    for f in _features(db):
        out["features"][f] = {"versions": [(v["version"], v["status"]) for v in versions(db, f)],
                              "baselines": [r[0] for r in db.q("SELECT id FROM src_baseline WHERE feature = ? ORDER BY ordinal", (f,))]}
    out["bytes"] = os.path.getsize(db.path) if db.path != ":memory:" and os.path.exists(db.path) else 0
    return out
