"""Lifecycle, review, baseline (V4, V6, V11; MECH-03). A baseline is a single transaction, including the version hash (issue 4.3 of v1)."""
import hashlib
import json

from ..config import check_feature
from ..errors import ImmutableError, NotFound, TsagentError
from ..ontology import ontology
from .db import now
from .tso import index, open_draft, versions

REVIEW = ("proposed", "approved", "rejected", "needsInfo")


def graph_hash(db, sv_id):
    """Hash recomputed from the real content of every node (the content_hash column is not trusted), to detect edits outside the API (T16)."""
    from .tso import load_spec
    sv = db.one("SELECT feature, version FROM tso_spec_version WHERE id = ?", (sv_id,))
    g = load_spec(db, sv["feature"], sv["version"])
    h = hashlib.sha256()
    for code, e in sorted(g.entities.items()):
        h.update(f"{code}|{e.cls}|{e.content_hash()}\n".encode())
    return h.hexdigest()


def _draft_rows(db, sv_id):
    """Rows created by the Draft (lifecycle Draft or InReview), with filter information."""
    o = ontology()
    rows = []
    for r in db.q("SELECT n.code, n.cls, i.tbl, i.row_id, lc.status, lc.review_status FROM tso_spec_version_item i "
                  "JOIN tso_node n ON n.id = i.node_id JOIN tso_lifecycle lc ON lc.tbl = i.tbl AND lc.row_id = i.row_id "
                  "WHERE i.spec_version_id = ? AND lc.status IN ('Draft', 'InReview') ORDER BY n.code", (sv_id,)):
        meta = db.one(f"SELECT derivation, confidence FROM {r['tbl']} WHERE id = ?", (r["row_id"],))
        extra = {}
        if r["cls"] == "TestObligation":
            extra["origin"] = db.one("SELECT origin FROM tso_test_obligation WHERE id = ?", (r["row_id"],))["origin"]
        rows.append({**dict(r), "derivation": meta["derivation"], "confidence": meta["confidence"],
                     "layer": o.cls(r["cls"]).layer, **extra})
    return rows


def pending(db, feature, derivation=None, classes=None, layer=None, review_status=("proposed",)):
    d = open_draft(db, check_feature(feature))
    if d is None:
        return []
    out = []
    for r in _draft_rows(db, d["id"]):
        if review_status and r["review_status"] not in review_status:
            continue
        if derivation and r["derivation"] not in (derivation if isinstance(derivation, (list, tuple, set)) else (derivation,)):
            continue
        if classes and r["cls"] not in classes:
            continue
        if layer and r["layer"] != layer:
            continue
        out.append(r)
    return out


def set_review(db, feature, status, by, note="", codes=None, classes=None, derivation=None, layer=None, min_confidence=None,
               only_proposed=False):
    """Set reviewStatus on nodes of the open Draft matching the filter. `codes=None` with no filter means every Draft node."""
    if status not in REVIEW:
        raise TsagentError(f"invalid reviewStatus: {status}")
    check_feature(feature)
    with db.tx():
        d = open_draft(db, feature)
        if d is None:
            raise NotFound(f"{feature}: no open Draft version")
        rows = _draft_rows(db, d["id"])
        if codes is not None:
            known = {r["code"] for r in rows}
            miss = [c for c in codes if c not in known]
            if miss:
                raise NotFound(f"no Draft nodes: {', '.join(miss[:10])}")
        n = 0
        for r in rows:
            if codes is not None and r["code"] not in codes:
                continue
            if classes and r["cls"] not in classes:
                continue
            if derivation and r["derivation"] not in (derivation if isinstance(derivation, (list, tuple, set)) else (derivation,)):
                continue
            if layer and r["layer"] != layer:
                continue
            if min_confidence is not None and r["confidence"] < min_confidence:
                continue
            if only_proposed and r["review_status"] != "proposed":
                continue
            db.x("UPDATE tso_lifecycle SET review_status = ?, reviewer = ?, reviewed_at = ?, note = ? WHERE tbl = ? AND row_id = ?",
                 (status, by, now(), note or None, r["tbl"], r["row_id"]))
            n += 1
    return {"version": d["version"], "updated": n, "reviewStatus": status}


def baseline(db, feature, by, retired_by=None):
    """Draft -> Baselined; previous version -> Superseded; replaced nodes -> Superseded; dropped nodes -> Retired (a CR is required)."""
    check_feature(feature)
    with db.tx():
        rows = versions(db, feature)
        d = open_draft(db, feature)
        if d is None:
            raise NotFound(f"{feature}: no Draft version to baseline")
        prev = next((r for r in reversed(rows) if r["version"] < d["version"] and r["status"] == "Baselined"), None)
        cur_idx = index(db, d["id"])
        n = 0
        for r in _draft_rows(db, d["id"]):
            db.x("UPDATE tso_lifecycle SET status = 'Baselined' WHERE tbl = ? AND row_id = ?", (r["tbl"], r["row_id"]))
            n += 1
        retired, superseded = [], []
        if prev:
            prev_idx = index(db, prev["id"])
            gone = sorted(set(prev_idx) - set(cur_idx))
            if gone:
                crs = json.loads(d["changed_by"]) or ([retired_by] if retired_by else [])
                if not crs:
                    raise ImmutableError(f"{feature}: {len(gone)} nodes dropped ({', '.join(gone[:5])}) but the version has no CR; "
                                         "cannot baseline because Retired nodes need retiredBy")
                for c in gone:
                    v = prev_idx[c]
                    db.x("UPDATE tso_lifecycle SET status = 'Retired', retired_by = ? WHERE tbl = ? AND row_id = ?",
                         (crs[0], v["tbl"], v["row_id"]))
                    retired.append(c)
            for c, v in cur_idx.items():
                p = prev_idx.get(c)
                if p and (p["tbl"], p["row_id"]) != (v["tbl"], v["row_id"]):
                    db.x("UPDATE tso_lifecycle SET status = 'Superseded' WHERE tbl = ? AND row_id = ? AND status = 'Baselined'",
                         (p["tbl"], p["row_id"]))
                    superseded.append(c)
            db.x("UPDATE tso_spec_version SET status = 'Superseded' WHERE id = ?", (prev["id"],))
        db.x("UPDATE tso_spec_version SET status = 'Baselined', graph_hash = ?, baselined_at = ?, baselined_by = ? WHERE id = ?",
             (graph_hash(db, d["id"]), now(), by, d["id"]))
    return {"version": d["version"], "baselined": n, "superseded": superseded, "retired": retired}
