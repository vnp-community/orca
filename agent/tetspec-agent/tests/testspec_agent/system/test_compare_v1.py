"""TST-21: compared with v1 on MiniPay CR-001: every v1 gap has a test case or a stated reason."""
import importlib.util
import os

import pytest

from testspec_agent.conftest import MINIPAY
from testspec_agent.helpers.pipeline import build_graph

pytestmark = pytest.mark.system


@pytest.fixture(scope="module")
def cr001(tmp_path_factory):
    from testspec_agent.conftest import make_cfg
    cfg = make_cfg(tmp_path_factory.mktemp("cmp"))
    pkg, _, g, _ = build_graph(cfg, [os.path.join(MINIPAY, f) for f in ("cr001-slice.tdd-kg.json", "solution.md", "supplement.md")],
                               "SCHEDULED-TRANSFER")
    spec = importlib.util.spec_from_file_location("cmp", os.path.join(os.path.dirname(__file__), "..", "..", "..", "scripts",
                                                                       "tsagent", "compare_baseline.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod, pkg, g


def test_every_v1_gap_has_testcase_or_reason(cr001):
    mod, pkg, g = cr001
    rows = {gid: (tcs, note) for gid, tcs, note in mod.bc_rows(g, pkg)}
    for gid in ("G1", "G2", "G3", "G4", "G5", "G7", "G8", "G14"):
        assert rows[gid][0], gid
    for gid in ("G9", "G10"):
        assert rows[gid][1], gid  # stated reason
    m = mod.bc_metrics(g, pkg)
    assert m["meaninglessNfr"] == 0                                  # G6
    assert m["numbered"] == m["tr"] and m["copied"] == 0              # G11
    assert m["scoped"] == m["tr"]                                     # G12
    assert m["assertsWithOracle"] == m["asserts"]
    assert m["tcByLevel"]["L1"] >= 3 and m["tc"] >= 183               # not fewer than v1
