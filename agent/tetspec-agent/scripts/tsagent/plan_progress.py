"""Count the checklists in TASKS.md and write the progress table into PLAN.md (between the progress:start / progress:end markers).

Run: python scripts/tsagent/plan_progress.py [--check]
--check: print only, no write; exit code 1 when the table in PLAN.md is stale. The table text is Vietnamese like PLAN.md.
"""
import re
import sys
from pathlib import Path

DIR = Path(__file__).resolve().parents[2] / "docs" / "tsagent" / "integrate"
TASKS, PLAN = DIR / "TASKS.md", DIR / "PLAN.md"
START, END = "<!-- progress:start -->", "<!-- progress:end -->"


def parse(text):
    phases, tasks, cur = [], [], None
    for line in text.splitlines():
        if m := re.match(r"^## (P\d+)\. (.+)", line):
            phases.append((m[1], m[2]))
        elif m := re.match(r"^### (T\d+\.\d+) (.+)", line):
            cur = {"id": m[1], "title": m[2], "phase": phases[-1][0], "done": 0, "total": 0, "blocked": None}
            tasks.append(cur)
        elif cur and (m := re.match(r"^- \[([ xX])\] ", line)):
            cur["total"] += 1
            cur["done"] += m[1] != " "
        elif cur and (m := re.match(r"^> BLOCKED: (.+)", line)):
            cur["blocked"] = m[1].strip()
    return phases, tasks


def status(t):
    if t["blocked"]:
        return "🔴 blocked"
    if t["total"] and t["done"] == t["total"]:
        return "✅ done"
    return "🟡 doing" if t["done"] else "⚪ todo"


def render(phases, tasks):
    done = sum(t["done"] for t in tasks)
    total = sum(t["total"] for t in tasks) or 1
    finished = sum(1 for t in tasks if t["total"] and t["done"] == t["total"])
    out = [f"**Tổng: {finished}/{len(tasks)} task xong · {done}/{total} mục ({100 * done // total} %)**", "",
           "| Pha | Task | Việc | Mục | Trạng thái |", "| --- | --- | --- | --- | --- |"]
    names = dict(phases)
    for t in tasks:
        note = f" ({t['blocked']})" if t["blocked"] else ""
        out.append(f"| {t['phase']} {names[t['phase']]} | {t['id']} | {t['title']} | {t['done']}/{t['total']} | {status(t)}{note} |")
    return "\n".join(out)


def main(argv):
    table = render(*parse(TASKS.read_text(encoding="utf-8")))
    plan = PLAN.read_text(encoding="utf-8")
    i, j = plan.index(START) + len(START), plan.index(END)
    new = plan[:i] + "\n" + table + "\n" + plan[j:]
    print(table)
    if "--check" in argv:
        return 0 if new == plan else 1
    PLAN.write_text(new, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
