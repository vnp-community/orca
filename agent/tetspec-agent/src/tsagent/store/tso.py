"""Test KG API: per-run staging, commit as a SpecVersion with per-node versions, read back (V1-V11, TDD v2 section 5.5).

Relations are stored by the stable identity `tso_node` (DEVIATIONS D2); the concrete node version inside a SpecVersion is resolved through
`tso_spec_version_item`.
"""
import json

from ..config import check_feature
from ..errors import ImmutableError, NotFound, TsagentError
from ..model import Evidence, Provenance, TsoEntity, TsoGraph
from ..ontology import fk_column, join_table, ontology
from .db import now


def _j(v):
    return json.dumps(v, ensure_ascii=False, sort_keys=True)


# ------------------------------------------------------------------------------------------- staging
def entity_to_payload(e):
    return {"cls": e.cls, "code": e.code, "attrs": e.attrs, "rels": e.rels,
            "prov": {"derivation": e.prov.derivation, "basedOn": e.prov.based_on,
                     "evidence": [{"sourceCode": x.source_code, "quote": x.quote} for x in e.prov.evidence],
                     "model": e.prov.model, "promptHash": e.prov.prompt_hash, "confidence": e.prov.confidence, "runId": e.prov.run_id}}


def payload_to_entity(p):
    pv = p.get("prov", {})
    prov = Provenance(pv.get("derivation", "rule"), list(pv.get("basedOn", [])),
                      [Evidence(x["sourceCode"], x.get("quote", "")) for x in pv.get("evidence", [])],
                      pv.get("model", ""), pv.get("promptHash", ""), float(pv.get("confidence", 0.7)), pv.get("runId", ""))
    return TsoEntity(p["cls"], p["code"], dict(p.get("attrs", {})), {k: list(v) for k, v in p.get("rels", {}).items()}, prov)


def stage_put(db, run_id, step, entities, replace=True):
    """Write the entities of one step into the run's stage (inside the step transaction opened by the caller)."""
    if replace:
        db.x("DELETE FROM stage_item WHERE run_id = ? AND step = ?", (run_id, step))
    for e in entities:
        db.x("INSERT INTO stage_item(run_id, step, code, class, payload) VALUES (?, ?, ?, ?, ?) "
             "ON CONFLICT(run_id, step, code) DO UPDATE SET class = excluded.class, payload = excluded.payload",
             (run_id, step, e.code, e.cls, _j(entity_to_payload(e))))


def stage_load(db, run_id, feature, steps=None):
    g = TsoGraph(feature)
    sql, params = "SELECT step, payload FROM stage_item WHERE run_id = ? AND step <> '_meta'", [run_id]
    if steps:
        sql += f" AND step IN ({','.join('?' * len(steps))})"
        params += list(steps)
    for r in db.q(sql + " ORDER BY step, code", params):
        g.add(payload_to_entity(json.loads(r["payload"])))
    meta = db.one("SELECT payload FROM stage_item WHERE run_id = ? AND step = '_meta' AND code = '_meta'", (run_id,))
    if meta:
        g.meta.update(json.loads(meta["payload"]))
    return g


def stage_meta_put(db, run_id, meta):
    db.x("INSERT INTO stage_item(run_id, step, code, class, payload) VALUES (?, '_meta', '_meta', '_meta', ?) "
         "ON CONFLICT(run_id, step, code) DO UPDATE SET payload = excluded.payload", (run_id, _j(meta)))


def stage_steps(db, run_id):
    return sorted({r["step"] for r in db.q("SELECT DISTINCT step FROM stage_item WHERE run_id = ? AND step <> '_meta'", (run_id,))})


def stage_clear(db, run_id):
    db.x("DELETE FROM stage_item WHERE run_id = ?", (run_id,))


# ------------------------------------------------------------------------------------------- versions
def versions(db, feature):
    check_feature(feature)
    return [dict(r) for r in db.q("SELECT id, version, status, base_baseline_id, changed_by, graph_hash, created_at, baselined_at, "
                                  "baselined_by FROM tso_spec_version WHERE feature = ? ORDER BY version", (feature,))]


def version_row(db, feature, version=None):
    rows = versions(db, feature)
    if not rows:
        raise NotFound(f"feature {feature!r} has no test specs yet")
    if version is None:
        return rows[-1]
    for r in rows:
        if r["version"] == int(version):
            return r
    raise NotFound(f"feature {feature!r} has no version {version}")


def open_draft(db, feature):
    rows = versions(db, feature)
    return rows[-1] if rows and rows[-1]["status"] == "Draft" else None


def index(db, spec_version_id):
    """code -> {tbl, row_id, node_id, node_version, hash, cls} of every node in a SpecVersion."""
    out = {}
    for r in db.q("SELECT n.code, n.cls, i.tbl, i.row_id, i.node_id FROM tso_spec_version_item i JOIN tso_node n ON n.id = i.node_id "
                  "WHERE i.spec_version_id = ?", (spec_version_id,)):
        row = db.one(f"SELECT node_version, content_hash FROM {r['tbl']} WHERE id = ?", (r["row_id"],))
        out[r["code"]] = {"tbl": r["tbl"], "row_id": r["row_id"], "node_id": r["node_id"], "cls": r["cls"],
                          "node_version": row["node_version"], "hash": row["content_hash"]}
    return out


def _node_id(db, feature, cls, code):
    r = db.one("SELECT id, cls FROM tso_node WHERE feature = ? AND code = ?", (feature, code))
    if r:
        if r["cls"] != cls:
            raise TsagentError(f"code {code} is already used by class {r['cls']}, it cannot be reused for {cls}")
        return r["id"]
    return db.x("INSERT INTO tso_node(feature, cls, code) VALUES (?, ?, ?)", (feature, cls, code)).lastrowid


def _drop_draft_rows(db, sv_id):
    """V3: rewrite a Draft in place - delete rows created by this Draft (lifecycle Draft), keep rows reused from older versions."""
    for r in db.q("SELECT i.tbl, i.row_id FROM tso_spec_version_item i JOIN tso_lifecycle lc ON lc.tbl = i.tbl AND lc.row_id = i.row_id "
                  "WHERE i.spec_version_id = ? AND lc.status = 'Draft'", (sv_id,)):
        used_elsewhere = db.one("SELECT 1 FROM tso_spec_version_item WHERE tbl = ? AND row_id = ? AND spec_version_id <> ?",
                                (r["tbl"], r["row_id"], sv_id))
        if used_elsewhere:
            continue
        db.x("DELETE FROM tso_based_on WHERE tbl = ? AND row_id = ?", (r["tbl"], r["row_id"]))
        db.x("DELETE FROM tso_evidence WHERE tbl = ? AND row_id = ?", (r["tbl"], r["row_id"]))
        db.x("DELETE FROM tso_spec_version_item WHERE spec_version_id = ? AND tbl = ? AND row_id = ?", (sv_id, r["tbl"], r["row_id"]))
        db.x(f"DELETE FROM {r['tbl']} WHERE id = ?", (r["row_id"],))
        db.x("DELETE FROM tso_lifecycle WHERE tbl = ? AND row_id = ?", (r["tbl"], r["row_id"]))
    db.x("DELETE FROM tso_spec_version_item WHERE spec_version_id = ?", (sv_id,))
    db.x("DELETE FROM tso_regression WHERE spec_version_id = ?", (sv_id,))


def _encode(f, v):
    if v is None:
        return None
    if f.type == "json":
        return _j(v)
    if f.type == "bool":
        return 1 if v else 0
    if f.type == "int" and not f.enum:
        return int(v)
    if f.type == "real":
        return float(v)
    return v


def _decode(f, v):
    if v is None:
        return None
    if f.type == "json":
        return json.loads(v)
    if f.type == "bool":
        return bool(v)
    return v


def commit(db, run_id, graph, base_baseline_id, changed_by=(), regression=()):
    """Write `graph` as a Draft SpecVersion (new, or rewriting the open Draft) in one transaction. Returns statistics."""
    o = ontology()
    feature = check_feature(graph.feature)
    with db.tx():
        rows = versions(db, feature)
        latest = rows[-1] if rows else None
        rewrite = latest is not None and latest["status"] == "Draft"
        if rewrite:
            sv_id, K = latest["id"], latest["version"]
            base = rows[-2] if len(rows) > 1 else None
            _drop_draft_rows(db, sv_id)
            # a Draft rewritten in place keeps every CR merged into it (a later pass may only carry the implicit CR of the addition)
            changed_by = sorted(set(json.loads(latest["changed_by"] or "[]")) | set(changed_by))
            db.x("UPDATE tso_spec_version SET base_baseline_id = ?, changed_by = ?, created_at = ? WHERE id = ?",
                 (base_baseline_id, _j(changed_by), now(), sv_id))
        else:
            K = (latest["version"] + 1) if latest else 1
            base = latest
            sv_id = db.x("INSERT INTO tso_spec_version(feature, version, status, base_baseline_id, changed_by, created_at) "
                         "VALUES (?, ?, 'Draft', ?, ?, ?)", (feature, K, base_baseline_id, _j(sorted(changed_by)), now())).lastrowid
        base_idx = index(db, base["id"]) if base else {}
        src_ids = {r["code"]: r["id"] for r in db.q("SELECT id, code FROM src_node WHERE baseline_id = ?", (base_baseline_id,))}
        node_ids = {code: _node_id(db, feature, e.cls, code) for code, e in sorted(graph.entities.items())}
        stats = {"version": K, "rewroteDraft": rewrite, "baseVersion": base["version"] if base else 0,
                 "new": [], "modified": [], "unchanged": 0}
        for code, e in sorted(graph.entities.items()):
            c = o.cls(e.cls)
            h = e.content_hash()
            b = base_idx.get(code)
            if b and b["hash"] == h and b["cls"] == e.cls:
                db.x("INSERT INTO tso_spec_version_item(spec_version_id, node_id, tbl, row_id) VALUES (?, ?, ?, ?)",
                     (sv_id, node_ids[code], b["tbl"], b["row_id"]))
                stats["unchanged"] += 1
                continue
            maxv = db.one(f"SELECT COALESCE(MAX(node_version), 0) FROM {c.table} WHERE node_id = ?", (node_ids[code],))[0]
            nv = maxv + 1
            cols = ["node_id", "node_version", "content_hash", "supersedes_id", "run_id", "derivation", "confidence", "model",
                    "prompt_hash", "created_at"]
            vals = [node_ids[code], nv, h, b["row_id"] if b and b["tbl"] == c.table else None, run_id or e.prov.run_id,
                    e.prov.derivation, max(0.0, min(1.0, float(e.prov.confidence))), e.prov.model or None, e.prov.prompt_hash or None, now()]
            unknown = set(e.attrs) - set(c.fields)
            if unknown:
                raise TsagentError(f"{e.cls} {code}: fields not in the ontology: {sorted(unknown)}")
            for f in c.fields.values():
                cols.append(f'"{f.column}"')
                vals.append(_encode(f, e.attrs.get(f.name)))
            for r in c.relations.values():
                ts = e.rel(r.name)
                missing = [t for t in ts if t not in node_ids]
                if missing:
                    raise TsagentError(f"{e.cls} {code}.{r.name} points to codes not in the version: {missing}")
                if not r.many:
                    if len(ts) > 1:
                        raise TsagentError(f"{e.cls} {code}.{r.name} takes a single target, got {len(ts)}")
                    cols.append(fk_column(r.name))
                    vals.append(node_ids[ts[0]] if ts else None)
            try:
                row_id = db.x(f"INSERT INTO {c.table}({', '.join(cols)}) VALUES ({', '.join('?' * len(cols))})", vals).lastrowid
            except Exception as ex:
                raise TsagentError(f"cannot write {e.cls} {code}: {ex}") from ex
            for r in c.relations.values():
                if r.many:
                    for t in e.rel(r.name):
                        db.x(f"INSERT INTO {join_table(c.name, r.name)}(row_id, node_id) VALUES (?, ?)", (row_id, node_ids[t]))
            for sc in sorted(set(e.prov.based_on)):
                if sc not in src_ids:
                    raise TsagentError(f"{e.cls} {code}: basedOn {sc} is not in baseline {base_baseline_id}")
                db.x("INSERT INTO tso_based_on(tbl, row_id, src_node_id) VALUES (?, ?, ?)", (c.table, row_id, src_ids[sc]))
            for ev in e.prov.evidence:
                if ev.source_code in src_ids:
                    db.x("INSERT INTO tso_evidence(tbl, row_id, src_node_id, quote) VALUES (?, ?, ?, ?)",
                         (c.table, row_id, src_ids[ev.source_code], ev.quote or ""))
            review = "approved" if e.prov.derivation in ("stated", "human") else "proposed"
            db.x("INSERT INTO tso_lifecycle(tbl, row_id, status, review_status) VALUES (?, ?, 'Draft', ?)", (c.table, row_id, review))
            db.x("INSERT INTO tso_spec_version_item(spec_version_id, node_id, tbl, row_id) VALUES (?, ?, ?, ?)",
                 (sv_id, node_ids[code], c.table, row_id))
            stats["modified" if b else "new"].append(code)
        for code in sorted(set(regression)):
            if code in node_ids:
                db.x("INSERT INTO tso_regression(spec_version_id, node_id) VALUES (?, ?)", (sv_id, node_ids[code]))
        stats["retiring"] = sorted(set(base_idx) - set(graph.entities))
        stats["included"] = len(graph.entities)
        if run_id:
            stage_clear(db, run_id)
    return stats


# ------------------------------------------------------------------------------------------- reading
def load_spec(db, feature, version=None):
    o = ontology()
    sv = version_row(db, feature, version)
    g = TsoGraph(feature)
    g.meta.update({"version": sv["version"], "status": sv["status"], "baseline": sv["base_baseline_id"],
                   "changedBy": json.loads(sv["changed_by"]), "lifecycle": {}, "nodeVersions": {}, "rows": {}})
    items = db.q("SELECT n.code, n.cls, i.tbl, i.row_id FROM tso_spec_version_item i JOIN tso_node n ON n.id = i.node_id "
                 "WHERE i.spec_version_id = ? ORDER BY n.code", (sv["id"],))
    by_tbl = {}
    for it in items:
        by_tbl.setdefault(it["tbl"], []).append(it)
    code_of_node = {r["id"]: r["code"] for r in db.q("SELECT id, code FROM tso_node WHERE feature = ?", (feature,))}
    for tbl, its in by_tbl.items():
        c = o.by_table(tbl)
        ids = [it["row_id"] for it in its]
        marks = ",".join("?" * len(ids))
        rows = {r["id"]: r for r in db.q(f"SELECT * FROM {tbl} WHERE id IN ({marks})", ids)}
        joins = {}
        for r in c.relations.values():
            if r.many:
                for jr in db.q(f"SELECT row_id, node_id FROM {join_table(c.name, r.name)} WHERE row_id IN ({marks})", ids):
                    joins.setdefault((jr["row_id"], r.name), []).append(code_of_node[jr["node_id"]])
        based = {}
        for br in db.q(f"SELECT b.row_id, s.code FROM tso_based_on b JOIN src_node s ON s.id = b.src_node_id "
                       f"WHERE b.tbl = ? AND b.row_id IN ({marks})", (tbl, *ids)):
            based.setdefault(br["row_id"], []).append(br["code"])
        evid = {}
        for er in db.q(f"SELECT e.row_id, s.code, e.quote FROM tso_evidence e JOIN src_node s ON s.id = e.src_node_id "
                       f"WHERE e.tbl = ? AND e.row_id IN ({marks}) ORDER BY e.id", (tbl, *ids)):
            evid.setdefault(er["row_id"], []).append(Evidence(er["code"], er["quote"]))
        lcs = {r["row_id"]: dict(r) for r in db.q(f"SELECT * FROM tso_lifecycle WHERE tbl = ? AND row_id IN ({marks})", (tbl, *ids))}
        for it in its:
            row = rows[it["row_id"]]
            attrs = {f.name: _decode(f, row[f.column]) for f in c.fields.values() if row[f.column] is not None}
            rels = {}
            for r in c.relations.values():
                if r.many:
                    v = sorted(joins.get((row["id"], r.name), []))
                else:
                    nid = row[fk_column(r.name)]
                    v = [code_of_node[nid]] if nid is not None else []
                if v:
                    rels[r.name] = v
            prov = Provenance(row["derivation"], sorted(based.get(row["id"], [])), evid.get(row["id"], []), row["model"] or "",
                              row["prompt_hash"] or "", row["confidence"], row["run_id"])
            g.add(TsoEntity(c.name, it["code"], attrs, rels, prov))
            g.meta["lifecycle"][it["code"]] = {k: lcs.get(row["id"], {}).get(k) for k in
                                               ("status", "review_status", "reviewer", "reviewed_at", "note", "retired_by")}
            g.meta["nodeVersions"][it["code"]] = row["node_version"]
            g.meta["rows"][it["code"]] = (tbl, row["id"])
    g.regression = sorted(code_of_node[r["node_id"]] for r in db.q("SELECT node_id FROM tso_regression WHERE spec_version_id = ?", (sv["id"],)))
    return g


def introduced_codes(db, feature, version):
    """Codes with rows newly created in version `version` (reused nodes excluded)."""
    sv = version_row(db, feature, version)
    prev = [r for r in versions(db, feature) if r["version"] < sv["version"]]
    prev_idx = index(db, prev[-1]["id"]) if prev else {}
    cur = index(db, sv["id"])
    return sorted(c for c, v in cur.items() if c not in prev_idx or prev_idx[c]["row_id"] != v["row_id"] or prev_idx[c]["tbl"] != v["tbl"])


def diff_versions(db, feature, a, b):
    ia, ib = index(db, version_row(db, feature, a)["id"]), index(db, version_row(db, feature, b)["id"])
    return {"from": int(a), "to": int(b), "added": sorted(set(ib) - set(ia)), "removed": sorted(set(ia) - set(ib)),
            "modified": sorted(c for c in set(ia) & set(ib) if (ia[c]["tbl"], ia[c]["row_id"]) != (ib[c]["tbl"], ib[c]["row_id"]))}


def history(db, feature):
    out, prev = [], None
    for r in versions(db, feature):
        idx = index(db, r["id"])
        new = mod = 0
        for c, v in idx.items():
            if prev is None or c not in prev:
                new += 1
            elif (prev[c]["tbl"], prev[c]["row_id"]) != (v["tbl"], v["row_id"]):
                mod += 1
        retired = len(set(prev) - set(idx)) if prev is not None else 0
        out.append({"version": r["version"], "status": r["status"], "baseline": r["base_baseline_id"],
                    "changedBy": json.loads(r["changed_by"]), "added": new, "modified": mod, "retired": retired,
                    "nodes": len(idx), "createdAt": r["created_at"], "baselinedAt": r["baselined_at"]})
        prev = idx
    return out


def ensure_mutable_draft(db, feature):
    d = open_draft(db, feature)
    if d is None:
        raise ImmutableError(f"{feature}: no open Draft version")
    return d
