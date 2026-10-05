"""task_create với parent_id (subtask), task_getSubtree, phụ thuộc giữa các task, tính lại tiến độ."""
from __future__ import annotations

from task_test_bed import TaskBed, is_rejected, new_uuid, task_id

from mcp_check_framework import Context, run_single

SUITE = "task-create-hierarchy"


def run(ctx: Context) -> None:
    bed = TaskBed.open(ctx)
    if bed is None:
        return

    parent = task_id(bed.create(bed.title("parent")))
    if not parent:
        ctx.check("tạo task cha", False, "không tạo được task cha")
        return

    child_reply = bed.create(bed.title("child"), parent_id=parent)
    child = task_id(child_reply)
    ctx.check("tạo subtask với parent_id", child is not None, child_reply.tool_text or child_reply.text[:200])
    if child:
        ctx.check("subtask có parentId đúng", child_reply.structured.get("parentId") == parent,
                  f"parentId={child_reply.structured.get('parentId')!r}")
        ctx.check("subtask cùng project với cha", child_reply.structured.get("projectId") == bed.project_id,
                  f"projectId={child_reply.structured.get('projectId')!r}")

        grand_reply = bed.create(bed.title("grand"), parent_id=child)
        grand = task_id(grand_reply)
        ctx.check("tạo cấp thứ ba (cháu)", grand is not None, grand_reply.tool_text or grand_reply.text[:160])

        listed = bed.client.call_tool("task_list", {"project_id": bed.project_id, "page_size": 197}).structured or {}
        by_id = {t.get("id"): t for t in listed.get("tasks") or []}
        ctx.check("task_list thể hiện quan hệ cha-con (parentId)",
                  by_id.get(child, {}).get("parentId") == parent and (grand is None or by_id.get(grand, {}).get("parentId") == child),
                  "parentId không khớp trong task_list")

        # task.getSubtree trả {} dù cây có con (cả kênh /ws lẫn MCP) — lỗi đã biết ở gateway/task-service.
        sub = bed.client.call_tool("task_getSubtree", {"root_id": parent})
        text = sub.tool_text
        ctx.known_defect("task_getSubtree trả về cả con và cháu", sub.status == 200 and child in text and (grand is None or grand in text),
                         f"HTTP {sub.status} kết quả={text[:80]!r}")

        done = bed.client.call_tool("task_recalculateProgress", {"root_id": parent})
        ctx.check("task_recalculateProgress không lỗi", done.status < 500 and done.error is None and not done.tool_is_error,
                  done.tool_text or done.text[:160])
        total = bed.get(parent).get("totalSubtasks", 0)
        ctx.known_defect("cha ghi nhận số subtask (totalSubtasks) sau khi tạo con và tính lại tiến độ", total >= 1,
                         f"totalSubtasks={total!r}")

    ghost = bed.create(bed.title("orphan"), parent_id=new_uuid())
    ctx.check("parent_id không tồn tại bị từ chối", task_id(ghost) is None,
              f"đã tạo task con của cha ma: parentId={(ghost.structured or {}).get('parentId')!r}")

    bad = bed.create(bed.title("badparent"), parent_id="khong-phai-uuid")
    ctx.check("parent_id sai định dạng bị từ chối", task_id(bad) is None, f"đã tạo: {bad.structured!r}")

    # --- phụ thuộc (depends_on) ------------------------------------------------------------
    a, b = task_id(bed.create(bed.title("dep-a"))), task_id(bed.create(bed.title("dep-b")))
    if a and b:
        edge = bed.client.call_tool("task_addEdge", {"from_task_id": a, "to_task_id": b, "type": "depends_on"})
        ctx.check("task_addEdge depends_on thành công", edge.status == 200 and not edge.tool_is_error and edge.error is None,
                  edge.tool_text or edge.text[:200])
        deps = bed.client.call_tool("task_getDependencies", {"task_id": a})
        ctx.check("task_getDependencies thấy phụ thuộc vừa thêm", b in deps.tool_text, deps.tool_text[:200])
        cycle = bed.client.call_tool("task_addEdge", {"from_task_id": b, "to_task_id": a, "type": "depends_on"})
        ctx.check("cạnh phụ thuộc vòng (a↔b) bị từ chối", is_rejected(cycle), f"HTTP {cycle.status} {cycle.text[:160]}")
        selfdep = bed.client.call_tool("task_addEdge", {"from_task_id": a, "to_task_id": a, "type": "depends_on"})
        ctx.check("task phụ thuộc chính nó bị từ chối", is_rejected(selfdep), f"HTTP {selfdep.status} {selfdep.text[:160]}")
        badtype = bed.client.call_tool("task_addEdge", {"from_task_id": a, "to_task_id": b, "type": "khong_hop_le"})
        ctx.check("loại cạnh không hợp lệ bị từ chối", is_rejected(badtype), f"HTTP {badtype.status} {badtype.text[:160]}")

    bed.purge()


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
