"""Tạo task song song (không trùng id/số thứ tự) và kiểm soát quyền: token chỉ-đọc không tạo được task."""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor

from task_test_bed import TaskBed, is_rejected, task_id

from mcp_check_framework import Context, run_single

SUITE = "task-create-concurrency-access"
PARALLEL = 8


def run(ctx: Context) -> None:
    bed = TaskBed.open(ctx)
    if bed is None:
        return

    # --- song song -------------------------------------------------------------------------------
    titles = [bed.title(f"par{i}") for i in range(PARALLEL)]
    with ThreadPoolExecutor(max_workers=PARALLEL) as pool:
        replies = list(pool.map(lambda t: bed.client.call_tool("task_create", {"title": t, "project_id": bed.project_id}), titles))
    for r in replies:
        bed.track(r)
    ids = [task_id(r) for r in replies]
    ctx.check(f"{PARALLEL} lời gọi task_create song song đều thành công", all(ids), f"thất bại: {[r.text[:80] for r, i in zip(replies, ids) if not i]}")
    ok = [i for i in ids if i]
    ctx.check("id các task song song không trùng nhau", len(set(ok)) == len(ok), f"{len(ok)} id, {len(set(ok))} khác nhau")
    nums = [(r.structured or {}).get("taskNumber") for r in replies if task_id(r)]
    ctx.check("taskNumber không trùng nhau khi tạo song song", len(set(nums)) == len(nums), f"taskNumber={sorted(n for n in nums if n)}")
    listed = set(bed.listed_ids())
    ctx.check("mọi task song song đều có trong task_list", set(ok) <= listed, f"thiếu: {sorted(set(ok) - listed)}")

    # --- quyền -----------------------------------------------------------------------------------------
    ro = ctx.client(ctx.token_for(["orca:read"]))
    ro.open_session()
    if ro.session_id:
        ctx.cleanup("đóng session chỉ-đọc", ro.delete_session)
    else:
        ctx.skip("token chỉ-đọc", "không mở được session")
        bed.purge()
        return
    ctx.check("token chỉ-đọc không thấy task_create", "task_create" not in {t.get("name") for t in ro.list_tools()}, "task_create lộ ra cho token orca:read")
    r = bed.track(ro.call_tool("task_create", {"title": bed.title("ro"), "project_id": bed.project_id}))
    ctx.check("token chỉ-đọc gọi task_create bị từ chối", is_rejected(r) and task_id(r) is None,
              f"HTTP {r.status}: đã tạo {r.structured!r}")

    anon = ctx.client(None)
    resp = anon.raw("POST", headers={"Content-Type": "application/json", "Accept": "application/json, text/event-stream"},
                    data='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"task_create","arguments":{"title":"x"}}}')
    ctx.check("không token không tạo được task (401)", resp.status_code == 401, f"HTTP {resp.status_code}")

    bed.purge()


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
