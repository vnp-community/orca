#!/usr/bin/env python3
"""Collects the numbers of a pilot Request from the local stack's databases (read only) for the T4.7 report.

  python3 pilot_metrics.py --request <id> --plan-task <id> [--out metrics.md] [--json metrics.json]

It reads request.analysis_runs (the solution and plan runs with the agent's own turn, tool and cost figures) and
task.task_execution_records (every attempt of every task: parse status, failure class and when it was recorded), and groups
the tasks under their phases. Only the dev stack's `orca-go-postgres` container is read; nothing is changed. Per-task cost is not
in the data (a contract run reports no usage yet): the report says so instead of estimating it.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from collections import defaultdict


def psql(db: str, sql: str) -> list[list[str]]:
    done = subprocess.run(["docker", "exec", "-i", "orca-go-postgres", "psql", "-U", "orca", "-d", db, "-tA", "-F", "\x1f"],
                          input=sql, capture_output=True, text=True, timeout=60)
    if done.returncode != 0:
        raise RuntimeError(done.stderr.strip()[:300])
    return [line.split("\x1f") for line in done.stdout.splitlines() if line]


def runs(request_id: str) -> list[dict]:
    rows = psql("request", f"""select id, kind, mode, engine, status, coalesce(error_code,''), started_at, finished_at,
        coalesce(run_stats::text,'{{}}'), extract(epoch from (finished_at - started_at))
        from request.analysis_runs where request_id = '{request_id}' order by started_at""")
    out = []
    for r in rows:
        stats = json.loads(r[8]) if r[8] else {}
        out.append({"id": r[0], "kind": r[1], "mode": r[2], "engine": r[3], "status": r[4], "error": r[5], "started": r[6], "finished": r[7],
                    "seconds": float(r[9]) if r[9] else None, "cost": (stats.get("usage") or {}).get("cost_usd"), "turns": (stats.get("usage") or {}).get("turns"),
                    "knowledge_calls": sum(n for k, n in (stats.get("tool_calls") or {}).items() if k.startswith("mcp__tdd-agent__")),
                    "tool_calls": sum((stats.get("tool_calls") or {}).values())})
    return out


def tasks(plan_task: str) -> list[dict]:
    phases = {r[0]: r[1] for r in psql("task", f"select id, title from task.tasks where parent_id = '{plan_task}'")}
    ids = ",".join(f"'{p}'" for p in phases) or "''"
    leaves = psql("task", f"select id, title, status, parent_id from task.tasks where parent_id in ({ids})")
    recs = defaultdict(list)
    for r in psql("task", f"select task_id, attempt, parse_status, failure_class, created_at from task.task_execution_records "
                          f"where task_id in (select id from task.tasks where parent_id in ({ids})) order by created_at"):
        recs[r[0]].append({"attempt": int(r[1]), "parse": r[2], "failure": r[3], "at": r[4]})
    return [{"id": l[0], "title": l[1], "status": l[2], "phase": phases.get(l[3], l[3]), "attempts": recs.get(l[0], [])} for l in leaves]


def render(request_id: str, rs: list[dict], ts: list[dict]) -> str:
    lines = [f"# Pilot metrics for Request {request_id}", "", "## Analysis and plan runs", "",
             "| run | kind | engine | status | seconds | turns | tool calls | KG calls | cost USD |", "|---|---|---|---|---|---|---|---|---|"]
    total = 0.0
    for r in rs:
        total += r["cost"] or 0
        lines.append(f"| {r['id'][:8]} | {r['kind']} | {r['engine']} | {r['status']}{(' ' + r['error']) if r['error'] else ''} | {round(r['seconds'] or 0)} | "
                     f"{r['turns'] or ''} | {r['tool_calls'] or ''} | {r['knowledge_calls'] or ''} | {('%.2f' % r['cost']) if r['cost'] else ''} |")
    lines += ["", f"Total reported by the agent for these runs: **${total:.2f}**. Task runs report no cost yet.", "", "## Tasks", "",
              "| phase | task | status | attempts | failures |", "|---|---|---|---|---|"]
    by_phase = defaultdict(list)
    for t in ts:
        by_phase[t["phase"]].append(t)
    done = failed_attempts = 0
    for ph, items in by_phase.items():
        for t in items:
            fails = [a["failure"] for a in t["attempts"] if a["failure"] and a["failure"] != "none"]
            failed_attempts += len(fails)
            done += t["status"] in ("done", "review")
            lines.append(f"| {ph[:34]} | {t['title'][:60]} | {t['status']} | {len(t['attempts'])} | {', '.join(fails) or ''} |")
    lines += ["", f"{done} of {len(ts)} task(s) done or in review; {failed_attempts} failed attempt(s) recorded."]
    return "\n".join(lines)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--request", required=True)
    ap.add_argument("--plan-task", required=True)
    ap.add_argument("--out")
    ap.add_argument("--json")
    args = ap.parse_args()
    rs, ts = runs(args.request), tasks(args.plan_task)
    text = render(args.request, rs, ts)
    if args.json:
        with open(args.json, "w") as f:
            json.dump({"runs": rs, "tasks": ts}, f, indent=1)
    if args.out:
        with open(args.out, "w") as f:
            f.write(text)
        print(f"wrote {args.out}")
    else:
        print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
