#!/usr/bin/env python3
"""Lists open Requests whose project no longer exists (read-only), and cancels them only when asked.

  python3 report_orphan_requests.py                 # report
  python3 report_orphan_requests.py --cancel        # cancel each orphan with reason 'orphan_project'

Requests and projects live in different databases, so the check goes through the gateway: request.list, then
project.get once per distinct project. Nothing is deleted; --cancel uses the normal request.cancel transition,
which is audited and frees the tenant's open-Request quota. Credentials come from deploy/dev/.env like the suites.
"""
from __future__ import annotations

import argparse
import sys

from orca_session import OrcaSession
from orca_stack_config import load_config
from ws_rpc_client import RpcError

OPEN_STATUSES = [
    "classifying", "awaiting_type_confirmation", "awaiting_information", "analyzing", "awaiting_analysis_approval",
    "planning", "awaiting_plan_approval", "executing", "request_backlog",
]


def list_open_requests(ws) -> list[dict]:
    out, token = [], ""
    while True:
        page = ws.call("request.list", {"status": OPEN_STATUSES, "pageSize": 100, "pageToken": token})
        out.extend(page.get("requests", []))
        token = page.get("nextPageToken") or ""
        if not token:
            return out


def project_exists(ws, project_id: str, cache: dict[str, bool]) -> bool:
    if project_id not in cache:
        try:
            ws.call("project.get", {"projectId": project_id})
            cache[project_id] = True
        except RpcError as exc:
            if "NOT_FOUND" not in exc.message:
                raise  # a failing lookup is not proof of an orphan
            cache[project_id] = False
    return cache[project_id]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--cancel", action="store_true", help="cancel the orphans found (reason: orphan_project)")
    args = parser.parse_args()

    session = OrcaSession(load_config("orphans"))
    if session.login().status != 200:
        print("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*", file=sys.stderr)
        return 2
    ws = session.ws()
    try:
        requests = list_open_requests(ws)
        cache: dict[str, bool] = {}
        orphans = [r for r in requests if r.get("projectId") and not project_exists(ws, r["projectId"], cache)]
        print(f"{len(requests)} open Requests across {len(cache)} projects; {len(orphans)} orphaned")
        for r in orphans:
            print(f"  {r.get('displayId') or r['id']}  status={r['status']}  project={r['projectId']}  title={r.get('title', '')[:60]!r}")
        if args.cancel:
            for r in orphans:
                ws.call("request.cancel", {"id": r["id"], "reason": "orphan_project", "expectedVersion": r["version"]})
                print(f"  cancelled {r.get('displayId') or r['id']}")
        elif orphans:
            print("re-run with --cancel to cancel them")
        return 0
    finally:
        session.close()


if __name__ == "__main__":
    sys.exit(main())
