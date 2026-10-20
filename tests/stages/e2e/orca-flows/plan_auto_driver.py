"""Orca side of the APG suite (CR-REQ-044): a project with automatic Plan generation on, a Request driven through the public WebSocket channels
to the Analysis approval, and reads of what the server did on its own (the autoPlan block, the Plan runs, the job rows).

The runs and jobs are counted in the request database of the local docker stack (`docker exec orca-go-postgres`): no gateway channel lists Plan
runs or jobs. When that is not possible the case is BLOCKED, never FAIL."""
from __future__ import annotations

import subprocess
from dataclasses import dataclass
from pathlib import Path

import analysis_revision_driver as arv
import analysis_revision_users as people
import base_ref_stack as stack
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ledgerlite_fixture import LedgerliteRepo
from pilot_register import psql, tenant_of_project

PLAN_WAIT = 90  # the task asks for the Plan within 90 seconds of the approval (CR-REQ-044)
PLAN_GATES = ("plan", "task_list")


@dataclass
class PlanRun:
    id: str
    status: str
    error_code: str
    key: str
    actor: str


def configure(ctx: AcceptanceContext, repo: LedgerliteRepo, suffix: str, plan_auto: bool = True) -> str:
    """A native-engine project on the fixture repo whose automatic Plan switch is the given one (register_project turns it off)."""
    pid = stack.register_project(ctx, repo, suffix)
    set_switch(ctx, pid, plan_auto)
    return pid


def set_switch(ctx: AcceptanceContext, project_id: str, enabled: bool) -> dict:
    cur = ctx.ws.call("request.engineSettings", {"projectId": project_id})
    return ctx.ws.call("request.engineSettingsSet", {"projectId": project_id, "engine": cur.get("engine") or "native", "openspecMinVersion": cur.get("openspecMinVersion", "") or "",
                                                     "solutionAgent": False, "executionMode": cur.get("executionMode", "") or "", "expectedVersion": cur.get("version", 0) or 0,
                                                     "planAutoGenerate": enabled})


def stub_state_dir(ctx: AcceptanceContext) -> Path:
    stub = ctx.state.get("ai_stub")
    if not stub:
        raise Blocked("the failure switches are fake claude files; they need --ai-stub")
    return Path(stub.state_dir)


def arm_plan_failure(ctx: AcceptanceContext, count: int, kind: str) -> None:
    """The next `count` Plan prompts fail the way `kind` says (error | invalid): see fail_plan_next in fake_claude_cli/claude."""
    (stub_state_dir(ctx) / "fail_plan_next").write_text(f"{count}:{kind}")


def disarm_plan_failure(ctx: AcceptanceContext) -> None:
    try:
        (stub_state_dir(ctx) / "fail_plan_next").unlink()
    except (FileNotFoundError, Blocked):
        pass


def failures_left(ctx: AcceptanceContext) -> int:
    try:
        return int((stub_state_dir(ctx) / "fail_plan_next").read_text().split(":")[0])
    except (OSError, ValueError):
        return 0


def tenant(project_id: str) -> str:
    try:
        return tenant_of_project(project_id)
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        raise Blocked(f"cannot read the request database of the local docker stack ({exc}); the run and job counts need `docker exec orca-go-postgres`") from exc


TAGS = ("BEGIN", "COMMIT", "SET", "ROLLBACK")


def rows(tenant_id: str, sql: str) -> list[list[str]]:
    """The rows `sql` selects inside a transaction scoped to the tenant (row security), split on `|`; command tags are dropped."""
    out = psql("request", f"begin; set local app.tenant_id = '{tenant_id}'; {sql}; commit;")
    return [ln.split("|") for ln in out.splitlines() if ln and ln not in TAGS and not ln.startswith("INSERT ")]


def plan_runs(ctx: AcceptanceContext, project_id: str, request_id: str) -> list[PlanRun]:
    """Plan runs of the Request, oldest first."""
    found = rows(tenant(project_id), f"select id, status, coalesce(error_code, ''), coalesce(idempotency_key, ''), coalesce(actor_id::text, '') "
                                     f"from request.analysis_runs where request_id = '{request_id}' and kind = 'plan' order by started_at, id")
    return [PlanRun(*parts) for parts in found if len(parts) == 5]


def jobs(ctx: AcceptanceContext, project_id: str, request_id: str) -> list[dict]:
    """Rows of request.plan_auto_jobs for the Request, oldest first: {origin, state, attempt, maxAttempts, reason}."""
    found = rows(tenant(project_id), f"select origin, state, attempt, max_attempts, coalesce(reason_code, '') "
                                     f"from request.plan_auto_jobs where request_id = '{request_id}' order by created_at, anchor")
    return [{"origin": p[0], "state": p[1], "attempt": int(p[2]), "maxAttempts": int(p[3]), "reason": p[4]} for p in found if len(p) == 5]


def auto_plan(ctx: AcceptanceContext, request_id: str) -> dict:
    """The autoPlan block of request.planProposal; {} when this gateway sends none."""
    return ctx.ws.call("request.planProposal", {"id": request_id}).get("autoPlan") or {}


def wait_state(ctx: AcceptanceContext, request_id: str, states: tuple[str, ...], timeout: float, what: str = "") -> dict:
    return wait_until(lambda: (lambda a: a if a.get("state") in states else None)(auto_plan(ctx, request_id)), timeout=timeout, interval=1.0,
                      what=what or f"autoPlan.state to be one of {states}")


def wait_plan(ctx: AcceptanceContext, request_id: str, timeout: float = PLAN_WAIT) -> dict:
    """The Request, once the Plan is committed and waits for approval."""
    return wait_until(lambda: (lambda r: r if r["status"] == "awaiting_plan_approval" and r.get("planTaskId") else None)(drv.get(ctx, request_id)),
                      timeout=timeout, interval=1.0, what="the Request to reach awaiting_plan_approval with a Plan")


def pending_plan_approvals(ctx: AcceptanceContext, request_id: str) -> list[dict]:
    pending = ctx.ws.call("approval.list", {"requestId": request_id, "status": "pending"}).get("approvals", [])
    return [a for a in pending if a["subjectType"] in PLAN_GATES]


def to_analysis_gate(ctx: AcceptanceContext, project_id: str, typ: str, title: str, size: str = "S") -> arv.Flow:
    """create -> classify -> confirm -> readiness answered -> the analysis ran and waits at its gate."""
    flow = arv.new_flow(ctx, project_id, typ, title, size)
    flow.first_attempt()
    return flow


def approve_analysis(ctx: AcceptanceContext, flow: arv.Flow) -> dict:
    pending = flow.pending()
    assert len(pending) == 1, f"{len(pending)} pending '{flow.subject}' approvals at the gate, expected one"
    a = pending[0]
    return ctx.ws.call("approval.approve", {"id": a["id"], "comment": "e2e", "expectedVersion": a["version"], "expectedDigest": a["subjectDigest"]})


def make_ready(ctx: AcceptanceContext, project_id: str, request_id: str) -> None:
    """Gives the Request an acceptance criterion so its type confirmation satisfies the Definition of Ready and goes straight to planning.
    No gateway channel sets criteria at creation, and without this the readiness Clarification is always asked, whose answer resumes planning
    with `information_provided`, a trigger that never starts a Plan by design (CR-REQ-044 D3)."""
    ac = '[{"id": "AC-1", "text": "The change is visible and verified by the e2e suite.", "status": "active"}]'
    rows(tenant(project_id), f"update request.requests set acceptance_criteria = '{ac}'::jsonb where id = '{request_id}'")


def name_pre_deploy_approver(ctx: AcceptanceContext, project_id: str, label: str = "apg-approver") -> None:
    """A second user who may approve the pre_deploy approval that a hotfix opens over its fix task (the reporter cannot approve it)."""
    user = people.create_user(ctx, label)
    people.name_approver(ctx, project_id, user)
    tenant_id = tenant(project_id)
    approvers = f'["reporter","role:admin","user:{user.id}"]'
    try:
        psql("request", f"begin; set local app.tenant_id = '{tenant_id}'; insert into request.approval_policies (id, tenant_id, project_id, subject_type, approvers, allow_requester_approve, "
                        f"priority, enabled, version, created_by) values (gen_random_uuid(), '{tenant_id}', '{project_id}', 'pre_deploy', '{approvers}'::jsonb, true, 10, true, 1, '{people.POLICY_AUTHOR}'); commit;")
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        raise Blocked(f"cannot write the pre_deploy approval policy in the local docker stack ({exc})") from exc
