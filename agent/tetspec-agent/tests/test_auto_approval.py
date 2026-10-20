"""Automatic approval mode: the whole change flow runs without a person, except where the rules, the read-only SAD or the evidence stop it."""
from __future__ import annotations

import pytest

from harness import ACTORS, make
from test_agents_change_flow import approve, baseline, handler, repo, start_change  # noqa: F401  (repo is a fixture)
from tdd_agent.agents.flows import default_flows
from tdd_agent.config import ConfigError, load_config
from tdd_agent.obs import bind
from tdd_agent.orchestrator import ApprovalRequest


def auto_runtime(tmp_path, **approvals):
    return make(default_flows(), handler, tmp_path, overrides={"approvals": {"mode": "auto", **approvals}})


def test_the_mode_is_auto_unless_the_project_asked_for_approvers_or_for_manual():
    resolve = lambda **ov: load_config(overrides={"approvals": ov}, env={}).approval_mode()  # noqa: E731
    assert resolve() == "auto", "the default: nobody is asked"
    assert resolve(actors=ACTORS) == "manual", "a project that registered approvers wants them"
    assert resolve(mode="auto", actors=ACTORS) == "auto" and resolve(mode="manual") == "manual"
    with pytest.raises(ConfigError) as e:
        load_config(overrides={"approvals": {"mode": "sometimes"}}, env={})
    assert e.value.code == "CONFIG_APPROVAL_MODE"
    with pytest.raises(ConfigError) as e:
        load_config(overrides={"approvals": {"required": ["G6"]}}, env={})
    assert e.value.code == "CONFIG_APPROVAL_REQUIRED", "G6 is a gate of evidence, never of approval"


def test_the_change_flow_runs_from_start_to_the_evidence_gate_and_on_to_acceptance_without_a_person(tmp_path, repo):
    o, know, ops, provider = auto_runtime(tmp_path)
    baseline(o, ops)
    start_change(o, repo)
    run = o.advance("cr1")
    assert run.status == "awaiting_evidence" and run.state == "spec_authored", (run.status, run.result)
    approvals = ops.list_approvals("cr1")
    assert [(a["gate"], a["actor_id"], a["actor_kind"], a["approval_grade"]) for a in approvals] == [("G1", "tdd-agent:auto", "automatic", "auto"), ("G2", "tdd-agent:auto", "automatic", "auto"),
                                                                                                  ("G5", "tdd-agent:auto", "automatic", "auto")]
    assert all(a["decision_hash"] for a in approvals) and "auto" in approvals[0]["rationale"]
    assert (repo / "openspec" / "changes" / "expiry-notifications" / "trace-manifest.json").is_file(), "the bundle was published"
    assert not [e for e in know.materialize()["elements"] if e.get("review_status") == "needs_review" and e["class"] in ("Requirement", "Component")], "the facts the spec reads were accepted by the engine"
    assert ops.list_audit(kind="elements_auto_reviewed") and ops.list_audit(kind="gate_auto_approved")
    assert o.advance("cr1").status == "awaiting_evidence", "G6 never passes without a receipt"
    o.record_test_receipt("cr1", "CS-001", ["AC-3"], "pass", "bob")
    o.record_test_receipt("cr1", "CS-002", ["AC-3"], "pass", "bob")
    run = o.advance("cr1")
    assert run.status == "completed" and run.state == "accepted", (run.status, run.result)
    assert [a["gate"] for a in ops.list_approvals("cr1")][-1] == "G7"
    assert know.get_element("CR-1/acceptance")["props"]["status"].startswith("accepted") and know.get_element("CR-1/acceptance")["props"]["accepted_by"] == "tdd-agent:auto"


def test_the_rules_still_stop_the_flow_in_auto_mode(tmp_path, repo):
    o, know, ops, provider = auto_runtime(tmp_path)
    baseline(o, ops)
    start_change(o, repo, code_targets={}, verifications={})
    run = o.advance("cr1")
    assert run.status == "blocked" and run.result["blocked_gate"] == "G4" and any(b.startswith("CS_") for b in run.result["blockers"])
    assert [a["gate"] for a in ops.list_approvals("cr1")] == ["G1", "G2"], "approval of a gate never covers a gate that did not pass"


def test_required_gates_stop_for_a_person_and_the_rest_still_run(tmp_path, repo):
    o, know, ops, provider = auto_runtime(tmp_path, required=["G5"], actors=ACTORS)
    baseline(o, ops)
    start_change(o, repo)
    run = o.advance("cr1")
    assert run.status == "blocked" and run.result["blocked_gate"] == "G5", "the person who will approve the bundle reviews the facts first: the engine does not do it for them"
    pending = [e["id"] for e in o.elements_needing_review()]
    assert o.review_elements(pending, "approved", "bob", "SA", "facts match the sources")["changed"] == len(pending)
    run = o.advance("cr1")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G5"
    assert not [a for a in ops.list_approvals("cr1") if a["gate"] == "G5"] and not (repo / "openspec" / "changes" / "expiry-notifications").exists()
    assert approve(o, "cr1", "G5", "bob", "SA").accepted
    assert o.get_run("cr1").status == "awaiting_evidence"
    assert {a["gate"]: a["approval_grade"] for a in ops.list_approvals("cr1")} == {"G1": "auto", "G2": "auto", "G5": "human"}
    assert not ops.list_audit(kind="elements_auto_reviewed"), "a gate a person approves is not preceded by an automatic review of its facts"


def test_manual_mode_keeps_waiting_and_a_pipeline_agent_cannot_approve_in_either_mode(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    baseline(o, ops)
    start_change(o, repo)
    run = o.advance("cr1")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G1"
    decision = o.get_decision("cr1", "G1", "alice")
    with bind(agent="impact"):
        res = o.submit_approval(ApprovalRequest("cr1", "G1", "alice", "BA", decision["decision_hash"], "self-approval"))
    assert not res.accepted and res.code == "PIPELINE_AGENT_CANNOT_APPROVE"


def test_switching_to_auto_while_a_gate_waits_approves_it_on_the_next_advance(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    baseline(o, ops)
    start_change(o, repo)
    assert o.advance("cr1").result["pending_gate"] == "G1"
    o.config.data["approvals"]["mode"] = "auto"
    run = o.advance("cr1")
    assert run.status == "awaiting_evidence", "the gate that was waiting, and the ones after it, went through"
    assert [a["approval_grade"] for a in ops.list_approvals("cr1")][0] == "auto" and ops.get_task("cr1", "G0")["status"] != "stale", "the mode is not an input of the gates"


def test_the_published_bundle_and_the_status_say_who_approved(tmp_path, repo):
    o, know, ops, provider = auto_runtime(tmp_path)
    baseline(o, ops)
    start_change(o, repo)
    o.advance("cr1")
    import json
    manifest = json.loads((repo / "openspec" / "changes" / "expiry-notifications" / "trace-manifest.json").read_text())
    assert [(a["gate"], a["actor_id"], a["approval_grade"]) for a in manifest["approvals"]] == [("G1", "tdd-agent:auto", "auto"), ("G2", "tdd-agent:auto", "auto"), ("G5", "tdd-agent:auto", "auto")]


def test_a_project_with_no_approver_can_still_record_test_receipts_under_the_name_of_who_ran_them(tmp_path, repo):
    o, know, ops, provider = auto_runtime(tmp_path, actors=[])
    baseline(o, ops)
    start_change(o, repo)
    assert o.advance("cr1").status == "awaiting_evidence"
    from tdd_agent.errors import TddAgentError
    with pytest.raises(TddAgentError) as e:
        o.record_test_receipt("cr1", "CS-001", ["AC-3"], "pass", "  ")
    assert e.value.code == "ACTOR_NOT_ALLOWLISTED"
    o.record_test_receipt("cr1", "CS-001", ["AC-3"], "pass", "dev-tester", evidence_ref="ci-run-42")
    o.record_test_receipt("cr1", "CS-002", ["AC-3"], "pass", "dev-tester")
    run = o.advance("cr1")
    assert run.status == "completed" and know.get_element("CR-1/acceptance")["props"]["accepted_by"] == "tdd-agent:auto"
    receipt = [e for e in know.materialize()["elements"] if e["class"] == "TestReceipt"][0]
    assert receipt["props"]["recorded_by"] == "dev-tester" and receipt["props"]["recorded_by_declared"] is False
