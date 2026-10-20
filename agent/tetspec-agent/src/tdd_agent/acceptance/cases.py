"""The ten cases of the design document, each mapped to the tests that exercise it (task 9.5).

A case is ``pass`` only when every mapped test passes through the runtime end to end;
``not_covered`` when there is no runtime end-to-end test.  Rule-level evidence is listed
separately and never upgrades a case.
"""
from __future__ import annotations

import subprocess
import sys
import time
from pathlib import Path

from .common import repo_root, write_json

T = "tests/"
CASES = [
    {"n": 1, "title": "Feature mới PBOM từ URD/SRS/SAD", "tests": [T + "test_agents_generation.py::test_generation_builds_typed_kg_with_verified_prose_facts_gaps_and_questions"],
     "real_provider": "generation-pbom (acceptance generation)", "expected": "TDD/KG draft hoặc ready theo evidence; thiếu SRS/SAD/contract giữ draft"},
    {"n": 2, "title": "CR bổ sung bằng chứng / delta SRS", "tests": [T + "test_agents_change_flow.py::test_change_request_flow_from_srs_delta_to_acceptance"],
     "expected": "Impact: update + no_change có lý do; mỗi CS có SRS + TDD + CR"},
    {"n": 3, "title": "Defect timeout API", "tests": [T + "test_agents_generation.py::test_generation_blocks_instead_of_guessing", T + "test_agents_impact_edit.py::test_prompt_cannot_override_an_upstream_value",
                                                    T + "test_agents_impact_edit.py::test_a_property_read_from_a_source_document_cannot_be_changed_by_a_prompt"],
     "expected": "Defect cần expected behavior; prompt không đổi được giá trị SRS (cần SRS clarification)"},
    {"n": 4, "title": "Đổi batch size 100 lên 200", "tests": [T + "test_agents_change_flow.py::test_parameter_change_inside_the_designed_range_is_no_change_but_still_gets_a_code_row"],
     "expected": "no_change nếu trong range đã thiết kế; vẫn có CS cho code/config"},
    {"n": 5, "title": "Schema migration", "tests": [T + "test_agents_cases.py::test_case5_schema_migration_without_rollback_is_blocked_with_a_gap"], "expected": "Thiếu rollback/backfill → blocked kèm gap"},
    {"n": 6, "title": "Tích hợp API/MQ/bên thứ ba", "tests": [T + "test_agents_cases.py::test_case6_external_integration_without_a_failure_policy_stays_blocked"], "expected": "Có contract nhưng chưa có failure policy → blocked"},
    {"n": 7, "title": "High-load, HA, giao dịch tài chính", "tests": [T + "test_agents_cases.py::test_case7_high_availability_needs_a_recovery_objective"], "expected": "HA không có RTO/RPO → gap",
     "limit": "Chỉ kiểm điều kiện chặn HA→RecoveryObjective; LoadProfile/CapacityModel/LedgerInvariant chưa có fixture end-to-end"},
    {"n": 8, "title": "Prompt 'Đổi API-P-01 thành Submit SBOM, timeout 30s'", "tests": [T + "test_agents_impact_edit.py::test_prompt_whose_id_and_description_disagree_is_a_target_mismatch_not_a_guess",
                                                                                     T + "test_agents_impact_edit.py::test_semantic_edit_without_an_upstream_lock_commits_atomically_rerenders_and_needs_review_again"],
     "expected": "ID và mô tả mâu thuẫn → target mismatch; sửa hợp lệ → revision mới"},
    {"n": 9, "title": "SRS đòi đổi message broker", "tests": [T + "test_agents_impact_edit.py::test_rule_conflict_with_the_sad_blocks_and_proposes_a_sad_cr"], "expected": "Luôn chặn, đề xuất CR cho SAD"},
    {"n": 10, "title": "Hai CR cùng sửa shared contract", "tests": [T + "test_agents_cases.py::test_case10_two_open_crs_changing_one_shared_interface_do_not_overwrite_each_other"],
     "expected": "Không last-write-wins; CR sau bị chặn kèm CR xung đột"},
]
RULE_LEVEL = {5: [T + "test_retrieval.py::test_05_profile_aware_gaps", T + "test_retrieval.py::test_q4_q5_gaps"], 6: [T + "test_retrieval.py::test_q4_q5_gaps"], 10: [T + "test_retrieval.py::test_q9_two_open_crs_on_one_interface_but_not_accepted_ones"]}


def run_case_table(out: Path, python: str | None = None) -> dict:
    root = repo_root()
    rows = []
    for case in CASES:
        t0 = time.monotonic()
        done = subprocess.run([python or sys.executable, "-m", "pytest", "-q", "--no-header", "-p", "no:cacheprovider", *case["tests"]], cwd=root, capture_output=True, text=True, timeout=1800)
        status = "pass" if done.returncode == 0 else "fail"
        rows.append({**case, "status": status, "seconds": round(time.monotonic() - t0, 1), "pytest_tail": "\n".join(done.stdout.strip().splitlines()[-4:]), "rule_level_tests": RULE_LEVEL.get(case["n"], [])})
    statuses = [r["status"] for r in rows]
    report = {"task": "9.5", "cases": rows, "summary": {"pass": statuses.count("pass"), "fail": statuses.count("fail"), "not_covered": statuses.count("not_covered")},
              "full_acceptance_claim": all(s == "pass" for s in statuses) and all(not r.get("limit") for r in rows)}
    write_json(out / "case-table.json", report)
    (out / "case-table.md").write_text(render_markdown(report), encoding="utf-8")
    return report


def render_markdown(report: dict) -> str:
    lines = ["| # | Case | Status | Tests | Giới hạn |", "|---|---|---|---|---|"]
    for r in report["cases"]:
        tests = "<br>".join(f"`{t.split('::')[-1]}`" for t in r["tests"])
        lines.append(f"| {r['n']} | {r['title']} | **{r['status']}** | {tests} | {r.get('limit', '')} |")
    return "\n".join(lines) + "\n"
