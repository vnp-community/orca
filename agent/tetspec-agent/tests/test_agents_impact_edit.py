import json

import pytest

from harness import ACTORS, make
from tdd_agent.agents.flows import default_flows
from tdd_agent.orchestrator import ApprovalRequest
from tdd_agent.store.base import SourceDoc

URD = "# URD\n| ID | Statement |\n|---|---|\n| REQ-1 | Evidence must be kept. |\n"
SAD = "# SAD\n| ID | Decision | Rationale | Alternatives | Tradeoffs |\n|---|---|---|---|---|\n| ADR-1 | Kafka | durable partitioned log | RabbitMQ | ops cost |\n"
SRS_1 = "# SRS\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST store evidence. | AC-1 |\n| SVC-1 | Evidence service | FR-1 |\n"
SRS_C = "# SRS-C\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-2 | Reports MUST be listed. | AC-2 |\n| SVC-9 | Report service | |\n"


def gen(o, run, tdd, srs_id, text):
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": tdd, "prose": False}}, run_id=run, sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make(srs_id, "1", "SRS", text), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance(run).status == "completed"


def impact(o, delta_changes, run="i1", **params):
    delta = {"ref": {"document_id": "SRS", "role": "SRS", "revision": "2", "content_hash": "h", "section_id": "delta"}, "cr_ref": {"document_id": "CR-1", "role": "CR", "revision": "1", "content_hash": "c", "section_id": "document"},
             "changes": [{"rationale": "changed", "section_ref": {"document_id": "SRS", "role": "SRS", "revision": "2", "content_hash": "h", "section_id": c["id"]}, **c} for c in delta_changes], "conflicts": []}
    o.start("impact-agent", {"mode": "change-request", "params": {"srs_delta": delta, **params}}, run_id=run, sources=[SourceDoc.make("SAD", "1", "SAD", SAD)])
    return o.advance(run)


def h_judge(covered, cited=("SVC-9",)):
    def h(req, n):
        skill = req.metadata.get("skill")
        if skill == "judge-design-coverage":
            return {"covered": covered, "cited": list(cited), "reason": "stated by the existing design" if covered else "needs an edit"}
        if skill == "detect-sad-conflict":
            return {"suspected": []}
        if skill == "resolve-entity-match":
            return {"same": False, "reason": "x"}
        raise AssertionError(skill)
    return h


def base(handler, tmp_path):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    gen(o, "ga", "TDD-A", "SRS", SRS_1)
    gen(o, "gc", "TDD-C", "SRS-C", SRS_C)
    return o, know, ops, provider


def entries(ops, run="i1"):
    return {e["tdd_id"]: e for e in ops.get_task(run, "G2")["output"]["report"]["entries"]}


def test_residue_tdd_is_judged_and_only_verified_coverage_gives_no_change(tmp_path):
    o, know, ops, provider = base(h_judge(True), tmp_path)
    run = impact(o, [{"id": "FR-5", "kind": "added", "after_quote": "| FR-5 | Evidence MUST be exported. |"}], tdd_for_new={})
    e = entries(ops)
    judged = [c for c in provider.calls if c.metadata.get("skill") == "judge-design-coverage"]
    assert judged, "TDD-C has no requirement trace edges, so independence cannot be proven by rules"
    assert e["TDD-C"]["action"] == "no_change" and "stated by the existing design" in e["TDD-C"]["rationale"]
    assert e["TDD-A"]["action"] in ("no_change", "update")
    assert run.status == "awaiting_approval"


def test_model_cannot_assert_coverage_without_citations_or_when_budget_is_exhausted(tmp_path):
    o, know, ops, provider = base(h_judge(True, cited=()), tmp_path)
    impact(o, [{"id": "FR-5", "kind": "added", "after_quote": "| FR-5 | x |"}])
    assert entries(ops)["TDD-C"]["action"] == "update", "covered-without-citations is treated as needs-update"
    o2, k2, ops2, p2 = base(h_judge(True), tmp_path / "b")
    impact(o2, [{"id": "FR-5", "kind": "added", "after_quote": "| FR-5 | x |"}], max_judgements=0)
    assert entries(ops2)["TDD-C"]["action"] == "update" and not [c for c in p2.calls if c.metadata.get("skill") == "judge-design-coverage"]


def test_rule_conflict_with_the_sad_blocks_and_proposes_a_sad_cr(tmp_path):
    o, know, ops, provider = base(h_judge(False), tmp_path)
    run = impact(o, [{"id": "FR-9", "kind": "added", "after_quote": "| FR-9 | Events MUST be published through RabbitMQ |", "quantities_after": []}])
    assert run.status == "blocked" and run.result["blockers"] == ["SAD_CHANGE_REQUIRED"]
    out = ops.get_task("i1", "G2")["output"]
    assert out["sad_cr_proposal"]["conflicts"][0]["origin"] == "rule" and out["sad_cr_proposal"]["conflicts"][0]["sad_element"] == "ADR-1"
    # a waiver nobody owns does not clear a finding; the SA's waiver (with the reason) does, and what was set aside is part of the decision
    change = [{"id": "FR-9", "kind": "added", "after_quote": "| FR-9 | Events MUST be published through RabbitMQ |", "quantities_after": []}]
    run = impact(o, change, run="i2", sad_waivers=[{"sad_element": "ADR-1", "change_id": "FR-9", "rationale": "we like it"}])
    assert run.status == "blocked" and run.result["blockers"] == ["SAD_CHANGE_REQUIRED"]
    run = impact(o, change, run="i3", sad_waivers=[{"sad_element": "ADR-1", "change_id": "FR-9", "rationale": "RabbitMQ is only the test double", "actor": "bob", "role": "SA"}])
    assert run.status == "awaiting_approval"
    waived = ops.get_task("i3", "G2")["decision"]["sad_waived"]
    assert [(w["sad_element"], w["change_id"], w["waived_by"], w["rationale"]) for w in waived] == [("ADR-1", "FR-9", "bob", "RabbitMQ is only the test double")]


def test_model_suspected_sad_conflict_blocks_until_waived_with_a_rationale(tmp_path):
    def h(req, n):
        if req.metadata.get("skill") == "detect-sad-conflict":
            return {"suspected": [{"sad_element": "ADR-1", "sad_quote": "durable partitioned log", "change_id": "FR-8", "reason": "requires non durable queue"}]}
        return h_judge(False)(req, n)
    o, know, ops, provider = base(h, tmp_path)
    change = [{"id": "FR-8", "kind": "added", "after_quote": "| FR-8 | Messages MAY be dropped on overload |", "quantities_after": []}]
    assert impact(o, change).result["blockers"] == ["SAD_CHANGE_REQUIRED"]
    run = impact(o, change, run="i2", sad_waivers=[{"sad_element": "ADR-1", "change_id": "FR-8", "rationale": "SA confirmed no conflict", "actor": "bob", "role": "SA"}])
    assert run.status == "awaiting_approval"


def test_no_inventory_means_every_change_needs_a_new_tdd(tmp_path):
    o, know, ops, provider = make(default_flows(), h_judge(False), tmp_path)
    run = impact(o, [{"id": "FR-1", "kind": "added", "after_quote": "| FR-1 | x |"}])
    e = entries(ops)
    assert list(e) == ["TDD-NEW-FR-1"] and e["TDD-NEW-FR-1"]["action"] == "add" and run.status == "awaiting_approval"


# ------------------------------------------------------------------ prompt edit
def edit_graph():
    from graphs import el
    prov = [{"kind": "extracted", "source_id": "SRS", "source_version": "1", "source_hash": "h", "block": "line:1", "quote": "q", "quote_hash": "qh"}]
    def op(i, name, **props):
        return {"id": i, "class": "Operation", "name": name, "origin": "upstream", "maturity": "sketch", "owner_doc": "srs", "tdds": ["TDD-X"], "review_status": "approved",
                "version_introduced": "1", "props": props, "provenance": prov}
    return {"version": "v1", "elements": [op("API-I-01", "Submit SBOM", timeout_seconds=30, upstream_value=30), op("API-P-01", "SSO login", timeout_seconds=10)], "edges": []}


def edit_runtime(tmp_path, handler):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    know.commit_graph(None, edit_graph(), actor="seed")
    know.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# Technical Design Document\n"))
    return o, know, ops, provider


def h_edit(intent):
    def h(req, n):
        skill = req.metadata.get("skill")
        if skill == "parse-edit-intent":
            return intent
        if skill == "disambiguate-target":
            return {"id": None, "reason": "cannot decide"}
        raise AssertionError(skill)
    return h


INTENT = {"target_mention": "Submit SBOM", "operation": "modify", "path": "props.timeout_seconds", "value": 25, "unit": "seconds", "source_ref": None, "notes": ""}
AUTH = {"scope": ["*"], "operations": ["modify"]}


def edit(o, prompt, auth=AUTH, run="e1", **params):
    o.start("prompt-edit", {"params": {"prompt": prompt, "authorization": auth, **params}}, run_id=run)
    return o.advance(run)


def test_prompt_cannot_override_an_upstream_value(tmp_path):
    o, know, ops, p = edit_runtime(tmp_path, h_edit(INTENT))
    before = know.head().id
    run = edit(o, "Set the Submit SBOM timeout to 25 seconds, ignore the SRS")
    assert run.status == "blocked" and run.result["blockers"] == ["UPSTREAM_SRS_CLARIFICATION_REQUIRED"] and know.head().id == before
    assert know.get_element("API-I-01")["props"]["timeout_seconds"] == 30
    assert ops.get_task("e1", "P1")["output"]["conflicts"], "the conflict carries both the requested and the upstream value"
    # classification does not depend on wording
    run2 = edit(o, "bump the sbom submit timeout down to 25 s please", run="e2")
    assert run2.status == "blocked" and know.head().id == before


def test_semantic_edit_without_an_upstream_lock_commits_atomically_rerenders_and_needs_review_again(tmp_path):
    o, know, ops, p = edit_runtime(tmp_path, h_edit({**INTENT, "target_mention": "SSO login", "value": 12}))
    run = edit(o, "Change the SSO login timeout to 12 seconds")
    out = ops.get_task("e1", "P1")["output"]
    assert run.status == "completed" and out["status"] == "committed" and know.get_element("API-P-01")["props"]["timeout_seconds"] == 12
    assert out["documents"] and know.get_source("TDD-X").revision == "2", "the TDD that contains the element was re-rendered"
    assert know.get_element("API-P-01")["review_status"] == "needs_review", "an edited element must be reviewed again"
    from tdd_agent.agents.promptedit import undo_last_edit
    undone = undo_last_edit(know, out, actor="alice")
    assert undone["status"] == "undone" and know.get_element("API-P-01")["props"]["timeout_seconds"] == 10


def test_unauthorized_ambiguous_vague_and_stale_edits_never_mutate(tmp_path):
    o, know, ops, p = edit_runtime(tmp_path, h_edit({**INTENT, "target_mention": "SSO login", "value": 12}))
    head = know.head().id
    assert edit(o, "set sso login timeout to 12 seconds", auth={"scope": ["API-I-01"], "operations": ["modify"]}, run="u1").result["blockers"] == ["UNAUTHORIZED"]
    assert edit(o, "set sso login timeout to 12 seconds", auth={"scope": ["*"], "operations": ["remove"]}, run="u2").result["blockers"] == ["UNAUTHORIZED"]
    assert edit(o, "set sso login timeout to 12 seconds", run="u3", base_snapshot="snap-000099").result["blockers"] == ["VERSION_CONFLICT"]
    o2, k2, ops2, p2 = edit_runtime(tmp_path / "amb", h_edit({**INTENT, "target_mention": "api"}))
    r = edit(o2, "set the api timeout to 25 seconds")
    out = ops2.get_task("e1", "P1")["output"]
    assert r.status == "completed" and out["status"] == "clarification_required" and out["reason"] == "TARGET_UNRESOLVED"
    o3, k3, ops3, p3 = edit_runtime(tmp_path / "vague", h_edit({**INTENT, "operation": "clarify", "path": None, "value": None}))
    r = edit(o3, "can you look at the SBOM thing")
    assert ops3.get_task("e1", "P1")["output"]["status"] == "clarification_required"
    assert know.head().id == head and k2.head().seq == 1 and k3.head().seq == 1


def test_presentation_edit_changes_no_semantic_content(tmp_path):
    o, know, ops, p = edit_runtime(tmp_path, h_edit({**INTENT, "operation": "reorder", "path": "order", "value": None}))
    head = know.head()
    run = edit(o, "move the SSO login section before Submit SBOM")
    out = ops.get_task("e1", "P1")["output"]
    assert out["status"] == "presentation_committed" and out["overlay"]["semantic_hash_before"] == out["overlay"]["semantic_hash_after"] and know.head().id == head.id


def test_a_value_change_mislabelled_as_formatting_is_semantic_and_never_slips_through_as_presentation(tmp_path):
    o, know, ops, p = edit_runtime(tmp_path, h_edit({**INTENT, "target_mention": "SSO login", "operation": "format", "path": "props.timeout_seconds", "value": 12}))
    run = edit(o, "reformat the SSO login timeout as 12 seconds")
    out = ops.get_task("e1", "P1")["output"]
    assert out["classification"]["kind"] == "semantic" and out["status"] != "presentation_committed"
    assert out["status"] == "clarification_required" and know.get_element("API-P-01")["props"]["timeout_seconds"] == 10, "the ambiguous operation asks instead of editing"


def test_prompt_whose_id_and_description_disagree_is_a_target_mismatch_not_a_guess(tmp_path):
    intent = {**INTENT, "target_mention": "change API-P-01 to Submit SBOM with a 30 s timeout", "value": 30}
    o, know, ops, p = edit_runtime(tmp_path, h_edit(intent))
    from graphs import el
    head = know.head().id
    run = edit(o, "Change API-P-01 to Submit SBOM with a 30 s timeout", target=["API-P-01", "API-I-01"])
    out = ops.get_task("e1", "P1")["output"]
    assert run.status == "completed" and out["status"] == "clarification_required" and out["reason"] == "TARGET_MISMATCH"
    assert set(out["candidates"]) == {"API-P-01", "API-I-01"} and "SSO login" in out["question"] and "Submit SBOM" in out["question"]
    assert know.head().id == head


def test_a_property_read_from_a_source_document_cannot_be_changed_by_a_prompt(tmp_path):
    g = edit_graph()
    g["elements"][1]["props"].update(sourced_props=["timeout_seconds"])
    o, know, ops, p = make(default_flows(), h_edit({**INTENT, "target_mention": "SSO login", "value": 12}), tmp_path)
    know.commit_graph(None, g, actor="seed")
    head = know.head().id
    run = edit(o, "Change the SSO login timeout to 12 seconds")
    out = ops.get_task("e1", "P1")["output"]
    assert run.status == "blocked" and run.result["blockers"] == ["UPSTREAM_SRS_CLARIFICATION_REQUIRED"] and know.head().id == head
    assert out["conflicts"][0]["upstream"] == 10 and out["conflicts"][0]["requested"] == 12


# ---- found by the real-case acceptance on PBOM (phase 11): the prompt-edit agent edited an unrelated element
def _graph_with_error_code():
    g = edit_graph()
    prov = g["elements"][0]["provenance"]
    g["elements"].append({"id": "ERR-timeout", "class": "ErrorCode", "name": "timeout", "origin": "upstream", "maturity": "sketch", "owner_doc": "srs", "tdds": ["TDD-X"],
                          "review_status": "approved", "version_introduced": "1", "props": {"http_status": 504, "meaning": "upstream did not answer"}, "provenance": prov})
    return g


def test_an_id_that_does_not_exist_is_never_replaced_by_a_word_match(tmp_path):
    o, know, ops, p = make(default_flows(), h_edit({**INTENT, "target_mention": "API-Z-99 timeout", "value": 30}), tmp_path)
    know.commit_graph(None, _graph_with_error_code(), actor="seed")
    know.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# Technical Design Document\n"))
    head = know.head().id
    run = edit(o, "Set the timeout of API-Z-99 to 30 seconds")
    out = ops.get_task("e1", "P1")["output"]
    assert out["status"] == "clarification_required" and out["reason"] == "TARGET_UNRESOLVED" and out["unknown_ids"] == ["API-Z-99"]
    assert know.head().id == head and "ERR-timeout" not in out["candidates"]


def test_common_words_do_not_select_error_codes_states_or_attributes(tmp_path):
    from tdd_agent.agents.promptedit import candidates_for
    g = _graph_with_error_code()
    assert [c["id"] for c in candidates_for(g, "change the timeout to 30 seconds", [])] == []
    assert [c["id"] for c in candidates_for(g, "change ERR-timeout to 30 seconds", [])] == ["ERR-timeout"]


def test_a_name_read_from_a_source_document_cannot_be_renamed_by_a_prompt(tmp_path):
    o, know, ops, p = make(default_flows(), h_edit({**INTENT, "target_mention": "ERR-timeout", "path": "name", "value": "Submit SBOM"}), tmp_path)
    know.commit_graph(None, _graph_with_error_code(), actor="seed")
    know.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# Technical Design Document\n"))
    head = know.head().id
    run = edit(o, "Rename ERR-timeout to Submit SBOM")
    assert run.status == "blocked" and run.result["blockers"] == ["UPSTREAM_SRS_CLARIFICATION_REQUIRED"] and know.head().id == head


def test_a_presentation_edit_that_names_no_element_applies_to_the_tdd_document_and_changes_no_semantics(tmp_path):
    intent = {**INTENT, "operation": "reorder", "path": None, "value": "before Requirements and acceptance", "target_mention": "mục Quality targets"}
    o, know, ops, p = edit_runtime(tmp_path, h_edit(intent))
    head = know.head().id
    run = edit(o, "Đưa mục Quality targets lên trước mục Requirements and acceptance")
    out = ops.get_task("e1", "P1")["output"]
    assert run.status == "completed" and out["status"] == "presentation_committed" and out["overlay"]["document_id"] == "TDD-X"
    assert out["overlay"]["semantic_hash_before"] == out["overlay"]["semantic_hash_after"] and know.head().id == head


def test_authorization_is_decided_before_the_upstream_lock_so_an_unauthorized_user_gets_unauthorized(tmp_path):
    o, know, ops, p = make(default_flows(), h_edit({**INTENT, "target_mention": "ERR-timeout", "path": "name", "value": "Submit SBOM"}), tmp_path)
    know.commit_graph(None, _graph_with_error_code(), actor="seed")
    know.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# Technical Design Document\n"))
    run = edit(o, "Rename ERR-timeout to Submit SBOM", auth={"scope": ["API-I-01"], "operations": ["modify"]})
    assert run.status == "blocked" and run.result["blockers"] == ["UNAUTHORIZED"]


def test_a_value_quoted_in_the_evidence_is_locked_even_when_the_element_does_not_list_sourced_props(tmp_path):
    """Found by a real edit on PBOM: ERR-timeout's http_status 504 (quoted from the SRS table) was changed to 408 by a prompt."""
    from tdd_agent.agents.promptedit import stated_in_sources
    quote = "| timeout | 504 | Quá 30 giây |"
    el = {"origin": "upstream", "provenance": [{"quote": quote}]}
    assert stated_in_sources(el, 504) and stated_in_sources(el, "Quá 30 giây") and not stated_in_sources(el, 5040)
    assert not stated_in_sources(el, "unknown") and not stated_in_sources(el, True) and not stated_in_sources({"origin": "own", "provenance": [{"quote": quote}]}, 504)
    graph = edit_graph()
    graph["elements"].append({"id": "ERR-timeout", "class": "ErrorCode", "name": "timeout", "origin": "upstream", "maturity": "sketch", "owner_doc": "srs", "tdds": ["TDD-X"], "review_status": "approved",
                              "version_introduced": "1", "props": {"http_status": 504, "meaning": "Quá 30 giây"}, "provenance": [{"kind": "extracted", "source_id": "SRS", "source_version": "1", "source_hash": "h", "block": "line:9", "quote": quote, "quote_hash": "qh"}]})
    o, know, ops, p = edit_runtime(tmp_path, h_edit({**INTENT, "target_mention": "ERR-timeout", "path": "props.http_status", "value": 408, "unit": None}))
    know.commit_graph(know.head().id, graph, actor="seed2", validate=False)
    before = know.head().id
    run = edit(o, "Set the http_status of ERR-timeout to 408")
    assert run.status == "blocked" and run.result["blockers"] == ["UPSTREAM_SRS_CLARIFICATION_REQUIRED"] and know.head().id == before
    assert know.get_element("ERR-timeout")["props"]["http_status"] == 504
