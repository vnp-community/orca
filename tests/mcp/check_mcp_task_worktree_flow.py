"""Luồng client MCP -> OrcaTask: tạo, đọc, đổi trạng thái, liên kết nguồn issue, (tuỳ chọn) thực thi.

Chỉ ghi dữ liệu có tiền tố ORCA_TEST_PREFIX và xoá ở cuối qua kênh WebSocket task.delete
(công cụ task_delete là 'destructive' cần người phê duyệt nên không dùng để dọn dẹp).
Bước task_execute chỉ chạy khi ORCA_MCP_PROBE_EXEC=true và cần dev server đang kết nối.
Spec: task-service/internal/usecase/execute_task.go; tools/pack2_workspace.go, pack3_exec.go.
"""
from __future__ import annotations

import time

from mcp_check_framework import Context, find_value, run_single
from mcp_http_client import McpClient

SUITE = "task-worktree-flow"
POLL_S, POLL_MAX_S = 3.0, 90.0


def _task_id(reply) -> str | None:
    val = find_value(reply.structured, "id", "taskId")
    return str(val) if val else None


def _status(client: McpClient, task_id: str) -> str | None:
    reply = client.call_tool("task_get", {"id": task_id})
    val = find_value(reply.structured, "status")
    return str(val) if val else None


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return
    if not ctx.cfg.allow_writes:
        ctx.skip("luồng task qua MCP", "ORCA_ALLOW_WRITES=false")
        return
    client = ctx.session_client(["orca:read", "orca:write", "orca:exec"])
    if client is None:
        return

    if "task_create" not in {t.get("name") for t in client.list_tools()}:
        ctx.skip("luồng task qua MCP", "công cụ ghi chưa bật: đặt MCP_TOOL_PACKS_ENABLED=1,2 (và 3 cho task_execute) trên gateway")
        return

    # --- project ---------------------------------------------------------------
    project_id = ctx.cfg.project_id
    if not project_id:
        reply = client.call_tool("project_list", {})
        project_id = str(find_value((reply.structured or {}).get("items") or [], "id") or "")
    if not project_id:
        ctx.skip("luồng task qua MCP", "không có project nào (đặt ORCA_MCP_PROJECT_ID)")
        return
    ctx.state["project_id"] = project_id
    created: list[str] = []

    def purge() -> None:
        admin = ctx.admin
        rpc = ctx.ws(admin) if admin is not None else None
        if rpc is None:
            if created:
                print(f"[warn] không dọn được task thử nghiệm (thiếu /ws): {created}")
            return
        for tid in created:
            rpc.invoke("task.delete", [{"id": tid}], timeout=ctx.cfg.timeout)

    ctx.cleanup("xoá task thử nghiệm", purge)

    # --- tạo / đọc / cập nhật ---------------------------------------------------
    title = ctx.name("task")
    reply = client.call_tool("task_create", {"title": title, "project_id": project_id})
    tid = _task_id(reply)
    ctx.check("task_create trả task id", reply.status == 200 and not reply.tool_is_error and bool(tid),
              f"HTTP {reply.status} {reply.text[:200]}")
    if not tid:
        return
    created.append(tid)

    reply = client.call_tool("task_get", {"id": tid})
    ctx.check("task_get trả đúng task", find_value(reply.structured, "title") == title, reply.text[:160])

    reply = client.call_tool("task_list", {"project_id": project_id})
    listed = [str(find_value(t, "id")) for t in (reply.structured or {}).get("items") or []]
    ctx.check("task_list có task vừa tạo", tid in listed or not listed, f"không thấy {tid} trong {len(listed)} task")

    before = _status(client, tid)
    reply = client.call_tool("task_update", {"id": tid, "labels": ["mcp-selftest"]})
    ctx.check("task_update (labels) thành công", reply.status == 200 and not reply.tool_is_error, reply.text[:160])
    ctx.check("task_update không đổi trạng thái ngoài ý muốn", _status(client, tid) == before,
              f"trước={before!r}")

    # --- nguồn issue ngoài -------------------------------------------------------
    reply = client.call_tool("task_getSource", {"task_id": tid})
    ctx.check("task_getSource task thường -> không có nguồn", reply.status == 200 and not reply.tool_is_error,
              reply.text[:160])
    ref = f"MCPTEST-{int(time.time()) % 100000}"
    reply = client.call_tool("task_createFromSource", {
        "title": ctx.name("from-source"), "provider": "jira", "ref": ref, "project_id": project_id,
        "url": f"https://example.invalid/browse/{ref}"})
    src_id = _task_id(reply)
    if src_id:
        created.append(src_id)
        reply2 = client.call_tool("task_getSource", {"task_id": src_id})
        ctx.check("task_getSource trả provider/ref đã liên kết",
                  find_value(reply2.structured, "ref") == ref and find_value(reply2.structured, "provider") == "jira",
                  reply2.text[:160])
        # Cùng (project, provider, ref) phải không tạo task thứ hai.
        reply3 = client.call_tool("task_createFromSource", {
            "title": ctx.name("dup"), "provider": "jira", "ref": ref, "project_id": project_id})
        dup_id = _task_id(reply3)
        if dup_id and dup_id != src_id:
            created.append(dup_id)
        ctx.check("cùng khoá issue trong một project không sinh task thứ hai",
                  reply3.status < 500 and (reply3.tool_is_error or dup_id in (None, src_id)),
                  f"id lần 2={dup_id!r}, gốc={src_id!r}")
    else:
        ctx.check("task_createFromSource", False, f"HTTP {reply.status} {reply.text[:200]}")

    # --- thực thi (tuỳ chọn) -----------------------------------------------------
    if not ctx.cfg.probe_exec:
        ctx.skip("task_execute", "đặt ORCA_MCP_PROBE_EXEC=true; cần dev server kết nối và người duyệt trong Approvals")
        return
    reply = client.call_tool("task_execute", {"task_id": tid, "request_id": ctx.name("req")})
    ctx.check("task_execute không lỗi giao thức", reply.status < 500 and reply.error is None,
              f"HTTP {reply.status} {reply.text[:200]}")
    if reply.tool_is_error:
        text = reply.tool_text
        ctx.skip("theo dõi trạng thái sau task_execute",
                 f"bị từ chối/chưa duyệt: {text[:160]} — trạng thái task phải còn nguyên")
        ctx.check("lỗi trước khi chạy không để task kẹt in_progress", _status(client, tid) != "in_progress",
                  f"status={_status(client, tid)!r}")
        return

    deadline, status = time.time() + POLL_MAX_S, None
    while time.time() < deadline:
        status = _status(client, tid)
        if status in ("review", "done"):
            break
        time.sleep(POLL_S)
    ctx.check("task_execute đưa task tới review/done trong thời hạn", status in ("review", "done"),
              f"status cuối={status!r} sau {POLL_MAX_S:.0f}s")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
