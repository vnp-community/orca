"""End-to-end G0-G7 on small documents with scripted (fake) model answers.

Everything except the model is real: stores, retrieval, skills, o5 gates, openspec CLI.
"""
import json
import re
import subprocess
from pathlib import Path

import pytest

from harness import ACTORS, doc, make, prompt_blocks
from tdd_agent.agents.flows import default_flows
from tdd_agent.orchestrator import ApprovalRequest
from tdd_agent.store.base import SourceDoc

URD = "# URD\n| ID | Statement |\n|---|---|\n| REQ-1 | Evidence must be kept. |\n"
SAD = "# SAD\n## Decisions\n| ID | Decision | Rationale | Alternatives | Tradeoffs |\n|---|---|---|---|---|\n| ADR-1 | Kafka | durable log | RabbitMQ | ops cost |\n"
SRS_V1 = ("# SRS\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST store SBOM evidence. | AC-1 |\n"
          "## 4. API\n| ID | Name | Satisfies |\n|---|---|---|\n| SVC-I-01 | Evidence service | FR-1 |\n")
SRS_V2 = SRS_V1 + "| FR-3 | The system SHALL notify owners about expiry. | AC-3 |\n| SVC-N-01 | Notification service | FR-3 |\n"
SRS_R = ("# SRS-R\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-2 | Reports MUST list expiring entitlements. | AC-2 |\n"
         "## 4. API\n| ID | Name | Satisfies |\n|---|---|---|\n| SVC-R-01 | Report service | FR-2 |\n")
CR = {"id": "CR-1", "revision": "1", "title": "Expiry notifications", "acceptance": ["AC-3"]}


def authoring_handler(req, n):
    """Plays the openspec-propose author / critic from the pinned pack (deterministic projection + line assertions)."""
    import spec_change
    from tdd_agent.core import o5_path
    o5_path.o5_root()
    if req.role == "critic":
        return {"verdict": "pass", "findings": []}
    pack = json.loads(req.prompt.split("## Pinned pack\n", 1)[1])
    artifacts = spec_change.project_change_spec_to_openspec(pack["canonical_change_spec"], pack["request"]["change_name"], pack["request"]["intent"], kg_context=pack.get("graph"))
    artifacts = {k: v.replace("'", "") for k, v in artifacts.items()}
    refs = ["kg:" + e["id"] for e in pack["graph"]["elements"] if pack["provenance"].get(e["id"])]
    assertions = [{"artifact": path, "line_text": line, "references": refs} for path, content in artifacts.items()
                  for i, line in enumerate(content.splitlines(), 1) if line.strip() and not line.startswith(("#", ">")) and not re.fullmatch(r"[| :\-]+", line)]
    return {"artifacts": [{"path": p, "content": c} for p, c in artifacts.items()], "assertions": assertions, "proposed_decisions": []}


def handler(req, n):
    skill = req.metadata.get("skill")
    if skill in ("author-spec-prose", "critique-spec"):
        return authoring_handler(req, n)
    if skill == "resolve-entity-match":
        return {"same": False, "reason": "different requirements"}
    if skill == "extract-facts-from-prose":
        return {"facts": []}
    if skill == "detect-sad-conflict":
        return {"suspected": []}
    raise AssertionError(f"unexpected model call from skill {skill!r}: logic-first violated")


@pytest.fixture()
def repo(tmp_path):
    root = tmp_path / "repo"
    (root / "openspec" / "specs").mkdir(parents=True); (root / "openspec" / "changes").mkdir()
    (root / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    return root


def baseline(o, ops):
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-A", "prose": False}}, run_id="gen-a",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", SRS_V1), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("gen-a").status == "completed"
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-B", "prose": False}}, run_id="gen-b",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS-R", "1", "SRS", SRS_R), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("gen-b").status == "completed"


def approve(o, run, gate, actor, role):
    d = o.get_decision(run, gate, actor, source="test")
    return o.submit_approval(ApprovalRequest(run, gate, actor, role, d["decision_hash"], f"reviewed {gate}"))


def start_change(o, repo, **extra):
    params = {"srs": {"id": "SRS", "before": "1", "after": "2"}, "cr": CR, "change_name": "expiry-notifications", "repo_root": str(repo),
              "tdd_for_new": {"FR-3": "TDD-A", "SVC-N-01": "TDD-A"},
              "capability": {"purpose": "Notify owners before entitlements expire", "scope": "Expiry notifications", "owner": "pbom-team"},
              "code_targets": {"FR-3": "src/notify.py::notify_owner", "SVC-N-01": "src/api.py::notifications"},
              "verifications": {"FR-3": "GIVEN an expiring entitlement WHEN the day arrives THEN the owner is notified",
                                "SVC-N-01": "GIVEN an owner WHEN the notification service is queried THEN pending notices are listed"}, **extra}
    return o.start("change-request", {"mode": "change-request", "params": params}, run_id="cr1",
                   sources=[SourceDoc.make("SRS", "1", "SRS", SRS_V1), SourceDoc.make("SRS", "2", "SRS", SRS_V2), SourceDoc.make("SAD", "1", "SAD", SAD)])


def test_change_request_flow_from_srs_delta_to_acceptance(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    baseline(o, ops)
    assert {d.id for d in know.list_sources("TDD")} == {"TDD-A", "TDD-B"}
    start_change(o, repo)
    run = o.advance("cr1")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G1", run.result
    delta = o.get_decision("cr1", "G1", "alice")["decision"]["srs_delta"]
    assert {(c["id"], c["kind"]) for c in delta["changes"]} == {("FR-3", "added"), ("SVC-N-01", "added")}
    assert approve(o, "cr1", "G1", "alice", "BA").accepted

    run = o.get_run("cr1")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G2", run.result
    report = o.get_decision("cr1", "G2", "bob")["decision"]
    entries = {e["tdd_id"]: e for e in report["entries"]}
    assert entries["TDD-A"]["action"] == "update" and entries["TDD-B"]["action"] == "no_change"
    assert "none of the" in entries["TDD-B"]["rationale"] and entries["TDD-B"]["srs_refs"] and entries["TDD-B"]["tdd_refs"]
    assert approve(o, "cr1", "G2", "claude", "SA/Dev").accepted    # a registered coding agent may approve

    run = o.get_run("cr1")
    assert run.status == "blocked" and run.result["blocked_gate"] == "G5", "unreviewed facts keep the spec a draft"
    assert any("review required" in str(x) for x in ops.get_task("cr1", "G5")["output"]["failing_rules"]["QG-03"])
    pending = [e["id"] for e in o.elements_needing_review()]
    assert pending and o.review_elements(pending, "approved", "bob", "SA", "facts match the sources")["changed"] == len(pending)
    run = o.advance("cr1")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G5", (run.status, run.result, ops.get_task("cr1", "G5")["output"].get("failing_rules"))
    t3 = ops.get_task("cr1", "G3")["output"]
    maps = {m["tdd_id"]: m for m in t3["tdd_set"]["mappings"]}
    assert maps["TDD-B"]["action"] == "no_change" and maps["TDD-B"]["before_revision"] == maps["TDD-B"]["after_revision"] and maps["TDD-B"]["design_reuse_proof"]
    assert maps["TDD-A"]["action"] == "update" and int(maps["TDD-A"]["after_revision"]) > int(maps["TDD-A"]["before_revision"])
    assert {s["section_id"] for s in maps["TDD-A"]["actual_sections"]} >= {"FR-3", "SVC-N-01", "document"}
    assert know.get_source("TDD-A").revision == "2" and know.get_source("TDD-B").revision == "1"

    t4 = ops.get_task("cr1", "G4")["output"]
    assert t4["change_spec"]["status"] == "eligible" and len(t4["change_spec"]["items"]) == 2
    for item in t4["change_spec"]["items"]:
        assert item["srs_refs"][0]["role"] == "SRS" and item["tdd_refs"][0]["role"] == "TDD" and item["cr_acceptance_refs"] == ["AC-3"] and item["verification"]
    assert {"expiry-notifications", "FR-3@2"} <= {e["id"] for e in know.materialize()["elements"]}

    decision = o.get_decision("cr1", "G5", "bob")["decision"]
    assert decision["change_name"] == "expiry-notifications" and decision["trace_manifest"]["status"] == "implementation_ready"
    assert not (repo / "openspec" / "changes" / "expiry-notifications").exists(), "nothing is published before the SA approves"
    res = approve(o, "cr1", "G5", "bob", "SA")
    assert res.accepted, res
    assert (repo / "openspec" / "changes" / "expiry-notifications" / "trace-manifest.json").is_file()
    check = subprocess.run(["openspec", "validate", "expiry-notifications", "--strict", "--no-interactive"], cwd=repo, capture_output=True, text=True)
    assert check.returncode == 0, check.stdout + check.stderr

    run = o.get_run("cr1")
    assert run.status == "awaiting_evidence" and run.state == "spec_authored", (run.status, run.state, run.result)
    assert run.result["blockers"]
    o.record_test_receipt("cr1", "CS-001", ["AC-3"], "pass", "bob")
    assert o.advance("cr1").status == "awaiting_evidence", "CS-002 still has no receipt"
    o.record_test_receipt("cr1", "CS-002", ["AC-3"], "pass", "bob")
    run = o.advance("cr1")
    assert run.status == "awaiting_approval" and run.state == "verified", (run.status, run.result)
    assert approve(o, "cr1", "G7", "claude", "BA/owner").accepted
    run = o.get_run("cr1")
    assert run.status == "completed" and run.state == "accepted"
    receipt = know.get_element("CR-1/acceptance")
    assert receipt["props"]["status"] == "accepted(agent)" and receipt["props"]["accepted_by"] == "claude"
    grades = {a["gate"]: a["approval_grade"] for a in ops.list_approvals("cr1")}
    assert grades == {"G1": "human", "G2": "agent", "G5": "human", "G7": "agent"}
    assert [t["to"] for t in ops.list_transitions("cr1")][-3:] == ["spec_authored", "verified", "accepted"]
    spans = ops.list_spans(run_id="cr1")
    assert {"G0", "G1", "G2", "G3", "G4", "G5", "G6", "G7"} <= {s["name"] for s in spans if s["kind"] == "gate"}
    calls = ops.list_llm_calls("cr1")
    assert {c["skill"]["id"] for c in calls} == {"detect-sad-conflict", "author-spec-prose", "critique-spec"} and all(c["payload_mode"] == "none" for c in calls)
    assert "judge-design-coverage" not in {c["skill"]["id"] for c in calls}, "coverage was decided by rules"


# ---------------------------------------------------------------- case 4 and case 8 of the design document
SRS_P1 = ("# SRS-P\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-4 | The worker MUST process batches of 100 records. | AC-4 |\n"
          "## 4. Components\n| ID | Name | Satisfies | Min | Max |\n|---|---|---|---|---|\n| SVC-4 | Batch worker | FR-4 | 1 | 500 |\n")
SRS_P2 = SRS_P1.replace("batches of 100 records", "batches of 200 records")


def test_a_draft_bundle_can_be_staged_for_inspection_but_is_never_approvable_or_published(tmp_path, repo):
    """allow_draft: the requester sees the authored bundle although the spec is not eligible; the gate stays blocked, nothing reaches openspec/changes, G6 never starts."""
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    baseline(o, ops)
    start_change(o, repo, allow_draft=True)
    assert o.advance("cr1").result["pending_gate"] == "G1" and approve(o, "cr1", "G1", "alice", "BA").accepted
    assert approve(o, "cr1", "G2", "claude", "SA/Dev").accepted
    run = o.get_run("cr1")
    assert run.status == "blocked" and run.result["blocked_gate"] == "G5" and "SPEC_DRAFT_ONLY" in run.result["blockers"]
    out = ops.get_task("cr1", "G5")["output"]
    assert out["draft"] is True and "RULE:QG-03" in out["draft_blockers"], "the reasons it is a draft are listed"
    assert {"proposal.md", "design.md", "tasks.md", "trace-manifest.json"} <= set(out["files"])
    from pathlib import Path
    assert "/drafts/" in out["draft_path"] and (Path(out["draft_path"]) / "proposal.md").read_text().startswith("> DRAFT:")
    assert not (repo / "openspec" / "changes" / "expiry-notifications").exists(), "a draft is never published"
    run = o.advance("cr1")   # a retry must not trip over the draft it left behind
    assert run.status == "blocked" and "SPEC_NAME_COLLISION" not in run.result["blockers"]
    assert ops.get_task("cr1", "G6") is None or ops.get_task("cr1", "G6").get("status") != "completed"


def test_parameter_change_inside_the_designed_range_is_no_change_but_still_gets_a_code_row(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-P", "prose": False}}, run_id="gp",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", SRS_P1), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("gp").status == "completed"
    params = {"srs": {"id": "SRS", "before": "1", "after": "2"}, "cr": {"id": "CR-4", "revision": "1", "title": "Bigger batches", "acceptance": ["AC-4"]}, "change_name": "bigger-batches",
              "repo_root": str(repo), "capability": {"purpose": "Throughput", "scope": "Batch size", "owner": "pbom-team"},
              "code_targets": {"FR-4": "config/worker.yaml::batch_size"}, "verifications": {"FR-4": "GIVEN a queue of 1000 records WHEN the worker runs THEN it processes 200 records per batch"}}
    o.start("change-request", {"mode": "change-request", "params": params}, run_id="cr4",
            sources=[SourceDoc.make("SRS", "1", "SRS", SRS_P1), SourceDoc.make("SRS", "2", "SRS", SRS_P2), SourceDoc.make("SAD", "1", "SAD", SAD)])
    o.advance("cr4")
    delta = o.get_decision("cr4", "G1", "alice")["decision"]["srs_delta"]["changes"]
    assert [(c["id"], c["kind"], c["parameter_only"]) for c in delta] == [("FR-4", "modified", True)]
    assert approve(o, "cr4", "G1", "alice", "BA").accepted
    report = o.get_decision("cr4", "G2", "bob")["decision"]
    [entry] = report["entries"]
    assert entry["tdd_id"] == "TDD-P" and entry["action"] == "no_change" and "inside the range" in entry["rationale"], entry
    assert approve(o, "cr4", "G2", "bob", "SA/Dev").accepted
    run = o.get_run("cr4")
    t4 = ops.get_task("cr4", "G4")
    assert t4["status"] == "completed", (run.status, run.result)
    [row] = t4["output"]["change_spec"]["items"]
    assert row["code_target"] == "config/worker.yaml::batch_size" and row["action"] == "modify" and row["cr_acceptance_refs"] == ["AC-4"], "the code/config change still has a canonical row"
    assert know.get_source("TDD-P").revision == "1", "the design document was not touched"


SRS_L1 = ("# SRS-L\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-7 | The service MUST accept an SBOM of at most 50 MB and 20.000 component | AC-7 |\n"
          "## 4. Components\n| ID | Name | Satisfies |\n|---|---|---|\n| SVC-7 | Ingest service | FR-7 |\n")


def _limit_change(tmp_path, repo, new):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-L", "prose": False}}, run_id="gl",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", SRS_L1), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("gl").status == "completed"
    params = {"srs": {"id": "SRS", "before": "1", "after": "2"}, "cr": {"id": "CR-L", "revision": "1", "title": "limit", "acceptance": ["AC-7"]}, "change_name": "sbom-limit", "repo_root": str(repo),
              "capability": {"purpose": "p", "scope": "s", "owner": "o"}, "code_targets": {"FR-7": "config/limits.yaml::sbom_max_components"}, "verifications": {"FR-7": "GIVEN a bigger SBOM WHEN submitted THEN it is rejected"}}
    o.start("change-request", {"mode": "change-request", "params": params}, run_id="cl",
            sources=[SourceDoc.make("SRS", "1", "SRS", SRS_L1), SourceDoc.make("SRS", "2", "SRS", SRS_L1.replace("20.000", new)), SourceDoc.make("SAD", "1", "SAD", SAD)])
    o.advance("cl"); approve(o, "cl", "G1", "alice", "BA")
    return o.get_decision("cl", "G2", "bob")["decision"]["entries"]


def test_lowering_a_cap_is_no_change_and_raising_it_beyond_the_design_is_an_update_with_a_capacity_obligation(tmp_path, repo):
    [lowered] = _limit_change(tmp_path / "a", repo, "15.000")
    assert lowered["action"] == "no_change" and "inside the range" in lowered["rationale"] and lowered["srs_refs"] and lowered["tdd_refs"]
    [raised] = _limit_change(tmp_path / "b", repo, "40.000")
    assert raised["action"] == "update" and "capacity and quality obligations must be revalidated" in raised["rationale"] and "component cap 20000 -> 40000" in raised["rationale"]


def test_a_new_row_with_an_unmapped_id_prefix_is_reported_instead_of_vanishing_from_the_delta(tmp_path, repo):
    """Found on PBOM (phase 11): EIF-xx rows have no class, so a new external interface produced an empty delta without saying why."""
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    v2 = SRS_V1 + "\n## 5. External\n| ID | System | Protocol |\n|---|---|---|\n| ZZZ-1 | ServiceNow | HTTPS |\n"
    def go(run, params=None):
        p = {"srs": {"id": "SRS", "before": "1", "after": "2"}, "cr": CR, "change_name": f"zzz-{run}", "repo_root": str(repo), **(params or {})}
        o.start("change-request", {"mode": "change-request", "params": p}, run_id=run, sources=[SourceDoc.make("SRS", "1", "SRS", SRS_V1), SourceDoc.make("SRS", "2", "SRS", v2), SourceDoc.make("SAD", "1", "SAD", SAD)])
        return o.advance(run)
    run = go("u1")
    assert run.status == "blocked" and run.result["blockers"] == ["SRS_DELTA_UNMAPPED_IDS"]
    assert ops.get_task("u1", "G1")["output"]["unmapped_ids"] == ["ZZZ-1"]
    run = go("u2", {"id_class_map": {"ZZZ": "Constraint"}})
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G1"


# ---------------------------------------------------------------- a modified requirement (TMS issues 11, 12): relations survive G3, the replaced version is history, the baseline comes from the SRS
def test_a_modified_requirement_keeps_its_component_and_reaches_a_publishable_spec(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    baseline(o, ops)
    srs_v3 = SRS_V1.replace("MUST store SBOM evidence.", "MUST store SBOM evidence for 5 years.")
    o.start("change-request", {"mode": "change-request", "params": {
        "srs": {"id": "SRS", "before": "1", "after": "3"}, "cr": {"id": "CR-M", "revision": "1", "title": "Retention", "acceptance": ["AC-1"]}, "change_name": "evidence-retention", "repo_root": str(repo), "baseline_from_srs": True,
        "capability": {"purpose": "Keep SBOM evidence for audit", "scope": "Evidence retention", "owner": "pbom-team"},
        "code_targets": {"FR-1": "src/evidence.py::retain"},
        "verifications": {"FR-1": "GIVEN stored evidence WHEN five years pass THEN it is still readable"}}}, run_id="crm",
        sources=[SourceDoc.make("SRS", "1", "SRS", SRS_V1), SourceDoc.make("SRS", "3", "SRS", srs_v3), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("crm").status == "awaiting_approval"
    assert approve(o, "crm", "G1", "alice", "BA").accepted
    assert approve(o, "crm", "G2", "claude", "SA/Dev").accepted
    pending = [e["id"] for e in o.elements_needing_review()]
    if pending:
        o.review_elements(pending, "approved", "bob", "SA", "facts match the sources")
        o.advance("crm")
    run = o.get_run("crm")
    edges = {(e["src"], e["rel"], e["dst"]) for e in know.materialize()["edges"]}
    assert ("SVC-I-01", "satisfies", "FR-1") in edges, "the component that satisfies the modified requirement is still linked to it"
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G5", (run.status, run.result, ops.get_task("crm", "G5").get("output", {}).get("failing_rules"))
    baseline_spec = (repo / "openspec" / "specs" / "evidence-retention" / "spec.md").read_text()
    assert "### Requirement: FR-1" in baseline_spec and "MUST store SBOM evidence." in baseline_spec, "the baseline of the modified requirement was written from the SRS it started from"


# ---------------------------------------------------------------- the author revises a draft the reviewer rejected
def _revision_run(tmp_path, repo, max_revisions, rejections):
    seen = {"authors": [], "critics": 0}

    def h(req, n):
        skill = req.metadata.get("skill")
        if skill == "author-spec-prose":
            seen["authors"].append("previous_findings" in req.prompt)
        if skill == "critique-spec":
            seen["critics"] += 1
            if seen["critics"] <= rejections:
                return {"verdict": "fail", "findings": [{"artifact": "proposal.md", "anchor": "document", "reason": "the retention period is not supported by the pack"}]}
        return handler(req, n)
    o, know, ops, provider = make(default_flows(), h, tmp_path, overrides={"openspec": {"max_revisions": max_revisions, "language": "Vietnamese"}})
    baseline(o, ops)
    start_change(o, repo)
    o.advance("cr1")
    approve(o, "cr1", "G1", "alice", "BA"); approve(o, "cr1", "G2", "claude", "SA/Dev")   # the run goes on to G5 and writes the bundle once, before the facts are reviewed
    return o.get_run("cr1"), seen, ops


def test_a_rejected_draft_is_written_again_with_the_findings_until_it_passes_or_the_rounds_run_out(tmp_path, repo):
    run, seen, ops = _revision_run(tmp_path, repo, max_revisions=2, rejections=1)
    assert seen["authors"] == [False, True], "the second draft was written knowing the finding"
    attempts = ops.get_task("cr1", "G5")["output"]["authoring_attempts"]
    assert [a["revision"] for a in attempts] == [0, 1] and attempts[0]["findings"] >= 1, "round 0 was rejected by the reviewer, round 1 was not"


def test_when_every_round_is_rejected_the_gate_is_blocked_and_says_how_many_were_tried(tmp_path, repo):
    run, seen, ops = _revision_run(tmp_path, repo, max_revisions=1, rejections=5)
    assert run.status == "blocked" and run.result["blocked_gate"] == "G5"
    assert seen["authors"] == [False, True] and [a["revision"] for a in ops.get_task("cr1", "G5")["output"]["authoring_attempts"]] == [0, 1]


def test_a_modified_requirement_nobody_is_recorded_as_satisfying_is_blocked_until_the_sa_names_the_element(tmp_path, repo):
    """TMS: the SRS has services and requirements but no row says which service satisfies FR-JIR-04."""
    unlinked = SRS_V1.replace("| SVC-I-01 | Evidence service | FR-1 |", "| SVC-I-01 | Evidence service |  |")
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-A", "prose": False}}, run_id="gen-a",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", unlinked), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("gen-a").status == "completed"
    srs_v3 = unlinked.replace("MUST store SBOM evidence.", "MUST store SBOM evidence for 5 years.")
    o.start("change-request", {"mode": "change-request", "params": {
        "srs": {"id": "SRS", "before": "1", "after": "3"}, "cr": {"id": "CR-M", "revision": "1", "title": "Retention", "acceptance": ["AC-1"]}, "change_name": "evidence-retention", "repo_root": str(repo), "baseline_from_srs": True,
        "capability": {"purpose": "Keep SBOM evidence for audit", "scope": "Evidence retention", "owner": "pbom-team"}, "code_targets": {"FR-1": "src/evidence.py::retain"},
        "verifications": {"FR-1": "GIVEN stored evidence WHEN five years pass THEN it is still readable"}}}, run_id="crm",
        sources=[SourceDoc.make("SRS", "1", "SRS", unlinked), SourceDoc.make("SRS", "3", "SRS", srs_v3), SourceDoc.make("SAD", "1", "SAD", SAD)])
    o.advance("crm"); approve(o, "crm", "G1", "alice", "BA"); approve(o, "crm", "G2", "claude", "SA/Dev")
    pending = [e["id"] for e in o.elements_needing_review()]
    o.review_elements(pending, "approved", "bob", "SA", "facts match the sources")
    run = o.advance("crm")
    failing = ops.get_task("crm", "G5")["output"]["failing_rules"]
    assert run.status == "blocked" and any("design trace missing" in str(x) for x in failing.get("TS-05", [])), failing
    o.update_sources("crm", [], keep_revisions=True, params={"satisfied_by": {"FR-1": "SVC-I-01"}})
    o.advance("crm")
    pending = [e["id"] for e in o.elements_needing_review()]
    o.review_elements(pending, "approved", "bob", "SA", "the rows rebuilt by the change are the same facts")
    run = o.advance("crm")
    assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G5", (run.status, run.result, ops.get_task("crm", "G5").get("output", {}).get("failing_rules"))
    bad = o.update_sources("crm", [], keep_revisions=True, params={"satisfied_by": {"FR-1": "NOPE-1"}})
    assert o.advance("crm").result["blockers"][0].startswith("SATISFIED_BY_UNKNOWN:FR-1:NOPE-1")
