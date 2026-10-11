#!/usr/bin/env python3
"""Lists repos registered more than once on the same dev server. Read-only: it only calls project.list / repo.list.

  report-duplicate-repos.sh            # human-readable report; exit 0 = no duplicates, 1 = duplicates found
  report-duplicate-repos.sh --json     # same groups as JSON

Why: project-service now refuses to register a repo whose (tenant, dev server, path) is already registered, but repos
stored before that rule can already exist twice (the same folder in two projects). They are left alone; this report
shows them so you can decide which project should keep each one. There is deliberately no flag that changes data.

Two repos are duplicates when they share the dev server id and the normalised path. The normalisation is the one in
project-service (domain.NormalizeRepoLocation): outer spaces, trailing and repeated separators are ignored, `C:\\x`
and `C:/x` are the same, URLs with `://` only lose a trailing `/`; letter case is never folded. It sees only the
projects the signed-in user is a member of (project.list), so a duplicate in a project you cannot list is not reported.
Credentials come from deploy/dev/.env (or ORCA_E2E_ADMIN_*) and are never printed.
"""
from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path

FLOWS_DIR = Path(__file__).resolve().parents[3] / "tests" / "stages" / "e2e" / "orca-flows"


def _is_windows_path(p: str) -> bool:
    if p.startswith("\\\\"):
        return True
    if len(p) >= 3 and p[1] == ":" and p[0].isascii() and p[0].isalpha() and p[2] in "\\/":
        return True
    return len(p) == 2 and p[1] == ":" and p[0].isascii() and p[0].isalpha()


def normalize_repo_location(raw: str) -> str:
    """Same rules as project-service's domain.NormalizeRepoLocation (keep both in step, tests share the cases)."""
    p = raw.strip()
    if not p:
        return ""
    if "://" in p:
        return p.rstrip("/")
    if _is_windows_path(p):
        p = p.replace("/", "\\")
        unc = p.startswith("\\\\")
        parts = [s for s in p.split("\\") if s]
        if unc:
            return "\\\\" + "\\".join(parts)
        if len(parts) == 1 and parts[0].endswith(":"):
            return parts[0] + "\\"
        return "\\".join(parts)
    rooted = p.startswith("/")
    joined = "/".join(s for s in p.split("/") if s)
    return "/" + joined if rooted else joined


def group_duplicates(rows: list[dict]) -> list[dict]:
    """rows: {devServerId, url, projectId, projectName, repoId, position?}. Returns groups of two or more."""
    groups: dict[tuple[str, str], list[dict]] = defaultdict(list)
    for row in rows:
        location = normalize_repo_location(row.get("url", ""))
        if not location:
            continue
        groups[((row.get("devServerId") or "").lower(), location)].append(row)
    out = []
    for (dev_server, location), members in sorted(groups.items()):
        if len(members) > 1:
            out.append({"devServerId": dev_server, "path": location, "repos": members})
    return out


def collect_rows(ws, rpc_error) -> list[dict]:
    rows = []
    for project in ws.call("project.list", {}):
        try:
            repos = ws.call("repo.list", {"projectId": project["id"]}).get("repos", [])
        except rpc_error as exc:
            print(f"  skipped project {project['id']} ({project.get('name', '')!r}): {exc.message[:120]}", file=sys.stderr)
            continue
        for repo in repos:
            rows.append({
                "devServerId": repo.get("devServerId", ""), "url": repo.get("url", ""),
                "projectId": project["id"], "projectName": project.get("name", ""),
                "repoId": repo["id"], "displayName": repo.get("displayName", ""),
            })
    return rows


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--json", action="store_true", help="print the duplicate groups as JSON")
    args = parser.parse_args()

    sys.path.insert(0, str(FLOWS_DIR))
    from orca_session import OrcaSession  # noqa: E402
    from orca_stack_config import load_config  # noqa: E402
    from ws_rpc_client import RpcError  # noqa: E402

    session = OrcaSession(load_config("duplicates"))
    if session.login().status != 200:
        print("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*", file=sys.stderr)
        return 2
    ws = session.ws()
    try:
        rows = collect_rows(ws, RpcError)
        duplicates = group_duplicates(rows)
        if args.json:
            print(json.dumps({"scannedRepos": len(rows), "duplicateGroups": duplicates}, indent=2))
        else:
            print(f"{len(rows)} repo(s) scanned, {len(duplicates)} duplicated location(s)")
            for group in duplicates:
                print(f"  dev server {group['devServerId'] or '(none)'}  path {group['path']!r}")
                for repo in group["repos"]:
                    print(f"    repo={repo['repoId']}  project={repo['projectId']} ({repo['projectName']!r})")
            if duplicates:
                print("read-only report: nothing changed. Remove or move the extra repos from the project that should not own them.")
        return 1 if duplicates else 0
    finally:
        session.close()


if __name__ == "__main__":
    sys.exit(main())
