"""Heavy parity test on the 12.8k-element legacy fixture (set TDD_AGENT_SLOW=1)."""
import json
import os

import pytest

from tdd_agent.core import o5_path
from tdd_agent.store import open_knowledge_store

pytestmark = pytest.mark.skipif(not os.environ.get("TDD_AGENT_SLOW"), reason="set TDD_AGENT_SLOW=1 to run")


@pytest.mark.parametrize("url", ["memory:", "sqlite:{tmp}/big.db"])
def test_kg5_migrated_hash_parity_and_roundtrip(url, tmp_path):
    o5 = o5_path.load()
    graph = json.loads((o5.root / "fixtures" / "kg5.migrated.json").read_text())
    store = open_knowledge_store(url.format(tmp=tmp_path))
    try:
        res = store.commit_graph(None, graph, actor="parity", validate=False)
        assert res.snapshot.hash == o5.snapshot_hash(graph)
        assert store.materialize() == graph
    finally:
        store.close()
