"""T5.3: context pack: content, size limit, allow list, stable hash."""
import json

import pytest

from tsagent.analyze.context import context_pack
from tsagent.analyze.seed_core import seed
from tsagent.ingest import ingest

pytestmark = pytest.mark.unit


@pytest.fixture
def cr001(cfg, minipay_docs):
    pkg, _ = ingest([minipay_docs["slice"], minipay_docs["solution"], minipay_docs["supplement"]], "ST", cfg)
    return pkg, seed(pkg, "b")


def groups(s):
    for subj, obs in sorted(s.obligations.items()):
        conds = [c.code for c in s.g.of("TestCondition") if c.rel("obligation")[0] in obs]
        yield subj, obs, conds


def test_pack_contents_and_whitelist(cr001):
    pkg, s = cr001
    p = context_pack(pkg, s.g, "FR-041", s.obligations["FR-041"],
                     [c.code for c in s.g.of("TestCondition") if c.rel("obligation")[0] in s.obligations["FR-041"]])
    assert p["subject"]["code"] == "FR-041" and p["conditions"] and p["conditions"][0]["oracle"]["oracleKind"]
    nb = {n["code"] for n in p["neighbors"]}
    assert "MOD-Scheduler" in nb and not any(n["class"] == "Document" for n in p["neighbors"])
    assert {"FR-041", "AC-FR-041-1"} <= set(p["allowedCodes"]) and nb <= set(p["allowedCodes"])
    assert "sourceLine" not in json.dumps(p)


def test_every_cr001_pack_under_limit(cr001):
    pkg, s = cr001
    for subj, obs, conds in groups(s):
        for i in range(0, max(1, len(conds)), 20):
            p = context_pack(pkg, s.g, subj, obs, conds[i:i + 20], max_chars=24000)
            assert len(json.dumps(p, ensure_ascii=False)) < 26000, subj


def test_truncation_noted_and_seeds_kept(cr001):
    pkg, s = cr001
    obs = s.obligations["MOD-Scheduler"] if "MOD-Scheduler" in s.obligations else next(iter(s.obligations.values()))
    subj = "MOD-Scheduler" if "MOD-Scheduler" in s.obligations else next(iter(s.obligations))
    conds = [c.code for c in s.g.of("TestCondition") if c.rel("obligation")[0] in obs]
    p = context_pack(pkg, s.g, subj, obs, conds, max_chars=1500)
    assert p["notes"] and len(p["conditions"]) == len(conds)


def test_stable_hash(cr001):
    pkg, s = cr001
    a = context_pack(pkg, s.g, "NFR-011", s.obligations["NFR-011"], [])
    _, s2 = cr001[0], seed(cr001[0], "b")
    b = context_pack(pkg, s2.g, "NFR-011", s2.obligations["NFR-011"], [])
    assert a["hash"] == b["hash"]
