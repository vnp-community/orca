#!/usr/bin/env python3
"""Checks that the Jira site configured in `.env` works through the gateway: connect with email + PAT, list issues, disconnect.

Reads JR_URL / JR_EMAIL / JR_PAT_TOKEN (or JIRA_SITE_URL / JIRA_EMAIL / JIRA_PAT_TOKEN) from the process environment, then from
`deploy/dev/.env` and the repo's `.env`. The token is never printed, logged or put on a command line; any error text has it
replaced by ***. By default the connection is removed afterwards so the Tasks tab keeps showing "Connect Jira" for the user
to connect by hand; pass --keep to leave it connected.

  python3 jira_env_check.py [--keep] [--limit 5]
Exit code: 0 ok, 1 a check failed, 2 missing configuration.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from orca_session import OrcaSession  # noqa: E402
from orca_stack_config import load_config  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[4]


def read_dotenv(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if path.is_file():
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            m = re.match(r"^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$", line)
            if m:
                values[m.group(1)] = m.group(2).strip().strip("'\"")
    return values


def settings() -> tuple[str, str, str]:
    merged: dict[str, str] = {}
    for path in (REPO_ROOT / "deploy" / "dev" / ".env", REPO_ROOT / ".env"):
        merged.update(read_dotenv(path))
    merged.update({k: v for k, v in os.environ.items() if k.startswith(("JR_", "JIRA_"))})
    site = merged.get("JIRA_SITE_URL") or merged.get("JR_URL") or ""
    email = merged.get("JIRA_EMAIL") or merged.get("JR_EMAIL") or ""
    token = merged.get("JIRA_PAT_TOKEN") or merged.get("JR_PAT_TOKEN") or ""
    return site, email, token


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--keep", action="store_true", help="leave the Jira connection in place")
    parser.add_argument("--limit", type=int, default=5)
    args = parser.parse_args()

    site, email, token = settings()
    if not site or not token:
        print("missing configuration: set JR_URL and JR_PAT_TOKEN (and JR_EMAIL) in .env, or JIRA_SITE_URL / JIRA_PAT_TOKEN / JIRA_EMAIL", file=sys.stderr)
        return 2

    def scrub(text: object) -> str:
        return str(text).replace(token, "***")

    session = OrcaSession(load_config("jira-env-check"))
    if session.login().status != 200:
        print("could not sign in to the gateway with the admin account in deploy/dev/.env", file=sys.stderr)
        return 1
    ws = session.ws()
    failures: list[str] = []
    connected_here = False
    try:
        if ws.call("jira.status", {}).get("connected"):
            print("a Jira connection already exists for this user; disconnect it first (this check would replace it)", file=sys.stderr)
            return 1
        print(f"site: {site}   email: {'set' if email else 'not set'}   token: set ({len(token)} characters)")

        out = ws.call("jira.connect", {"siteUrl": site, "email": email, "apiToken": token})
        if not out.get("ok"):
            failures.append(f"connect failed: {scrub(out.get('error'))[:300]}")
        else:
            connected_here = True
            viewer = out.get("viewer") or {}
            print(f"connected as: {viewer.get('displayName') or viewer.get('accountId') or '(no display name)'}")
            projects = ws.call("jira.listProjects", {})
            keys = [p.get("key") for p in projects][:8] if isinstance(projects, list) else []
            print(f"projects visible: {len(projects) if isinstance(projects, list) else '?'}  e.g. {keys}")
            issues = ws.call("jira.searchIssues", {"jql": "assignee = currentUser() ORDER BY updated DESC", "limit": args.limit})
            if not isinstance(issues, list):
                failures.append(f"jira.searchIssues did not return a list: {type(issues).__name__}")
            else:
                print(f"issues assigned to the token's user: {len(issues)}")
                for issue in issues[: args.limit]:
                    print(f"  {issue.get('key')}  {str(issue.get('title') or issue.get('summary') or '')[:80]}")
                if token in json.dumps(issues):
                    failures.append("the token appeared in an issue payload")
    except Exception as exc:  # noqa: BLE001 - report, never crash with the token in a traceback
        failures.append(f"{type(exc).__name__}: {scrub(exc)[:300]}")
    finally:
        if connected_here and not args.keep:
            try:
                ws.call("jira.disconnect", {})
                print("disconnected again (use --keep to stay connected)")
            except Exception as exc:  # noqa: BLE001
                print(f"could not disconnect: {scrub(exc)[:200]}", file=sys.stderr)
        session.close()

    if failures:
        for f in failures:
            print(f"FAIL: {f}", file=sys.stderr)
        return 1
    print("OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
