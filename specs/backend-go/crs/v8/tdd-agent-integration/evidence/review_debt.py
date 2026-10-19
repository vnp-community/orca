#!/usr/bin/env python3
"""Review debt of a tdd-agent knowledge graph: how many elements wait for review, by class and by document.

  python3 review_debt.py show --repo /path/to/repo
  python3 review_debt.py snapshot --repo /path/to/repo --out before.json     # take one before a write-back is applied
  python3 review_debt.py snapshot --repo /path/to/repo --out after.json      # and one after (or after `tdd-agent generate`)
  python3 review_debt.py compare before.json after.json

Orca never reads the graph, so the numbers come from tdd-agent's own commands: `status --json` for the totals and
`review --list` for the elements still waiting. Nothing here writes to the graph.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone


def run_json(tdd_agent: str, repo: str, *args: str):
    out = subprocess.run([tdd_agent, "-C", repo, "--json", *args], capture_output=True, text=True, timeout=120)
    if out.returncode != 0:
        raise RuntimeError(f"tdd-agent {' '.join(args)} failed: {out.stderr.strip() or out.stdout.strip()}")
    try:
        return json.loads(out.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"tdd-agent {' '.join(args)} did not print JSON: {exc}") from exc


def take_snapshot(repo: str, tdd_agent: str = "tdd-agent") -> dict:
    status = run_json(tdd_agent, repo, "status")
    pending = run_json(tdd_agent, repo, "review", "--list")
    knowledge = status.get("knowledge", {})
    by_class: dict[str, int] = {}
    by_doc: dict[str, int] = {}
    for item in pending:
        by_class[item.get("class", "?")] = by_class.get(item.get("class", "?"), 0) + 1
        by_doc[item.get("owner_doc", "?")] = by_doc.get(item.get("owner_doc", "?"), 0) + 1
    elements = int(knowledge.get("elements", 0))
    return {
        "repo": repo,
        "taken_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "snapshot": knowledge.get("snapshot", ""),
        "elements": elements,
        "needs_review": len(pending),
        "reviewed": max(elements - len(pending), 0),
        "by_class": dict(sorted(by_class.items())),
        "by_doc": dict(sorted(by_doc.items())),
    }


def compare(before: dict, after: dict) -> dict:
    def delta(a: dict, b: dict) -> dict:
        return {k: b.get(k, 0) - a.get(k, 0) for k in sorted(set(a) | set(b)) if b.get(k, 0) != a.get(k, 0)}

    return {
        "snapshot": [before.get("snapshot", ""), after.get("snapshot", "")],
        "needs_review": [before["needs_review"], after["needs_review"]],
        "added_to_review": max(after["needs_review"] - before["needs_review"], 0),
        "reviewed": [before["reviewed"], after["reviewed"]],
        "by_class": delta(before["by_class"], after["by_class"]),
        "by_doc": delta(before["by_doc"], after["by_doc"]),
    }


def render(snap: dict) -> str:
    lines = [f"{snap['repo']}  snapshot {snap['snapshot'] or '?'}",
             f"  {snap['needs_review']} of {snap['elements']} element(s) wait for review ({snap['reviewed']} reviewed)"]
    for label, key in (("class", "by_class"), ("document", "by_doc")):
        if snap[key]:
            lines.append(f"  by {label}: " + ", ".join(f"{k} {v}" for k, v in snap[key].items()))
    return "\n".join(lines)


def render_compare(c: dict) -> str:
    b, a = c["needs_review"]
    lines = [f"needs_review {b} -> {a}  ({'+' if a - b >= 0 else ''}{a - b}); reviewed {c['reviewed'][0]} -> {c['reviewed'][1]}"]
    if c["added_to_review"]:
        lines.append(f"  {c['added_to_review']} element(s) went back to review")
    for label, key in (("class", "by_class"), ("document", "by_doc")):
        if c[key]:
            lines.append(f"  by {label}: " + ", ".join(f"{k} {'+' if v > 0 else ''}{v}" for k, v in c[key].items()))
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--tdd-agent", default="tdd-agent", help="the tdd-agent executable")
    sub = parser.add_subparsers(dest="cmd", required=True)
    show = sub.add_parser("show")
    show.add_argument("--repo", required=True)
    snap = sub.add_parser("snapshot")
    snap.add_argument("--repo", required=True)
    snap.add_argument("--out", required=True)
    cmp_ = sub.add_parser("compare")
    cmp_.add_argument("before")
    cmp_.add_argument("after")
    args = parser.parse_args(argv)
    try:
        if args.cmd == "show":
            print(render(take_snapshot(args.repo, args.tdd_agent)))
        elif args.cmd == "snapshot":
            s = take_snapshot(args.repo, args.tdd_agent)
            with open(args.out, "w", encoding="utf-8") as f:
                json.dump(s, f, indent=1, ensure_ascii=False)
            print(render(s))
        else:
            with open(args.before, encoding="utf-8") as f, open(args.after, encoding="utf-8") as g:
                print(render_compare(compare(json.load(f), json.load(g))))
    except (RuntimeError, OSError, KeyError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
