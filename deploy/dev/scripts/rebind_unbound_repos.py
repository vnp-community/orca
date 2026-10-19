#!/usr/bin/env python3
"""Lists repos that are bound to no dev server and, only with --apply, binds them to one.

  rebind-unbound-repos.sh                                          # dry-run: list unbound repos (default)
  rebind-unbound-repos.sh --apply --dev-server dev-local-mac       # bind every unbound repo to that dev server
  rebind-unbound-repos.sh --apply --dev-server <id> --repo <repoId> [--repo <repoId> ...]   # only these repos

Why: repos stored before repos had to name a host (dev_server_id empty) cannot be operated on by a Docker
git-gateway (REPO_HOST_UNBOUND). Which host owns each path is something only you know, so nothing is guessed:
--apply needs an explicit --dev-server, which must exist, be approved and be connected right now. Each repo goes
through the product's own repo.rebindDevServer (owner-only, refused while the project has active workflows or
task executions). Binding does NOT verify the folder exists on that host. Credentials come from deploy/dev/.env
(or ORCA_E2E_ADMIN_*) and are never printed. Reuses the gateway session client of tests/stages/e2e/orca-flows.
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

FLOWS_DIR = Path(__file__).resolve().parents[3] / "tests" / "stages" / "e2e" / "orca-flows"
sys.path.insert(0, str(FLOWS_DIR))

from orca_session import OrcaSession  # noqa: E402
from orca_stack_config import load_config  # noqa: E402
from ws_rpc_client import RpcError  # noqa: E402


def find_unbound(ws, only_repos: set[str], only_project: str) -> list[dict]:
    unbound = []
    for project in ws.call("project.list", {}):
        if only_project and project["id"] != only_project:
            continue
        try:
            repos = ws.call("repo.list", {"projectId": project["id"]}).get("repos", [])
        except RpcError as exc:
            print(f"  skipped project {project['id']} ({project.get('name', '')!r}): {exc.message[:120]}")
            continue
        for repo in repos:
            if repo.get("devServerId"):
                continue
            if only_repos and repo["id"] not in only_repos:
                continue
            unbound.append({"project": project, "repo": repo})
    return unbound


def pick_dev_server(ws, wanted: str) -> dict:
    servers = ws.call("devServer.list", {})
    match = [s for s in servers if wanted in (s.get("id"), s.get("name"))]
    if len(match) != 1:
        names = ", ".join(f"{s.get('name')} ({s.get('status')})" for s in servers) or "none"
        raise SystemExit(f"--dev-server {wanted!r} matched {len(match)} dev servers; available: {names}")
    server = match[0]
    if server.get("approvalStatus") != "approved":
        raise SystemExit(f"dev server {server.get('name')!r} is not approved (approvalStatus={server.get('approvalStatus')!r})")
    if server.get("status") != "connected":
        raise SystemExit(f"dev server {server.get('name')!r} is {server.get('status')!r}; start its agent first, then re-run")
    return server


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--apply", action="store_true", help="bind the listed repos (default is a dry-run)")
    parser.add_argument("--dev-server", default="", help="dev server id or name to bind to (required with --apply)")
    parser.add_argument("--repo", action="append", default=[], help="limit to this repo id (repeatable)")
    parser.add_argument("--project", default="", help="limit to this project id")
    args = parser.parse_args()
    if args.apply and not args.dev_server:
        parser.error("--apply requires --dev-server <id|name>; no host is ever guessed")

    session = OrcaSession(load_config("rebind"))
    if session.login().status != 200:
        print("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*", file=sys.stderr)
        return 2
    ws = session.ws()
    try:
        unbound = find_unbound(ws, set(args.repo), args.project)
        print(f"{len(unbound)} repo(s) with no dev server")
        for item in unbound:
            repo, project = item["repo"], item["project"]
            print(f"  repo={repo['id']}  name={repo.get('displayName', '')!r}  path={repo.get('url', '')!r}  "
                  f"project={project['id']} ({project.get('name', '')!r})")
        if not args.apply:
            if unbound:
                print("dry-run: nothing changed. Re-run with --apply --dev-server <id|name> to bind them.")
            return 0
        if not unbound:
            return 0

        server = pick_dev_server(ws, args.dev_server)
        print(f"binding {len(unbound)} repo(s) to dev server {server.get('name')!r} ({server['id']})")
        failed = 0
        for item in unbound:
            repo = item["repo"]
            try:
                done = ws.call("repo.rebindDevServer", {"repoId": repo["id"], "newDevServerId": server["id"]})
                print(f"  bound   {repo['id']} -> {done.get('devServerId')}")
            except RpcError as exc:
                failed += 1
                print(f"  FAILED  {repo['id']}: {exc.message[:200]}")
        print(f"{len(unbound) - failed} bound, {failed} failed")
        return 1 if failed else 0
    finally:
        session.close()


if __name__ == "__main__":
    sys.exit(main())
