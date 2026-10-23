"""The contract block of TCIR: what the TDD says about calling the system, and what it leaves open."""
import pytest

from tsagent.model import SourcePackage, SrcNode
from tsagent.render import tcir_contract

pytestmark = pytest.mark.unit


def notes_like() -> SourcePackage:
    """The shape of the notes project: endpoints only in sentences, fields tied to the entity by id, no interface contract."""
    p = SourcePackage("TDD-NOTES")
    p.add_node(SrcNode("FR-NOTE-01", "FunctionalRequirement", {}, {"text": "The service shall store a note of at most 280 characters sent with POST /notes."}))
    p.add_node(SrcNode("FR-NOTE-01-AC1", "AcceptanceCriterion", {}, {"statement": "A note of 281 characters is refused with 422."}))
    p.add_node(SrcNode("FR-NOTE-02", "FunctionalRequirement", {}, {"text": "The service shall return stored notes with GET /notes."}))
    p.add_node(SrcNode("BRL-01", "FunctionalRequirement", {}, {"text": "A note is text of 1 to 280 characters."}))
    p.add_node(SrcNode("ENT-01", "DataEntity", {}, {"tddAgent": {"id": "ENT-01"}}))
    for name in ("id", "text", "created_at"):
        p.add_node(SrcNode(f"ENT-01-attribute-{name}", "Field", {}, {"name": name, "tddAgent": {"id": f"ENT-01:attribute:{name}"}}))
    p.add_edge("FR-NOTE-01", "hasAcceptance", "FR-NOTE-01-AC1")
    return p


def test_endpoints_entities_and_statuses_from_sentences():
    c = tcir_contract.build(notes_like(), {"FR-NOTE-01"})
    eps = {(e["method"], e["path"]): e for e in c["endpoints"]}
    assert set(eps) == {("POST", "/notes"), ("GET", "/notes")}
    assert eps[("POST", "/notes")]["related"] and not eps[("GET", "/notes")]["related"]
    assert c["endpoints"][0]["path"] == "/notes" and c["endpoints"][0]["method"] == "POST"   # related first
    assert [f["name"] for f in c["entities"][0]["fields"]] == ["created_at", "id", "text"]
    assert [(s["status"], s["from"]) for s in c["statusCodes"]] == [(422, "FR-NOTE-01-AC1")]   # 280 and 281 are counts, not statuses
    assert any("POST /notes" in u for u in c["unknown"]) and any("types of the fields of ENT-01" in u for u in c["unknown"])


def test_a_defined_interface_contract_wins():
    p = notes_like()
    p.add_node(SrcNode("EP-1", "Endpoint", {}, {"method": "post", "path": "/notes"}))
    p.add_node(SrcNode("REQ-1", "Message", {}, {}))
    p.add_node(SrcNode("F-TEXT", "Field", {}, {"name": "text", "type": "string", "required": True}))
    p.add_node(SrcNode("RESP-422", "Response", {}, {"statusCode": 422, "errorCode": "NOTE_TOO_LONG", "condition": "text longer than 280"}))
    p.add_edge("EP-1", "hasRequest", "REQ-1")
    p.add_edge("REQ-1", "hasField", "F-TEXT")
    p.add_edge("EP-1", "hasResponse", "RESP-422")
    c = tcir_contract.build(p, {"FR-NOTE-01"})
    ep = next(e for e in c["endpoints"] if e["method"] == "POST")
    assert ep["defined"] and ep["requestFields"] == [{"name": "text", "type": "string", "required": True}]
    assert ep["responses"] == [{"status": 422, "errorCode": "NOTE_TOO_LONG", "condition": "text longer than 280", "from": "RESP-422"}]
    assert not any("POST /notes" in u for u in c["unknown"])
    assert any(s["status"] == 422 and s["from"] == "RESP-422" for s in c["statusCodes"])


def test_nothing_named():
    c = tcir_contract.build(SourcePackage("F"), {"X"})
    assert c["endpoints"] == [] and c["unknown"] == ["no endpoint is named anywhere in the TDD"]
    assert tcir_contract.build(None, {"X"}) == {}
