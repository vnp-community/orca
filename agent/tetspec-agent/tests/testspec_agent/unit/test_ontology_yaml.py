"""T1.3: tso.yaml is valid and the loader reports located errors."""
import pytest

from tsagent.errors import InputError
from tsagent.ontology import load_ontology, ontology, table_of

pytestmark = pytest.mark.unit

LAYER_A = ["TestBasisItem", "TestObligation", "TestCondition", "TestOracle", "FaultModel", "Partition", "ScenarioPath",
           "RiskRating", "TestGap"]
LAYER_B = ["TestRequirement", "PassCriterion", "TestDesign", "CoverageItem"]
LAYER_C = ["TestCase", "TestStep", "Assertion", "TestData", "DataField", "Fixture", "StubSpec", "FaultInjection",
           "TestEnvironment", "TestSet"]


def test_packaged_yaml_has_all_classes_of_the_ontology_doc():
    o = ontology()
    for layer, names in (("A", LAYER_A), ("B", LAYER_B), ("C", LAYER_C)):
        for n in names:
            assert o.cls(n).layer == layer
    assert len(o.enums["conditionKind"]) == 21
    assert o.cls("TestCondition").relations["oracle"].required
    assert o.cls("TestCase").relations["covers"].many
    assert table_of("TestBasisItem") == "tso_test_basis_item"
    assert {"src_baseline", "src_node", "src_edge", "src_derived_node"} <= set(o.source)


def test_dangling_relation_is_reported_with_line():
    text = """version: 1
enums: {level: [L1]}
classes:
  A:
    layer: A
    fields: {x: {type: text, card: "!"}}
    relations:
      toB: {to: B, card: "!"}
"""
    with pytest.raises(InputError) as e:
        load_ontology(path=None, text=text)
    d = e.value.diagnostics[0]
    assert "does not exist" in d.message and d.loc == "8"


def test_unknown_enum_and_bad_card_are_reported():
    text = """version: 1
enums: {}
classes:
  A:
    layer: A
    fields:
      k: {type: "enum:nope", card: "!"}
      arr: {type: text, card: "+"}
"""
    with pytest.raises(InputError) as e:
        load_ontology(path=None, text=text)
    msgs = " | ".join(d.message for d in e.value.diagnostics)
    assert "enum does not exist" in msgs and "array fields" in msgs
