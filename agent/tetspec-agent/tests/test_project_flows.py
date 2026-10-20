"""Project mode, part 2: change request (impact -> change spec -> OpenSpec), editing the TDD with a prompt, and the MCP registration.

Everything except the model is real (stores, retrieval, skills, o5 gates, openspec CLI); the model answers are scripted.
"""
from __future__ import annotations

import json
import subprocess
from types import SimpleNamespace

import pytest

from harness import prompt_blocks
from test_agents_change_flow import SAD, SRS_V1, SRS_V2, URD, handler
from tdd_agent import project
from tdd_agent.agents.flows import default_flows
from tdd_agent.config import load_config
from tdd_agent.errors import TddAgentError
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.obs import Observer
from tdd_agent.orchestrator import Orchestrator
from tdd_agent.store.base import SourceDoc
from tdd_agent.store.memory import MemoryKnowledgeStore, MemoryOperationalStore

APPROVERS = ["alice:human:BA,BA/owner", "bob:human:SA/Dev,SA", "claude:coding_agent"]


def runtime(root, answers, mode="manual"):
    config = load_config(root / ".tdd-agent" / "config.toml", base_dir=root, env={}, overrides={"paths": {"work_dir": str(root / ".tdd-agent")}, "approvals": {"mode": mode}})
    ops, know = MemoryOperationalStore(), MemoryKnowledgeStore()
    ob = Observer(ops)
    gateway = LlmGateway(config, ob, {"claude_code": FakeProvider(answers)})
    return SimpleNamespace(config=config, knowledge=know, operational=ops, orchestrator=Orchestrator(config, know, ops, gateway, ob, default_flows()), observer=ob)


@pytest.fixture()
def proj(tmp_path):
    root = tmp_path / "P"
    (root / "docs").mkdir(parents=True)
    for name, text in (("URD", URD), ("SRS", SRS_V1), ("SAD", SAD)):
        (root / "docs" / f"{name}.md").write_text(text, encoding="utf-8")
    project.init_project(root, name="P", sources={r: str(root / "docs" / f"{r.upper()}.md") for r in ("urd", "srs", "sad")}, approvers=APPROVERS)
    return root


CR_TEXT = ("# CR-1 Keep evidence for five years\nEvidence must be kept for 5 years.\n\n"
           "- AC-3: GIVEN stored evidence WHEN 5 years have passed THEN it may be purged\n")
CR_EDIT = {"op": "modify", "id": "FR-1", "find": "store SBOM evidence", "replace": "store SBOM evidence for 5 years", "near_id": None, "statement": None, "acceptance_ids": ["AC-3"],
           "cr_quote": "Evidence must be kept for 5 years.", "rationale": "the retention period is new"}
CR_ADD_TEXT = ("# CR-1 Expiry notifications\nOwners must be notified before an entitlement expires.\n\n"
               "- AC-3: GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified\n")
CR_ADD = {"op": "add", "id": None, "find": None, "replace": None, "near_id": "FR-1", "statement": "The system SHALL notify owners about expiry.", "acceptance_ids": ["AC-3"],
          "cr_quote": "Owners must be notified before an entitlement expires.", "rationale": "a requirement the SRS does not state"}


def answers(edits=None, unmatched=None):
    """Scripted model: the change-request flow of the fixtures, plus the reading of the CR."""
    def h(req, n):
        if req.metadata.get("skill") == "propose-srs-edits-from-cr":
            return {"edits": [CR_EDIT] if edits is None else edits, "unmatched": unmatched or []}
        return handler(req, n)
    return h


def generated(proj, model=None):
    rt = runtime(proj, model or answers())
    project.generate(rt, {}, tdd_id=None, out=None, prose=False, prose_limit=None, force=False)
    return rt


def edit_srs(proj):
    (proj / "docs" / "SRS.md").write_text(SRS_V2, encoding="utf-8")


def start(rt, **kw):
    args = dict(cr_file=None, cr_text=CR_TEXT, title=None, acceptance=[], cr_id=None, srs_after=None, purpose=None, scope=None, owner="pbom-team",
                code_targets=["FR-1=src/evidence.py::retain"], verifications=["FR-1=GIVEN stored evidence WHEN 5 years have passed THEN it may be purged"],
                new_in=[], allow_draft=False)
    args.update(kw)
    return project.change_start(rt, "expiry-notifications", **args)


def test_init_registers_approvers_with_their_roles(proj):
    config = load_config(proj / ".tdd-agent" / "config.toml", base_dir=proj, env={})
    assert {a.id: (a.kind, a.roles) for a in config.actors()} == {"alice": ("human", ("BA", "BA/owner")), "bob": ("human", ("SA/Dev", "SA")), "claude": ("coding_agent", ("BA", "SA/Dev", "SA", "BA/owner"))}
    with pytest.raises(TddAgentError) as e:
        project.parse_approver("x:robot")
    assert e.value.code == "ARGUMENT_INVALID"


def test_a_change_needs_a_baseline_a_cr_and_a_valid_name(proj):
    rt = runtime(proj, answers())
    with pytest.raises(TddAgentError) as e:
        start(rt)
    assert e.value.code == "NO_BASELINE_TDD"
    rt = generated(proj)
    with pytest.raises(TddAgentError) as e:
        start(rt, cr_text=None)
    assert e.value.code == "CR_REQUIRED"
    with pytest.raises(TddAgentError) as e:
        project.change_start(rt, "Bad Name", cr_file=None, cr_text=CR_TEXT, title=None, acceptance=[], cr_id=None, srs_after=None, purpose=None, scope=None, owner=None, code_targets=[],
                             verifications=[], new_in=[], allow_draft=False)
    assert e.value.code == "CHANGE_NAME_INVALID"
    with pytest.raises(TddAgentError) as e:
        start(rt, cr_text=CR_TEXT, cr_file=str(proj / "docs" / "SRS.md"))
    assert e.value.code == "ARGUMENT_INVALID", "a CR is a file or text, not both"


def test_the_cr_is_read_against_the_srs_in_force_and_the_ba_confirms_the_resulting_delta(proj):
    rt = generated(proj, answers([CR_ADD]))
    before_text = (proj / "docs" / "SRS.md").read_text(encoding="utf-8")
    result, lines = start(rt, cr_text=CR_ADD_TEXT, code_targets=[], verifications=[])
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G1", "no edited SRS was needed"
    assert (proj / "docs" / "SRS.md").read_text(encoding="utf-8") == before_text, "the project's SRS is never rewritten by the flow"
    prop = result["proposal"]
    assert [(e["op"], e["id"]) for e in prop["edits"]] == [("add", "FR-2")] and prop["edits"][0]["cr_quote"].startswith("Owners must be notified")
    proposed = (proj / ".tdd-agent" / "changes" / "expiry-notifications" / "SRS.proposed.md").read_text(encoding="utf-8")
    assert "| FR-2 | The system SHALL notify owners about expiry. |" in proposed and "AC-3: GIVEN an expiring entitlement" in proposed
    assert "+| FR-2 |" in (proj / ".tdd-agent" / "changes" / "expiry-notifications" / "srs.diff").read_text(encoding="utf-8")
    assert any("Reading of the CR" in line and "1 change(s)" in line for line in lines) and any("← CR:" in line for line in lines)
    decision, _ = project.change_decision(rt, None, "alice")
    read = decision["decision"]["from_change_request"]
    assert read["edits"][0]["id"] == "FR-2" and read["proposed_srs"]["revision"] == "2" and any("+| FR-2 |" in d for d in read["diff"])
    assert decision["decision"]["srs_delta"]["changes"][0]["id"] == "FR-2", "the delta the BA confirms is read back from the proposed SRS"
    srs_docs = [(d.id, d.revision) for d in rt.knowledge.list_sources("SRS")]
    assert srs_docs == [("SRS", "1"), ("SRS", "2")], "the proposal is the next revision of the SRS, registered once"


def test_a_proposal_that_the_cr_does_not_support_is_refused_before_anything_is_registered(proj):
    bad = dict(CR_ADD, statement="The system SHALL notify owners 45 days before expiry.")
    rt = generated(proj, answers([bad]))
    result, lines = start(rt, cr_text=CR_ADD_TEXT)
    assert result["status"] == "blocked" and "EDIT_NUMBER_NOT_IN_CR:#1" in result["blockers"]
    assert [(d.id, d.revision) for d in rt.knowledge.list_sources("SRS")] == [("SRS", "1")], "nothing was registered"
    rt3 = generated(proj, answers([]))  # a model that finds nothing to change
    result, _ = project.change_start(rt3, "no-change", cr_file=None, cr_text=CR_TEXT, title=None, acceptance=[], cr_id=None, srs_after=None, purpose=None, scope=None, owner=None,
                                     code_targets=[], verifications=[], new_in=[], allow_draft=False)
    assert "CR_NO_REQUIREMENT_CHANGE" in result["blockers"]


def test_the_cr_can_be_a_markdown_file_and_what_it_cannot_map_is_reported(proj):
    cr_file = proj / "docs" / "cr-1.md"
    cr_file.write_text(CR_ADD_TEXT + "\nAlso make the logo bigger.\n", encoding="utf-8")
    rt = generated(proj, answers([CR_ADD], unmatched=[{"cr_quote": "Also make the logo bigger.", "reason": "no requirement row about the logo"}]))
    result, lines = start(rt, cr_text=None, cr_file=str(cr_file), code_targets=[], verifications=[])
    assert result["status"] == "awaiting_approval" and result["proposal"]["not_mapped"][0]["cr_quote"] == "Also make the logo bigger."
    assert any("NOT mapped to any requirement" in line and "logo" in line for line in lines)
    cr_doc = [d for d in rt.knowledge.list_sources("CR")]
    assert [(d.id, d.revision) for d in cr_doc] == [("CR-1", "1")], "the CR is a source document with the id it carries"


def test_change_request_from_the_cr_through_impact_realization_and_change_spec_to_the_openspec_gate(proj):
    rt = generated(proj)
    result, lines = start(rt)
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G1" and result["approvers"] == ["alice", "claude"]
    assert (proj / "openspec" / "config.yaml").is_file() and "OpenSpec skeleton" in lines[0], "the skeleton is created and said so"
    assert any("tdd-agent change decision" in line for line in lines)
    with pytest.raises(TddAgentError) as e:
        project.change_approve(rt, None, "bob", "ok", "x" * 8)
    assert e.value.code == "APPROVER_NOT_ALLOWED", "bob is not a BA"
    decision, _ = project.change_decision(rt, None, "alice")
    assert decision["gate"] == "G1"
    approved, _ = project.change_approve(rt, None, "alice", "delta matches the CR", decision["decision_hash"])
    assert approved["accepted"] and approved["role"] == "BA" and approved["pending_gate"] == "G2"
    d2, _ = project.change_decision(rt, None, "claude")
    assert {e["tdd_id"]: e["action"] for e in d2["decision"]["entries"]} == {"TDD-P": "update"}, "the TDD that states the requirement is the one to update"
    approved, lines = project.change_approve(rt, "change-expiry-notifications", "claude", "impact reviewed", d2["decision_hash"])
    assert approved["accepted"] and approved["status"] == "blocked" and approved["blocked_gate"] == "G5"
    assert "docs/tdd/TDD-P.md" in approved["documents_written"], "the realized TDD revision is written into the project"
    text = (proj / "docs" / "tdd" / "TDD-P.md").read_text(encoding="utf-8")
    assert "for 5 years" in text and "Phụ lục" in text
    assert "RULE:TS-10" not in approved["blockers"], "the OpenSpec baseline of a MODIFIED requirement is written from the SRS the change started from"
    assert (proj / "openspec" / "specs" / "expiry-notifications" / "spec.md").is_file()
    why, why_lines = project.change_status(rt, None, why=True)
    assert [c["rule"] for c in why["why"]] == ["QG-03"], "TS-04 only quotes QG-03: one cause, not two"
    assert any("tdd-agent review" in line for line in why_lines) and any("Why it is blocked" in line for line in why_lines)
    assert any("tdd-agent change status --why" in line for line in project.change_status(rt, None)[1])
    pending = [e["id"] for e in rt.orchestrator.elements_needing_review()]
    assert rt.orchestrator.review_elements(pending, "approved", "bob", "SA", "facts match the sources")["changed"] == len(pending)
    result, _ = project.change_advance(rt, None)
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G5", "a modified requirement reaches the SA's approval of the bundle"


def test_verification_is_read_from_the_crs_given_when_then_criteria_and_what_is_missing_can_be_supplied_later(proj):
    rt = generated(proj)
    start(rt, code_targets=[], verifications=[])
    d1, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    d2, _ = project.change_decision(rt, None, "claude")
    result, lines = project.change_approve(rt, None, "claude", "ok", d2["decision_hash"])
    assert result["status"] == "blocked" and result["blocked_gate"] == "G4"
    assert any(b.startswith("CS_CODE_TARGET_MISSING") for b in result["blockers"]), "no code in the project: the target is not invented"
    assert not any(b.startswith("CS_VERIFICATION_MISSING") for b in result["blockers"]), "the CR's AC-3 is written as GIVEN/WHEN/THEN, so the verification comes from it"
    assert any("change update --code-target" in line for line in lines)
    with pytest.raises(TddAgentError) as e:
        project.change_update(rt, None, [], [], [])
    assert e.value.code == "ARGUMENT_INVALID"
    result, lines = project.change_update(rt, None, ["FR-1=src/evidence.py::retain"], [], [])
    assert result["blocked_gate"] == "G5", "G4 passed with the target that was supplied"
    row = [e for e in rt.knowledge.materialize()["elements"] if e["class"] == "ChangeSpecItem"][0]["props"]
    assert row["code_target"] == "src/evidence.py::retain" and "5 years have passed THEN it may be purged" in str(row["verification"])


def test_the_openspec_bundle_is_published_after_the_sa_approves_when_the_spec_is_eligible(proj):
    """The publication machinery, with an SRS edited by hand that also describes the design (the flow of a change that is complete)."""
    rt = generated(proj)
    edit_srs(proj)
    start(rt, srs_after=str(proj / "docs" / "SRS.md"), new_in=["FR-3=TDD-P", "SVC-N-01=TDD-P"], code_targets=["FR-3=src/notify.py::notify_owner", "SVC-N-01=src/api.py::notifications"],
          verifications=["FR-3=GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified", "SVC-N-01=GIVEN an owner WHEN the notification service is queried THEN pending notices are listed"],
          cr_text=CR_ADD_TEXT)
    d1, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    d2, _ = project.change_decision(rt, None, "claude")
    approved, _ = project.change_approve(rt, None, "claude", "ok", d2["decision_hash"])
    assert approved["blocked_gate"] == "G5"
    pending = [e["id"] for e in rt.orchestrator.elements_needing_review()]
    assert rt.orchestrator.review_elements(pending, "approved", "bob", "SA", "facts match the sources")["changed"] == len(pending)
    result, _ = project.change_advance(rt, None)
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G5" and result["approvers"] == ["bob", "claude"]
    d5, _ = project.change_decision(rt, None, "bob")
    done, lines = project.change_approve(rt, None, "bob", "bundle reviewed", d5["decision_hash"])
    assert done["accepted"] and done["role"] == "SA" and done["status"] == "awaiting_evidence"
    published = proj / "openspec" / "changes" / "expiry-notifications"
    assert (published / "proposal.md").is_file() and (published / "trace-manifest.json").is_file()
    assert subprocess.run(["openspec", "validate", "expiry-notifications", "--strict", "--no-interactive"], cwd=proj, capture_output=True).returncode == 0
    assert any("tdd-agent receipt" in line for line in lines), "the next step (test receipts) is stated"
    listing, _ = project.change_list(rt)
    assert [r["run_id"] for r in listing] == ["change-expiry-notifications"]


def test_an_srs_already_edited_by_hand_can_replace_the_derivation_from_the_cr(proj):
    rt = generated(proj)
    edit_srs(proj)
    result, _ = start(rt, srs_after=str(proj / "docs" / "SRS.md"), new_in=["FR-3=TDD-P"], code_targets=["FR-3=src/notify.py::notify_owner"],
                      verifications=["FR-3=GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified"], cr_text=CR_ADD_TEXT)
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G1" and "proposal" not in result
    d, _ = project.change_decision(rt, None, "alice")
    assert d["decision"]["srs_delta"]["changes"][0]["id"] in ("FR-3", "SVC-N-01") and "from_change_request" not in d["decision"]
    same = proj / "docs" / "same.md"
    same.write_text(SRS_V1, encoding="utf-8")
    with pytest.raises(TddAgentError) as e:
        project.change_start(rt, "same-srs", cr_file=None, cr_text=CR_ADD_TEXT, title=None, acceptance=[], cr_id="CR-2", srs_after=str(same), purpose=None, scope=None,
                             owner=None, code_targets=[], verifications=[], new_in=[], allow_draft=False)
    assert e.value.code == "NO_SRS_CHANGE"


def test_allow_draft_writes_a_draft_inside_the_project_and_never_publishes_it(proj):
    rt = generated(proj)
    start(rt, allow_draft=True)
    d1, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    d2, _ = project.change_decision(rt, None, "claude")
    result, lines = project.change_approve(rt, None, "claude", "ok", d2["decision_hash"])
    assert result["status"] == "blocked" and "SPEC_DRAFT_ONLY" in result["blockers"] and result["draft_path"]
    assert any("DRAFT OpenSpec bundle" in line for line in lines)
    assert not (proj / "openspec" / "changes" / "expiry-notifications").exists()


# ---------------------------------------------------------------- edit with a prompt
def seeded(tmp_path):
    from test_agents_impact_edit import edit_graph
    root = tmp_path / "E"
    (root / "docs").mkdir(parents=True)
    project.init_project(root, name="X", tdd_id="TDD-X", sources={})
    rt = runtime(root, None)
    rt.knowledge.commit_graph(None, edit_graph(), actor="seed")
    rt.knowledge.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# old\n"))
    return root, rt


def intent(**over):
    base = {"target_mention": "SSO login", "operation": "modify", "path": "props.timeout_seconds", "value": 12, "unit": "seconds", "source_ref": None, "notes": ""}
    base.update(over)

    def h(req, n):
        skill = req.metadata.get("skill")
        if skill == "parse-edit-intent":
            return base
        if skill == "disambiguate-target":
            return {"id": None, "reason": "cannot decide"}
        raise AssertionError(skill)
    return h


def test_edit_with_a_prompt_updates_the_tdd_file_and_can_be_undone(tmp_path):
    root, rt = seeded(tmp_path)
    rt = _with_answers(root, rt, intent())
    result, lines = project.edit(rt, "Change the SSO login timeout to 12 seconds", scope=[], operations=["modify"], user="alice", undo=None)
    assert result["status"] == "committed" and result["documents_written"] == ["docs/tdd/TDD-X.md"], result
    assert "timeout_seconds: 12" in (root / "docs" / "tdd" / "TDD-X.md").read_text(encoding="utf-8")
    assert "tdd-agent edit --undo edit-1" in lines[-1]
    undone, _ = project.edit(rt, None, scope=[], operations=[], user="alice", undo="edit-1")
    assert undone["status"] == "undone"
    text = (root / "docs" / "tdd" / "TDD-X.md").read_text(encoding="utf-8")
    assert "timeout_seconds: 10" in text and "timeout_seconds: 12" not in text, "the file follows the graph back"


def _with_answers(root, rt, answers):
    config = rt.config
    ob = Observer(rt.operational)
    gateway = LlmGateway(config, ob, {"claude_code": FakeProvider(answers)})
    rt.orchestrator = Orchestrator(config, rt.knowledge, rt.operational, gateway, ob, default_flows())
    return rt


def test_an_edit_the_user_may_not_make_changes_nothing_and_says_how_to_allow_it(tmp_path):
    root, rt = seeded(tmp_path)
    rt = _with_answers(root, rt, intent())
    before = rt.knowledge.head().id
    result, lines = project.edit(rt, "set sso login timeout to 12 seconds", scope=["API-I-01"], operations=["modify"], user="alice", undo=None)
    assert result["status"] != "committed" and "UNAUTHORIZED" in result["blockers"] and rt.knowledge.head().id == before
    assert any("--allow" in line and "--scope" in line for line in lines)
    with pytest.raises(TddAgentError) as e:
        project.edit(rt, "  ", scope=[], operations=["modify"], user="alice", undo=None)
    assert e.value.code == "PROMPT_REQUIRED"


def test_an_ambiguous_edit_asks_a_question_instead_of_guessing(tmp_path):
    root, rt = seeded(tmp_path)
    rt = _with_answers(root, rt, intent(target_mention="api", value=25))
    result, lines = project.edit(rt, "set the api timeout to 25 seconds", scope=[], operations=["modify"], user="alice", undo=None)
    assert result["status"] == "clarification_required" and result["question"] and lines[0].startswith("Nothing changed.")


# ---------------------------------------------------------------- MCP registration
def test_mcp_enable_registers_the_server_for_the_project_and_disable_removes_it(proj):
    out = project.mcp_enable(proj, write=False)
    data = json.loads((proj / ".mcp.json").read_text())
    assert data["mcpServers"]["tdd-agent"] == {"command": "tdd-agent", "args": ["-C", str(proj), "mcp", "serve"]} and "read-only" in out["tools"] and "write:" not in out["tools"]
    config = load_config(proj / ".tdd-agent" / "config.toml", base_dir=proj, env={})
    assert config.get("mcp.write") is False and config.get("mcp.approvals") is False
    (proj / ".mcp.json").write_text(json.dumps({"mcpServers": {"other": {"command": "x"}}}))
    project.mcp_enable(proj, write=True, approvals=True, client_actor="claude")
    data = json.loads((proj / ".mcp.json").read_text())
    assert set(data["mcpServers"]) == {"other", "tdd-agent"}, "an existing registration of another server is kept"
    config = load_config(proj / ".tdd-agent" / "config.toml", base_dir=proj, env={})
    assert config.get("mcp.write") is True and config.get("mcp.approvals") is True and config.get("mcp.client_actor") == "claude"
    assert config.get("project.name") == "P" and [a.id for a in config.actors()] == ["alice", "bob", "claude"], "the rest of the configuration is untouched"
    assert (proj / ".tdd-agent" / "config.toml").read_text().count("[mcp]") == 1
    project.mcp_disable(proj)
    data = json.loads((proj / ".mcp.json").read_text())
    assert set(data["mcpServers"]) == {"other"}
    config = load_config(proj / ".tdd-agent" / "config.toml", base_dir=proj, env={})
    assert config.get("mcp.write") is False and config.get("mcp.approvals") is False


def test_mcp_approvals_need_a_registered_client_identity(proj):
    with pytest.raises(TddAgentError) as e:
        project.mcp_enable(proj, write=False, approvals=True, client_actor="")
    assert e.value.code == "ARGUMENT_INVALID"
    with pytest.raises(TddAgentError) as e:
        project.mcp_enable(proj, write=False, approvals=True, client_actor="mallory")
    assert "mallory" in str(e.value) and not (proj / ".mcp.json").exists(), "nothing is written when the request is refused"


def test_baseline_writes_the_requirements_a_change_modifies_from_the_srs_it_started_from(proj):
    rt = generated(proj)
    start(rt)
    decision, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "the delta matches the CR", decision["decision_hash"])
    result, lines = project.baseline(rt, None)
    spec = (proj / "openspec" / "specs" / "expiry-notifications" / "spec.md").read_text(encoding="utf-8")
    assert result["written"] == ["FR-1"] and "### Requirement: FR-1\nThe system MUST store SBOM evidence." in spec and "for 5 years" not in spec
    again, _ = project.baseline(rt, None)
    assert again["written"] == [] and again["kept"] == ["FR-1"]
    forced, _ = project.baseline(rt, None, force=True)
    assert forced["written"] == ["FR-1"] and spec.count("### Requirement: FR-1") == 1


def test_the_sa_can_waive_a_sad_finding_with_a_reason_and_it_is_part_of_the_impact_decision(proj):
    edit = dict(CR_ADD, statement="The system SHALL publish events through RabbitMQ.", cr_quote="Events must go through RabbitMQ.")
    cr = "# CR-1 Events\nEvents must go through RabbitMQ.\n\n- AC-3: GIVEN an event WHEN it is published THEN a consumer receives it\n"
    rt = generated(proj, answers([edit]))
    start(rt, cr_text=cr, code_targets=[], verifications=[])
    d1, _ = project.change_decision(rt, None, "alice")
    result, _ = project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    assert result["blocked_gate"] == "G2" and "SAD_CHANGE_REQUIRED" in result["blockers"]
    for who, reason, code in (("alice", "fine", "WAIVE_NOT_PERMITTED"), ("bob", "", "RATIONALE_REQUIRED")):
        with pytest.raises(TddAgentError) as e:
            project.change_waive(rt, None, "ADR-1", "FR-2", who, reason)
        assert e.value.code == code
    with pytest.raises(TddAgentError) as e:
        project.change_waive(rt, None, "ADR-9", "FR-2", "bob", "x")
    assert e.value.code == "NO_SUCH_FINDING"
    result, _ = project.change_waive(rt, None, "ADR-1", "FR-2", "bob", "RabbitMQ is only the test double")
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G2"
    d2, _ = project.change_decision(rt, None, "claude")
    assert d2["decision"]["sad_waived"][0]["waived_by"] == "bob" and d2["decision"]["sad_waived"][0]["rationale"] == "RabbitMQ is only the test double"


def test_one_command_starts_a_change_from_the_cr_names_it_and_continues_it_when_run_again(proj):
    rt = generated(proj, answers([CR_ADD]))
    cr_file = proj / "docs" / "cr-1.md"
    cr_file.write_text(CR_ADD_TEXT, encoding="utf-8")
    asked = []
    result, lines = project.change_run(rt, str(cr_file), precheck=False, ask=lambda q: asked.append(q) or "pbom-team")
    assert result["name"] == "cr-1-expiry-notifications" and result["status"] == "awaiting_approval" and result["pending_gate"] == "G1" and len(asked) == 1
    again, _ = project.change_run(rt, str(cr_file), precheck=False, ask=lambda q: asked.append(q) or "x")
    assert again["run_id"] == "change-cr-1-expiry-notifications" and len(asked) == 1, "the same CR is the same change: it continues and asks nothing"
    assert [r["run_id"] for r in project.change_list(rt)[0]] == ["change-cr-1-expiry-notifications"]
    cr_file.write_text(CR_ADD_TEXT + "\nAlso notify the SA.\n", encoding="utf-8")
    other, _ = project.change_run(rt, str(cr_file), precheck=False)
    assert other["name"] == "cr-1-expiry-notifications-2", "another text of the CR is another change, and the folder name never collides"
    with pytest.raises(TddAgentError) as e:
        project.change_run(rt, str(cr_file), name="cr-1-expiry-notifications", precheck=False)
    assert e.value.code == "RUN_EXISTS"


def test_a_change_without_an_owner_falls_back_to_the_project_and_says_so(proj):
    rt = generated(proj, answers([CR_ADD]))
    cr_file = proj / "docs" / "cr.md"
    cr_file.write_text(CR_ADD_TEXT, encoding="utf-8")
    result, lines = project.change_run(rt, str(cr_file), precheck=False)
    assert any("No owner was given" in line for line in lines)


def test_the_precheck_stops_before_any_model_call_when_a_tool_is_missing(proj, monkeypatch):
    rt = generated(proj, answers([CR_ADD]))
    cr_file = proj / "docs" / "cr.md"
    cr_file.write_text(CR_ADD_TEXT, encoding="utf-8")
    monkeypatch.setattr("shutil.which", lambda cmd: None)
    with pytest.raises(TddAgentError) as e:
        project.change_run(rt, str(cr_file))
    assert e.value.code == "PRECHECK_FAILED" and any(c["name"] == "openspec" for c in e.value.details["checks"])
    assert project.change_list(rt)[0] == [], "nothing was started"


def test_a_cr_without_acceptance_criteria_starts_and_is_asked_for_them_at_the_gate_that_needs_them(proj):
    cr = "# CR-1 Expiry notifications\nOwners must be notified before an entitlement expires.\n"
    rt = generated(proj, answers([dict(CR_ADD, acceptance_ids=[])]))
    result, _ = start(rt, cr_text=cr, code_targets=["FR-2=src/notify.py::notify_owner"], verifications=["FR-2=GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified"])
    assert result["status"] == "awaiting_approval" and result["pending_gate"] == "G1", "the CR was read; criteria are not needed yet"
    d1, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    d2, _ = project.change_decision(rt, None, "claude")
    result, lines = project.change_approve(rt, None, "claude", "ok", d2["decision_hash"])
    assert result["blocked_gate"] == "G4" and "CR_ACCEPTANCE_MISSING" in result["blockers"]
    readings = lambda: [c for c in rt.operational.list_llm_calls("change-expiry-notifications") if c["skill"]["id"] == "propose-srs-edits-from-cr"]
    reads = len(readings())
    result, lines = project.change_update(rt, None, [], [], [], acceptance=["AC-3"], owner="pbom-team")
    assert result["blocked_gate"] != "G4" and len(readings()) == reads == 1, "the CR was not read again"
    assert (rt.operational.get_task("change-expiry-notifications", "G0") or {}).get("status") != "stale"


def test_review_md_clean_and_the_cost_of_a_run(proj):
    rt = generated(proj, answers([CR_ADD]))
    result, _ = start(rt, cr_text=CR_ADD_TEXT, code_targets=[], verifications=[])
    review = (proj / result["review"]).read_text(encoding="utf-8")
    assert "# Review of the change expiry-notifications" in review and "FR-2 (added)" in review and "## Impact" not in review, "only what the gates have recorded so far"
    d1, _ = project.change_decision(rt, None, "alice")
    result, _ = project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    assert "## Impact on the TDDs" in (proj / result["review"]).read_text(encoding="utf-8") and "## Approvals" in (proj / result["review"]).read_text(encoding="utf-8")
    cost = project.run_cost(rt, "change-expiry-notifications")
    assert cost["calls"] >= 1 and "propose-srs-edits-from-cr" in cost["by_skill"] and any("model call" in line for line in project.cost_lines(cost))
    work = proj / ".tdd-agent"
    (proj / ".tdd-runs" / "old").mkdir(parents=True); (proj / ".tdd-runs" / "old" / "x.md").write_text("x")
    (work / "staging" / "gone-run").mkdir(parents=True); (work / "changes" / "never-started").mkdir(parents=True)
    listed, lines = project.clean(rt)
    assert {i["path"] for i in listed["candidates"]} == {".tdd-runs", ".tdd-agent/staging/gone-run", ".tdd-agent/changes/never-started"} and (proj / ".tdd-runs").exists()
    assert (work / "changes" / "expiry-notifications" / "review.md").exists(), "the change that exists is not a candidate"
    done, _ = project.clean(rt, yes=True)
    assert not (proj / ".tdd-runs").exists() and not (work / "staging" / "gone-run").exists() and (proj / "docs" / "tdd").is_dir() and (work / "changes" / "expiry-notifications" / "review.md").exists()


def test_the_number_of_model_calls_is_estimated_from_the_documents_without_the_model(proj):
    rt = runtime(proj, {})
    prose = "\n".join(f"- The service MUST keep the audit record number {i} for 5 years." for i in range(60))
    (proj / "docs" / "SRS.md").write_text(SRS_V1 + "\n" + prose + "\n", encoding="utf-8")
    est, line = project.estimate_generate(rt.config, {}, prose=True, prose_limit=40)
    assert est["by_document"]["SRS"] == 2 and est["model_calls"] == 2 and "about 2 call(s)" in line, "40 passages (the limit) in batches of 25"
    est, line = project.estimate_generate(rt.config, {}, prose=False)
    assert est["model_calls"] == 0 and "No model call" in line


def proposing(base):
    """The scripted model of the fixtures plus the two skills that fill what a CR left out."""
    import re

    def h(req, n):
        skill = req.metadata.get("skill")
        if skill == "propose-acceptance-from-cr":
            ids = re.findall(r"^- (\S+) \(", req.prompt.split("Changed requirements:", 1)[1], re.M)
            return {"criteria": [{"change_id": i, "given": "an owner with an expiring entitlement", "when": "the day arrives", "then": "the owner is notified",
                                  "cr_quote": "Owners must be notified before an entitlement expires."} for i in ids]}
        if skill == "propose-satisfying-element":
            return {"id": "SVC-I-01", "reason": "the evidence service is where owners and their entitlements are handled"}
        return base(req, n)
    return h


def test_in_automatic_mode_what_the_cr_left_out_is_proposed_marked_and_listed_and_the_change_reaches_openspec(proj):
    cr = "# CR-1 Expiry notifications\nOwners must be notified before an entitlement expires.\n"
    rt = runtime(proj, proposing(answers([dict(CR_ADD, acceptance_ids=[])])), mode="auto")
    project.generate(rt, {}, tdd_id=None, out=None, prose=False, prose_limit=None, force=False)
    result, lines = start(rt, cr_text=cr, code_targets=[], verifications=[])
    assert result["status"] == "awaiting_evidence", (result["status"], result["blockers"], rt.operational.get_task("change-expiry-notifications", "G5")["output"].get("failing_rules"))
    proposed = rt.operational.get_task("change-expiry-notifications", "G4")["output"]["proposed"]
    assert [c["change_id"] for c in proposed["acceptance"]] == ["FR-2"] and proposed["acceptance"][0]["cr_quote"].startswith("Owners must be notified")
    assert proposed["satisfied_by"]["FR-2"]["id"] == "SVC-I-01" and "not identified yet" in proposed["code_targets"]["FR-2"]
    review = (proj / result["review"]).read_text(encoding="utf-8")
    assert "Proposed by tdd-agent, not given by a person" in review and "SVC-I-01" in review
    published = (proj / "openspec" / "changes" / "expiry-notifications" / "tasks.md").read_text(encoding="utf-8")
    assert "FR-2" in published or "CS-001" in published


def test_in_manual_mode_nothing_is_proposed_and_the_gate_asks(proj):
    cr = "# CR-1 Expiry notifications\nOwners must be notified before an entitlement expires.\n"
    rt = runtime(proj, proposing(answers([dict(CR_ADD, acceptance_ids=[])])), mode="manual")
    project.generate(rt, {}, tdd_id=None, out=None, prose=False, prose_limit=None, force=False)
    start(rt, cr_text=cr, code_targets=[], verifications=[])
    d1, _ = project.change_decision(rt, None, "alice")
    project.change_approve(rt, None, "alice", "ok", d1["decision_hash"])
    d2, _ = project.change_decision(rt, None, "claude")
    result, _ = project.change_approve(rt, None, "claude", "ok", d2["decision_hash"])
    assert result["blocked_gate"] == "G4" and "CR_ACCEPTANCE_MISSING" in result["blockers"]
    assert not any(c["skill"]["id"] == "propose-acceptance-from-cr" for c in rt.operational.list_llm_calls("change-expiry-notifications"))
