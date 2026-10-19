"""The extra people of the ARV approver cases: two signed-in users who are neither the reporter nor an admin, and a project policy that
names one of them as an approver of the analysis gates.

There is no gateway channel for approval policies yet (see pilot_register.py), so the policy rows are written in the request database of the
local docker stack, scoped to the throwaway project and removed again. Anything the environment cannot do is a Blocked, never a FAIL."""
from __future__ import annotations

import dataclasses
import subprocess
from dataclasses import dataclass, field

from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked
from orca_session import OrcaSession
from pilot_register import psql, tenant_of_project
from ws_rpc_client import RpcError

POLICY_AUTHOR = "e2e-analysis-revision"
GATES = ("solution", "findings", "answer")


@dataclass
class ExtraUser:
    id: str
    email: str
    session: OrcaSession = field(repr=False)

    @property
    def ws(self):
        return self.session.ws()


def create_user(ctx: AcceptanceContext, label: str) -> ExtraUser:
    """An ordinary (role user) account made through the admin channel, signed in on its own session. The one-time password the
    server generates is used for the sign-in and never printed."""
    email = f"{ctx.cfg.run_prefix}-{label}@example.com"
    try:
        made = ctx.ws.call("admin.createUser", {"email": email, "name": f"{label} ({ctx.cfg.run_prefix})", "role": "user"})
    except RpcError as exc:
        raise Blocked(f"cannot create the {label} user through admin.createUser: {exc.message[:160]}") from exc
    user, password = made.get("user") or {}, made.get("generatedPassword") or ""
    if not user.get("id") or not password:
        raise Blocked(f"admin.createUser returned no user id or one-time password for {label}")
    ctx.on_cleanup(f"deactivate user {email}", lambda: ctx.ws.call("admin.deactivateUser", {"userId": user["id"]}))
    session = OrcaSession(dataclasses.replace(ctx.cfg, admin_email=email, admin_password=password))
    ctx.on_cleanup(f"close the session of {email}", session.close)
    reply = session.login()
    if reply.status != 200:
        raise Blocked(f"the {label} user cannot sign in (HTTP {reply.status}); a first-login password change may be required on this stack")
    return ExtraUser(id=user["id"], email=email, session=session)


def name_approver(ctx: AcceptanceContext, project_id: str, user: ExtraUser) -> None:
    """The approver joins the project as a member (analysis needs to read its repository, so steering is for members), then project-scoped policies for the three analysis gates: the reporter, admins and `user`. The approvers of a pending approval are
    snapshotted when it opens, so this must run before the Request reaches the gate."""
    try:
        ctx.ws.call("project.addMember", {"projectId": project_id, "userId": user.id, "role": "member"})
    except RpcError as exc:
        raise Blocked(f"cannot add the approver to the project through project.addMember: {exc.message[:160]}") from exc
    ctx.on_cleanup(f"remove {user.email} from project {project_id}", lambda: ctx.ws.call("project.removeMember", {"projectId": project_id, "userId": user.id}))
    try:
        tenant = tenant_of_project(project_id)
        approvers = f'["reporter","role:admin","user:{user.id}"]'
        rows = ",".join(f"(gen_random_uuid(), '{tenant}', '{project_id}', '{gate}', '{approvers}'::jsonb, true, 10, true, 1, '{POLICY_AUTHOR}')" for gate in GATES)
        psql("request", f"begin; set local app.tenant_id = '{tenant}'; insert into request.approval_policies "
                        f"(id, tenant_id, project_id, subject_type, approvers, allow_requester_approve, priority, enabled, version, created_by) values {rows}; commit;")
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        raise Blocked(f"cannot write the approval policy in the local docker stack ({exc}); the approver cases need `docker exec orca-go-postgres`") from exc

    def drop() -> None:
        psql("request", f"begin; set local app.tenant_id = '{tenant}'; delete from request.approval_policies where project_id = '{project_id}' "
                        f"and created_by = '{POLICY_AUTHOR}'; commit;")
    ctx.on_cleanup(f"remove the approval policies of project {project_id}", drop)
