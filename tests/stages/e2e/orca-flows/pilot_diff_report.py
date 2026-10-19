#!/usr/bin/env python3
"""Reads what the agents of a pilot Request changed, without taking their word for it (T4.5).

  python3 pilot_diff_report.py --repo <pilot worktree> --branch-prefix task/ [--base main] [--run-go-tests] [--out report.md]

For each branch under refs/heads/<prefix>: the commits and files it holds against the base, which of those files lie in a
protected place (a change that must stay untouched, such as integration-hub, openspec/, docs/tdd/, .tdd-agent/), and, with
--run-go-tests, the Go tests of every package the branch changed, run in that branch's own worktree. A red test or a protected
path is reported, never repaired: a person fixing a task worktree by hand and calling it green would hide a flaw of the flow.
Exit code 1 when any branch has a protected path or a red test.
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

DEFAULT_PROTECTED = (
    "backend/services/infrastructure/integration-hub/",
    "openspec/",
    "docs/tdd/",
    ".tdd-agent/",
)


def git(repo: str | Path, *args: str, check: bool = True) -> str:
    done = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True, timeout=120)
    if check and done.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed in {repo}: {done.stderr.strip()[:300]}")
    return done.stdout


def task_branches(repo: str | Path, prefix: str) -> list[str]:
    out = git(repo, "for-each-ref", "--format=%(refname:short)", f"refs/heads/{prefix}")
    return sorted(line for line in out.splitlines() if line)


def worktree_of(repo: str | Path, branch: str) -> str | None:
    """The directory where git has the branch checked out, if it is checked out anywhere."""
    current = None
    for line in git(repo, "worktree", "list", "--porcelain").splitlines():
        if line.startswith("worktree "):
            current = line[len("worktree "):]
        elif line.startswith("branch ") and line[len("branch "):] == f"refs/heads/{branch}":
            return current
    return None


def changed_files(repo: str | Path, base: str, branch: str) -> list[str]:
    merge_base = git(repo, "merge-base", base, branch).strip()
    return [p for p in git(repo, "diff", "--name-only", f"{merge_base}..{branch}").splitlines() if p]


def working_tree_files(worktree: str | Path) -> list[str]:
    """Files changed but not committed in the worktree. The agents of a Request's tasks leave their code uncommitted, and all the
    tasks of one plan share one worktree, so this is where most of what they did is."""
    out = git(worktree, "status", "--porcelain=v1", "-z", "--untracked-files=all")
    files, entries, skip = [], [e for e in out.split("\0") if e], False
    for entry in entries:
        if skip:
            skip = False
            continue
        if entry[0] in "RC":
            skip = True
        files.append(entry[3:])
    return files


def protected_hits(files: list[str], protected: tuple[str, ...]) -> list[str]:
    return [f for f in files if any(f.startswith(p) for p in protected)]


def go_packages(files: list[str], root: Path) -> list[tuple[Path, str]]:
    """(module dir, package path relative to it) for every Go file that changed and still exists."""
    seen: dict[tuple[Path, str], None] = {}
    for f in files:
        if not f.endswith(".go") or not (root / f).exists():
            continue
        d = (root / f).parent
        mod = d
        while mod != root.parent and not (mod / "go.mod").exists():
            mod = mod.parent
        if not (mod / "go.mod").exists():
            continue
        seen[(mod, "./" + str(d.relative_to(mod)).replace("\\", "/") if d != mod else ".")] = None
    return list(seen)


def run_go_tests(root: Path, packages: list[tuple[Path, str]]) -> list[tuple[str, bool, str]]:
    results = []
    for mod, pkg in packages:
        done = subprocess.run(["go", "test", "-count=1", pkg], cwd=mod, capture_output=True, text=True, timeout=600)
        label = f"{mod.relative_to(root)}:{pkg}"
        results.append((label, done.returncode == 0, (done.stdout + done.stderr).strip().splitlines()[-1] if (done.stdout + done.stderr).strip() else ""))
    return results


def report(repo: str, branch_prefix: str, base: str, protected: tuple[str, ...], run_tests: bool) -> tuple[str, bool]:
    lines = [f"# Pilot diff report: {repo}", "", f"base `{base}`, branches `{branch_prefix}*`", ""]
    bad = False
    branches = task_branches(repo, branch_prefix)
    if not branches:
        lines.append("No branch under that prefix.")
    for br in branches:
        committed = changed_files(repo, base, br)
        wt = worktree_of(repo, br)
        uncommitted = working_tree_files(wt) if wt else []
        files = sorted(set(committed) | set(uncommitted))
        stat = git(repo, "diff", "--shortstat", f"{git(repo, 'merge-base', base, br).strip()}..{br}").strip()
        hits = protected_hits(files, protected)
        lines += [f"## {br}", "", f"{len(files)} file(s) in all: {len(committed)} committed ({stat or 'no change'}), {len(uncommitted)} changed but not committed", ""]
        lines += [f"- `{f}`" for f in files[:40]] + ([f"- ... {len(files) - 40} more"] if len(files) > 40 else [])
        if hits:
            bad = True
            lines += ["", f"**PROTECTED PATHS CHANGED ({len(hits)}):** " + ", ".join(f"`{h}`" for h in hits[:10])]
        if run_tests:
            if wt is None:
                lines += ["", "Tests not run: the branch is not checked out in a worktree."]
            else:
                results = run_go_tests(Path(wt), go_packages(files, Path(wt)))
                lines += ["", "Go tests of the changed packages (re-run here, not the agent's report):"]
                for label, ok, last in results:
                    bad = bad or not ok
                    lines.append(f"- {'PASS' if ok else 'FAIL'} `{label}` {last}")
                if not results:
                    lines.append("- no Go package changed")
        lines.append("")
    return "\n".join(lines), bad


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--branch-prefix", default="task/")
    ap.add_argument("--base", default="main", help="the ref the task branches were made from: for the pilot, the pilot branch itself")
    ap.add_argument("--protected", action="append", default=[], help="a path prefix that must stay untouched (adds to the defaults)")
    ap.add_argument("--run-go-tests", action="store_true")
    ap.add_argument("--out")
    args = ap.parse_args()
    text, bad = report(args.repo, args.branch_prefix, args.base, DEFAULT_PROTECTED + tuple(args.protected), args.run_go_tests)
    if args.out:
        Path(args.out).write_text(text)
        print(f"wrote {args.out}")
    else:
        print(text)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
