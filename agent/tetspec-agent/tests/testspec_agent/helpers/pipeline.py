"""Run the rule pipeline (no orchestrator) for tests: ingest -> baseline -> A -> B -> C -> commit."""
from tsagent.analyze import analyze
from tsagent.design import run_design_step
from tsagent.ingest import ingest
from tsagent.llm import make_client
from tsagent.specify import run_specify_step
from tsagent.store import source
from tsagent.store import tso as store_tso


def build_graph(cfg, docs, feature, db=None):
    pkg, _ = ingest(docs, feature, cfg)
    bid = source.save_baseline(db, pkg)["baseline"] if db is not None else "b"
    client = make_client(cfg)
    g, _, rep, s = analyze(pkg, bid, client, cfg)
    expected = {e.code for e in g.of("TestCondition", "TestObligation") if e.prov.derivation != "llm" or True}
    run_design_step(g, pkg, bid, client, cfg)
    run_specify_step(g, pkg, bid, client, cfg)
    return pkg, bid, g, expected


def commit_full(cfg, db, docs, feature, changed_by=()):
    pkg, bid, g, _ = build_graph(cfg, docs, feature, db)
    stats = store_tso.commit(db, "", g, bid, changed_by)
    return pkg, bid, g, stats
