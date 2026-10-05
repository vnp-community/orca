"""Sau khi tạo: task_update (title, labels, status), task_addComment/task_listComments."""
from __future__ import annotations

from task_test_bed import TaskBed, is_rejected, task_id

from mcp_check_framework import Context, run_single

SUITE = "task-create-update"


def run(ctx: Context) -> None:
    bed = TaskBed.open(ctx)
    if bed is None:
        return
    tid = task_id(bed.create(bed.title("upd")))
    if not tid:
        ctx.check("tạo task để cập nhật", False, "không tạo được task")
        return

    r = bed.client.call_tool("task_update", {"id": tid, "labels": ["mcp-selftest", "bug"]})
    ctx.check("task_update labels thành công", r.status == 200 and not r.tool_is_error and r.error is None, r.tool_text or r.text[:200])
    cur = bed.get(tid)
    ctx.check("labels được lưu", sorted(cur.get("labels") or []) == ["bug", "mcp-selftest"], str(cur.get("labels")))

    r = bed.client.call_tool("task_update", {"id": tid, "labels": []})
    ctx.check("labels = [] xoá toàn bộ nhãn", (r.structured or {}).get("labels") in ([], None), str((r.structured or {}).get("labels")))

    new_title = bed.title("renamed")
    r = bed.client.call_tool("task_update", {"id": tid, "title": new_title})
    ctx.check("đổi title", (r.structured or {}).get("title") == new_title, r.tool_text or str((r.structured or {}).get("title")))

    # Máy trạng thái (domain/task.go): in_progress chỉ do ExecuteTask đặt; done/cancelled là trạng thái cuối.
    cur = bed.get(tid)
    ctx.check("trạng thái ban đầu là open", cur.get("status") == "open", str(cur.get("status")))
    for target in ("blocked", "open", "review"):
        r = bed.client.call_tool("task_update", {"id": tid, "status": target})
        got = (r.structured or {}).get("status") if not r.tool_is_error else None
        ctx.check(f"đổi status sang {target}", got == target, r.tool_text or str(got))

    r = bed.client.call_tool("task_update", {"id": tid, "status": "in_progress"})
    ctx.check("status in_progress không đặt được qua task_update (chỉ ExecuteTask)", r.tool_is_error and "TASK_INVALID_STATUS_TRANSITION" in r.tool_text,
              f"isError={r.tool_is_error} {r.tool_text[:120]}")
    bad = bed.client.call_tool("task_update", {"id": tid, "status": "trang_thai_khong_co"})
    ctx.check("status không hợp lệ bị từ chối", bad.tool_is_error and bad.status < 500, f"{bad.tool_text[:120] or bad.text[:120]}")
    cur = bed.get(tid)
    ctx.check("status không đổi sau các lần bị từ chối", cur.get("status") == "review", str(cur.get("status")))

    done = bed.client.call_tool("task_update", {"id": tid, "status": "done"})
    ctx.check("đổi status sang done", (done.structured or {}).get("status") == "done", done.tool_text or done.text[:120])
    reopen = bed.client.call_tool("task_update", {"id": tid, "status": "open"})
    ctx.check("done là trạng thái cuối: không mở lại được", reopen.tool_is_error and "TASK_INVALID_STATUS_TRANSITION" in reopen.tool_text,
              f"isError={reopen.tool_is_error} {reopen.tool_text[:120]}")

    missing = bed.client.call_tool("task_update", {"labels": ["x"]})
    ctx.check("task_update thiếu id bị từ chối", is_rejected(missing), missing.text[:160])

    # --- bình luận ----------------------------------------------------------------------------
    content = f"{ctx.cfg.prefix} bình luận thử nghiệm"
    c = bed.client.call_tool("task_addComment", {"task_id": tid, "content": content})
    ctx.check("task_addComment thành công", c.status == 200 and not c.tool_is_error and c.error is None, c.tool_text or c.text[:200])
    lst = bed.client.call_tool("task_listComments", {"task_id": tid})
    ctx.check("task_listComments có bình luận vừa thêm", content in lst.tool_text, lst.tool_text[:200])
    long = bed.client.call_tool("task_addComment", {"task_id": tid, "content": "x" * 8001})
    ctx.check("bình luận > 8000 ký tự bị từ chối", is_rejected(long), f"HTTP {long.status} {long.text[:120]}")
    empty = bed.client.call_tool("task_addComment", {"task_id": tid, "content": ""})
    ctx.check("bình luận rỗng không gây 5xx", empty.status < 500, f"HTTP {empty.status}")

    bed.purge()


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
