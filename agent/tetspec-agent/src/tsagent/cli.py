"""CLI tsagent v2.

  tsagent init
  tsagent gen --feature F --src a.tdd-kg.json solution.md [--from-step design] [--steps ...] [--dry-run] [--force]
  tsagent render|history|diff|validate --feature F
  tsagent review list|approve|reject|needs-info|baseline --feature F [filters]
  tsagent kg migrate|check|stats|snapshot|restore|export
  tsagent ontology codegen|check
  tsagent run status|resume RUN_ID
  tsagent serve [--stdio] [--host H --port P]
  tsagent selfcheck
"""
import argparse
import json
import os
import sys

from . import __version__
from .errors import BusyError, ConfigError, InputError, TsagentError, ValidationFailed

STEP_CHOICES = ("ingest", "analysis", "design", "specification", "validate", "commit", "render")


def _out(obj):
    print(json.dumps(obj, ensure_ascii=False, indent=1, default=str))


def _filters(p):
    p.add_argument("--codes", nargs="*")
    p.add_argument("--classes", nargs="*")
    p.add_argument("--derivation", choices=["stated", "rule", "llm", "human"])
    p.add_argument("--layer", choices=["A", "B", "C"])
    p.add_argument("--min-confidence", type=float)


def build_parser():
    ap = argparse.ArgumentParser(prog="tsagent", description="testspec-agent v2", formatter_class=argparse.RawDescriptionHelpFormatter,
                                 epilog=__doc__)
    ap.add_argument("--version", action="version", version=__version__)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("init", help="create the data folders and the DB")

    g = sub.add_parser("gen", help="run the test-spec pipeline")
    g.add_argument("--feature", required=True)
    g.add_argument("--src", nargs="+", required=True, help="tdd-kg/v1 .json, .md")
    g.add_argument("--from-step", choices=STEP_CHOICES)
    g.add_argument("--steps", nargs="*", choices=STEP_CHOICES)
    g.add_argument("--dry-run", action="store_true")
    g.add_argument("--force", action="store_true", help="create a new run even if a run with the same runKey exists")

    r = sub.add_parser("render", help="render the three md documents")
    r.add_argument("--feature", required=True)
    r.add_argument("--version", type=int)
    r.add_argument("--out", help="output folder (required when rendering an old version); '-' prints to stdout")
    r.add_argument("--force", action="store_true", help="overwrite hand-edited files too")

    h = sub.add_parser("history", help="version history")
    h.add_argument("--feature", required=True)
    d = sub.add_parser("diff", help="differences between two versions")
    d.add_argument("--feature", required=True)
    d.add_argument("a", type=int)
    d.add_argument("b", type=int)
    v = sub.add_parser("validate", help="check the TSO rules for a gate")
    v.add_argument("--feature", required=True)
    v.add_argument("--gate", choices=["GA", "GB", "GS"], default="GS")
    v.add_argument("--version", type=int)

    rv = sub.add_parser("review", help="review and baseline")
    rs = rv.add_subparsers(dest="action", required=True)
    rl = rs.add_parser("list")
    rl.add_argument("--feature", required=True)
    rl.add_argument("--origin")
    _filters(rl)
    for name in ("approve", "reject", "needs-info"):
        p = rs.add_parser(name)
        p.add_argument("--feature", required=True)
        p.add_argument("--by", default=os.environ.get("USER", "cli"))
        p.add_argument("--note", default="")
        _filters(p)
    b = rs.add_parser("baseline")
    b.add_argument("--feature", required=True)
    b.add_argument("--by", default=os.environ.get("USER", "cli"))
    b.add_argument("--retired-by")

    kg = sub.add_parser("kg", help="manage the database")
    ks = kg.add_subparsers(dest="action", required=True)
    ks.add_parser("migrate")
    kc = ks.add_parser("check")
    kc.add_argument("--feature")
    ks.add_parser("stats")
    sn = ks.add_parser("snapshot")
    sn.add_argument("--out")
    rsto = ks.add_parser("restore")
    rsto.add_argument("path")
    rsto.add_argument("--into", required=True, help="new DB file (must not exist)")
    ex = ks.add_parser("export")
    ex.add_argument("--feature", required=True)
    ex.add_argument("--version", type=int)
    ex.add_argument("--out")

    on = sub.add_parser("ontology", help="generate code from and check tso.yaml")
    on.add_argument("action", choices=["codegen", "check"])

    ru = sub.add_parser("run", help="run status, resume a run")
    rus = ru.add_subparsers(dest="action", required=True)
    for name in ("status", "resume"):
        p = rus.add_parser(name)
        p.add_argument("run_id")

    sv = sub.add_parser("serve", help="MCP gateway")
    sv.add_argument("--stdio", action="store_true")
    sv.add_argument("--host", default=os.environ.get("TSAGENT_HOST", "127.0.0.1"))
    sv.add_argument("--port", type=int, default=int(os.environ.get("TSAGENT_PORT", "8766")))
    sub.add_parser("selfcheck", help="check configuration and schema, try the pipeline on sample data (main DB untouched)")
    return ap


def main(argv=None):
    a = build_parser().parse_args(argv)
    from . import log
    log.setup()
    try:
        return dispatch(a) or 0
    except ValidationFailed as e:
        print(f"error: {e}", file=sys.stderr)
        return 3
    except InputError as e:
        print(f"input error: {e}", file=sys.stderr)
        for dg in e.diagnostics[:50]:
            print(f"  {dg}", file=sys.stderr)
        return 2
    except BusyError as e:
        print(f"busy: {e}", file=sys.stderr)
        return 4
    except ConfigError as e:
        print(f"configuration error: {e}", file=sys.stderr)
        return 2
    except TsagentError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1


def dispatch(a):
    from .config import Config
    cfg = Config()
    if a.cmd == "ontology":
        from .gateway import write_tool_catalog
        from .ontology import codegen
        if a.action == "codegen":
            for p in codegen.write_all():
                print(p)
            print(write_tool_catalog())
            return 0
        stale = codegen.stale()
        if stale:
            print("generated files differ from tso.yaml, run `make codegen`:\n  " + "\n  ".join(stale), file=sys.stderr)
            return 1
        print("generated files match tso.yaml")
        return 0
    if a.cmd == "serve":
        from .gateway.server import serve_http, serve_stdio
        from .llm import check_llm
        from .store.db import open_db
        check_llm(cfg)  # the gateway has a pipeline tool: refuse to start without an LLM mode
        open_db(cfg).close()
        if a.stdio:
            serve_stdio(cfg)
        else:
            serve_http(cfg, a.host, a.port)
        return 0
    if a.cmd == "kg" and a.action == "restore":
        from .store import ops
        _out(ops.restore(a.path, a.into))
        return 0
    if a.cmd == "selfcheck":
        return selfcheck(cfg)

    from .store.db import open_db
    db = open_db(cfg)
    try:
        return run_cmd(a, cfg, db)
    finally:
        db.close()


def run_cmd(a, cfg, db):
    if a.cmd == "init":
        for d in (cfg.cache_dir, cfg.snapshot_dir, cfg.specs_root, cfg.inbox()):
            os.makedirs(d, exist_ok=True)
        print(f"DB {cfg.db}: schema version {db.schema_version()['version']}", file=sys.stderr)
        return 0
    if a.cmd == "gen":
        from .orchestrator import Orchestrator
        st = Orchestrator(db, cfg).start(a.feature, [os.path.abspath(p) for p in a.src], dry_run=a.dry_run, force=a.force,
                                         from_step=a.from_step, steps=a.steps)
        _print_run(st)
        return 0 if st["state"] == "done" else 3
    if a.cmd == "render":
        from .render import build, render
        if a.out == "-":
            docs, _ = build(db, cfg, a.feature, a.version)
            for _, (name, text) in sorted(docs.items()):
                print(f"<!-- {name} -->\n{text}")
            return 0
        _out(render(db, cfg, a.feature, a.version, out_dir=a.out, force=a.force))
        return 0
    if a.cmd == "history":
        from .store import tso
        _out(tso.history(db, a.feature))
        return 0
    if a.cmd == "diff":
        from .store import tso
        _out(tso.diff_versions(db, a.feature, a.a, a.b))
        return 0
    if a.cmd == "validate":
        return validate_cmd(a, cfg, db)
    if a.cmd == "review":
        return review_cmd(a, cfg, db)
    if a.cmd == "kg":
        from .store import ops
        if a.action == "migrate":
            _out(db.migrate())
        elif a.action == "check":
            probs = ops.check(db, a.feature)
            _out(probs)
            return 1 if probs else 0
        elif a.action == "stats":
            _out(ops.stats(db))
        elif a.action == "snapshot":
            import time
            path = a.out or os.path.join(cfg.snapshot_dir, f"tsagent-{time.strftime('%Y%m%d-%H%M%S')}.db")
            _out(ops.snapshot(db, path))
        elif a.action == "export":
            data = ops.export_json(db, a.feature, a.version)
            text = json.dumps(data, ensure_ascii=False, indent=1, default=str)
            if a.out:
                with open(a.out, "w", encoding="utf-8") as fh:
                    fh.write(text)
                print(a.out)
            else:
                print(text)
        return 0
    if a.cmd == "run":
        from .orchestrator import Orchestrator
        o = Orchestrator(db, cfg)
        st = o.status(a.run_id) if a.action == "status" else o.execute(a.run_id)
        _print_run(st)
        return 0 if st["state"] in ("done", "running") else 3
    return 0


def _print_run(st):
    r = dict(st)
    res = dict(r.get("result") or {})
    if "report" in res:
        res["report"] = {k: res["report"][k] for k in ("gate", "passed", "blocking")}
    r["result"] = res
    _out(r)


def validate_cmd(a, cfg, db):
    from .orchestrator.review import gate_report
    from .store import source
    from .store import tso as store_tso
    from .validate.tso_rules import Ctx, validate
    d = store_tso.open_draft(db, a.feature)
    if d is not None and a.version in (None, d["version"]) and a.gate == "GS":
        rep, _, _ = gate_report(db, cfg, a.feature)
    else:
        g = store_tso.load_spec(db, a.feature, a.version)
        pkg = source.load_source(db, a.feature, g.meta["baseline"])
        rv = {c: (lc or {}).get("review_status") for c, lc in g.meta["lifecycle"].items()}
        rep = validate(Ctx(g, pkg, g.meta["baseline"], auto_approve=cfg.auto_approve, review=rv), a.gate)
    _out(rep.to_dict())
    print(rep.summary(), file=sys.stderr)
    return 0 if rep.passed else 3


def review_cmd(a, cfg, db):
    from .orchestrator import review as rv
    kw = {}
    if a.action != "baseline":
        kw = {k: getattr(a, k) for k in ("classes", "derivation", "layer", "min_confidence") if getattr(a, k, None) is not None}
    if a.action == "list":
        _out(rv.pending_review(db, a.feature, origin=a.origin, **kw))
        return 0
    if a.action == "baseline":
        _out(rv.baseline_spec(db, cfg, a.feature, a.by, a.retired_by))
        return 0
    status = {"approve": "approved", "reject": "rejected", "needs-info": "needsInfo"}[a.action]
    _out(rv.review(db, a.feature, status, a.by, a.note, codes=a.codes, **kw))
    return 0


def selfcheck(cfg):
    """Check configuration and generated files, run the rule pipeline on the PAY sample in a temporary DB."""
    import tempfile

    from .ontology import codegen
    from .orchestrator import Orchestrator
    from .store.db import Database
    problems = []
    if codegen.stale():
        problems.append("generated files differ from tso.yaml")
    # src/tsagent/cli.py -> repo root tdd-agentic-ai -> tests/testspec_agent/fixtures/pay
    sample = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))), "tests", "testspec_agent",
                          "fixtures", "pay")
    with tempfile.TemporaryDirectory(prefix="tsagent-selfcheck-") as tmp:
        c2 = type(cfg)()
        c2.db, c2.specs_root, c2.cache_dir, c2.inbox_dir = (os.path.join(tmp, x) for x in ("db", "specs", "cache", "inbox"))
        c2.llm_provider, c2.source_roots = "none", (os.path.realpath(sample), tmp)
        if os.path.isdir(sample):
            db = Database(c2.db)
            db.init_schema()
            st = Orchestrator(db, c2).start("SELFCHECK", [os.path.join(sample, f) for f in ("urd.md", "srs.md", "tdd.md")])
            db.close()
            if st["state"] != "done":
                problems.append(f"sample pipeline failed: {st.get('error') or st.get('notes')}")
        else:
            problems.append("sample data tests/testspec_agent/fixtures/pay not found (installed without tests)")
    _out({"llm": cfg.llm_provider, "db": cfg.db, "problems": problems})
    return 1 if problems else 0
