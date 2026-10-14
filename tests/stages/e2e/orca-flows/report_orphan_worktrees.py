#!/usr/bin/env python3
"""Lists proposal worktrees that no Request needs (read-only). Exits 1 when it finds one.

  python3 report_orphan_worktrees.py --repo /path/to/project-repo            # asks Orca for the Request statuses
  python3 report_orphan_worktrees.py --repo /a --repo /b --number 12:cancelled --number 13:executing   # no Orca needed

A proposal worktree is on the branch `request/<number>-proposal`. It is an orphan when its Request is cancelled or unknown
to Orca. A completed Request's worktree is only listed: it waits for the archive step and the retention. Branches are never
reported, because they are kept on purpose. Credentials for the Orca lookup come from deploy/dev/.env like the suites.
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

from proposal_worktrees import find_orphans, list_proposal_branches

STATUSES = ["cancelled", "completed", "classifying", "awaiting_type_confirmation", "awaiting_information", "analyzing",
            "awaiting_analysis_approval", "planning", "awaiting_plan_approval", "executing", "request_backlog"]


def statuses_from_orca() -> dict[int, str]:
    from orca_session import OrcaSession
    from orca_stack_config import load_config

    session = OrcaSession(load_config("orphans"))
    if session.login().status != 200:
        raise SystemExit("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*")
    ws = session.ws()
    try:
        out: dict[int, str] = {}
        token = ""
        while True:
            page = ws.call("request.list", {"status": STATUSES, "pageSize": 100, "pageToken": token})
            for r in page.get("requests", []):
                if r.get("number"):
                    out[int(r["number"])] = r["status"]
            token = page.get("nextPageToken") or ""
            if not token:
                return out
    finally:
        session.close()


def parse_numbers(items: list[str]) -> dict[int, str]:
    out: dict[int, str] = {}
    for item in items:
        number, _, status = item.partition(":")
        if not number.isdigit() or not status:
            raise SystemExit(f"--number wants <number>:<status>, got {item!r}")
        out[int(number)] = status
    return out


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--repo", action="append", required=True, help="a project repository to inspect (repeatable)")
    parser.add_argument("--number", action="append", default=[], help="<request number>:<status>; replaces the Orca lookup")
    args = parser.parse_args()

    statuses = parse_numbers(args.number) if args.number else statuses_from_orca()
    orphans, held = [], []
    for repo in args.repo:
        o, h = find_orphans(Path(repo), statuses)
        orphans += o
        held += h
        print(f"{repo}: {len(list_proposal_branches(Path(repo)))} proposal branch(es) kept")
    for wt in held:
        print(f"  held   {wt.path}  ({wt.branch}, completed: waits for the archive step and the retention)")
    for o in orphans:
        print(f"  ORPHAN {o.worktree.path}  ({o.worktree.branch}): {o.reason}")
    print(f"{len(orphans)} orphaned proposal worktree(s)")
    if orphans:
        print("remove one with: orca request cleanup <request-id>  (or: git -C <repo> worktree remove --force <path>)")
    return 1 if orphans else 0


if __name__ == "__main__":
    sys.exit(main())
