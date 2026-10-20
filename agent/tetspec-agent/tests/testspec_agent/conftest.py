"""Shared fixtures: config pointing to temp folders, an initialised temp DB, the minipay fixture package and the PAY md."""
import json
import os

import pytest

from tsagent.config import Config
from tsagent.store.db import Database

FIX = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fixtures")
MINIPAY = os.path.join(FIX, "minipay")
PAY = os.path.join(FIX, "pay")


@pytest.fixture(autouse=True)
def _llm_mode_none(monkeypatch):
    """TSAGENT_LLM has no default any more: tests set `none` explicitly; a test needing another mode sets it itself."""
    monkeypatch.setenv("TSAGENT_LLM", "none")


def make_cfg(tmp_path, **kw):
    cfg = Config()
    cfg.db = str(tmp_path / "tsagent.db")
    cfg.specs_root = str(tmp_path / "specs")
    cfg.cache_dir = str(tmp_path / "cache")
    cfg.snapshot_dir = str(tmp_path / "snap")
    cfg.inbox_dir = str(tmp_path / "inbox")
    cfg.llm_provider = "none"
    cfg.source_roots = (str(tmp_path), FIX)
    for k, v in kw.items():
        setattr(cfg, k, v)
    return cfg


@pytest.fixture
def cfg(tmp_path):
    return make_cfg(tmp_path)


@pytest.fixture
def tmp_db(cfg):
    db = Database(cfg.db)
    db.init_schema()
    yield db
    db.close()


@pytest.fixture
def minipay_pkg():
    with open(os.path.join(MINIPAY, "cr001-slice.tdd-kg.json"), encoding="utf-8") as fh:
        return json.load(fh)


@pytest.fixture
def minipay_full():
    with open(os.path.join(MINIPAY, "full.tdd-kg.json"), encoding="utf-8") as fh:
        return json.load(fh)


@pytest.fixture
def minipay_docs():
    return {k: os.path.join(MINIPAY, f) for k, f in (("slice", "cr001-slice.tdd-kg.json"), ("solution", "solution.md"),
                                                       ("supplement", "supplement.md"), ("srs", "srs.md"), ("cr", "cr.md"))}


@pytest.fixture
def pay_docs():
    return [os.path.join(PAY, f) for f in ("urd.md", "srs.md", "tdd.md")]


@pytest.fixture
def fake_llm(cfg):
    """Fake LLM bound to the config: `fake_llm.script = ...`, then `make_client(cfg)` uses it (TSAGENT_LLM=fake)."""
    from tsagent.llm.fake import FakeLLM
    fake = FakeLLM()
    FakeLLM.current = fake
    cfg.llm_provider = "fake"
    yield fake
    FakeLLM.current = None
