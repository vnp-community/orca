"""task_createFromSource: tạo task từ issue ngoài (provider + khoá), liên kết duy nhất theo (project, provider, khoá).

Qua MCP chưa có tham số `site` (xem docs/guides/mcp/task-worktree-tools.md), nên các task ở đây có site rỗng.
"""
from __future__ import annotations

import time

from task_test_bed import TaskBed, is_rejected, task_id

from mcp_check_framework import Context, find_value, run_single

SUITE = "task-create-from-source"


def run(ctx: Context) -> None:
    bed = TaskBed.open(ctx)
    if bed is None:
        return
    ref = f"MCPTEST-{int(time.time()) % 1_000_000}"
    url = f"https://example.invalid/browse/{ref}"

    r = bed.track(bed.client.call_tool("task_createFromSource", {
        "title": bed.title("src"), "provider": "jira", "ref": ref, "url": url, "project_id": bed.project_id}))
    tid = task_id(r)
    ctx.check("task_createFromSource (jira) thành công", tid is not None, r.tool_text or r.text[:200])
    ctx.check("lần đầu created=true", (r.structured or {}).get("created") is True, str(r.structured)[:120])
    if not tid:
        return

    src = bed.client.call_tool("task_getSource", {"task_id": tid}).structured or {}
    ctx.check("task_getSource trả đúng provider", find_value(src, "provider") == "jira", str(src)[:160])
    ctx.check("task_getSource trả đúng khoá issue", find_value(src, "ref") == ref, str(src)[:160])
    ctx.check("task_getSource trả đúng url", find_value(src, "url") == url, str(src)[:160])

    before = len(bed.listed_ids())
    dup = bed.track(bed.client.call_tool("task_createFromSource", {
        "title": bed.title("dup"), "provider": "jira", "ref": ref, "project_id": bed.project_id}))
    dup_id = task_id(dup)
    ctx.check("cùng (project, provider, khoá) trả lại task cũ với created=false",
              dup_id == tid and (dup.structured or {}).get("created") is False,
              f"id lần hai={dup_id!r}, created={(dup.structured or {}).get('created')!r}")
    ctx.check("cùng khoá không làm tăng số task trong project", len(bed.listed_ids()) == before,
              f"số task {before} -> {len(bed.listed_ids())}")

    other = bed.track(bed.client.call_tool("task_createFromSource", {
        "title": bed.title("other"), "provider": "jira", "ref": ref + "-B", "project_id": bed.project_id}))
    ctx.check("khoá issue khác sinh task mới", task_id(other) not in (None, tid), other.tool_text or other.text[:160])

    for provider in ("github", "gitlab", "linear"):
        p = bed.track(bed.client.call_tool("task_createFromSource", {
            "title": bed.title(f"src-{provider}"), "provider": provider, "ref": f"{ref}-{provider}", "project_id": bed.project_id}))
        ctx.check(f"provider '{provider}' được chấp nhận", task_id(p) is not None, p.tool_text or p.text[:160])

    bad = bed.track(bed.client.call_tool("task_createFromSource", {
        "title": bed.title("badprov"), "provider": "khong_ho_tro", "ref": ref + "-X", "project_id": bed.project_id}))
    ctx.check("provider không hỗ trợ bị từ chối", task_id(bad) is None and is_rejected(bad), f"HTTP {bad.status} {bad.text[:160]}")

    noref = bed.track(bed.client.call_tool("task_createFromSource", {
        "title": bed.title("noref"), "provider": "jira", "ref": "", "project_id": bed.project_id}))
    ctx.check("khoá issue rỗng bị từ chối", task_id(noref) is None, f"đã tạo: {noref.structured!r}")

    missing = bed.track(bed.client.call_tool("task_createFromSource", {"title": bed.title("miss"), "provider": "jira"}))
    ctx.check("thiếu ref bị từ chối", is_rejected(missing) and task_id(missing) is None, missing.text[:160])

    # Cùng khoá ở project khác phải được phép (liên kết duy nhất theo project) — không tạo ở project khác để khỏi
    # đụng dữ liệu thật; chỉ kiểm tra tại project đích.
    bed.purge()


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
