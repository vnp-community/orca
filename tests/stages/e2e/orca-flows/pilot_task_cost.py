#!/usr/bin/env python3
"""Estimates what the task runs of a pilot cost, from the Claude session transcripts they left (T4.7).

  python3 pilot_task_cost.py --worktree <task worktree path> [--request <id>]

A contract run reports no usage to Orca yet, so the cost of a task is not recorded anywhere. Claude keeps a transcript of each
session under ~/.claude/projects/<the worktree path with / and . replaced by ->/ with the token counts of every reply. This sums them
and prices them with per-million-token rates fitted (least squares) on the analysis and plan runs of the same stack that DO report
their cost. It is an estimate and says how well the fit held; it is not a bill.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

FIELDS = ("input_tokens", "output_tokens", "cache_read_input_tokens", "cache_creation_input_tokens")


def solve(a: list[list[float]], b: list[float]) -> list[float]:
    """Gaussian elimination with partial pivoting on a small square system."""
    n = len(b)
    m = [row[:] + [b[i]] for i, row in enumerate(a)]
    for c in range(n):
        p = max(range(c, n), key=lambda r: abs(m[r][c]))
        m[c], m[p] = m[p], m[c]
        if abs(m[c][c]) < 1e-12:
            raise ValueError("the reported runs do not pin the prices down")
        for r in range(n):
            if r != c:
                f = m[r][c] / m[c][c]
                m[r] = [x - f * y for x, y in zip(m[r], m[c])]
    return [m[i][n] / m[i][i] for i in range(n)]


def fit(rows: list[tuple[float, float, float, float, float]]) -> tuple[list[float], float]:
    """Least squares of cost = sum(rate_i * tokens_i): the normal equations. Returns (rates, worst relative error)."""
    ata = [[sum(r[i] * r[j] for r in rows) for j in range(4)] for i in range(4)]
    atb = [sum(r[i] * r[4] for r in rows) for i in range(4)]
    rates = solve(ata, atb)
    worst = max(abs(sum(rt * r[i] for i, rt in enumerate(rates)) - r[4]) / r[4] for r in rows)
    return rates, worst


def reported_runs() -> list[tuple[float, float, float, float, float]]:
    sql = "select run_stats::text from request.analysis_runs where run_stats::text like '%cost_usd%'"
    out = subprocess.run(["docker", "exec", "-i", "orca-go-postgres", "psql", "-U", "orca", "-d", "request", "-tA"], input=sql, capture_output=True, text=True, timeout=60)
    rows = []
    for line in out.stdout.splitlines():
        u = json.loads(line).get("usage") or {}
        if u.get("cost_usd"):
            rows.append((u["input_tokens"], u["output_tokens"], u["cache_read_tokens"], u["cache_creation_tokens"], u["cost_usd"]))
    return rows


def transcript_dir(worktree: str) -> Path:
    return Path.home() / ".claude" / "projects" / worktree.replace("/", "-").replace(".", "-").replace("_", "-")


def session_usage(path: Path) -> tuple[list[float], int]:
    """(tokens per field, replies) of one transcript; a streamed reply appears once per message id."""
    seen: set[str] = set()
    total = [0.0] * 4
    for line in path.read_text(errors="replace").splitlines():
        try:
            ev = json.loads(line)
        except json.JSONDecodeError:
            continue
        msg = ev.get("message") or {}
        usage = msg.get("usage")
        if ev.get("type") != "assistant" or not usage:
            continue
        mid = msg.get("id") or ev.get("uuid") or ""
        if mid in seen:
            continue
        seen.add(mid)
        for i, f in enumerate(FIELDS):
            total[i] += usage.get(f) or 0
    return total, len(seen)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--worktree", action="append", required=True, help="a task worktree path (repeatable)")
    args = ap.parse_args()
    rows = reported_runs()
    if len(rows) < 4:
        print(f"only {len(rows)} run(s) report a cost: not enough to fit prices", file=sys.stderr)
        return 1
    rates, worst = fit(rows)
    print(f"prices fitted on {len(rows)} reported run(s), per million tokens: input {rates[0]*1e6:.2f}, output {rates[1]*1e6:.2f}, "
          f"cache read {rates[2]*1e6:.2f}, cache write {rates[3]*1e6:.2f}; worst error {worst*100:.1f}%")
    grand = 0.0
    for wt in args.worktree:
        d = transcript_dir(wt)
        files = sorted(d.glob("*.jsonl")) if d.is_dir() else []
        print(f"\n{wt}: {len(files)} session(s)")
        for f in files:
            tokens, replies = session_usage(f)
            cost = sum(r * t for r, t in zip(rates, tokens))
            grand += cost
            print(f"  {f.name[:8]}  {replies:4d} replies  ~${cost:6.2f}")
    print(f"\nestimated total: ~${grand:.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
