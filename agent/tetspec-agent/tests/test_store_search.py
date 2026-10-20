"""The SQLite full-text index is derived data: it follows commits, can be rebuilt, and never changes what the knowledge graph says."""
from __future__ import annotations

import pytest

from tdd_agent.errors import ChangeSetRejected
from tdd_agent.store import open_knowledge_store
from tdd_agent.store.contract import _doc, pbom_graph


@pytest.fixture()
def store(tmp_path):
    s = open_knowledge_store(f"sqlite:{tmp_path}/k.db")
    yield s
    s.close()


def test_a_rejected_commit_leaves_nothing_in_the_index(store):
    store.commit_graph(None, {"version": "v1", "elements": [_doc("FR-1", "Một", "Giữ bằng chứng")], "edges": []}, actor="t", validate=False)
    bad = {"version": "v2", "elements": [_doc("FR-1", "Một", "Giữ bằng chứng"), {"id": "BAD-1", "class": "NoSuchClass", "name": "Lỗi", "props": {"statement": "không hợp lệ"}}], "edges": []}
    with pytest.raises(ChangeSetRejected):
        store.commit_graph(store.head().id, bad, actor="t")
    assert store.search("khong hop le") == [] and store.index_check()["ok"] and len(store.snapshots()) == 1


def test_reindex_rebuilds_the_index_without_touching_a_snapshot_or_its_hash(store):
    graph = pbom_graph()
    store.commit_graph(None, graph, actor="t")
    hashes = [(s.id, s.hash) for s in store.snapshots()]
    before = [h.id for h in store.search("entitlement", limit=10)]
    assert before and store.index_check()["ok"]
    checked = store.reindex()
    assert checked["ok"] and checked["rows"] == len(graph["elements"]) and [(s.id, s.hash) for s in store.snapshots()] == hashes
    assert [h.id for h in store.search("entitlement", limit=10)] == before
    store.db.conn.execute("DROP TABLE element_fts")
    store.db.conn.execute("CREATE VIRTUAL TABLE element_fts USING fts5(id, name, body, content='', tokenize='unicode61 remove_diacritics 2')")
    assert not store.index_check()["ok"] and store.reindex()["ok"], "a damaged index is found and rebuilt"
    assert store.materialize() == graph


def test_a_database_made_before_the_index_existed_gets_one_when_it_is_opened(tmp_path):
    path = f"sqlite:{tmp_path}/old.db"
    first = open_knowledge_store(path)
    graph = {"version": "v1", "elements": [_doc("FR-AST-13", "Cảnh báo thuê bao", "Cảnh báo Entitlement")], "edges": []}
    first.commit_graph(None, graph, actor="t", validate=False)
    digest = first.head().hash
    first.db.conn.execute("DROP TABLE element_fts"); first.db.conn.execute("DROP TABLE element_tri")
    first.db.conn.execute("DELETE FROM schema_version WHERE name='knowledge_fts'")
    first.close()
    reopened = open_knowledge_store(path)
    try:
        assert [h.id for h in reopened.search("canh bao")] == ["FR-AST-13"] and reopened.head().hash == digest and reopened.materialize() == graph
        assert reopened.db.conn.execute("SELECT version FROM schema_version WHERE name='knowledge_fts'").fetchone()[0] == 1
    finally:
        reopened.close()


def test_pbom_finds_an_element_by_the_words_of_its_name_without_accents_and_by_a_fragment_of_its_id(store):
    from tdd_agent.store.search import fold
    graph = pbom_graph()
    store.commit_graph(None, graph, actor="t")
    target = next(e for e in graph["elements"] if e["class"] == "RequirementVersion" and e["id"].startswith("FR-") and len([w for w in fold(e["props"]["statement"]).split() if len(w) > 4 and w.isalpha()]) > 2)
    fragment = target["id"].split("-", 1)[1]
    assert target["id"] in [h.id for h in store.search(fragment, limit=50)], "a fragment of the id"
    assert store.search(target["id"].lower())[0].id == target["id"], "the whole id, in any case"
    word = next(w for w in fold(target["props"]["statement"]).split() if len(w) > 4 and w.isalpha() and w not in ("quy", "tac"))
    assert any(h.id == target["id"] for h in store.search(word, limit=200)), "a word of its statement, accents or not"
