"""Backend contract suite.

``KNOWLEDGE_CONTRACT`` / ``OPERATIONAL_CONTRACT`` are lists of test functions
taking a zero-argument factory that returns a fresh, empty store.  A backend is
supported only when every function passes.  The suite is backend-agnostic: the
repository's own tests parametrize it over the SQLite and in-memory backends and
a third-party backend can do the same.
"""
from __future__ import annotations

import copy
import json
import threading
from pathlib import Path
from typing import Callable

from ..core import o5_path
from ..errors import ChangeSetRejected, NotFound, SourceConflict, SourceRoleError, VersionConflict
from .base import KnowledgeStore, OperationalStore, RunRecord, SourceDoc, now_iso

KnowledgeFactory = Callable[[], KnowledgeStore]
OperationalFactory = Callable[[], OperationalStore]


def pbom_graph() -> dict:
    """A real, ontology-valid graph (574 elements) produced by an earlier pilot run."""
    root = o5_path.o5_root()
    path = root.parent.parent / "test-pbom" / "run-20261006" / "tdd.initial.kg.json"
    return json.loads(path.read_text(encoding="utf-8"))


def _o5():
    return o5_path.load()


def _op(graph: dict) -> dict:
    return next(e for e in graph["elements"] if e["class"] == "Operation")


def _modify_ops(graph: dict, target: str, path: str, after, version: str = "PBOM-TDD-v2", cs_id: str = "CS-1") -> dict:
    el = next(e for e in graph["elements"] if e["id"] == target)
    container = el
    parts = path.split(".")
    for part in parts[:-1]:
        container = container[part]
    return {"id": cs_id, "base_version": graph["version"], "result_version": version,
            "ops": [{"seq": 1, "op": "modify", "target": target, "path": path, "before": container.get(parts[-1]), "after": after}]}


# --------------------------------------------------------------------- knowledge
def k_import_roundtrip_and_hash_parity(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    res = store.commit_graph(None, graph, actor="t", run_id="r1")
    assert res.snapshot.id == "snap-000001" and res.snapshot.parent is None
    assert res.snapshot.hash == _o5().snapshot_hash(graph)
    out = store.materialize()
    assert out == graph, "materialized graph must equal the committed graph exactly (including order)"
    assert _o5().snapshot_hash(out) == res.snapshot.hash
    assert store.head().id == res.snapshot.id
    assert store.stats()["elements"] == len(graph["elements"])


def k_changeset_new_snapshot_old_unchanged(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    target = _op(graph)["id"]
    cs = _modify_ops(graph, target, "name", "Renamed operation")
    second = store.commit_changeset(first.id, cs, actor="t", run_id="r2")
    assert second.snapshot.parent == first.id and second.snapshot.seq == first.seq + 1
    assert second.modified == (target,) or target in second.modified
    assert store.get_element(target, first.id)["name"] != "Renamed operation"
    assert store.get_element(target)["name"] == "Renamed operation"
    expected, report = _o5().apply_changeset(copy.deepcopy(graph), cs)
    assert report["ok"]
    assert store.materialize() == expected
    assert second.snapshot.hash == _o5().snapshot_hash(expected)
    assert store.materialize(first.id) == graph
    versions = store.history(target)
    assert len(versions) == 2 and versions[0].last_snapshot == second.snapshot.id and versions[1].last_snapshot is None


def k_version_conflict_leaves_graph_untouched(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    target = _op(graph)["id"]
    store.commit_changeset(first.id, _modify_ops(graph, target, "name", "A"), actor="a")
    head_after = store.head()
    other = _modify_ops(graph, next(e["id"] for e in graph["elements"] if e["class"] == "Entity"), "name", "B", cs_id="CS-2")
    try:
        store.commit_changeset(first.id, other, actor="b")
    except VersionConflict as exc:
        assert exc.code == "VERSION_CONFLICT"
    else:
        raise AssertionError("stale base must raise VersionConflict")
    assert store.head().id == head_after.id and store.head().hash == head_after.hash


def k_rejected_changeset_is_atomic(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    bad = _modify_ops(graph, _op(graph)["id"], "name", "X")
    bad["ops"][0]["before"] = "not-the-current-value"
    try:
        store.commit_changeset(first.id, bad, actor="t")
    except ChangeSetRejected as exc:
        assert exc.code == "PRECONDITION_FAILED"
    else:
        raise AssertionError("precondition failure must be rejected")
    assert store.head().id == first.id and store.materialize() == graph
    wrong_version = _modify_ops(graph, _op(graph)["id"], "name", "X")
    wrong_version["base_version"] = "nope"
    try:
        store.commit_changeset(first.id, wrong_version, actor="t")
    except VersionConflict:
        pass
    else:
        raise AssertionError("graph-version mismatch must raise VersionConflict")


def k_interrupted_write_rolls_back(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    original = store._write_snapshot

    def broken(*args, **kwargs):
        original(*args, **kwargs)
        raise RuntimeError("simulated crash after partial write")

    store._write_snapshot = broken  # type: ignore[method-assign]
    try:
        store.commit_changeset(first.id, _modify_ops(graph, _op(graph)["id"], "name", "Y"), actor="t")
    except RuntimeError:
        pass
    else:
        raise AssertionError("expected the simulated crash")
    store._write_snapshot = original  # type: ignore[method-assign]
    assert store.head().id == first.id and store.materialize() == graph and len(store.snapshots()) == 1


def k_ordering_parity_for_add_and_remove(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    victim = next(e["id"] for e in graph["elements"] if e["class"] == "Scenario")
    new_el = copy.deepcopy(_op(graph)); new_el["id"] = "OP-NEW-CONTRACT"; new_el["name"] = "Contract op"
    cs = {"id": "CS-ord", "base_version": graph["version"], "result_version": "PBOM-TDD-v3", "ops": [
        {"seq": 1, "op": "remove", "target": victim, "cascade": "detach"},
        {"seq": 2, "op": "add", "after": new_el}]}
    expected, report = _o5().apply_changeset(copy.deepcopy(graph), cs)
    assert report["ok"], report
    res = store.commit_changeset(first.id, cs, actor="t")
    assert store.materialize() == expected and res.snapshot.hash == _o5().snapshot_hash(expected)
    assert victim in res.removed and "OP-NEW-CONTRACT" in res.added
    assert store.get_element(victim) is None and store.get_element(victim, first.id) is not None


def k_neighbors_direction_and_filter(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    store.commit_graph(None, graph, actor="t")
    edge = next(e for e in graph["edges"] if e["rel"] == "has_scenario")
    out_edges = store.neighbors([edge["src"]], ["has_scenario"], "out")
    assert edge in out_edges and all(e["src"] == edge["src"] for e in out_edges)
    in_edges = store.neighbors([edge["dst"]], ["has_scenario"], "in")
    assert edge in in_edges and all(e["dst"] == edge["dst"] for e in in_edges)
    assert store.neighbors([edge["src"]], ["has_scenario"], "in") == [e for e in in_edges if e["dst"] == edge["src"]]
    assert all(e["rel"] == "has_scenario" for e in store.neighbors([edge["src"], edge["dst"]], ["has_scenario"], "both"))
    assert store.neighbors([edge["src"]], [], "both") == []
    both = store.neighbors([edge["src"]], None, "both")
    assert edge in both
    assert store.neighbors([], None, "both") == []
    try:
        store.neighbors([edge["src"]], None, "sideways")
    except ValueError:
        pass
    else:
        raise AssertionError("invalid direction must be rejected")
    first = store.head().id
    other = _modify_ops(graph, _op(graph)["id"], "name", "Z")
    store.commit_changeset(first, other, actor="t")
    assert edge in store.neighbors([edge["src"]], ["has_scenario"], "out", snapshot=first)


def k_elements_by_class_and_ids(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    store.commit_graph(None, graph, actor="t")
    expected = [e for e in graph["elements"] if e["class"] == "Entity"]
    assert store.by_class("Entity") == expected
    ids = [expected[2]["id"], expected[0]["id"]]
    assert [e["id"] for e in store.get_elements(ids)] == ids
    assert store.get_element("DOES-NOT-EXIST") is None
    assert store.by_class("NoSuchClass") == []
    try:
        store.get_snapshot("snap-999999")
    except NotFound as exc:
        assert exc.code == "SNAPSHOT_NOT_FOUND"
    else:
        raise AssertionError("unknown snapshot must raise NotFound")


def k_noop_commit(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    res = store.commit_graph(first.id, copy.deepcopy(graph), actor="t")
    assert res.noop and res.snapshot.id == first.id and len(store.snapshots()) == 1


def k_sources_roles_and_idempotence(factory: KnowledgeFactory) -> None:
    store = factory()
    srs = store.register_source(SourceDoc.make("SRS-1", "1", "srs", "# SRS\nbody"))
    assert srs.role == "SRS" and srs.content_hash and srs.registered_at
    assert store.register_source(SourceDoc.make("SRS-1", "1", "SRS", "# SRS\nbody")).content_hash == srs.content_hash
    try:
        store.register_source(SourceDoc.make("SRS-1", "1", "SRS", "# SRS\nchanged"))
    except SourceConflict:
        pass
    else:
        raise AssertionError("same revision with different content must conflict")
    try:
        store.register_source(SourceDoc.make("SRS-1", "1", "TDD", "# SRS\nbody"))
    except SourceConflict:
        pass
    else:
        raise AssertionError("same revision with different role must conflict")
    store.register_source(SourceDoc.make("SRS-1", "2", "SRS", "# SRS v2"))
    assert store.get_source("SRS-1").revision == "2"
    assert store.get_source("SRS-1", "1").text == "# SRS\nbody"
    try:
        store.get_source("SRS-1", "1", role="TDD")
    except SourceRoleError as exc:
        assert exc.details["actual"] == "SRS"
    else:
        raise AssertionError("wrong role must raise SourceRoleError")
    try:
        store.get_source("NOPE")
    except NotFound:
        pass
    else:
        raise AssertionError("missing source must raise NotFound")
    store.register_source(SourceDoc.make("SAD-1", "1", "SAD", "sad"))
    assert [d.id for d in store.list_sources("SAD")] == ["SAD-1"] and len(store.list_sources()) == 3


def k_concurrent_writers_single_winner(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph()
    first = store.commit_graph(None, graph, actor="t").snapshot
    targets = [e["id"] for e in graph["elements"] if e["class"] == "Entity"][:8]
    results: list[str] = []
    lock = threading.Lock()

    def worker(i: int) -> None:
        cs = _modify_ops(graph, targets[i], "name", f"writer-{i}", cs_id=f"CS-w{i}")
        try:
            store.commit_changeset(first.id, cs, actor=f"w{i}")
            outcome = "ok"
        except VersionConflict:
            outcome = "conflict"
        with lock:
            results.append(outcome)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(len(targets))]
    for t in threads: t.start()
    for t in threads: t.join()
    assert results.count("ok") == 1 and results.count("conflict") == len(targets) - 1, results
    assert len(store.snapshots()) == 2


def k_graph_meta_and_version_preserved(factory: KnowledgeFactory) -> None:
    store = factory()
    graph = pbom_graph(); graph["migration"] = {"note": "kept"}
    store.commit_graph(None, graph, actor="t")
    out = store.materialize()
    assert out["migration"] == {"note": "kept"} and out["version"] == graph["version"]


def _doc(ident: str, name: str, statement: str, cls: str = "Requirement") -> dict:
    return {"id": ident, "class": cls, "name": name, "origin": "own", "maturity": "sketch", "review_status": "approved", "props": {"statement": statement},
            "provenance": [{"kind": "extracted", "source_id": "S", "source_version": "1", "source_hash": "h", "block": "line:1", "quote": statement, "quote_hash": "q"}]}


def _search_store(factory: KnowledgeFactory):
    store = factory()
    graph = {"version": "v1", "elements": [_doc("FR-AST-13", "Cảnh báo thuê bao", "Hệ thống phải cảnh báo Entitlement của thuê bao trước khi hết hạn"),
                                           _doc("FR-AST-14", "Gia hạn", "Cho phép gia hạn thuê bao"), _doc("BRL-7", "Quy tắc", "Một giao dịch chỉ được ghi nhận một lần", "BusinessRule")], "edges": []}
    store.commit_graph(None, graph, actor="t", validate=False)
    return store, graph


def k_search_matches_without_accents_ids_and_stays_inside_the_snapshot(factory: KnowledgeFactory) -> None:
    store, graph = _search_store(factory)
    first = store.head().id
    ids = lambda q, **kw: [h.id for h in store.search(q, **kw)]  # noqa: E731
    assert "FR-AST-13" in ids("canh bao thue bao") and ids("canh bao thue bao")[0] == "FR-AST-13", "no accents, no capitals"
    assert ids("FR-AST-13")[0] == ids("fr-ast-13")[0] == ids("AST-13")[0] == "FR-AST-13", "an id in full or in part"
    assert ids("FR-AST-13") == ["FR-AST-13"] or ids("FR-AST-13")[0] == "FR-AST-13"
    assert ids("thue bao", classes=["BusinessRule"]) == [] and ids("giao dich", classes=["BusinessRule"]) == ["BRL-7"]
    hit = store.search("canh bao")[0]
    assert (hit.cls, hit.snapshot) == ("Requirement", first) and "cảnh báo" in hit.excerpt.lower() and hit.score > 0 and hit.matched
    edited = {**graph, "version": "v2", "elements": [_doc("FR-AST-13", "Cảnh báo thuê bao", "Hệ thống gửi thư nhắc trước 30 ngày"), graph["elements"][1]]}   # FR-AST-13 reworded, BRL-7 removed
    second = store.commit_graph(first, edited, actor="t", validate=False).snapshot.id
    assert "FR-AST-13" in ids("entitlement", snapshot=first) and "FR-AST-13" not in ids("entitlement", snapshot=second), "the old wording lives only in the old snapshot"
    assert "FR-AST-13" in ids("thu nhac", snapshot=second) and ids("giao dich", snapshot=first) == ["BRL-7"] and ids("giao dich", snapshot=second) == [], "removed in the new snapshot only"


def k_search_cannot_be_broken_by_the_query_and_orders_stably(factory: KnowledgeFactory) -> None:
    store, _ = _search_store(factory)
    for hostile in ('a" OR (b', "NEAR(x", "*", "-", "AND OR NOT", "fr:ast", '"', "ab*cd", "thuê:bao"):
        assert isinstance(store.search(hostile), list), hostile
    assert store.search("") == [] and store.search("   ") == []
    again = [h.id for h in store.search("thue bao")]
    assert again == [h.id for h in store.search("thue bao")] and len(store.search("thue bao", limit=1)) == 1
    assert [h.id for h in store.search("AST")] == sorted(h.id for h in store.search("AST")) or len(store.search("AST")) >= 1


def k_search_is_all_or_nothing_for_backends_without_it(factory: KnowledgeFactory) -> None:
    from ..errors import SearchNotSupported
    store, _ = _search_store(factory)
    if getattr(store, "supports_search", False):
        return
    try:
        store.search("x")
    except SearchNotSupported as exc:
        assert exc.code == "SEARCH_NOT_SUPPORTED"
    else:
        raise AssertionError("a backend without search must say so, not answer with nothing found")


KNOWLEDGE_CONTRACT: list[Callable[[KnowledgeFactory], None]] = [
    k_import_roundtrip_and_hash_parity, k_changeset_new_snapshot_old_unchanged, k_version_conflict_leaves_graph_untouched,
    k_rejected_changeset_is_atomic, k_interrupted_write_rolls_back, k_ordering_parity_for_add_and_remove,
    k_neighbors_direction_and_filter, k_elements_by_class_and_ids, k_noop_commit, k_sources_roles_and_idempotence,
    k_concurrent_writers_single_winner, k_graph_meta_and_version_preserved, k_search_matches_without_accents_ids_and_stays_inside_the_snapshot,
    k_search_cannot_be_broken_by_the_query_and_orders_stably, k_search_is_all_or_nothing_for_backends_without_it,
]


# ------------------------------------------------------------------- operational
def _run(run_id: str = "run-1", flow: str = "generation", status: str = "running") -> RunRecord:
    ts = now_iso()
    return RunRecord(run_id=run_id, flow=flow, status=status, state="intake", request={"a": 1}, result={}, created_at=ts, updated_at=ts, trace_id="t-" + run_id)


def o_run_lifecycle(factory: OperationalFactory) -> None:
    store = factory()
    store.create_run(_run("run-1"))
    store.create_run(_run("run-2", flow="impact", status="blocked"))
    updated = store.update_run("run-1", state="srs_ready", result={"ok": True}, status="running")
    assert updated.state == "srs_ready" and updated.result == {"ok": True} and updated.updated_at >= updated.created_at
    assert store.get_run("run-1").request == {"a": 1}
    assert [r.run_id for r in store.list_runs(flow="impact")] == ["run-2"]
    assert [r.run_id for r in store.list_runs(status="running")] == ["run-1"]
    assert len(store.list_runs(limit=1)) == 1 and len(store.list_runs(limit=5, offset=1)) == 1
    assert store.count_runs_since("1970-01-01T00:00:00+00:00") == 2 and store.count_runs_since("9999-01-01T00:00:00+00:00") == 0
    try:
        store.get_run("nope")
    except NotFound:
        pass
    else:
        raise AssertionError("missing run must raise NotFound")
    try:
        store.update_run("run-1", bogus=1)
    except ValueError:
        pass
    else:
        raise AssertionError("unknown field must be rejected")


def o_transitions_and_ledger(factory: OperationalFactory) -> None:
    store = factory()
    store.create_run(_run())
    a = store.add_transition("run-1", {"from": "intake", "to": "srs_ready", "actor": "x"})
    b = store.add_transition("run-1", {"from": "srs_ready", "to": "blocked"})
    assert (a["n"], b["n"]) == (1, 2) and [t["to"] for t in store.list_transitions("run-1")] == ["srs_ready", "blocked"]
    assert store.get_task("run-1", "T1") is None
    store.put_task("run-1", "T1", {"status": "running", "input_hash": "h1"})
    store.put_task("run-1", "T1", {"status": "completed", "input_hash": "h1"})
    assert store.get_task("run-1", "T1")["status"] == "completed"
    store.put_task("run-1", "T2", {"status": "stale"})
    assert [t["task_id"] for t in store.list_tasks("run-1")] == ["T1", "T2"]


def o_approvals_receipts_audit(factory: OperationalFactory) -> None:
    store = factory()
    store.create_run(_run())
    store.add_approval({"run_id": "run-1", "gate": "G2", "actor_id": "u1", "approval_grade": "human"})
    assert store.list_approvals("run-1")[0]["actor_id"] == "u1" and store.list_approvals("other") == []
    assert store.find_receipt("run-1", "u1", "h") is None
    store.add_receipt({"run_id": "run-1", "actor_id": "u1", "decision_hash": "h", "source": "impact_get"})
    assert store.find_receipt("run-1", "u1", "h")["source"] == "impact_get"
    assert store.find_receipt("run-1", "u2", "h") is None and store.find_receipt("run-1", "u1", "other") is None
    store.add_audit("approval_rejected", {"why": "x"}, "run-1")
    store.add_audit("other", {}, None)
    assert len(store.list_audit()) == 2 and len(store.list_audit(kind="approval_rejected", run_id="run-1")) == 1


def o_spans_llm_and_metrics(factory: OperationalFactory) -> None:
    store = factory()
    store.add_span({"span_id": "s1", "trace_id": "t1", "run_id": "run-1", "parent_id": None, "name": "run", "start": "2026-01-01T00:00:00+00:00", "status": "open"})
    store.add_span({"span_id": "s2", "trace_id": "t1", "run_id": "run-1", "parent_id": "s1", "name": "gate", "start": "2026-01-01T00:00:01+00:00", "status": "open"})
    store.end_span("s2", status="ok", end="2026-01-01T00:00:02+00:00", duration_ms=1000)
    store.end_span("missing", status="ok")
    spans = store.list_spans(trace_id="t1")
    assert [s["span_id"] for s in spans] == ["s1", "s2"] and spans[1]["status"] == "ok" and spans[1]["duration_ms"] == 1000
    assert store.list_spans(run_id="none") == []
    store.add_llm_call({"run_id": "run-1", "provider": "p", "model": "m", "ts": now_iso()})
    store.add_llm_call({"run_id": "run-2", "provider": "p", "model": "m", "ts": now_iso()})
    assert len(store.list_llm_calls("run-1")) == 1 and len(store.list_llm_calls()) == 2
    store.incr_metric("runs", {"status": "ok"}); store.incr_metric("runs", {"status": "ok"}, 2.0); store.incr_metric("runs", {"status": "blocked"})
    metrics = {m["labels"]["status"]: m["value"] for m in store.list_metrics("runs")}
    assert metrics == {"ok": 3.0, "blocked": 1.0} and store.list_metrics("none") == []


OPERATIONAL_CONTRACT: list[Callable[[OperationalFactory], None]] = [
    o_run_lifecycle, o_transitions_and_ledger, o_approvals_receipts_audit, o_spans_llm_and_metrics,
]
