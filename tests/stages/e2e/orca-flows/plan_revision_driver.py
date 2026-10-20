"""One Request whose Plan is revised by prompt (CR-REQ-045), driven through the public WebSocket channels the way the UI does.

A PlanFlow starts from a `bug` that reached awaiting_plan_approval with the Plan the server wrote by itself (CR-REQ-044), and offers the reads and
waits the PRV cases share. The fake claude echoes a revision into the Plan's notes ("[revised, round N, asked: ...]"), which proves the ask and
the earlier version reached the model."""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import analysis_revision_users as people
import plan_auto_driver as apg
import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked

SEED_SCRIPT = Path(__file__).resolve().parent / "plan_documents_seed.py"
REVISION_WAIT = 150  # a stub answer takes seconds; [stub:slow] adds six


def echo(round_no: int, ask: str) -> str:
    """The marker the fake claude writes into the notes of a revised Plan (see fake_claude_cli/claude, plan_revision_note)."""
    return f"[revised, round {round_no}, asked: {' '.join(ask.split())[:60]}]"


class PlanFlow:
    def __init__(self, ctx: AcceptanceContext, project_id: str, request_id: str):
        self.ctx, self.project_id, self.id = ctx, project_id, request_id

    # ── reads ─────────────────────────────────────────────────────────────────────────────────────
    def status(self) -> str:
        return drv.get(self.ctx, self.id)["status"]

    def versions(self, ws=None) -> list[dict]:
        """Newest first, as request.planRevisions returns them."""
        return (ws or self.ctx.ws).call("request.planRevisions", {"id": self.id, "pageSize": 100}).get("revisions", [])

    def pending(self, ws=None) -> list[dict]:
        pending = (ws or self.ctx.ws).call("approval.list", {"requestId": self.id, "status": "pending"}).get("approvals", [])
        return [a for a in pending if a["subjectType"] in apg.PLAN_GATES]

    def cancelled(self) -> list[dict]:
        return [a for a in self.ctx.ws.call("approval.list", {"requestId": self.id, "status": "cancelled"}).get("approvals", []) if a["subjectType"] in apg.PLAN_GATES]

    def proposal(self, revision: int = 0, ws=None) -> dict:
        args = {"id": self.id}
        if revision:
            args["revision"] = revision
        return (ws or self.ctx.ws).call("request.planProposal", args)

    def notes(self, revision: int = 0) -> str:
        return (self.proposal(revision).get("proposal") or {}).get("notes", "")

    def activity(self, kind: str) -> list[dict]:
        return self.ctx.ws.call("request.activity", {"id": self.id, "kinds": [kind], "pageSize": 100}).get("entries", [])

    def jobs(self) -> list[dict]:
        return apg.jobs(self.ctx, self.project_id, self.id)

    def plan_runs(self) -> list[apg.PlanRun]:
        return apg.plan_runs(self.ctx, self.project_id, self.id)

    # ── steps ─────────────────────────────────────────────────────────────────────────────────────
    def revise(self, ask: str = "", key: str = "", ws=None) -> dict:
        """request.generatePlan in its propose mode, with the ask when there is one. No commit is ever sent: the server's `revision` job commits."""
        args: dict = {"id": self.id}
        if ask:
            args["feedback"] = ask
        if key:
            args["idempotencyKey"] = key
        return (ws or self.ctx.ws).call("request.generatePlan", args, timeout=60)

    def wait_new_gate(self, old_ids: set[str], timeout: float = REVISION_WAIT) -> dict:
        """The new pending Plan approval, once the Request is back at the gate after a revision."""
        def back():
            if self.status() != "awaiting_plan_approval":
                return None
            fresh = [a for a in self.pending() if a["id"] not in old_ids]
            return fresh[0] if len(fresh) == 1 else None
        return wait_until(back, timeout=timeout, interval=2.0, what="the revised Plan to wait for approval with a new Approval")

    def revise_and_wait(self, ask: str, key: str = "") -> dict:
        before = {a["id"] for a in self.pending()}
        self.revise(ask, key)
        return self.wait_new_gate(before)

    def assert_one_pending(self) -> dict:
        pending = self.pending()
        assert len(pending) == 1, f"{len(pending)} pending Plan approvals, expected exactly one"
        return pending[0]


def new_plan_flow(ctx: AcceptanceContext, project_id: str, title: str) -> PlanFlow:
    """create -> analysis -> approved -> the server writes the Plan: the Request waits at awaiting_plan_approval."""
    flow = apg.to_analysis_gate(ctx, project_id, "bug", title)
    apg.approve_analysis(ctx, flow)
    apg.wait_plan(ctx, flow.id)
    return PlanFlow(ctx, project_id, flow.id)


def name_plan_approver(ctx: AcceptanceContext, project_id: str, label: str):
    """A user who is a project member and an approver of the Plan gates (plan, task_list), written for the throwaway project and removed again.
    Policies are snapshotted when an approval opens, so call this before the Request reaches the gate."""
    user = people.create_user(ctx, label)
    people.name_approver(ctx, project_id, user)  # member + the analysis gates
    tenant_id = apg.tenant(project_id)
    approvers = f'["reporter","role:admin","user:{user.id}"]'
    rows = ",".join(f"(gen_random_uuid(), '{tenant_id}', '{project_id}', '{subject}', '{approvers}'::jsonb, true, 10, true, 1, '{people.POLICY_AUTHOR}')" for subject in apg.PLAN_GATES)
    try:
        apg.psql("request", f"begin; set local app.tenant_id = '{tenant_id}'; insert into request.approval_policies (id, tenant_id, project_id, subject_type, approvers, "
                            f"allow_requester_approve, priority, enabled, version, created_by) values {rows}; commit;")
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        raise Blocked(f"cannot write the Plan approval policy in the local docker stack ({exc})") from exc
    return user


def member_only(ctx: AcceptanceContext, project_id: str, label: str):
    """A project member who is not an approver of anything."""
    user = people.create_user(ctx, label)
    try:
        ctx.ws.call("project.addMember", {"projectId": project_id, "userId": user.id, "role": "member"})
    except Exception as exc:  # noqa: BLE001 - any refusal means the environment cannot make this user a member
        raise Blocked(f"cannot add {label} to the project through project.addMember: {str(exc)[:160]}") from exc
    ctx.on_cleanup(f"remove {user.email} from project {project_id}", lambda: ctx.ws.call("project.removeMember", {"projectId": project_id, "userId": user.id}))
    return user


def run_seed(ctx: AcceptanceContext, request_id: str) -> dict:
    """plan_documents_seed.py seed: plants an OpenSpec change (FAKE data, local stack only) and registers its removal. Returns the facts it prints."""
    done = subprocess.run([sys.executable, str(SEED_SCRIPT), "seed", request_id], capture_output=True, text=True, timeout=90)
    if done.returncode != 0:
        raise Blocked(f"plan_documents_seed.py seed failed ({done.returncode}): {(done.stderr or done.stdout).strip()[-300:]}")
    ctx.on_cleanup(f"remove the planted OpenSpec change of {request_id}",
                   lambda: subprocess.run([sys.executable, str(SEED_SCRIPT), "clean", request_id], capture_output=True, text=True, timeout=90, check=True))
    return json.loads(done.stdout.strip().splitlines()[-1])
