"""task_create: tham số title/project_id, giá trị biên, ký tự đặc biệt, đọc lại và có trong task_list.

Hợp đồng (tools/pack2_workspace.go): title bắt buộc, tối đa 500 ký tự; project_id và parent_id tuỳ chọn.
"""
from __future__ import annotations

import re

from task_test_bed import TaskBed, is_rejected, new_uuid, task_id

from mcp_check_framework import Context, run_single

SUITE = "task-create-fields"
UUID_RE = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)


def run(ctx: Context) -> None:
    bed = TaskBed.open(ctx)
    if bed is None:
        return
    print(f"[info] project đích: {bed.project_name} ({bed.project_id})")

    # --- tạo tối thiểu ----------------------------------------------------------
    title = bed.title("min")
    r = bed.create(title)
    tid = task_id(r)
    ctx.check("task_create (title + project_id) thành công", tid is not None, f"HTTP {r.status} {r.tool_text or r.text[:200]}")
    if not tid:
        return
    t = r.structured
    ctx.check("id là uuid", bool(UUID_RE.match(tid)), tid)
    ctx.check("title khớp", t.get("title") == title, f"title={t.get('title')!r}")
    ctx.check("projectId khớp project đích", t.get("projectId") == bed.project_id, f"projectId={t.get('projectId')!r}")
    ctx.check("trạng thái ban đầu là open", t.get("status") == "open", f"status={t.get('status')!r}")
    ctx.check("taskNumber là số nguyên dương", isinstance(t.get("taskNumber"), int) and t["taskNumber"] > 0,
              f"taskNumber={t.get('taskNumber')!r}")
    ctx.check("chưa có task cha", not t.get("parentId"), f"parentId={t.get('parentId')!r}")

    back = bed.get(tid)
    ctx.check("task_get đọc lại đúng task vừa tạo", back.get("id") == tid and back.get("title") == title, str(back)[:200])
    ctx.check("task_list của project có task vừa tạo", tid in bed.listed_ids(), "không thấy trong task_list")

    # --- số thứ tự tăng dần -------------------------------------------------------
    r2 = bed.create(bed.title("min2"))
    n1, n2 = t.get("taskNumber"), (r2.structured or {}).get("taskNumber")
    ctx.check("taskNumber tăng dần giữa hai task liên tiếp", isinstance(n2, int) and n2 > n1, f"{n1} -> {n2}")

    # --- giá trị biên của title -----------------------------------------------------
    one = bed.create("x")
    ctx.check("title 1 ký tự được chấp nhận", task_id(one) is not None, one.tool_text or one.text[:160])

    edge = bed.create("T" * 500)
    ctx.check("title 500 ký tự được chấp nhận", task_id(edge) is not None, edge.tool_text or edge.text[:160])

    over = bed.create("T" * 501)
    ctx.check("title 501 ký tự bị từ chối", is_rejected(over) and task_id(over) is None, f"HTTP {over.status} {over.text[:160]}")

    empty = bed.create("")
    ctx.check("title rỗng bị từ chối", is_rejected(empty) and task_id(empty) is None, f"HTTP {empty.status} {empty.text[:160]}")

    blank = bed.create("   ")
    # domain.NewTask chỉ chặn title == "" (không trim) nên task có title toàn khoảng trắng vẫn được tạo.
    ctx.known_defect("title chỉ có khoảng trắng bị từ chối", task_id(blank) is None,
                     f"server tạo task title={(blank.structured or {}).get('title')!r}")

    missing = bed.client.call_tool("task_create", {"project_id": bed.project_id})
    ctx.check("thiếu title bị từ chối", is_rejected(missing) and task_id(missing) is None, f"HTTP {missing.status} {missing.text[:160]}")

    wrong_type = bed.client.call_tool("task_create", {"title": 12345, "project_id": bed.project_id})
    ctx.check("title sai kiểu (số) bị từ chối", is_rejected(wrong_type) and task_id(wrong_type) is None, wrong_type.text[:160])

    extra = bed.client.call_tool("task_create", {"title": bed.title("extra"), "project_id": bed.project_id, "priority": "high"})
    bed.track(extra)
    ctx.check("tham số ngoài hợp đồng (priority) bị từ chối, không bị bỏ qua im lặng",
              is_rejected(extra) and task_id(extra) is None,
              f"tạo được task, priority server trả: {(extra.structured or {}).get('priority')!r}")

    # --- ký tự đặc biệt ---------------------------------------------------------------
    for label, text in (
        ("tiếng Việt có dấu", "Sửa lỗi đăng nhập — người dùng Việt Nam"),
        ("emoji", "Triển khai 🚀 bản vá"),
        ("ký tự HTML/SQL", "<script>alert(1)</script>'; DROP TABLE tasks;--"),
        ("xuống dòng", "dòng một\ndòng hai"),
    ):
        want = f"{bed.title('chr')} {text}"
        r = bed.create(want)
        got = (r.structured or {}).get("title") if task_id(r) else None
        # Chấp nhận chuẩn hoá khoảng trắng đầu/cuối nhưng nội dung phải nguyên vẹn.
        ctx.check(f"title {label} lưu nguyên vẹn", got is not None and got.strip() == want.strip(),
                  f"gửi {want!r} nhận {got!r} ({r.tool_text[:100]})")

    # --- project_id ---------------------------------------------------------------------
    bad_fmt = bed.create(bed.title("badfmt"), project="khong-phai-uuid")
    ctx.check("project_id sai định dạng bị từ chối", task_id(bad_fmt) is None, f"đã tạo: {bad_fmt.structured!r}")

    ghost = bed.create(bed.title("ghost"), project=new_uuid())
    # Theo thiết kế hiện tại task-service không kiểm tra project tồn tại (domain/task.go: "never validates").
    ctx.known_defect("project_id không tồn tại bị từ chối (không tạo task mồ côi)", task_id(ghost) is None,
                     f"đã tạo task trỏ tới project ma: projectId={(ghost.structured or {}).get('projectId')!r}")

    nop = bed.create(bed.title("noproject"), project=None)
    if task_id(nop):
        ctx.check("không có project_id: task không gắn project", not (nop.structured or {}).get("projectId"),
                  f"projectId={(nop.structured or {}).get('projectId')!r}")
        ctx.check("task không gắn project không lẫn vào danh sách của project đích", task_id(nop) not in bed.listed_ids(),
                  "xuất hiện trong task_list của project đích")
    else:
        ctx.check("không có project_id bị từ chối có kiểm soát", is_rejected(nop), f"HTTP {nop.status} {nop.text[:160]}")

    bed.purge()


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
