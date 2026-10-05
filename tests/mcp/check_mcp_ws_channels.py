"""Kênh WebSocket mcp.* (giao diện Settings → MCP): đọc, phân quyền admin, độ phủ so với mã Go.

Chỉ gọi kênh CHỈ-ĐỌC. Kênh ghi (token.create/revoke, admin.*.set/upsert/delete, approval.decide,
externalServer.*, session.close...) được liệt kê ở cuối là 'chưa gọi' — token.* được kiểm tra riêng
ở check_mcp_token_lifecycle.py. Spec: wscompat/channels_mcp*.go.
"""
from __future__ import annotations

import re
from pathlib import Path

from mcp_check_framework import Context, WsRpc, run_single

SUITE = "ws-channels"
_WSCOMPAT = (Path(__file__).resolve().parents[2] / "backend-go" / "services" / "api-gateway"
             / "internal" / "adapter" / "wscompat")
_REG = re.compile(r'\.Register\(\s*"(mcp\.[A-Za-z0-9_.]+)"')

USER_READS = ["mcp.server.info", "mcp.token.list", "mcp.session.list", "mcp.approval.list",
              "mcp.grant.list", "mcp.externalServer.list"]
ADMIN_READS = ["mcp.admin.tool.list", "mcp.admin.settings.get", "mcp.admin.killswitch.list",
               "mcp.admin.policy.list", "mcp.admin.client.list", "mcp.admin.grant.list",
               "mcp.admin.session.list", "mcp.admin.prompt.list"]


def _discover() -> set[str]:
    found: set[str] = set()
    if _WSCOMPAT.is_dir():
        for f in _WSCOMPAT.glob("channels_mcp*.go"):
            if not f.name.endswith("_test.go"):
                found |= set(_REG.findall(f.read_text(encoding="utf-8")))
    return found


def _bad(msg: dict) -> str | None:
    """Mô tả lỗi nếu phản hồi là lỗi nội bộ/chưa cài; None nếu ổn hoặc lỗi nghiệp vụ hợp lệ."""
    if msg.get("type") != "error":
        return None
    text = str(msg.get("message", ""))
    for marker in ("not yet implemented", "MCP_INTERNAL", "internal"):
        if marker in text:
            return text[:160]
    return None


def _invoke_all(ctx: Context, rpc: WsRpc, channels: list[str], who: str, called: set[str]) -> None:
    for ch in channels:
        msg = rpc.invoke(ch, [], timeout=ctx.cfg.timeout)
        called.add(ch)
        text = str(msg.get("message", ""))
        if "MCP_DISABLED" in text:
            ctx.skip(f"{ch} ({who})", "MCP_DISABLED trên gateway")
            continue
        problem = _bad(msg)
        ctx.check(f"{ch} ({who})", problem is None, problem or "")


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return
    admin = ctx.need_admin()
    if admin is None:
        return
    rpc = ctx.ws(admin)
    if rpc is None:
        return
    called: set[str] = set()

    info = rpc.invoke("mcp.server.info", [], timeout=ctx.cfg.timeout)
    called.add("mcp.server.info")
    ctx.check("mcp.server.info trả result", info.get("type") == "result", str(info)[:200])
    enabled = (info.get("result") or {}).get("enabled") if isinstance(info.get("result"), dict) else None
    ctx.check("mcp.server.info báo enabled=true khi /mcp được mount", enabled is not False, f"result={info.get('result')}")

    _invoke_all(ctx, rpc, USER_READS[1:], "admin", called)
    _invoke_all(ctx, rpc, ADMIN_READS, "admin", called)

    msg = rpc.invoke("mcp.admin.audit.query", [{}], timeout=ctx.cfg.timeout)
    called.add("mcp.admin.audit.query")
    ctx.check("mcp.admin.audit.query (bộ lọc rỗng)", _bad(msg) is None, _bad(msg) or "")

    # --- phân quyền: người dùng thường không được gọi kênh admin --------------
    if ctx.user is None:
        ctx.skip("kênh admin với người dùng thường -> MCP_NOT_ADMIN", "đặt ORCA_USER_EMAIL/ORCA_USER_PASSWORD")
    else:
        urpc = ctx.ws(ctx.user)
        if urpc is not None:
            _invoke_all(ctx, urpc, USER_READS, "user", called)
            for ch in ADMIN_READS[:3]:
                m = urpc.invoke(ch, [], timeout=ctx.cfg.timeout)
                ctx.check(f"{ch} (user thường) bị từ chối MCP_NOT_ADMIN",
                          m.get("type") == "error" and "MCP_NOT_ADMIN" in str(m.get("message", "")), str(m)[:200])

    # --- độ phủ ----------------------------------------------------------------
    known = _discover()
    if known:
        missing = sorted(known - called)
        print(f"[info] {SUITE}: {len(called & known)}/{len(known)} kênh mcp.* đã gọi; chưa gọi (ghi/hành động): "
              + ", ".join(missing))
        ctx.check("mọi kênh gọi ở trên có trong mã Go", called <= known, f"không có trong mã: {sorted(called - known)}")
    else:
        ctx.skip("độ phủ kênh mcp.*", "không tìm thấy mã wscompat để quét")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
