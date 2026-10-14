#!/usr/bin/env python3
"""Checks, against the live stack and without any AI, the file and git calls the Request service's proposal workspace makes.

  python3 workspace_probe.py --worktree-id <id> --worktree-path <dir>

It is for a throwaway worktree (the proposal worktree of a pilot Request), because it writes a file, commits it and takes the
commit back with `git reset --hard HEAD~1` in <dir>. The first real plan run failed three times on things a fake could not show
(a missing path reported as a relay failure, a stat that read every path as missing, an idempotency key that was never stored);
running this before an expensive agent run finds that class of problem for nothing.
"""
from __future__ import annotations

import argparse
import base64
import subprocess
import sys

from orca_session import OrcaSession
from orca_stack_config import load_config


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--worktree-id", required=True)
    ap.add_argument("--worktree-path", required=True)
    args = ap.parse_args()
    wt, path = args.worktree_id, args.worktree_path
    probe_dir = "openspec/changes/probe-workspace"
    session = OrcaSession(load_config("workspace-probe"))
    if session.login().status != 200:
        print("login failed", file=sys.stderr)
        return 2
    ws = session.ws()
    failures: list[str] = []

    def check(name: str, ok: bool, detail: object = "") -> None:
        print(f"[{'ok' if ok else 'FAIL'}] {name}" + ("" if ok else f"  {detail}"))
        if not ok:
            failures.append(name)

    def git(*a: str) -> str:
        return subprocess.run(["git", "-C", path, "-c", "user.name=probe", "-c", "user.email=probe@example.com", *a], capture_output=True, text=True, timeout=60).stdout

    try:
        # Stat: a directory and a file that exist, and paths that do not.
        st = ws.call("files.stat", {"worktreeId": wt, "path": "docs"}) or {}
        check("stat of a directory that exists", st.get("exists") is True and (st.get("is_directory") or st.get("isDirectory")) is True, st)
        readme = ws.call("files.stat", {"worktreeId": wt, "path": "README.md"}) or {}
        check("stat of a file that exists", readme.get("exists") is True and not (readme.get("is_directory") or readme.get("isDirectory")), readme)
        missing = ws.call("files.stat", {"worktreeId": wt, "path": "backend/shared/pkg/aiclient/client.go"}) or {}
        check("stat of a file that is not there is exists=false, not an error", not missing.get("exists"), missing)

        # ReadDir of a missing directory is an empty list; of an existing one is not.
        check("readDir of a directory that exists", len(ws.call("files.readDir", {"worktreeId": wt, "path": "docs"}) or []) > 0)
        try:
            gone = ws.call("files.readDir", {"worktreeId": wt, "path": "no/such/dir"}) or []
            check("readDir of a missing directory is empty or NotFound", gone == [], gone)
        except Exception as exc:  # noqa: BLE001  the workspace adapter reads NotFound as an empty list
            check("readDir of a missing directory is empty or NotFound", "code = NotFound" in str(exc), str(exc)[:200])

        # Write (with parents), read back, stat.
        body = "## Why\nprobe\n"
        ws.call("files.write", {"worktreeId": wt, "path": f"{probe_dir}/proposal.md", "content": body, "createParents": True})
        got = ws.call("files.read", {"worktreeId": wt, "path": f"{probe_dir}/proposal.md"}) or {}
        raw = got.get("content") or ""
        text = base64.b64decode(raw).decode() if raw and got.get("encoding") != "base64" and not raw.startswith("##") else raw  # protojson sends bytes as base64
        check("write then read back", text == body, got)
        st = ws.call("files.stat", {"worktreeId": wt, "path": f"{probe_dir}/proposal.md"}) or {}
        check("stat of the file just written", st.get("exists") is True, st)

        # Status lists it as untracked; stage and commit only that path; the commit contains just it.
        status = ws.call("git.status", {"worktree": f"id:{wt}"}) or {}
        files = status.get("files") or status.get("entries") or []
        check("status lists the new file", any(probe_dir in str(f) for f in files), str(status)[:300])
        ws.call("git.stage", {"worktree": f"id:{wt}", "paths": [f"{probe_dir}/proposal.md"]})
        res = ws.call("git.commit", {"worktree": f"id:{wt}", "message": "probe: proposal", "paths": [f"{probe_dir}/proposal.md"]}) or {}
        head = git("rev-parse", "HEAD").strip()
        check("commit returns the sha of the new HEAD", (res.get("commit_sha") or res.get("commitSha")) == head, [res, head])
        changed = git("show", "--name-only", "--format=", "HEAD").split()
        check("the commit holds only the probe file", changed == [f"{probe_dir}/proposal.md"], changed)
        # DiscardAll: an untracked file is removed, a tracked one restored.
        ws.call("files.write", {"worktreeId": wt, "path": f"{probe_dir}/stray.md", "content": "x", "createParents": True})
        ws.call("git.bulkDiscard", {"worktree": f"id:{wt}", "paths": [f"{probe_dir}/stray.md"]})
        st = ws.call("files.stat", {"worktreeId": wt, "path": f"{probe_dir}/stray.md"}) or {}
        check("discarding an untracked file removes it", not st.get("exists"), st)
    finally:
        git("reset", "--hard", "HEAD~1") if "probe: proposal" in git("log", "-1", "--format=%s") else None
        git("clean", "-fdq", "--", probe_dir)
        session.close()
    left = git("status", "--porcelain", "--", probe_dir).strip()
    check("the probe left nothing behind", left == "", left)
    print(f"{len(failures)} failure(s)")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
