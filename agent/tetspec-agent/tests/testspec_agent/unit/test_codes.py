"""T5.1: node codes derived from source codes: stable, unique, length-limited."""
import pytest

from tsagent.analyze import codes
from tsagent.analyze.seed_core import seed
from tsagent.ingest.kg_json import load_kg_json

pytestmark = pytest.mark.unit


def test_patterns():
    assert codes.obligation("PAY", "FR-001", "func") == "TOB-PAY-FR-001-func"
    assert codes.condition("PAY", "INT-01", "errorHandling", "5xx") == "TCND-PAY-INT-01-errorHandling-5xx"
    assert codes.oracle("PAY", "ERR-01") == "TORC-PAY-ERR-01"
    assert codes.requirement("PAY", "L2", "FR-001", "func") == "TR-PAY-L2-FR-001-func"
    assert codes.design("PAY", "L3", "EP-1", "boundaryValue") == "TD-PAY-L3-EP-1-boundaryValue"
    c = codes.condition("PAY", "FLD-a", "inputValidation", "max")
    assert codes.coverage("PAY", "L3", c, "over") == "TCOV-PAY-L3-FLD-a-inputValidation-max-over"
    assert codes.testcase("L3", "PAY", c, "over") == "TC-L3-PAY-FLD-a-inputValidation-max-over"


def test_normalize_and_length():
    assert codes.norm("a b/c::d") == "a-b-c-d" and codes.norm("--x--") == "x"
    long = codes.condition("F", "X" * 200, "functional", "k")
    assert len(long) == codes.MAX_LEN and long == codes.condition("F", "X" * 200, "functional", "k")
    assert long != codes.condition("F", "X" * 199 + "Y", "functional", "k")


def test_stable_and_unique_on_fixture(minipay_pkg):
    pkg, _ = load_kg_json(minipay_pkg, "ST")
    a, b = seed(pkg, "b-1").g, seed(pkg, "b-2").g
    assert sorted(a.entities) == sorted(b.entities)  # independent of baseline and order
    assert all(len(c) <= codes.MAX_LEN for c in a.entities)
