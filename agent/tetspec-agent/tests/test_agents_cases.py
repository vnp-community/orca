"""Cases 5, 6, 7 and 10 of the design document, end to end through the runtime."""
from harness import make
from test_agents_change_flow import (CR, SAD, URD, approve, baseline, handler, repo as repo_fixture, SRS_V1, SRS_R, start_change)  # noqa: F401
from tdd_agent.agents.flows import default_flows
from tdd_agent.store.base import SourceDoc

import pytest

repo = repo_fixture


def generate(tmp_path, srs_text, tdd="TDD-C"):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": tdd, "prose": False}}, run_id="g",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", srs_text), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("g").status == "completed"
    return know.materialize(), ops.get_task("g", "G3")["output"]


MIG_HEAD = "| ID | Name | From schema | To schema | Stages | Backfill | Lock | Compatibility | Rollback |\n|---|---|---|---|---|---|---|---|---|\n"


def test_case5_schema_migration_without_rollback_is_blocked_with_a_gap(tmp_path):
    g, out = generate(tmp_path, "# SRS\n" + MIG_HEAD + "| MIG-1 | Add expiry column | v1 | v2 | expand,backfill,contract | batch job | none | backward compatible | - |\n")
    assert "MIG-1" not in {e["id"] for e in g["elements"]} and out["status"] != "implementation_ready"
    gap = next(x for x in g["gaps"] if x["target"] == "MIG-1")
    assert gap["need"] == "valid_element" and "rollback" in gap["reason"]
    complete = "| MIG-1 | Add expiry column | v1 | v2 | expand,backfill,contract | batch job | none | backward compatible | drop column |\n"
    g2, out2 = generate(tmp_path / "ok", "# SRS\n" + MIG_HEAD + complete)
    mig = next(e for e in g2["elements"] if e["id"] == "MIG-1")
    assert mig["class"] == "Migration" and mig["props"]["rollback"] == "drop column" and not [x for x in g2["gaps"] if x["target"] == "MIG-1"]


def test_case6_external_integration_without_a_failure_policy_stays_blocked(tmp_path):
    srs = ("# SRS\n| ID | Name | Auth | Request | Response | Errors | Integration |\n|---|---|---|---|---|---|---|\n"
           "| API-X-01 | POST /v1/partner/notify | token | body | 202 | 502 | true |\n")
    g, out = generate(tmp_path, srs)
    codes = {x.get("code") for x in g["gaps"] if x["target"] == "API-X-01"}
    needs = {x["need"] for x in g["gaps"] if x["target"] == "API-X-01"}
    assert "MISSING_FAILURE_POLICY" in codes or "missing_failure_policy" in needs, g["gaps"]
    assert {"idempotency", "concurrency"} & needs, "a write operation also needs its idempotency/concurrency contract"
    assert out["status"] != "implementation_ready" and out["questions"]


HA_HEAD = "# SRS\n| ID | Name | Replicas | Failure domains | Failover |\n|---|---|---|---|---|\n"


def test_case7_high_availability_needs_a_recovery_objective(tmp_path):
    g, out = generate(tmp_path, HA_HEAD + "| HA-1 | Ledger cluster | 3 | zone-a, zone-b | automatic |\n")
    assert any(x.get("code") == "MISSING_RECOVERY_OBJECTIVE" and x["target"] == "HA-1" for x in g["gaps"]) and out["status"] != "implementation_ready"
    rto = ("\n| ID | Name | Rto | Rpo | Recovery procedure |\n|---|---|---|---|---|\n| RTO-1 | Ledger recovery | 5 minutes | 0 | promote replica |\n")
    g2, out2 = generate(tmp_path / "ok", HA_HEAD + "| HA-1 | Ledger cluster | 3 | zone-a, zone-b | automatic |\n" + rto)
    assert {"HA-1", "RTO-1"} <= {e["id"] for e in g2["elements"]} and not any(x.get("code") == "MISSING_RECOVERY_OBJECTIVE" for x in g2["gaps"])


IFACE_V1 = ("# SRS\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST store SBOM evidence. | AC-1 |\n"
            "## 4. Interfaces\n| ID | Name | Satisfies |\n|---|---|---|\n| IF-1 | Evidence interface | FR-1 |\n")
IFACE_A = IFACE_V1.replace("| IF-1 | Evidence interface | FR-1 |", "| IF-1 | Evidence interface v2 | FR-1 |")
IFACE_B = IFACE_V1.replace("| IF-1 | Evidence interface | FR-1 |", "| IF-1 | Evidence interface with signature | FR-1 |")


def test_case10_two_open_crs_changing_one_shared_interface_do_not_overwrite_each_other(tmp_path, repo):
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-A", "prose": False}}, run_id="ga",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", IFACE_V1), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("ga").status == "completed"
    def change(run, cr, text, name):
        params = {"srs": {"id": "SRS", "before": "1", "after": run}, "cr": {"id": cr, "revision": "1", "title": f"change {cr}", "acceptance": [f"AC-{cr}"]}, "change_name": name, "repo_root": str(repo),
                  "capability": {"purpose": "p", "scope": "s", "owner": "o"}, "code_targets": {"IF-1": f"src/{name}.py::api"},
                  "verifications": {"IF-1": "GIVEN an interface WHEN it is called THEN it answers"}}
        o.start("change-request", {"mode": "change-request", "params": params}, run_id=f"run-{cr}",
                sources=[SourceDoc.make("SRS", "1", "SRS", IFACE_V1), SourceDoc.make("SRS", run, "SRS", text), SourceDoc.make("SAD", "1", "SAD", SAD)])
        o.advance(f"run-{cr}"); approve(o, f"run-{cr}", "G1", "alice", "BA"); approve(o, f"run-{cr}", "G2", "bob", "SA/Dev")
        return o.get_run(f"run-{cr}")
    first = change("2", "CR-A", IFACE_A, "interface-a")
    assert first.status in ("blocked", "awaiting_approval") and not [b for b in first.result.get("blockers", []) if b.startswith("SHARED_CONTRACT")], first.result
    second = change("3", "CR-B", IFACE_B, "interface-b")
    assert second.status == "blocked" and second.result["blocked_gate"] == "G4"
    assert any(b.startswith("SHARED_CONTRACT_CONFLICT:IF-1:CR-A") for b in second.result["blockers"]), second.result
    q9 = o.operational.get_task("run-CR-B", "G4")["output"]["shared_contract_conflicts"]
    row = next(r for r in q9 if r["interface"] == "IF-1")
    assert row["change_requests"] == ["CR-A", "CR-B"], "the requirement that states the interface is reported as shared too"


def test_two_open_crs_changing_the_same_requirement_conflict_even_when_the_kg_has_no_interface_element(tmp_path, repo):
    """Found on PBOM (phase 11): the API contract is not an element there, the requirement that states it is."""
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    srs = "# SRS\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-9 | The system MUST answer with `id`. | AC-9 |\n"
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-A", "prose": False}}, run_id="ga",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", srs), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("ga").status == "completed"

    def change(rev, cr, extra):
        text = srs.replace("`id`", f"`id` and `{extra}`")
        params = {"srs": {"id": "SRS", "before": "1", "after": rev}, "cr": {"id": cr, "revision": "1", "title": cr, "acceptance": [f"AC-{cr}"]}, "change_name": f"change-{cr.lower()}", "repo_root": str(repo),
                  "capability": {"purpose": "p", "scope": "s", "owner": "o"}, "code_targets": {"FR-9": f"src/{cr}.py::answer"}, "verifications": {"FR-9": "GIVEN a request WHEN answered THEN the field is present"}}
        o.start("change-request", {"mode": "change-request", "params": params}, run_id=f"run-{cr}", sources=[SourceDoc.make("SRS", "1", "SRS", srs), SourceDoc.make("SRS", rev, "SRS", text), SourceDoc.make("SAD", "1", "SAD", SAD)])
        o.advance(f"run-{cr}"); approve(o, f"run-{cr}", "G1", "alice", "BA"); approve(o, f"run-{cr}", "G2", "bob", "SA/Dev")
        return o.get_run(f"run-{cr}")

    first = change("2a", "CR-A", "etag")
    assert not [b for b in first.result.get("blockers", []) if b.startswith("SHARED_CONTRACT")]
    second = change("2b", "CR-B", "received_at")
    assert second.status == "blocked" and second.result["blocked_gate"] == "G4"
    assert any(b.startswith("SHARED_CONTRACT_CONFLICT:FR-9:CR-A") for b in second.result["blockers"]), second.result


def test_an_external_interface_without_a_failure_policy_is_a_gap_even_before_an_operation_uses_it(tmp_path, repo):
    """Found on PBOM (phase 11): EIF-xx rows are contracts with systems we do not control; Q4 only looked at operations."""
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    srs = ("# SRS\n## 4. Giao diện bên ngoài\n| ID | System | Protocol |\n|---|---|---|\n| EIF-1 | ServiceNow | HTTPS |\n"
           "## 5. Internal\n| ID | System | Protocol |\n|---|---|---|\n| IF-5 | Cache | local |\n")
    o.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-A", "prose": False, "id_class_map": {"EIF": "Interface"}}}, run_id="ge",
            sources=[SourceDoc.make("URD", "1", "URD", URD), SourceDoc.make("SRS", "1", "SRS", srs), SourceDoc.make("SAD", "1", "SAD", SAD)])
    assert o.advance("ge").status == "completed"
    g = know.materialize()
    assert know.get_element("EIF-1")["props"]["external"] is True and know.get_element("IF-5")["props"]["external"] is False
    codes = {(x["target"], x.get("code")) for x in g["gaps"]}
    assert ("EIF-1", "MISSING_FAILURE_POLICY") in codes and ("IF-5", "MISSING_FAILURE_POLICY") not in codes
