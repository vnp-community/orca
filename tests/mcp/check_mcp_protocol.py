"""Giao thức MCP 2025-06-18: initialize, ping, tools/resources/prompts, lỗi JSON-RPC, vòng đời session.

Spec: mcpserver/{engine,protocol_versions,session_host}.go.
"""
from __future__ import annotations

from mcp_check_framework import Context, run_single
from mcp_http_client import PROTOCOL_VERSION

SUITE = "protocol"


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return
    token = ctx.token_for(["orca:read"])
    if not token:
        ctx.skip("giao thức MCP", "không có PAT (đăng nhập admin hoặc ORCA_MCP_TOKEN)")
        return

    # --- initialize ----------------------------------------------------
    c = ctx.client(token)
    reply = c.initialize()
    ctx.check("initialize -> 200", reply.status == 200, f"HTTP {reply.status}: {reply.text[:160]}")
    res = reply.result if isinstance(reply.result, dict) else {}
    ctx.check("initialize trả Mcp-Session-Id", bool(c.session_id), "thiếu header Mcp-Session-Id")
    ctx.check(f"protocolVersion = {PROTOCOL_VERSION}", res.get("protocolVersion") == PROTOCOL_VERSION,
              f"protocolVersion={res.get('protocolVersion')!r}")
    ctx.check("serverInfo.name có giá trị", bool((res.get("serverInfo") or {}).get("name")), str(res)[:160])
    caps = res.get("capabilities") or {}
    ctx.check("capabilities.tools có mặt", "tools" in caps, f"capabilities={caps}")
    if reply.status != 200:
        return
    ctx.cleanup("đóng session MCP", c.delete_session)
    ack = c.notify("notifications/initialized")
    ctx.check("notifications/initialized -> 202", ack.status in (200, 202), f"HTTP {ack.status}")

    # Phiên bản không hỗ trợ: server phải trả phiên bản mới nhất của nó (lifecycle spec).
    other = ctx.client(token)
    r2 = other.initialize("1999-01-01")
    ctx.check("initialize phiên bản lạ -> thương lượng về bản server hỗ trợ",
              r2.status == 200 and (r2.result or {}).get("protocolVersion") == PROTOCOL_VERSION,
              f"HTTP {r2.status} {str(r2.result)[:120]}")
    if other.session_id:
        ctx.cleanup("đóng session phụ", other.delete_session)

    # --- các phương thức cơ bản -----------------------------------------
    r = c.rpc("ping")
    ctx.check("ping -> result rỗng", r.status == 200 and r.error is None, f"HTTP {r.status} {r.text[:120]}")

    tools = c.list_tools()
    ctx.check("tools/list có công cụ", len(tools) > 0, "danh sách rỗng")
    bad = [t.get("name") for t in tools if not t.get("name") or (t.get("inputSchema") or {}).get("type") != "object"]
    ctx.check("mọi tool có name + inputSchema kiểu object", not bad, f"sai: {bad[:5]}")
    names = [t.get("name") for t in tools]
    ctx.check("tên tool không trùng", len(names) == len(set(names)), "có tên trùng")

    for method, key in (("resources/list", "resources"), ("prompts/list", "prompts")):
        r = c.rpc(method, {})
        if r.error and r.error.get("code") == -32601:
            ctx.skip(method, "server không khai báo capability này")
            continue
        res = r.result if isinstance(r.result, dict) else {}
        ctx.check(f"{method} trả mảng '{key}'", r.status == 200 and isinstance(res.get(key), list),
                  f"HTTP {r.status} {r.text[:120]}")

    r = c.rpc("logging/setLevel", {"level": "info"})
    ctx.check("logging/setLevel không lỗi 5xx", r.status < 500, f"HTTP {r.status}")

    # --- lỗi JSON-RPC -----------------------------------------------------
    r = c.rpc("orca/doesNotExist", {})
    # SDK trả JSON-RPC -32601 hoặc HTTP 400 "not handled" tuỳ đường đi; cả hai đều là từ chối có kiểm soát.
    rejected = (r.error or {}).get("code") == -32601 or (r.status == 400 and "not handled" in r.text.lower())
    ctx.check("phương thức lạ bị từ chối có kiểm soát (-32601 hoặc 400)", rejected, f"HTTP {r.status} {r.text[:160]}")

    r = c.call_tool("tool_khong_ton_tai_" + ctx.name("x"))
    ctx.check("tools/call tool lạ -> lỗi (JSON-RPC hoặc isError)", r.error is not None or r.tool_is_error,
              f"HTTP {r.status} {r.text[:160]}")

    r = c.rpc("tools/list", {"cursor": "cursor-hong"})
    ctx.check("cursor hỏng -> lỗi, không 5xx", r.status < 500 and (r.error is not None or r.status >= 400),
              f"HTTP {r.status} {r.text[:160]}")

    # --- vòng đời session -----------------------------------------------
    ghost = ctx.client(token)
    ghost.session_id = "00000000-0000-0000-0000-000000000000"
    ghost.protocol_version = PROTOCOL_VERSION
    r = ghost.rpc("ping")
    ctx.check("session id không tồn tại -> 404", r.status == 404, f"HTTP {r.status}")

    closing = ctx.client(token)
    closing.open_session()
    if closing.session_id:
        code = closing.delete_session()
        ctx.check("DELETE session -> 2xx", 200 <= code < 300, f"HTTP {code}")
        r = closing.rpc("ping")
        ctx.check("dùng lại session đã đóng -> 404", r.status == 404, f"HTTP {r.status}")

    # session của người khác không dùng được (session gắn với chủ sở hữu)
    token2 = ctx.token_for(["orca:read", "orca:write"])
    if token2 and token2 != token and c.session_id:
        thief = ctx.client(token2)
        thief.session_id, thief.protocol_version = c.session_id, PROTOCOL_VERSION
        r = thief.rpc("ping")
        # cùng người dùng nhưng khác token: chỉ khẳng định không 5xx; nếu khác chủ phải bị chặn.
        ctx.check("dùng session với token khác không gây 5xx", r.status < 500, f"HTTP {r.status}")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
