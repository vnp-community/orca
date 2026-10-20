"""Performance measurement (T9.2, TST-16, TST-20): rule steps for a 300-requirement feature (NFR-005), p95 of MCP queries on a multi-feature DB
(NFR-006), snapshot RTO (NFR-009), DB/WAL size after 10 Draft rewrites.

python scripts/tsagent/perf.py --features 50 --out /tmp/perf.json
"""
import argparse
import json
import os
import pathlib
import statistics
import sys
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]  # scripts/tsagent/x.py -> repo root
sys.path.insert(0, str(ROOT / "src"))

from tsagent.config import Config  # noqa: E402
from tsagent.gateway.server import GatewayCtx, call_tool  # noqa: E402
from tsagent.orchestrator import Orchestrator  # noqa: E402
from tsagent.store import ops  # noqa: E402
from tsagent.store import tso as store_tso  # noqa: E402
from tsagent.store.db import Database  # noqa: E402


def synth_kg(feature, n_req=300, n_mod=20, n_ep=20):
    """Synthetic tdd-kg/v1 package: n_req FRs (2 ACs each), NFRs, inter-dependent modules, endpoints with fields/responses in a separate md (Vietnamese sample text)."""
    nodes = [{"code": "DOC-TDD", "class": "Document", "status": "Baselined", "primary": True, "props": {"docType": "TDD"}}]
    edges = []
    for m in range(n_mod):
        nodes.append({"code": f"MOD-{m}", "class": "Module", "label": f"Module {m}", "props": {"responsibility": f"việc {m}"}})
        if m:
            edges.append([f"MOD-{m}", "dependsOn", f"MOD-{m - 1}", {}])
    for i in range(n_req):
        fr = f"FR-{i:04d}"
        nodes.append({"code": fr, "class": "FunctionalRequirement", "label": f"Yêu cầu {i}", "props": {"text": f"Hệ thống làm việc {i}", "priority": "Must"}})
        edges.append([fr, "allocatedTo", f"MOD-{i % n_mod}", {}])
        for k in (1, 2):
            ac = f"AC-{fr}-{k}"
            nodes.append({"code": ac, "class": "AcceptanceCriterion", "props": {"statement": f"Tiêu chí {k} của {fr}: trả 200"}})
            edges.append([fr, "hasAcceptance", ac, {}])
    for j in range(n_req // 10):
        nfr = f"NFR-{j:03d}"
        nodes.append({"code": nfr, "class": "NonFunctionalRequirement", "label": f"p95 {j}",
                      "props": {"text": f"p95 thao tác {j} dưới 300 ms khi 100 người dùng đồng thời", "priority": "Should",
                                "targetValue": 300, "targetUnit": "ms"}})
    rows = ["| Mã | Method | Path | implements |", "| --- | --- | --- | --- |"]
    fld = ["| Mã | Của endpoint | name | type | required | max |", "| --- | --- | --- | --- | --- | --- |"]
    rsp = ["| Mã | Phản hồi của | statusCode | condition |", "| --- | --- | --- | --- |"]
    for e in range(n_ep):
        rows.append(f"| EP-{e} | POST | /v1/x{e} | FR-{e:04d} |")
        for f in range(3):
            fld.append(f"| FLD-{e}-{f} | EP-{e} | f{f} | integer | có | 100 |")
        for s, c in ((200, "thành công"), (400, "f0 sai hoặc thiếu"), (401, "thiếu khóa")):
            rsp.append(f"| RSP-{e}-{s} | EP-{e} | {s} | {c} |")
    md = ("---\ndoc_id: SOL-PERF\ndoc_type: solution\nstatus: active\n---\n\n# Solution\n\n## Phụ lục: đặc tả có cấu trúc\n\n"
          + "\n".join(rows) + "\n\n" + "\n".join(fld) + "\n\n" + "\n".join(rsp) + "\n")
    rels = [{"name": "allocatedTo", "domain": ["FunctionalRequirement"], "range": ["Module"]},
            {"name": "hasAcceptance", "domain": ["FunctionalRequirement"], "range": ["AcceptanceCriterion"]},
            {"name": "dependsOn", "domain": ["Module"], "range": ["Module"]}]
    classes = [{"name": c} for c in ("Document", "Module", "FunctionalRequirement", "AcceptanceCriterion", "NonFunctionalRequirement")]
    kg = {"format": "tdd-kg/v1", "ontologyVersion": "tdd-core-1", "feature": feature, "package": {"kind": "full"},
          "ontology": {"classes": classes, "relations": rels}, "nodes": nodes, "edges": edges}
    return kg, md


def p95(xs):
    xs = sorted(xs)
    return xs[max(0, int(round(0.95 * len(xs))) - 1)]


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--features", type=int, default=50)
    ap.add_argument("--requirements", type=int, default=300)
    ap.add_argument("--out", required=True)
    a = ap.parse_args(argv)
    tmp = pathlib.Path(tempfile.mkdtemp(prefix="tsagent-perf-"))
    cfg = Config()
    cfg.db, cfg.specs_root, cfg.cache_dir, cfg.inbox_dir, cfg.snapshot_dir = (str(tmp / x) for x in ("perf.db", "specs", "cache", "inbox", "snap"))
    cfg.llm_provider, cfg.source_roots = "none", (str(tmp),)
    db = Database(cfg.db)
    db.init_schema()
    res = {"features": a.features, "requirements": a.requirements, "steps": []}
    orch = Orchestrator(db, cfg)
    for i in range(a.features):
        f = f"PERF-{i:02d}"
        kg, md = synth_kg(f, a.requirements)
        (tmp / f"{f}.json").write_text(json.dumps(kg), encoding="utf-8")
        (tmp / f"{f}.md").write_text(md, encoding="utf-8")
        t = time.perf_counter()
        st = orch.start(f, [str(tmp / f"{f}.json"), str(tmp / f"{f}.md")], steps=["ingest", "analysis", "design", "specification",
                                                                                   "validate", "commit", "render"])
        dt = time.perf_counter() - t
        if st["state"] != "done":
            raise SystemExit(f"{f}: {st}")
        res["steps"].append(round(dt, 2))
        print(f"{f}: {dt:.1f}s, {st['result'].get('version')}", flush=True)
    res["ruleStepsSeconds"] = {"p50": statistics.median(res["steps"]), "p95": p95(res["steps"]), "max": max(res["steps"])}
    g = store_tso.load_spec(db, "PERF-00")
    res["perFeature"] = {"testCases": sum(1 for _ in g.of("TestCase")), "nodes": len(g.entities)}
    res["rows"] = sum(db.one(f"SELECT COUNT(*) FROM {r['name']}")[0] for r in db.q("SELECT name FROM sqlite_master WHERE type='table'"))
    # p95 of read queries through the MCP tools (in process, HTTP not included)
    ctx = GatewayCtx(cfg)
    tcs = sorted(e.code for e in g.of("TestCase"))[:50]
    calls = [("get_testcase", lambda k: {"feature": f"PERF-{k % a.features:02d}", "code": tcs[k % len(tcs)].replace("PERF-00", f"PERF-{k % a.features:02d}")}),
             ("get_coverage", lambda k: {"feature": f"PERF-{k % a.features:02d}"}),
             ("list_gaps", lambda k: {"feature": f"PERF-{k % a.features:02d}"}),
             ("sql_select", lambda k: {"query": f"SELECT code, objective FROM v_test_case WHERE feature = 'PERF-{k % a.features:02d}' LIMIT 50"}),
             ("get_spec", lambda k: {"feature": f"PERF-{k % a.features:02d}", "docKind": "TestCases", "max_chars": 20000}),
             ("list_specs", lambda k: {})]
    res["mcp"] = {}
    for name, args in calls:
        cold, warm = [], []
        for bucket in (cold, warm):  # pass 1: no cache yet; pass 2: same arguments, DB unchanged
            for k in range(20):
                t = time.perf_counter()
                out = call_tool(ctx, name, args(k))
                bucket.append((time.perf_counter() - t) * 1000)
                if out["isError"]:
                    raise SystemExit(f"{name}: {out['content'][0]['text'][:300]}")
        res["mcp"][name] = {"coldP50ms": round(statistics.median(cold), 1), "coldP95ms": round(p95(cold), 1),
                            "warmP95ms": round(p95(warm), 1)}
    # RTO: snapshot + restore into an empty file + check
    t = time.perf_counter()
    snap = ops.snapshot(db, str(tmp / "snap" / "s.db"))
    t1 = time.perf_counter()
    ops.restore(snap["path"], str(tmp / "restored.db"))
    t2 = time.perf_counter()
    res["snapshot"] = {"snapshotSeconds": round(t1 - t, 2), "restoreCheckSeconds": round(t2 - t1, 2),
                       "dbMB": round(os.path.getsize(cfg.db) / 1e6, 1)}
    # 10 Draft rewrites of the same feature
    sizes = []
    g0 = store_tso.load_spec(db, "PERF-00")
    for k in range(10):
        e = next(g0.of("TestCase"))
        e.attrs["objective"] = f"lần ghi {k}"
        store_tso.commit(db, "", g0, g0.meta["baseline"])
        wal = cfg.db + "-wal"
        sizes.append((round(os.path.getsize(cfg.db) / 1e6, 1), round(os.path.getsize(wal) / 1e6, 1) if os.path.exists(wal) else 0))
    res["draftRewrites"] = sizes
    pathlib.Path(a.out).write_text(json.dumps(res, ensure_ascii=False, indent=1), encoding="utf-8")
    print(json.dumps({k: v for k, v in res.items() if k != "steps"}, ensure_ascii=False, indent=1))


if __name__ == "__main__":
    main()
