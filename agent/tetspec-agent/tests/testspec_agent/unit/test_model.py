"""T1.2: domain types."""
import pytest

from tsagent.model import OntologyDecl, Provenance, SourcePackage, SrcNode, TsoEntity, TsoGraph

pytestmark = pytest.mark.unit


def pkg():
    p = SourcePackage("F")
    p.add_node(SrcNode("FR-1", "FunctionalRequirement", {"status": "Baselined"}, {"text": "a", "priority": "Must"}))
    p.add_node(SrcNode("MOD-A", "Module", {}, {"responsibility": "x"}))
    p.add_node(SrcNode("MOD-B", "Module", {"testScope": "context"}, {"responsibility": "y"}))
    p.add_node(SrcNode("SCN-1", "Scenario", {}, {"responseBudgetMs": 1000, "steps": ["a", "b"]}))
    p.add_edge("FR-1", "allocatedTo", "MOD-A")
    p.add_edge("MOD-A", "participatesIn", "SCN-1")
    p.add_edge("SCN-1", "participatesIn", "MOD-B")
    p.add_edge("MOD-A", "dependsOn", "MOD-B", {"note": "đồng bộ"})
    return p


def test_queries_both_directions():
    p = pkg()
    assert [n.code for n in p.targets("FR-1", "allocatedTo")] == ["MOD-A"]
    assert [n.code for n in p.sources("MOD-A", "allocatedTo")] == ["FR-1"]
    assert [n.code for n in p.participants("SCN-1")] == ["MOD-A", "MOD-B"]
    assert [n.code for n in p.of("Module")] == ["MOD-A", "MOD-B"]
    assert not p.get("MOD-B").in_scope and p.get("MOD-A").in_scope


def test_edge_props_kept():
    e = next(e for e in pkg().edges if e.rel == "dependsOn")
    assert e.props_dict == {"note": "đồng bộ"}


def test_hash_is_stable_and_sensitive():
    a, b = pkg(), pkg()
    assert a.content_hash() == b.content_hash()
    b.get("FR-1").props["text"] = "b"
    assert a.content_hash() != b.content_hash()
    c = pkg()
    c.get("FR-1").props["sourceLine"] = 99  # the position in the file does not change the content
    assert a.get("FR-1").content_hash() == c.get("FR-1").content_hash()


def test_same_code_two_classes_is_rejected():
    p = pkg()
    with pytest.raises(ValueError, match="two classes"):
        p.add_node(SrcNode("FR-1", "Module"))


def test_ontology_inheritance_required():
    o = OntologyDecl({"Entity": {"parent": None, "required": ["code"]},
                      "Requirement": {"parent": "Entity", "required": ["code", "text"]},
                      "FunctionalRequirement": {"parent": "Requirement", "required": ["code", "text"]}}, {})
    assert o.is_a("FunctionalRequirement", "Requirement") and o.required("FunctionalRequirement") == ["code", "text"]


def test_tso_entity_hash_ignores_volatile_provenance():
    e1 = TsoEntity("TestCondition", "TCND-1", {"statement": "x"}, prov=Provenance("llm", ["FR-1"], model="m1", confidence=0.6))
    e2 = TsoEntity("TestCondition", "TCND-1", {"statement": "x"}, prov=Provenance("llm", ["FR-1"], model="m2", confidence=0.9))
    assert e1.content_hash() == e2.content_hash()
    e2.link("oracle", "TORC-1")
    assert e1.content_hash() != e2.content_hash()


def test_tso_graph_referencing():
    g = TsoGraph("F")
    g.add(TsoEntity("TestOracle", "TORC-1"))
    g.add(TsoEntity("TestCondition", "TCND-1").link("oracle", "TORC-1"))
    assert [e.code for e in g.referencing("TORC-1", "oracle")] == ["TCND-1"]
    with pytest.raises(ValueError):
        g.add(TsoEntity("TestCase", "TCND-1"))
    with pytest.raises(ValueError):
        Provenance("guess")
