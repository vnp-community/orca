#!/usr/bin/env python3
"""Registers (and later removes) the throwaway Orca project of a pilot run, and switches the knowledge-hub flags on for it.

  python3 pilot_register.py create --repo <worktree path> [--name pilot-tdd-2026-10-14] [--self-approval]
  python3 pilot_register.py self-approval --project <id>      # add the policies below to an existing pilot project
  python3 pilot_register.py show   --project <id>
  python3 pilot_register.py delete --project <id>

`create` makes a private project on the dev server (`dev-local-mac` unless ORCA_E2E_DEV_SERVER_NAME says otherwise) whose repo is the
given worktree, then sets engine=openspec and solution_agent=true through the admin session (the CLI token carries no role, so
`orca engine set` is refused) and prints the readiness check. It never touches another project.

`--self-approval` is for a stack with ONE user: the built-in policy forbids the reporter to approve the Solution, Plan and task list of a
size L request, so the pilot could never pass its gates. With the user's explicit permission it adds three project-scoped policies (those
gates only) that let the reporter approve them; `delete` removes them with the project. Nothing outside the pilot project changes. The
policy admin has no gateway channel yet, so this writes the rows in the request database of the local docker stack.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time

from orca_session import OrcaSession
from orca_stack_config import load_config


def blockers_text(pre: dict | None) -> str:
    if not pre:
        return "(no preflight)"
    lines = [f"preflight: ok={pre.get('ok')} openspec={pre.get('openspecVersion')!r}"]
    for kind in ("blockers", "gaps"):
        for n in pre.get(kind) or []:
            lines.append(f"  {kind[:-1]} {n.get('code')}: {n.get('message')}  -> {n.get('fixHint')}")
    lines.extend(f"  warning: {w}" for w in pre.get("warnings") or [])
    return "\n".join(lines)


SELF_APPROVAL_GATES = ("solution", "plan", "task_list")
POLICY_AUTHOR = "pilot_register"


def psql(db: str, sql: str) -> str:
    done = subprocess.run(["docker", "exec", "-i", "orca-go-postgres", "psql", "-U", "orca", "-d", db, "-v", "ON_ERROR_STOP=1", "-tA"],
                          input=sql, capture_output=True, text=True, timeout=60)
    if done.returncode != 0:
        raise RuntimeError(f"psql {db}: {done.stderr.strip()[:300]}")
    return done.stdout.strip()


def tenant_of_project(project_id: str) -> str:
    tenant = psql("project", f"select tenant_id from project.projects where id = '{project_id}'")
    if len(tenant) != 36:
        raise RuntimeError(f"cannot find the tenant of project {project_id}")
    return tenant


def allow_self_approval(project_id: str) -> None:
    tenant = tenant_of_project(project_id)
    rows = ",".join(
        f"(gen_random_uuid(), '{tenant}', '{project_id}', '{gate}', '[\"reporter\",\"role:admin\"]'::jsonb, true, 10, true, 1, '{POLICY_AUTHOR}')"
        for gate in SELF_APPROVAL_GATES)
    psql("request", f"begin; set local app.tenant_id = '{tenant}'; insert into request.approval_policies "
                    f"(id, tenant_id, project_id, subject_type, approvers, allow_requester_approve, priority, enabled, version, created_by) values {rows}; commit;")
    print(f"self-approval policies added for {', '.join(SELF_APPROVAL_GATES)} of project {project_id}")


def drop_self_approval(project_id: str) -> None:
    tenant = tenant_of_project(project_id)
    psql("request", f"begin; set local app.tenant_id = '{tenant}'; delete from request.approval_policies where project_id = '{project_id}' "
                    f"and created_by = '{POLICY_AUTHOR}'; commit;")
    print(f"self-approval policies of project {project_id} removed")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("action", choices=["create", "show", "delete", "self-approval"])
    ap.add_argument("--repo")
    ap.add_argument("--name", default=time.strftime("pilot-tdd-%Y-%m-%d"))
    ap.add_argument("--project")
    ap.add_argument("--self-approval", action="store_true", help="create: let the reporter approve the size L gates of this project (one-user stack)")
    args = ap.parse_args()

    cfg = load_config("pilot-register")
    session = OrcaSession(cfg)
    if session.login().status != 200:
        print("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*", file=sys.stderr)
        return 2
    ws = session.ws()
    try:
        if args.action == "create":
            if not args.repo:
                ap.error("create needs --repo")
            server = next((s for s in ws.call("devServer.list", {}) if s.get("name") == cfg.dev_server_name or s.get("hostname") == cfg.dev_server_name), None)
            if not server or server.get("status") != "connected":
                print(f"dev server {cfg.dev_server_name!r} is missing or not connected", file=sys.stderr)
                return 1
            proj = ws.call("project.create", {"name": args.name, "description": "pilot of the tdd-agent knowledge hub (throwaway)", "devServerId": server["id"],
                                               "repoPath": args.repo, "defaultBranch": "main", "visibility": "private"})
            print(f"project {proj['id']} created on dev server {server['id']}")
            args.project = proj["id"]
            cur = ws.call("request.engineSettings", {"projectId": args.project})
            res = ws.call("request.engineSettingsSet", {"projectId": args.project, "engine": "openspec", "solutionAgent": True,
                                                         "expectedVersion": cur.get("version", 0)})
            print(f"engine={res.get('engine')} solutionAgent={res.get('solutionAgent')} version={res.get('version')}")
            if args.self_approval:
                allow_self_approval(args.project)
        if not args.project:
            ap.error(f"{args.action} needs --project")
        if args.action == "self-approval":
            allow_self_approval(args.project)
            return 0
        if args.action in ("create", "show"):
            res = ws.call("request.engineSettings", {"projectId": args.project, "check": True})
            print(json.dumps({k: res.get(k) for k in ("engine", "solutionAgent", "openspecMinVersion", "version")}))
            print(blockers_text(res.get("preflight")))
            return 0 if (res.get("preflight") or {}).get("ok") else 1
        drop_self_approval(args.project)
        ws.call("project.delete", {"projectId": args.project})
        print(f"project {args.project} deleted")
        return 0
    finally:
        session.close()


if __name__ == "__main__":
    sys.exit(main())
