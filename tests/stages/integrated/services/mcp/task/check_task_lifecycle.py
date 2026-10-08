"""Vòng đời task qua MCP: tạo -> tồn tại -> chưa thực thi -> (tuỳ chọn) thực thi -> đã thực thi.

Phần thực thi chỉ chạy khi ORCA_MCP_TASK_EXECUTE=true và gateway bật pack 3 (MCP_TOOL_PACKS_ENABLED=1,2,3);
nếu không thì SKIP có lý do. Mọi kiểm tra đều qua công cụ MCP; cấu hình project đọc từ .env (task_settings.py).
"""
from __future__ import annotations

import uuid

from task_execution_probe import RUNNING, execute_tool_available, find_task, inspect_task
from task_settings import load_task_settings
from task_test_bed import TaskBed, task_id

from mcp_check_framework import Context, run_single
from mcp_list_projects import fetch_projects  # noqa: F401  (đảm bảo bộ giãn nhịp được nạp)

SUITE = "task-lifecycle"


def run(ctx: Context) -> None:
    st = load_task_settings()
    scopes = ["orca:read", "orca:write"] + (["orca:exec"] if st.execute else [])
    bed = TaskBed.open(ctx, scopes=scopes, auto_purge=not st.keep)
    if bed is None:
        return
    print(f"[info] project đích: {bed.project_name} ({bed.project_id}); thực thi={'bật' if st.execute else 'tắt'}")

    title = bed.title("life")
    reply = bed.create(title)
    tid = task_id(reply)
    ctx.check("tạo task qua MCP", tid is not None, reply.tool_text or reply.text[:200])
    if not tid:
        return

    # --- tồn tại ---------------------------------------------------------------------------------
    rep = inspect_task(bed.client, bed.project_id, tid, variation=1)
    ctx.check("task_get xác nhận task tồn tại", rep.exists and rep.task.get("title") == title, str(rep.task)[:160])
    ctx.check("task có trong task_list của project", rep.in_list, "không thấy trong task_list")
    by_number = find_task(bed.client, bed.project_id, number=rep.task.get("taskNumber"), variation=2)
    ctx.check("tìm được task theo taskNumber", bool(by_number) and by_number.get("id") == tid, f"taskNumber={rep.task.get('taskNumber')!r}")
    ghost = inspect_task(bed.client, bed.project_id, str(uuid.uuid4()), variation=3, with_project_signals=False)
    ctx.check("task không tồn tại được báo là không tồn tại", not ghost.exists, f"verdict={ghost.verdict}")

    # --- chưa thực thi -----------------------------------------------------------------------------
    ctx.check("task mới ở trạng thái open", rep.task.get("status") == "open", f"status={rep.task.get('status')!r}")
    ctx.check("task mới chưa có actualHours", not rep.task.get("actualHours"), f"actualHours={rep.task.get('actualHours')!r}")
    ctx.check("bộ kiểm tra kết luận 'chưa thực thi' cho task mới", not rep.executed and not rep.running, rep.verdict)
    ctx.check("project_active/agent_sessions đọc được qua MCP", rep.project_active is not None and rep.agent_sessions is not None,
              f"hasActiveExecutions={rep.project_active!r} agentSessions={rep.agent_sessions!r}")

    # đặt tay review không được coi là đã thực thi (chỉ ExecuteTask ghi actualHours)
    bed.client.call_tool("task_update", {"id": tid, "status": "review"})
    manual = inspect_task(bed.client, bed.project_id, tid, variation=4, with_project_signals=False)
    ctx.check("đặt tay status=review không bị tính là 'đã thực thi'", not manual.executed, manual.verdict)
    bed.client.call_tool("task_update", {"id": tid, "status": "open"})

    # --- thực thi (tuỳ chọn) ------------------------------------------------------------------------
    if not st.execute:
        ctx.skip("thực thi task qua MCP", "đặt ORCA_MCP_TASK_EXECUTE=true trong .env (cần pack 3 và dev server kết nối)")
        return
    if not execute_tool_available(bed.client):
        ctx.skip("thực thi task qua MCP", "gateway chưa công bố task_execute (MCP_TOOL_PACKS_ENABLED=1,2,3)")
        return
    args = {"task_id": tid, "request_id": str(uuid.uuid4())}
    if st.prompt:
        args["prompt"] = st.prompt
    ex = bed.client.call_tool("task_execute", args)
    if ex.tool_is_error or ex.error is not None:
        ctx.skip("thực thi task qua MCP", f"task_execute bị từ chối: {ex.tool_text[:160] or ex.text[:160]} "
                                           "(cần dev server kết nối và phê duyệt trong Settings → MCP → Approvals)")
        after = inspect_task(bed.client, bed.project_id, tid, variation=5, with_project_signals=False)
        ctx.check("lỗi trước khi chạy không làm task kẹt in_progress", after.task.get("status") != RUNNING,
                  f"status={after.task.get('status')!r}")
        return
    ctx.check("task_execute được Orca nhận", True)
    from mcp_task_lifecycle import wait_for_result
    final = wait_for_result(bed.client, bed.project_id, tid, st, "open")
    for line in final.lines():
        print("   ", line)
    ctx.check("task đã thực thi xong ở Orca (actualHours > 0, trạng thái review/done)",
              final.executed and not final.running and final.task.get("status") in ("review", "done"), final.verdict)


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
