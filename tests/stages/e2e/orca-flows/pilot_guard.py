#!/usr/bin/env python3
"""Guards a repo of the user while a real agent works on it (the vnp-qa-platform pilot), for any repo, with git only.

  python3 pilot_guard.py snapshot --repo <path> --out before.json
  python3 pilot_guard.py compare  --repo <path> --before before.json            # while the run goes: only Orca's growth is allowed
  python3 pilot_guard.py compare  --repo <path> --before before.json --strict   # after cleanup: the repo must be what it was
  python3 pilot_guard.py watch    --repo <path> --before before.json --interval 15
  python3 pilot_guard.py orca-snapshot --out orca-before.json                   # projects and Requests of the user in Orca
  python3 pilot_guard.py orca-compare  --before orca-before.json [--allow-project <id>] [--strict]

The rules are those of ui-live/live-main-checkout-rules.ts: Orca may add task branches (refs/heads/*) and worktrees named
`<repo>-*` next to the repo, and nothing else. Moving HEAD, changing a file, the repo-local config or a ref that existed, a tag, a
stash, a note, a worktree of the user disappearing or changing: all are violations. Remote-tracking refs may move, because
creating a worktree fetches. The exit code is 1 on any violation, so `watch` can stop a run.

The check detects and stops; it does not prevent every write outside a worktree (risk B4 of SOL-TDD-002 stays open).
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import time
from pathlib import Path

MAX_HASHED_BYTES = 16 * 1024 * 1024
# Files the operating system rewrites on its own (Finder); a change to them is not the agent touching the repo.
OS_NOISE = {".DS_Store"}


def git(repo: str | Path, *args: str) -> str:
    done = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True, timeout=60)
    if done.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed in {repo}: {done.stderr.strip()[:300]}")
    return done.stdout


def file_hash(path: Path) -> str:
    try:
        if path.is_symlink():
            return "link:" + os.readlink(path)
        if path.is_dir():
            return "dir"
        if path.stat().st_size > MAX_HASHED_BYTES:
            return f"big:{path.stat().st_size}"
        return hashlib.sha256(path.read_bytes()).hexdigest()
    except FileNotFoundError:
        return "deleted"


def fingerprint(worktree: str | Path) -> dict:
    """HEAD, branch, repo-local config and every changed or untracked file with a content hash."""
    root = Path(worktree)
    head = git(root, "rev-parse", "HEAD").strip()
    branch = git(root, "rev-parse", "--abbrev-ref", "HEAD").strip()
    config = git(root, "config", "--local", "--list").strip()
    raw = git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
    files: dict[str, str] = {}
    entries = [e for e in raw.split("\0") if e]
    skip_next = False
    for i, entry in enumerate(entries):
        if skip_next:  # the old name of a rename
            skip_next = False
            continue
        xy, name = entry[:2], entry[3:]
        if xy[0] in "RC":
            skip_next = True
        if Path(name).name in OS_NOISE:
            continue
        files[name] = f"{xy}:{file_hash(root / name)}"
    return {"head": head, "branch": branch, "config": hashlib.sha256(config.encode()).hexdigest(), "files": files}


def list_worktrees(repo: str | Path) -> list[str]:
    out = git(repo, "worktree", "list", "--porcelain")
    return [line[len("worktree "):] for line in out.splitlines() if line.startswith("worktree ")]


def snapshot(repo: str | Path) -> dict:
    repo = str(Path(repo).resolve())
    worktrees = [str(Path(p).resolve()) for p in list_worktrees(repo)]
    main = worktrees[0]
    refs = {}
    for line in git(repo, "for-each-ref", "--format=%(refname) %(objectname)").splitlines():
        name, _, sha = line.partition(" ")
        refs[name] = sha
    if (Path(main) / ".git").is_dir() and "refs/stash" not in refs:
        stash = subprocess.run(["git", "-C", main, "rev-parse", "--verify", "-q", "refs/stash"], capture_output=True, text=True)
        if stash.returncode == 0:
            refs["refs/stash"] = stash.stdout.strip()
    others = {}
    for wt in worktrees:
        if wt not in (main, repo):
            others[wt] = fingerprint(wt)
    parents = sorted({str(Path(main).parent), str(Path(repo).parent)})
    return {
        "repo": repo, "main": main, "taken_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "pilot": fingerprint(repo), "main_checkout": fingerprint(main) if main != repo else None,
        "refs": refs, "worktrees": others,
        "neighbours": {p: sorted(os.listdir(p)) for p in parents if os.path.isdir(p)},
    }


def is_user_ref_growth(ref: str) -> bool:
    return ref.startswith("refs/heads/")


def allowed_worktree(path: str, before: dict) -> bool:
    names = {Path(before["repo"]).name, Path(before["main"]).name}
    parents = {str(Path(before["repo"]).parent), str(Path(before["main"]).parent)}
    p = Path(path)
    return str(p.parent) in parents and any(p.name.startswith(n + "-") for n in names)


def compare(before: dict, now: dict, strict: bool = False, allow_neighbours: frozenset[str] = frozenset()) -> tuple[list[str], list[str]]:
    """(violations, growth). strict allows no growth at all: it is the check after cleanup.
    allow_neighbours names directories Orca's own tools legitimately create beside the repo (the agent's `tmp/` for MCP configs, in its work dir)."""
    problems: list[str] = []
    growth: list[str] = []
    for label, was, is_ in (("the pilot worktree", before["pilot"], now["pilot"]), ("the main checkout", before["main_checkout"], now["main_checkout"])):
        if was is None or is_ is None:
            continue
        if was["head"] != is_["head"]:
            problems.append(f"{label} moved HEAD ({was['head'][:10]} -> {is_['head'][:10]})")
        if was["branch"] != is_["branch"]:
            problems.append(f"{label} switched branch ({was['branch']} -> {is_['branch']})")
        if was["config"] != is_["config"]:
            problems.append(f"{label}: the repo-local git config changed")
        changed = sorted(f for f in set(was["files"]) | set(is_["files"]) if was["files"].get(f) != is_["files"].get(f))
        if changed:
            problems.append(f"{len(changed)} file(s) of {label} changed: {', '.join(changed[:8])}")
    for ref, sha in before["refs"].items():
        if now["refs"].get(ref) == sha:
            continue
        if ref.startswith("refs/remotes/"):
            growth.append(f"remote-tracking ref {ref} {'moved' if ref in now['refs'] else 'was removed'} (a fetch)")
        else:
            problems.append(f"ref {ref} {'moved' if ref in now['refs'] else 'was deleted'}")
    for ref in now["refs"]:
        if ref in before["refs"]:
            continue
        if is_user_ref_growth(ref):
            growth.append(f"new branch {ref}")
        elif ref.startswith("refs/remotes/"):
            growth.append(f"new remote-tracking ref {ref} (a fetch)")
        else:
            problems.append(f"new ref outside refs/heads/: {ref} (a tag, a stash or a note)")
    for path, was in before["worktrees"].items():
        is_ = now["worktrees"].get(path)
        if is_ is None:
            problems.append(f"worktree {path} of the user disappeared")
        elif was != is_:
            problems.append(f"worktree {path} of the user changed")
    new_worktrees = [w for w in now["worktrees"] if w not in before["worktrees"]]
    for w in new_worktrees:
        if allowed_worktree(w, before):
            growth.append(f"new worktree {w}")
        else:
            problems.append(f"a worktree appeared outside the allowed place `<parent>/<repo>-*`: {w}")
    grown_dirs = {Path(w).name for w in new_worktrees}
    for parent, was_list in before["neighbours"].items():
        now_list = now["neighbours"].get(parent, [])
        gained = [n for n in now_list if n not in was_list and n not in grown_dirs and n not in allow_neighbours]
        lost = [n for n in was_list if n not in now_list]
        if gained:
            problems.append(f"new entries next to the repo in {parent}: {', '.join(gained[:8])}")
        if lost:
            problems.append(f"entries next to the repo disappeared from {parent}: {', '.join(lost[:8])}")
    if strict:
        problems.extend(f"left behind: {g}" for g in growth if "remote-tracking" not in g)
    return problems, growth


# --- what the user has in Orca -------------------------------------------------------------------------------------------


def orca_snapshot() -> dict:
    from orca_session import OrcaSession
    from orca_stack_config import load_config

    session = OrcaSession(load_config("pilot-guard"))
    if session.login().status != 200:
        raise RuntimeError("login failed; check deploy/dev/.env or ORCA_E2E_ADMIN_*")
    ws = session.ws()
    try:
        projects = {p["id"]: {"name": p.get("name", ""), "repos": sorted(str(r.get("path", r)) for r in p.get("repos", []) or [])} for p in ws.call("project.list", {})}
        requests: dict[str, dict] = {}
        token = ""
        while True:
            page = ws.call("request.list", {"pageSize": 100, "pageToken": token})
            for r in page.get("requests", []):
                requests[r["id"]] = {"number": r.get("number"), "status": r.get("status"), "project": r.get("projectId"), "title": r.get("title", "")[:80]}
            token = page.get("nextPageToken") or ""
            if not token:
                break
        return {"projects": projects, "requests": requests}
    finally:
        session.close()


def orca_compare(before: dict, now: dict, allowed_projects: set[str], strict: bool) -> list[str]:
    problems = []
    for pid, was in before["projects"].items():
        if now["projects"].get(pid) != was:
            problems.append(f"project {pid} ({was['name']}) {'disappeared' if pid not in now['projects'] else 'changed'}")
    for rid, was in before["requests"].items():
        is_ = now["requests"].get(rid)
        if is_ is None:
            problems.append(f"Request #{was['number']} disappeared")
        elif is_ != was:
            problems.append(f"Request #{was['number']} of the user changed ({was['status']} -> {is_['status']})")
    for pid in now["projects"]:
        if pid not in before["projects"] and (strict or pid not in allowed_projects):
            problems.append(f"new project {pid} ({now['projects'][pid]['name']}) that is not from the pilot")
    for rid, r in now["requests"].items():
        if rid not in before["requests"] and (strict or r["project"] not in allowed_projects):
            problems.append(f"new Request #{r['number']} in a project that is not from the pilot")
    return problems


def read_json(path: str) -> dict:
    return json.loads(Path(path).read_text())


def report(problems: list[str], growth: list[str]) -> int:
    for g in growth:
        print(f"  allowed growth: {g}")
    for p in problems:
        print(f"VIOLATION: {p}", file=sys.stderr)
    print(f"{len(problems)} violation(s), {len(growth)} allowed change(s)")
    return 1 if problems else 0


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name in ("snapshot", "orca-snapshot"):
        p = sub.add_parser(name)
        p.add_argument("--out", required=True)
        if name == "snapshot":
            p.add_argument("--repo", required=True)
    for name in ("compare", "watch", "orca-compare"):
        p = sub.add_parser(name)
        p.add_argument("--before", required=True)
        p.add_argument("--strict", action="store_true", help="after cleanup: no growth is allowed")
        p.add_argument("--allow-neighbour", action="append", default=[], help="a directory the agent may create next to the repo, e.g. tmp")
        if name != "orca-compare":
            p.add_argument("--repo", required=True)
        if name == "watch":
            p.add_argument("--interval", type=float, default=15.0)
        if name == "orca-compare":
            p.add_argument("--allow-project", action="append", default=[], help="a project the pilot created")
    args = ap.parse_args(argv)

    if args.cmd == "snapshot":
        Path(args.out).write_text(json.dumps(snapshot(args.repo), indent=2, sort_keys=True))
        print(f"wrote {args.out}")
        return 0
    if args.cmd == "orca-snapshot":
        snap = orca_snapshot()
        Path(args.out).write_text(json.dumps(snap, indent=2, sort_keys=True))
        print(f"wrote {args.out}: {len(snap['projects'])} project(s), {len(snap['requests'])} Request(s)")
        return 0
    if args.cmd == "orca-compare":
        problems = orca_compare(read_json(args.before), orca_snapshot(), set(args.allow_project), args.strict)
        return report(problems, [])
    before = read_json(args.before)
    if args.cmd == "compare":
        return report(*compare(before, snapshot(args.repo), args.strict, frozenset(args.allow_neighbour)))
    while True:  # watch
        problems, growth = compare(before, snapshot(args.repo), args.strict, frozenset(args.allow_neighbour))
        for _ in range(2):
            if not problems:
                break
            # Orca makes a worktree directory a moment before git lists it; only a violation that is still there counts.
            time.sleep(5)
            problems, growth = compare(before, snapshot(args.repo), args.strict, frozenset(args.allow_neighbour))
        if problems:
            print(f"[{time.strftime('%H:%M:%S')}] the repo of the user changed: stop the run", file=sys.stderr)
            return report(problems, growth)
        time.sleep(args.interval)


if __name__ == "__main__":
    sys.exit(main())
