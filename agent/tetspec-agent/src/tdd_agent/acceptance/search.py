"""Acceptance of the full-text index: known-item queries over the PBOM knowledge graph, index against the in-memory scan.

The queries are written from the SRS sentences (``oracles/search-pbom.json``), never from what ``search`` returns; the oracle only counts once an allowlisted
actor approved its hash, and the report records who approved it and at which grade.
"""
from __future__ import annotations

import json
import statistics
import time
from pathlib import Path

from ..retrieval.candidates import LexicalCandidates
from ..store import open_knowledge_store
from ..store.contract import pbom_graph
from ..store.search import fold
from . import oracles


def _rank(ids: list[str], expected: list[str]) -> int | None:
    for n, i in enumerate(ids, 1):
        if i in expected:
            return n
    return None


def _metrics(ranks: list[int | None]) -> dict:
    n = len(ranks)
    return {"queries": n, "recall@5": round(sum(1 for r in ranks if r and r <= 5) / n, 4), "recall@20": round(sum(1 for r in ranks if r and r <= 20) / n, 4),
            "mrr": round(sum(1 / r for r in ranks if r) / n, 4)}


def run_search_acceptance(config, out: Path) -> dict:
    oracle = oracles.load("search-pbom")
    approval = oracles.approval_for(out, "search-pbom")
    graph = pbom_graph()
    queries = oracle["queries"]
    report: dict = {"oracle": "search-pbom", "oracle_hash": oracles.oracle_hash(oracle), "oracle_approval": approval, "graded": bool(approval), "elements": len(graph["elements"]),
                    "config_hash": config.fingerprint(), "retrieval": config.section("retrieval"), "sqlite_fts5": True, "variants": {}}
    for backend in ("sqlite", "memory"):
        bench = out / "search-bench.db"
        for leftover in out.glob("search-bench.db*"):   # a file of an earlier run would not be empty
            leftover.unlink()
        store = open_knowledge_store(f"sqlite:{bench}" if backend == "sqlite" else "memory:")
        started = time.perf_counter()
        store.commit_graph(None, graph, actor="acceptance", validate=False)
        commit_s = round(time.perf_counter() - started, 3)
        scan = LexicalCandidates(store)
        scan_build = round(time.perf_counter() - started - commit_s, 3)
        for form, shape in (("with accents", lambda q: q), ("without accents", fold)):
            ranks_idx, ranks_scan, t_idx, t_scan = [], [], [], []
            for item in queries:
                q = shape(item["query"])
                t0 = time.perf_counter(); hits = [h.id for h in store.search(q, limit=20)]; t_idx.append(time.perf_counter() - t0)
                t0 = time.perf_counter(); found = [i for i, _ in scan.search(q, k=20)]; t_scan.append(time.perf_counter() - t0)
                ranks_idx.append(_rank(hits, item["expected"])); ranks_scan.append(_rank(found, item["expected"]))
            report["variants"][f"{backend} / {form}"] = {"index": {**_metrics(ranks_idx), "median_ms": round(statistics.median(t_idx) * 1000, 3)},
                                                         "scan": {**_metrics(ranks_scan), "median_ms": round(statistics.median(t_scan) * 1000, 3), "build_s": scan_build},
                                                         "commit_s": commit_s}
        store.close()
        for leftover in out.glob("search-bench.db*"):
            leftover.unlink()
    (out / "search-acceptance.json").write_text(json.dumps(report, indent=2, ensure_ascii=False), encoding="utf-8")
    return report
