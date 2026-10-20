"""tsagent MCP tools (TDD v2 section 6.2, B19). Tool names of v1 are kept; get_analysis, list_gaps, sql_select are added.

Each tool: an input schema (JSON Schema), a handler (ctx, args) -> dict|list|str, a write flag. Large results are cut by the server.
"""
import collections
import os
import sqlite3
import threading
import time

from ..config import check_feature
from ..errors import NotFound, TsagentError
from ..ontology import ontology
from ..orchestrator import Orchestrator
from ..orchestrator.review import baseline_spec, gate_report, pending_summary, review
from ..render import build
from ..store import source
from ..store import tso as store_tso
from ..store.db import Database
from ..validate.tso_rules import Ctx, validate

FEATURE = {"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$"}
VERSION = {"type": ["integer", "null"], "minimum": 1}
KINDS = ("TestRequirements", "TestDesign", "TestCases")
LEVEL_V1 = {"block": "block", "blockBaseline": "block_baseline", "warn": "warn"}
TOOLS = {}


def tool(name, description, props, required=(), write=False, review_only=False):
    def deco(fn):
        TOOLS[name] = {"name": name, "description": description, "write": write, "reviewOnly": review_only, "fn": fn,
                       "inputSchema": {"type": "object", "properties": props, "required": list(required), "additionalProperties": False}}
        return fn
    return deco


_CACHE = {}
_CACHE_LOCK = threading.Lock()
CACHE_MAX = 64


def _fingerprint(path):
    """State stamp of the DB + WAL files: every write transaction (also from other processes) changes mtime/size."""
    out = []
    for p in (path, path + "-wal"):
        try:
            st = os.stat(p)
            out += [st.st_mtime_ns, st.st_size]
        except OSError:
            out += [0, 0]
    return tuple(out)


def cached(ctx, kind, key, fn):
    """Cache of heavy reads (whole version load, render, gate) keyed by the DB state stamp (NFR-006)."""
    k = (ctx.cfg.db, kind, key, _fingerprint(ctx.cfg.db))
    with _CACHE_LOCK:
        if k in _CACHE:
            return _CACHE[k]
    val = fn()
    with _CACHE_LOCK:
        if len(_CACHE) >= CACHE_MAX:
            _CACHE.pop(next(iter(_CACHE)))
        _CACHE[k] = val
    return val


def _spec(ctx, feature, version=None):
    check_feature(feature)
    try:
        return cached(ctx, "spec", (feature, version), lambda: store_tso.load_spec(ctx.db, feature, version))
    except NotFound:
        raise
    except Exception as e:
        raise NotFound(f"{feature}: no test specs yet ({e})") from None


def _entity(e):
    return {"code": e.code, "class": e.cls, **e.attrs, "rels": e.rels, "derivation": e.prov.derivation,
            "basedOn": e.prov.based_on, "confidence": e.prov.confidence}


# ------------------------------------------------------------------------------------------------- reads
@tool("list_specs", "Features with test specs, their latest version and status",
      {"limit": {"type": "integer", "minimum": 1, "maximum": 500}, "offset": {"type": "integer", "minimum": 0}})
def list_specs(ctx, a):
    rows = ctx.db.q("SELECT feature, MAX(version) AS version FROM tso_spec_version GROUP BY feature ORDER BY feature")
    out = []
    for r in rows:
        v = store_tso.version_row(ctx.db, r["feature"], r["version"])
        out.append({"feature": r["feature"], "version": v["version"], "status": v["status"], "baseline": v["base_baseline_id"]})
    off, lim = a.get("offset", 0), a.get("limit", 100)
    return {"total": len(out), "items": out[off:off + lim]}


@tool("get_spec", "One md document (TestRequirements, TestDesign, TestCases); read on with offset",
      {"feature": FEATURE, "docKind": {"enum": list(KINDS)}, "version": VERSION, "offset": {"type": "integer", "minimum": 0},
       "max_chars": {"type": "integer", "minimum": 1000, "maximum": 30000}}, ["feature", "docKind"])
def get_spec(ctx, a):
    docs, meta = cached(ctx, "build", (a["feature"], a.get("version")), lambda: build(ctx.db, ctx.cfg, a["feature"], a.get("version")))
    text = docs[a["docKind"]][1]
    off, n = a.get("offset", 0), a.get("max_chars", 20000)
    part = text[off:off + n]
    nxt = off + n if off + n < len(text) else None
    return {"feature": a["feature"], "docKind": a["docKind"], "version": meta["version"], "status": meta["status"],
            "offset": off, "next": nxt, "total": len(text), "text": part}


@tool("get_testcase", "A test case with its steps, assertions, oracles, data, fixtures, stubs, fault injections, coverage, conditions",
      {"feature": FEATURE, "code": {"type": "string"}, "version": VERSION}, ["feature", "code"])
def get_testcase(ctx, a):
    g = _spec(ctx, a["feature"], a.get("version"))
    tc = g.get(a["code"])
    if tc is None or tc.cls != "TestCase":
        raise NotFound(f"no test case {a['code']}")
    kids = lambda cls, rel: [_entity(e) for e in g.of(cls) if e.rel(rel) == [tc.code]]  # noqa: E731
    asserts = kids("Assertion", "testCase")
    for x in asserts:
        o = g.get(x["rels"]["oracle"][0]) if x["rels"].get("oracle") else None
        x["oracle"] = _entity(o) if o else None
    cov = [g.get(c) for c in tc.rel("covers")]
    conds = [g.get(c.rel("realizes")[0]) for c in cov if c is not None and c.rel("realizes")]
    data = [g.get(d) for d in tc.rel("needsData")]
    return {"testCase": _entity(tc), "steps": sorted(kids("TestStep", "testCase"), key=lambda s: s["ordinal"]),
            "assertions": sorted(asserts, key=lambda s: s["ordinal"]),
            "data": [{**_entity(d), "fields": [_entity(f) for f in g.of("DataField") if f.rel("data") == [d.code]]} for d in data if d],
            "fixtures": [_entity(g.get(x)) for x in tc.rel("needsFixture")], "stubs": [_entity(g.get(x)) for x in tc.rel("needsStub")],
            "faultInjections": [_entity(g.get(x)) for x in tc.rel("injects")],
            "coverage": [_entity(c) for c in cov if c], "conditions": [_entity(c) for c in conds if c],
            "lifecycle": g.meta["lifecycle"].get(tc.code)}


@tool("get_analysis", "Layer A: obligations, conditions with oracles, gaps; filter by source subject",
      {"feature": FEATURE, "subject": {"type": ["string", "null"]}, "version": VERSION}, ["feature"])
def get_analysis(ctx, a):
    g = _spec(ctx, a["feature"], a.get("version"))
    subj = a.get("subject")
    out = []
    for ob in g.of("TestObligation"):
        subs = [g.get(s).attrs["tddCode"] for s in ob.rel("subject")]
        if subj and subj not in subs:
            continue
        conds = []
        for c in g.of("TestCondition"):
            if c.rel("obligation") != [ob.code]:
                continue
            d = _entity(c)
            for rel in ("oracle", "faultModel", "partition", "path"):
                t = g.get(c.rel(rel)[0]) if c.rel(rel) else None
                d[rel] = _entity(t) if t else None
            conds.append(d)
        out.append({**_entity(ob), "subjects": subs, "conditions": conds})
    gaps = [_entity(x) for x in g.of("TestGap") if not subj or subj in x.prov.based_on]
    return {"feature": a["feature"], "version": g.meta["version"], "obligations": out, "gaps": gaps}


@tool("list_gaps", "TestGaps with question, owner, review status; the list is cut at limit (total is the full count)",
      {"feature": FEATURE, "open_only": {"type": "boolean"}, "version": VERSION,
       "limit": {"type": "integer", "minimum": 1, "maximum": 500}, "offset": {"type": "integer", "minimum": 0}}, ["feature"])
def list_gaps(ctx, a):
    g = _spec(ctx, a["feature"], a.get("version"))
    out = []
    for x in g.of("TestGap"):
        st = (g.meta["lifecycle"].get(x.code) or {}).get("review_status")
        if a.get("open_only", True) and st in ("approved", "rejected"):
            continue
        out.append({"code": x.code, "missing": x.attrs["missing"][:300], "askedQuestion": x.attrs["askedQuestion"][:300],
                    "owner": x.attrs["owner"], "dueDate": x.attrs.get("dueDate"), "blocks": len(x.rel("blocks")),
                    "blocksObligation": x.rel("blocksObligation"), "reviewStatus": st})
    off, lim = a.get("offset", 0), a.get("limit", 40)
    return {"feature": a["feature"], "version": g.meta["version"], "total": len(out), "offset": off,
            "next": off + lim if off + lim < len(out) else None, "gaps": out[off:off + lim]}


@tool("get_traceability", "Source -> obligation -> condition -> TR -> coverage item -> test case",
      {"feature": FEATURE, "version": VERSION, "compact": {"type": "boolean"}}, ["feature"])
def get_traceability(ctx, a):
    from ..analyze.codes import primary_subject
    g = _spec(ctx, a["feature"], a.get("version"))
    trs = collections.defaultdict(list)
    for tr in g.of("TestRequirement"):
        for c in tr.rel("groups"):
            trs[c].append(tr.code)
    covs = collections.defaultdict(list)
    for cv in g.of("CoverageItem"):
        covs[cv.rel("realizes")[0]].append(cv.code)
    tcs = collections.defaultdict(list)
    for tc in g.of("TestCase"):
        for cv in tc.rel("covers"):
            tcs[cv].append(tc.code)
    rows = []
    for c in g.of("TestCondition"):
        ob = g.get(c.rel("obligation")[0])
        rows.append({"source": primary_subject(g, ob), "obligation": ob.code, "condition": c.code, "requirements": trs[c.code],
                     "testCases": sorted({t for cv in covs[c.code] for t in tcs[cv]}), "basedOn": c.prov.based_on,
                     "conditionKind": c.attrs["conditionKind"], "coverageItems": covs[c.code]})
    if not a.get("compact"):
        return rows
    agg = {}
    for r in rows:  # compact: one row per source subject node, with counts
        x = agg.setdefault(r["source"], {"source": r["source"], "conditions": 0, "requirements": set(), "testCases": 0})
        x["conditions"] += 1
        x["requirements"] |= set(r["requirements"])
        x["testCases"] += len(r["testCases"])
    return [{**x, "requirements": sorted(x["requirements"])} for _, x in sorted(agg.items())]


def _report(ctx, feature, version=None):
    return cached(ctx, "report", (feature, version), lambda: _report_uncached(ctx, feature, version))


def _report_uncached(ctx, feature, version=None):
    d = store_tso.open_draft(ctx.db, feature)
    if d is not None and (version is None or version == d["version"]):
        rep, _, g = gate_report(ctx.db, ctx.cfg, feature)
        return rep, g
    g = _spec(ctx, feature, version)
    pkg = source.load_source(ctx.db, feature, g.meta["baseline"])
    rv = {c: (lc or {}).get("review_status") for c, lc in g.meta["lifecycle"].items()}
    return validate(Ctx(g, pkg, g.meta["baseline"], auto_approve=ctx.cfg.auto_approve, review=rv), "GS"), g


@tool("get_coverage", "Node counts by class and level; gate result", {"feature": FEATURE, "version": VERSION}, ["feature"])
def get_coverage(ctx, a):
    rep, g = _report(ctx, a["feature"], a.get("version"))
    cnt = collections.Counter(e.cls for e in g.entities.values())
    return {"feature": a["feature"], "version": g.meta["version"], "status": g.meta["status"],
            "counts": {"obligations": cnt["TestObligation"], "conditions": cnt["TestCondition"], "oracles": cnt["TestOracle"],
                       "gaps": cnt["TestGap"], "testRequirements": cnt["TestRequirement"], "testDesigns": cnt["TestDesign"],
                       "coverageItems": cnt["CoverageItem"], "testCases": cnt["TestCase"]},
            "byClass": dict(sorted(cnt.items())),
            "testCasesByLevel": dict(sorted(collections.Counter(t.attrs["level"] for t in g.of("TestCase")).items())),
            "gate": {"gate": rep.gate, "passed": rep.passed, "blocking": [r.rule for r in rep.blocking]}}


@tool("get_validation", "Results of rules TSO-001..021 for gate GS (v1-compatible shape)", {"feature": FEATURE, "version": VERSION},
      ["feature"])
def get_validation(ctx, a):
    rep, g = _report(ctx, a["feature"], a.get("version"))
    rules = [{"rule": r.rule, "level": LEVEL_V1[r.level], "title": r.title, "checked": r.checked, "passed": r.ok,
              "failures": [f"{v['code']}: {v['message']}" for v in r.violations[:20]], "violationCount": len(r.violations)}
             for r in rep.results]
    bad = lambda lv: sum(1 for r in rules if not r["passed"] and r["level"] == lv)  # noqa: E731
    return {"feature": a["feature"], "version": g.meta["version"], "gate": rep.gate, "passed": rep.passed,
            "blocking": bad("block"), "baselineBlockers": bad("block_baseline"), "warnings": bad("warn"), "rules": rules}


@tool("get_version_history", "Version history: CRs, nodes added/changed/dropped", {"feature": FEATURE}, ["feature"])
def get_version_history(ctx, a):
    check_feature(a["feature"])
    return [{**h, "changes": h["changedBy"]} for h in store_tso.history(ctx.db, a["feature"])]


@tool("diff_versions", "Node differences between two versions", {"feature": FEATURE, "from": {"type": "integer", "minimum": 1},
                                                          "to": {"type": "integer", "minimum": 1}}, ["feature", "from", "to"])
def diff_versions(ctx, a):
    return store_tso.diff_versions(ctx.db, check_feature(a["feature"]), a["from"], a["to"])


@tool("search", "Find test KG nodes (and source nodes) by code or text",
      {"feature": FEATURE, "query": {"type": "string", "minLength": 2}, "classes": {"type": "array", "items": {"type": "string"}},
       "limit": {"type": "integer", "minimum": 1, "maximum": 200}}, ["feature", "query"])
def search(ctx, a):
    g = _spec(ctx, a["feature"])
    q = a["query"].lower()
    out = []
    for e in g.entities.values():
        if a.get("classes") and e.cls not in a["classes"]:
            continue
        text = " ".join(str(v) for v in e.attrs.values() if isinstance(v, (str, int)))
        if q in e.code.lower() or q in text.lower():
            out.append({"code": e.code, "class": e.cls, "layer": ontology().cls(e.cls).layer, "text": text[:200]})
    pkg = source.load_source(ctx.db, a["feature"], g.meta["baseline"])
    for n in pkg.nodes.values():
        if q in n.code.lower() or q in str(n.base.get("label", "")).lower() or q in str(n.props.get("text", "")).lower():
            out.append({"code": n.code, "class": n.cls, "layer": "source", "text": str(n.base.get("label") or n.props.get("text", ""))[:200]})
    out.sort(key=lambda x: (x["layer"] != "source", x["code"]))
    return {"total": len(out), "items": out[:a.get("limit", 50)]}


@tool("get_neighbors", "Neighbours of a node: test KG nodes by relation, or source nodes by TDD edge",
      {"feature": FEATURE, "code": {"type": "string"}, "depth": {"type": "integer", "minimum": 1, "maximum": 3}}, ["feature", "code"])
def get_neighbors(ctx, a):
    g = _spec(ctx, a["feature"])
    e = g.get(a["code"])
    if e is not None:
        incoming = [{"code": x.code, "class": x.cls, "rel": rel} for x in g.entities.values() for rel, ts in x.rels.items() if e.code in ts]
        return {"code": e.code, "class": e.cls, "out": e.rels, "in": sorted(incoming, key=lambda x: (x["rel"], x["code"])),
                "basedOn": e.prov.based_on}
    return source.neighbors(ctx.db, g.meta["baseline"], a["code"], a.get("depth", 1))


SQL_TIMEOUT_S = 2.0


@tool("sql_select", "SELECT only on v_* views (mode=ro, time and row limits)",
      {"query": {"type": "string", "minLength": 6, "maxLength": 8000}, "limit": {"type": "integer", "minimum": 1, "maximum": 1000}},
      ["query"])
def sql_select(ctx, a):
    con = sqlite3.connect(f"file:{ctx.cfg.db}?mode=ro", uri=True, timeout=1.0, check_same_thread=False)
    try:
        real = {r[0] for r in con.execute("SELECT name FROM sqlite_master WHERE type = 'table'")}
        real |= {"sqlite_master", "sqlite_schema", "sqlite_temp_master", "sqlite_sequence"}

        def auth(action, arg1, arg2, dbname, source_name):
            if action == sqlite3.SQLITE_SELECT or action == sqlite3.SQLITE_FUNCTION:
                return sqlite3.SQLITE_OK
            if action == sqlite3.SQLITE_READ:
                # real tables may only be read through v_* views (source_name is the view name); CTEs and subqueries are not real tables
                if arg1 not in real or (source_name or "").startswith("v_"):
                    return sqlite3.SQLITE_OK
                return sqlite3.SQLITE_DENY
            if action == sqlite3.SQLITE_RECURSIVE:
                return sqlite3.SQLITE_OK
            return sqlite3.SQLITE_DENY
        con.set_authorizer(auth)
        deadline = time.monotonic() + SQL_TIMEOUT_S
        con.set_progress_handler(lambda: 1 if time.monotonic() > deadline else 0, 2000)
        try:
            cur = con.execute(a["query"])
        except sqlite3.DatabaseError as e:
            msg = str(e)
            if "interrupted" in msg:
                raise TsagentError(f"sql_select exceeded {SQL_TIMEOUT_S}s") from None
            raise TsagentError(f"sql_select refused: {msg} (only SELECT on v_* views)") from None
        lim = a.get("limit", 200)
        try:
            rows = cur.fetchmany(lim + 1)
        except sqlite3.OperationalError as e:
            raise TsagentError(f"sql_select exceeded {SQL_TIMEOUT_S}s" if "interrupted" in str(e) else str(e)) from None
        cols = [d[0] for d in cur.description or []]
        return {"columns": cols, "rows": [list(r) for r in rows[:lim]], "truncated": len(rows) > lim}
    finally:
        con.close()


# ------------------------------------------------------------------------------------------------- run
DOC = {"anyOf": [{"type": "string"}, {"type": "object", "properties": {"name": {"type": "string"}, "content": {"type": "string"}},
                                       "required": ["name", "content"], "additionalProperties": False}]}


@tool("start_run", "Run the test-spec pipeline (in the background, returns runId). documents: tdd-kg/v1 JSON and md as {name, content}; "
      "paths: paths inside TSAGENT_SOURCE_ROOTS",
      {"feature": FEATURE, "documents": {"type": "array", "items": DOC, "maxItems": 50},
       "paths": {"type": "array", "items": {"type": "string"}, "maxItems": 50}, "dry_run": {"type": "boolean"},
       "force": {"type": "boolean"}, "from_step": {"type": ["string", "null"]},
       "steps": {"type": "array", "items": {"type": "string"}}}, ["feature"], write=True)
def start_run(ctx, a):
    docs = list(a.get("documents") or []) + list(a.get("paths") or [])
    orch = Orchestrator(ctx.db, ctx.cfg, ctx.client())
    run_id, pkg, dedup = orch.prepare(a["feature"], docs, a.get("dry_run", False), a.get("force", False), a.get("from_step"),
                                      a.get("steps"))
    if dedup:
        return {"runId": run_id, "deduplicated": True, "state": "done"}

    def work():
        db = Database(ctx.cfg.db, busy_timeout_ms=ctx.cfg.busy_timeout_ms)
        try:
            Orchestrator(db, ctx.cfg, ctx.client()).execute(run_id, pkg, from_step=a.get("from_step"))
        except Exception:  # the error status is already recorded in the run table
            pass
        finally:
            db.close()
    t = threading.Thread(target=work, name=f"run-{run_id}", daemon=True)
    ctx.threads.append(t)
    t.start()
    return {"runId": run_id, "deduplicated": False, "state": "running"}


@tool("get_run_status", "Run status and each step", {"runId": {"type": "string"}}, ["runId"])
def get_run_status(ctx, a):
    return Orchestrator(ctx.db, ctx.cfg, ctx.client()).status(a["runId"])


@tool("list_pending_review", "Nodes awaiting review (LLM proposals first), gap-blocked nodes, unreviewed LLM nodes; the list is cut at limit",
      {"feature": FEATURE, "limit": {"type": "integer", "minimum": 1, "maximum": 500}}, ["feature"])
def list_pending_review(ctx, a):
    check_feature(a["feature"])
    return pending_summary(ctx.db, a["feature"], a.get("limit", 50))


FILTERS = {"codes": {"type": "array", "items": {"type": "string"}}, "classes": {"type": "array", "items": {"type": "string"}},
           "derivation": {"enum": ["stated", "rule", "llm", "human"]}, "layer": {"enum": ["A", "B", "C"]}}


@tool("approve_nodes", "Approve Draft nodes (by code or filter)", {"feature": FEATURE, "by": {"type": "string", "minLength": 1},
                                                                    "note": {"type": "string"}, **FILTERS}, ["feature", "by"],
      write=True, review_only=True)
def approve_nodes(ctx, a):
    return review(ctx.db, a["feature"], "approved", a["by"], a.get("note", ""), codes=a.get("codes"), classes=a.get("classes"),
                  derivation=a.get("derivation"), layer=a.get("layer"))


@tool("reject_nodes", "Reject Draft nodes", {"feature": FEATURE, "by": {"type": "string", "minLength": 1}, "note": {"type": "string"},
                                              **FILTERS}, ["feature", "by"], write=True, review_only=True)
def reject_nodes(ctx, a):
    return review(ctx.db, a["feature"], "rejected", a["by"], a.get("note", ""), codes=a.get("codes"), classes=a.get("classes"),
                  derivation=a.get("derivation"), layer=a.get("layer"))


@tool("baseline_spec", "Baseline the Draft version (gate GS, primary TDD Baselined, dropped nodes with a CR)",
      {"feature": FEATURE, "by": {"type": "string", "minLength": 1}, "retired_by": {"type": ["string", "null"]}}, ["feature", "by"],
      write=True, review_only=True)
def baseline_spec_tool(ctx, a):
    return baseline_spec(ctx.db, ctx.cfg, a["feature"], a["by"], a.get("retired_by"))


def tool_list(cfg):
    return [{"name": t["name"], "description": t["description"], "inputSchema": t["inputSchema"]}
            for t in TOOLS.values() if not t["reviewOnly"] or cfg.mcp_review]
