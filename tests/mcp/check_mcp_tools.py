"""Danh mục công cụ MCP: lọc theo scope, hình dạng kết quả, quét mọi tool chỉ-đọc không cần tham số.

Spec: mcpserver/tools/{catalog,spec,pack1_*,pack2_*,pack3_exec}.go. Mức rủi ro -> scope:
read=orca:read, write=orca:write, exec=orca:exec, destructive/admin=orca:admin.
"""
from __future__ import annotations

import uuid

from mcp_check_framework import Context, run_single

SUITE = "tools"
# Công cụ nguy hiểm: token chỉ-đọc không được thấy chúng.
EXEC_OR_DESTRUCTIVE = ("task_execute", "worktree_rm", "project_delete", "task_delete")
WRITE_TOOLS = ("task_create", "task_update", "worktree_create")


def _names(tools: list[dict]) -> set[str]:
    return {str(t.get("name")) for t in tools}


def _required(tool: dict) -> list[str]:
    return list((tool.get("inputSchema") or {}).get("required") or [])


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return

    read_client = ctx.session_client(["orca:read"])
    if read_client is None:
        return
    read_tools = read_client.list_tools()
    names = _names(read_tools)
    ctx.check("token orca:read thấy công cụ đọc (project_list, task_list)",
              {"project_list", "task_list"} <= names, f"thiếu: {sorted({'project_list', 'task_list'} - names)}")
    leaked = [n for n in EXEC_OR_DESTRUCTIVE + WRITE_TOOLS if n in names]
    ctx.check("token orca:read KHÔNG thấy công cụ ghi/exec/xoá", not leaked, f"lộ: {leaked}")

    # --- quét mọi tool đọc không cần tham số ---------------------------------
    swept = 0
    for tool in read_tools:
        name = str(tool.get("name"))
        if _required(tool):
            continue
        reply = read_client.call_tool(name, {})
        swept += 1
        if reply.status >= 500 or reply.error is not None:
            ctx.check(f"gọi {name}", False, f"HTTP {reply.status} {reply.text[:160]}")
            continue
        text = reply.tool_text
        # isError hợp lệ khi thiếu hạ tầng ngoài (git host, Jira...), nhưng lỗi nội bộ thì không.
        ctx.check(f"gọi {name} không lỗi nội bộ", "MCP_INTERNAL" not in text and "internal error" not in text.lower(),
                  text[:160])
    ctx.check("đã quét ít nhất một công cụ đọc", swept > 0, "không có tool đọc nào không cần tham số")

    # --- hình dạng kết quả ----------------------------------------------------
    reply = read_client.call_tool("project_list", {})
    if reply.status == 200 and reply.error is None and not reply.tool_is_error:
        structured = reply.structured
        ctx.check("project_list: có content text + structuredContent", bool(reply.tool_text) and structured is not None,
                  reply.text[:160])
        ctx.check("project_list: danh sách bọc trong {items: [...]}",
                  isinstance(structured, dict) and isinstance(structured.get("items"), list), str(structured)[:160])
    else:
        ctx.skip("hình dạng kết quả project_list", f"gọi không thành công: {reply.text[:120]}")

    # --- tham số sai ----------------------------------------------------------
    reply = read_client.call_tool("task_get", {})
    ctx.check("thiếu tham số bắt buộc -> lỗi có kiểm soát", reply.error is not None or reply.tool_is_error,
              f"HTTP {reply.status} {reply.text[:160]}")
    reply = read_client.call_tool("task_get", {"id": 12345})
    ctx.check("sai kiểu tham số -> lỗi có kiểm soát", reply.error is not None or reply.tool_is_error,
              f"HTTP {reply.status} {reply.text[:160]}")

    # --- scope mở rộng thì công cụ xuất hiện ----------------------------------
    full = ctx.session_client(["orca:read", "orca:write", "orca:exec"])
    if full is None:
        return
    full_names = _names(full.list_tools())
    # Pack công cụ ghi/exec chỉ có khi gateway bật MCP_TOOL_PACKS_ENABLED (mặc định chỉ pack 1 = đọc).
    if "task_create" in full_names:
        ctx.check("pack ghi bật: token read+write thấy task_create", True)
    else:
        ctx.skip("token read+write thấy task_create", "pack 2 chưa bật (MCP_TOOL_PACKS_ENABLED=1,2 trên gateway)")
    if "task_execute" in full_names:
        ctx.check("pack exec bật: token có exec thấy task_execute", True)
    else:
        ctx.skip("token có exec thấy task_execute", "pack 3 chưa bật (MCP_TOOL_PACKS_ENABLED=1,2,3 trên gateway)")
    ctx.check("không có công cụ ngoài danh mục gateway công bố (tên hợp lệ)",
              all(n.replace("_", "").isalnum() for n in full_names), "tên tool lạ")
    ctx.check("token không có orca:admin không thấy công cụ admin",
              not any(n.startswith("admin_") for n in full_names), "lộ công cụ admin")

    # --- cổng phê duyệt của công cụ exec (chỉ khi được phép) -------------------
    if not ctx.cfg.probe_exec:
        ctx.skip("gọi công cụ exec", "đặt ORCA_MCP_PROBE_EXEC=true để thử (có thể tạo yêu cầu phê duyệt)")
        return
    reply = full.call_tool("task_execute", {"task_id": str(uuid.uuid4())})
    ctx.check("task_execute với task giả không thành công im lặng",
              reply.status < 500 and (reply.error is not None or reply.tool_is_error or "approval" in reply.text.lower()),
              f"HTTP {reply.status} {reply.text[:200]}")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
