"""Small design graph (tdd-agent KG shape) for the test-spec branch tests."""


def el(id_, cls, name="", props=None, tdds=("TDD-A",), **kw):
    return {"id": id_, "class": cls, "name": name or id_, "props": dict(props or {}), "tdds": list(tdds), "owner_doc": "SRS", **kw}


def ed(src, rel, dst):
    return {"src": src, "rel": rel, "dst": dst}


def design_graph():
    els = [
        el("FR-3", "Requirement", "Notify owners", {"statement": "The system SHALL notify owners about expiry.", "acceptance": "AC-3"}),
        el("NFR-1", "QualityTarget", "Notice latency", {"target": {"slot_status": "tbd"}, "characteristic": "performance"},
           gaps=[{"slot": "target", "status": "tbd", "reason": "Giá trị chưa có trong văn bản nguồn"}]),
        el("SVC-N-01", "Component", "Notification service", {"responsibility": "Send notices"}),
        el("IF-1", "Interface", "Notify API", {"protocol": "REST", "style": "sync"}),
        el("API-1", "Operation", "POST /notices", {"method": "POST"}, ext={"path": "/notices"}),
        el("MSG-1", "Message", "Notice request"),
        el("Attribute:notice.owner", "Attribute", "owner", attrs={"datatype": "string", "nullable": False}),
        el("E-1", "ErrorCode", "duplicate", attrs={"HTTP": "409", "Mã": "E-1", "Điều kiện": "Notice already sent"}),
        el("RP-1@T01", "RoutePolicy", "RP-1", attrs={"timeout": "5 giây", "retry": "3 lần"}),
        el("SM-1", "StateMachine", "Notice", {"initial_state": "Pending"}),
        el("S-P", "State", "Pending"), el("S-S", "State", "Sent", {"final": True}),
        el("Transition:notice.Pending->Sent", "Transition", "Pending sang Sent", attrs={"trigger": "job send"}),
        el("OI-1", "OpenIssue", "Retry policy?", {"question": "How many retries?", "owner": "SA"}),
        el("SEC-1", "Section", "4.1"),
        el("FR-9", "Requirement", "Other TDD", {"statement": "Other."}, tdds=("TDD-B",)),
        {"id": "CR-1/CS-001", "class": "ChangeSpecItem", "name": "CS-001", "tdds": [], "owner_doc": "CS",
         "props": {"cr_acceptance_refs": ["AC-3"], "cr_id": "CR-1",
                   "srs_refs": [{"document_id": "SRS", "role": "SRS", "section_id": "FR-3"}],
                   "tdd_refs": [{"document_id": "TDD-A", "role": "TDD", "section_id": "document"},
                                {"document_id": "TDD-A", "role": "TDD", "section_id": "SVC-N-01"},
                                {"document_id": "TDD-A", "role": "TDD", "section_id": "API-1"},
                                {"document_id": "TDD-A", "role": "TDD", "section_id": "NFR-1"},
                                {"document_id": "TDD-A", "role": "TDD", "section_id": "SEC-1"},
                                {"document_id": "TDD-A", "role": "TDD", "section_id": "GONE-1"}]}},
    ]
    edges = [
        ed("SVC-N-01", "satisfies", "FR-3"), ed("IF-1", "exposes", "API-1"), ed("API-1", "accepts", "MSG-1"),
        ed("MSG-1", "has_attribute", "Attribute:notice.owner"), ed("API-1", "may_fail_with", "E-1"),
        ed("IF-1", "governed_by", "RP-1@T01"), ed("SM-1", "has_state", "S-P"), ed("SM-1", "has_state", "S-S"),
        ed("Transition:notice.Pending->Sent", "in_machine", "SM-1"), ed("Transition:notice.Pending->Sent", "transition_from", "S-P"),
        ed("Transition:notice.Pending->Sent", "transition_to", "S-S"), ed("SVC-N-01", "satisfies", "SM-1"),
        ed("FR-3", "blocked_by", "OI-1"), ed("SEC-1", "describes", "FR-3"), ed("SVC-N-01", "provides", "IF-1"),
        ed("SVC-N-01", "logs_with", "FR-3"),
    ]
    return {"version": "v1", "elements": els, "edges": edges}


CR_PARAMS = {"cs_elements": ["CR-1/CS-001"], "cr_id": "CR-1",
             "acceptance": [{"id": "AC-3", "text": "GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified"}]}
