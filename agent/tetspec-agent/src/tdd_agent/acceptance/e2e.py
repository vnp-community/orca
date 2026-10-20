"""Task 7.6: the change-request flow (real provider) on the PBOM documents, driven like a user would through the orchestrator.

PBOM is not implementation-ready (appendices A/B/C are missing), so the flow is expected to stop at G5 with machine-readable
reasons; the run is recorded as such and no element is bulk-reviewed to get past it.  ``run_second_to_publication`` drives
the same flow to a published change on the synthetic payment fixture.

G6/G7 need test receipts of implemented code.  Nothing is implemented in an acceptance run, so the run
stops at ``awaiting_evidence`` and that stop is the expected, recorded result: a receipt is never invented.
"""
from __future__ import annotations

import json
import subprocess
from pathlib import Path

from ..config import Config
from ..orchestrator import ApprovalRequest
from . import oracles
from .common import Timer, fresh_runtime, pbom_sources, repo_root, usage_summary, write_json
from .isolated import seed_runtime

OLD = "| FR-AST-13 | Hệ thống phải cảnh báo Entitlement thuê bao tại T-60, T-30, T-7, T-0 |"
NEW = "| FR-AST-13 | Hệ thống phải cảnh báo Entitlement thuê bao tại T-60, T-30, T-14, T-7, T-0 |"


def run_second_to_publication(base: Config, out: Path) -> dict:
    """G0-G5 plus publication and strict validation on the synthetic payment fixture; G6 then waits for receipts (none exist)."""
    from .isolated import _changespec_case
    rec = _changespec_case(base, out, oracles.load("isolated-agents")["changespec"], 1, True, finish=True, name="e2e-second")
    write_json(out / "e2e-second.json", rec)
    return rec


def run_pbom_change_request(base: Config, out: Path, allow_draft: bool = False) -> dict:
    actor = "coding-agent"
    src = out / "generation-pbom-r1"
    rt, groups = seed_runtime(base, src, out, "e2e-pbom", partition=True)
    repo = out / "e2e-pbom" / "repo"
    (repo / "openspec" / "specs").mkdir(parents=True); (repo / "openspec" / "changes").mkdir()
    (repo / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    try:
        urd, srs1, sad = pbom_sources(repo_root())
        assert OLD in srs1.text, "PBOM SRS no longer contains the FR-AST-13 row used by the e2e fixture"
        from ..store.base import SourceDoc
        srs2 = SourceDoc.make("SRS", "2.0", "SRS", srs1.text.replace(OLD, NEW, 1))
        params = {"srs": {"id": "SRS", "before": srs1.revision, "after": srs2.revision}, "cr": {"id": "CR-PBOM-1", "revision": "1", "title": "Add the T-14 reminder milestone", "acceptance": ["AC-REM-14"]},
                  "change_name": "entitlement-reminder-t14", "repo_root": str(repo), "capability": {"purpose": "Remind owners of expiring subscription entitlements", "scope": "Reminder milestones", "owner": "pbom-team"},
                  "allow_draft": allow_draft, "code_targets": {"FR-AST-13": "internal/entitlement/reminder.go::Milestones"},
                  "verifications": {"FR-AST-13": "GIVEN a subscription Entitlement that expires in 14 days WHEN the daily job runs THEN exactly one T-14 reminder is sent"}}
        record: dict = {"task": "7.6", "flow": "change-request", "input": "PBOM SRS 1.0 -> 2.0 (FR-AST-13 gains T-14)", "steps": []}
        o = rt.orchestrator
        with Timer() as t:
            o.start("change-request", {"mode": "change-request", "params": params}, run_id="pbom-cr", sources=[srs1, srs2, sad])
            for _ in range(8):
                run = o.advance("pbom-cr")
                step = {"status": run.status, "state": run.state, "pending_gate": (run.result or {}).get("pending_gate"), "blocked_gate": (run.result or {}).get("blocked_gate"), "blockers": (run.result or {}).get("blockers")}
                record["steps"].append(step)
                if run.status == "awaiting_approval":
                    gate = run.result["pending_gate"]
                    role = {"G1": "BA", "G2": "SA/Dev", "G5": "SA", "G7": "BA/owner"}[gate]
                    d = o.get_decision("pbom-cr", gate, actor, source="acceptance")
                    res = o.submit_approval(ApprovalRequest("pbom-cr", gate, actor, role, d["decision_hash"], f"e2e acceptance: {gate} decision reviewed from the generated record (agent grade)"))
                    step["approved"] = {"gate": gate, "accepted": res.accepted}
                    if gate == "G5":
                        run = o.get_run("pbom-cr")
                        record["steps"].append({"status": run.status, "state": run.state, "blockers": (run.result or {}).get("blockers")})
                        break
                    continue
                break
        final = o.get_run("pbom-cr")
        g = {name: (rt.operational.get_task("pbom-cr", name) or {}) for name in ("G1", "G2", "G3", "G4", "G5", "G6")}
        published = (repo / "openspec" / "changes" / "entitlement-reminder-t14")
        validation = subprocess.run(["openspec", "validate", "entitlement-reminder-t14", "--strict", "--no-interactive"], cwd=repo, capture_output=True, text=True) if published.exists() else None
        impact = (g["G2"].get("output") or {}).get("report", {}).get("entries", [])
        record.update(final_status=final.status, final_state=final.state, final_blockers=(final.result or {}).get("blockers"), seconds=t.seconds, usage=usage_summary(rt.operational, "pbom-cr"),
                      gates={k: v.get("status") for k, v in g.items()}, impact_actions={e["tdd_id"]: e["action"] for e in impact},
                      change_spec_rows=len(((g["G4"].get("output") or {}).get("change_spec") or {}).get("items", [])), published=published.exists(),
                      openspec_validate_strict=None if validation is None else {"returncode": validation.returncode, "stdout": validation.stdout[-300:]},
                      g6_expected="awaiting_evidence: no implementation exists, so no test receipt may be recorded",
                      provider=rt.gateway.describe(), config_hash=rt.config.fingerprint(), approvals=[{k: a[k] for k in ("gate", "actor_id", "approval_grade")} for a in rt.operational.list_approvals("pbom-cr")])
        draft = (g["G5"].get("output") or {})
        if draft.get("draft"):
            record["draft"] = {"path": draft["draft_path"], "files": draft["files"], "blockers": draft["draft_blockers"], "note": "inspection only: written to openspec/drafts, never approved or published, G6/G7 not started"}
        write_json(out / "e2e-pbom.json", record)
        return record
    finally:
        rt.close()
