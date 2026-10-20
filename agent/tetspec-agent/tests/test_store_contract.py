import pytest

from tdd_agent.store import open_knowledge_store, open_operational_store
from tdd_agent.store.contract import KNOWLEDGE_CONTRACT, OPERATIONAL_CONTRACT

KNOWLEDGE_URLS = ["memory:", "sqlite::memory:", "sqlite:{tmp}/k.db"]
OPERATIONAL_URLS = ["memory:", "sqlite::memory:", "sqlite:{tmp}/o.db"]


def _ids(fns):
    return [f.__name__ for f in fns]


@pytest.mark.parametrize("url", KNOWLEDGE_URLS)
@pytest.mark.parametrize("contract", KNOWLEDGE_CONTRACT, ids=_ids(KNOWLEDGE_CONTRACT))
def test_knowledge_contract(contract, url, tmp_path):
    opened = []

    def factory():
        store = open_knowledge_store(url.format(tmp=tmp_path))
        opened.append(store)
        return store

    try:
        contract(factory)
    finally:
        for s in opened:
            s.close()


@pytest.mark.parametrize("url", OPERATIONAL_URLS)
@pytest.mark.parametrize("contract", OPERATIONAL_CONTRACT, ids=_ids(OPERATIONAL_CONTRACT))
def test_operational_contract(contract, url, tmp_path):
    opened = []

    def factory():
        store = open_operational_store(url.format(tmp=tmp_path))
        opened.append(store)
        return store

    try:
        contract(factory)
    finally:
        for s in opened:
            s.close()
