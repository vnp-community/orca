"""Source API (`src_*`): store immutable baselines, read them back as a SourcePackage, neighbours by recursive CTE."""
import json

from ..config import check_feature
from ..errors import NotFound
from ..model import OntologyDecl, SourcePackage, SrcNode
from .db import now


def _j(v):
    return json.dumps(v, ensure_ascii=False, sort_keys=True)


def baseline_id_of(pkg):
    return "b-" + pkg.content_hash()[:12]


def list_baselines(db, feature):
    check_feature(feature)
    return [dict(r) for r in db.q("SELECT id, feature, ordinal, format, ontology_version, primary_doc, content_hash, docs, ingested_at "
                                  "FROM src_baseline WHERE feature = ? ORDER BY ordinal", (feature,))]


def latest_baseline(db, feature):
    rows = list_baselines(db, feature)
    return rows[-1]["id"] if rows else None


def save_baseline(db, pkg, derived=()):
    """Store a source package as an immutable baseline. A baseline with the same content is reused (`reused`), but derived nodes
    (`derived`, e.g. the implicit CR) are still written to src_derived_node so they are not lost (issue 4.1 of v1)."""
    check_feature(pkg.feature)
    bid = pkg.baseline_id or baseline_id_of(pkg)
    pkg.baseline_id = bid
    with db.tx():
        row = db.one("SELECT feature FROM src_baseline WHERE id = ?", (bid,))
        reused = row is not None
        if reused and row["feature"] != pkg.feature:
            # same content but another feature: add a suffix so each feature has its own baseline chain
            bid = pkg.baseline_id = f"{bid}-{pkg.feature}"[:80]
            reused = db.one("SELECT 1 FROM src_baseline WHERE id = ?", (bid,)) is not None
        if not reused:
            _insert_baseline(db, pkg, bid)
        added = _insert_derived(db, bid, derived)
    return {"baseline": bid, "reused": reused, "nodes": len(pkg.nodes), "edges": len(pkg.edges), "derived": added}


def _insert_baseline(db, pkg, bid):
    prev = latest_baseline(db, pkg.feature)
    ordinal = (db.one("SELECT COALESCE(MAX(ordinal), 0) FROM src_baseline WHERE feature = ?", (pkg.feature,))[0]) + 1
    db.x("INSERT INTO src_baseline(id, feature, ordinal, format, ontology_version, primary_doc, content_hash, docs, ingested_at) "
         "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
         (bid, pkg.feature, ordinal, pkg.meta.get("format", "tdd-kg/v1"), pkg.meta.get("ontologyVersion", "tdd-core-1+ext-0"),
          pkg.meta.get("primaryDoc"), pkg.content_hash(), _j(pkg.meta.get("docs", [])), now()))
    for name, c in sorted(pkg.ontology.classes.items()):
        db.x("INSERT INTO src_onto_class(baseline_id, name, parent, required, attrs, doc) VALUES (?, ?, ?, ?, ?, ?)",
             (bid, name, c.get("parent"), _j(c.get("required", [])), _j(c.get("attrs", [])), c.get("doc")))
    for name, r in sorted(pkg.ontology.relations.items()):
        db.x("INSERT INTO src_onto_relation(baseline_id, name, domain, range, doc) VALUES (?, ?, ?, ?, ?)",
             (bid, name, _j(r.get("domain", [])), _j(r.get("range", [])), r.get("doc")))
    prev_nodes = {}
    if prev:
        prev_nodes = {r["code"]: (r["id"], r["content_hash"]) for r in db.q("SELECT id, code, content_hash FROM src_node WHERE baseline_id = ?", (prev,))}
    renamed = {s["new"]: s["old"] for s in pkg.supersede}
    ids = {}
    for code, n in sorted(pkg.nodes.items()):
        h = n.content_hash()
        sup = None
        old_code = renamed.get(code, code)
        if old_code in prev_nodes and (old_code != code or prev_nodes[old_code][1] != h):
            sup = prev_nodes[old_code][0]
        cur = db.x("INSERT INTO src_node(baseline_id, code, class, base, props, test_scope, origin, content_hash, supersedes_node_id) "
                   "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
                   (bid, code, n.cls, _j(n.base), _j(n.props), n.base.get("testScope"), n.origin or None, h, sup))
        ids[code] = cur.lastrowid
    for e in sorted(pkg.edges, key=lambda e: (e.src, e.rel, e.dst)):
        db.x("INSERT INTO src_edge(baseline_id, src_id, rel, dst_id, props) VALUES (?, ?, ?, ?, ?)",
             (bid, ids[e.src], e.rel, ids[e.dst], _j(e.props_dict)))
    for s in pkg.supersede:
        db.x("INSERT INTO src_supersede(baseline_id, old_code, new_code) VALUES (?, ?, ?)", (bid, s["old"], s["new"]))


def _insert_derived(db, bid, derived):
    added = 0
    for d in derived:
        node, edges, reason = d["node"], d.get("edges", []), d.get("reason", "")
        if db.one("SELECT 1 FROM src_derived_node WHERE baseline_id = ? AND code = ?", (bid, node.code)):
            continue
        db.x("INSERT INTO src_derived_node(baseline_id, code, class, props, edges, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
             (bid, node.code, node.cls, _j({**node.base, **node.props}), _j(edges), reason, now()))
        added += 1
    return added


def load_source(db, feature, baseline=None, include_derived=True):
    check_feature(feature)
    if baseline is None:
        baseline = latest_baseline(db, feature)
        if baseline is None:
            raise NotFound(f"feature {feature!r} has no source baseline yet")
    b = db.one("SELECT * FROM src_baseline WHERE id = ?", (baseline,))
    if b is None:
        raise NotFound(f"no baseline {baseline}")
    pkg = SourcePackage(b["feature"], baseline_id=baseline)
    pkg.meta = {"format": b["format"], "ontologyVersion": b["ontology_version"], "primaryDoc": b["primary_doc"],
                "docs": json.loads(b["docs"]), "ordinal": b["ordinal"]}
    onto = OntologyDecl()
    for r in db.q("SELECT * FROM src_onto_class WHERE baseline_id = ?", (baseline,)):
        onto.classes[r["name"]] = {"parent": r["parent"], "required": json.loads(r["required"]), "attrs": json.loads(r["attrs"]), "doc": r["doc"]}
    for r in db.q("SELECT * FROM src_onto_relation WHERE baseline_id = ?", (baseline,)):
        onto.relations[r["name"]] = {"domain": json.loads(r["domain"]), "range": json.loads(r["range"]), "doc": r["doc"]}
    pkg.ontology = onto
    code_of = {}
    for r in db.q("SELECT id, code, class, base, props, origin FROM src_node WHERE baseline_id = ?", (baseline,)):
        pkg.nodes[r["code"]] = SrcNode(r["code"], r["class"], json.loads(r["base"]), json.loads(r["props"]), r["origin"] or "")
        code_of[r["id"]] = r["code"]
    for r in db.q("SELECT src_id, rel, dst_id, props FROM src_edge WHERE baseline_id = ?", (baseline,)):
        pkg.add_edge(code_of[r["src_id"]], r["rel"], code_of[r["dst_id"]], json.loads(r["props"]))
    pkg.supersede = [{"old": r["old_code"], "new": r["new_code"]}
                     for r in db.q("SELECT old_code, new_code FROM src_supersede WHERE baseline_id = ?", (baseline,))]
    if include_derived:
        for r in db.q("SELECT code, class, props, edges FROM src_derived_node WHERE baseline_id = ?", (baseline,)):
            props = json.loads(r["props"])
            pkg.nodes[r["code"]] = SrcNode(r["code"], r["class"], {k: v for k, v in props.items() if k == "status"},
                                           {k: v for k, v in props.items() if k != "status"}, "derived")
            for s, rel, d in json.loads(r["edges"]):
                if s in pkg.nodes and d in pkg.nodes:
                    pkg.add_edge(s, rel, d)
    return pkg


def node_ids(db, baseline):
    return {r["code"]: r["id"] for r in db.q("SELECT id, code FROM src_node WHERE baseline_id = ?", (baseline,))}


def neighbors(db, baseline, code, depth=1, direction="both", limit=500):
    """Nodes within `depth` hops (at most 3) of `code`, by recursive CTE over src_edge."""
    depth = max(1, min(int(depth), 3))
    start = db.one("SELECT id FROM src_node WHERE baseline_id = ? AND code = ?", (baseline, code))
    if start is None:
        raise NotFound(f"no node {code} in {baseline}")
    hop = {
        "out": "SELECT e.dst_id, w.d + 1 FROM src_edge e JOIN walk w ON e.src_id = w.id WHERE e.baseline_id = :b AND w.d < :depth",
        "in": "SELECT e.src_id, w.d + 1 FROM src_edge e JOIN walk w ON e.dst_id = w.id WHERE e.baseline_id = :b AND w.d < :depth",
    }
    parts = [hop["out"], hop["in"]] if direction == "both" else [hop[direction]]
    sql = ("WITH RECURSIVE walk(id, d) AS (SELECT :start, 0 UNION " + " UNION ".join(parts) + ") "
           "SELECT DISTINCT n.code, n.class, MIN(w.d) AS d FROM walk w JOIN src_node n ON n.id = w.id GROUP BY n.code, n.class "
           "ORDER BY d, n.code LIMIT :limit")
    nodes = [dict(r) for r in db.q(sql, {"start": start["id"], "b": baseline, "depth": depth, "limit": limit})]
    codes = [n["code"] for n in nodes]
    marks = ",".join("?" * len(codes))
    edges = [dict(r) for r in db.q(
        f"SELECT s.code AS src, e.rel, d.code AS dst FROM src_edge e JOIN src_node s ON s.id = e.src_id JOIN src_node d ON d.id = e.dst_id "
        f"WHERE e.baseline_id = ? AND s.code IN ({marks}) AND d.code IN ({marks}) ORDER BY 1, 2, 3", (baseline, *codes, *codes))]
    return {"nodes": nodes, "edges": edges}


def get_node(db, baseline, code):
    r = db.one("SELECT code, class, base, props, origin, content_hash FROM src_node WHERE baseline_id = ? AND code = ?", (baseline, code))
    if r is None:
        raise NotFound(f"no node {code} in {baseline}")
    return SrcNode(r["code"], r["class"], json.loads(r["base"]), json.loads(r["props"]), r["origin"] or "")
