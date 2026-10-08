"""Cổng vào /mcp: xác thực bearer, Origin, kích thước body, thứ tự initialize, PAT không đẻ PAT.

Spec: mcpserver/middleware.go (authenticate, rejectCookieOnly, guardOrigin, limitBody,
requireInitializeFirst) và adapter/httpgateway/auth_mcp_token_routes.go.
"""
from __future__ import annotations

import json

from mcp_check_framework import Context, run_single

SUITE = "auth-enforcement"
INIT = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
    "protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "selftest", "version": "1"}}})


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return

    anon = ctx.client(None)
    r = anon.raw("POST", headers={"Content-Type": "application/json", "Accept": "application/json, text/event-stream"},
                 data=INIT)
    ctx.check("không token -> 401", r.status_code == 401, f"HTTP {r.status_code}")
    www = r.headers.get("WWW-Authenticate", "")
    ctx.check("401 kèm WWW-Authenticate: Bearer resource_metadata",
              www.lower().startswith("bearer") and "resource_metadata" in www, f"WWW-Authenticate={www!r}")

    bad = ctx.client("not-a-real-token")
    r = bad.raw("POST", headers={"Authorization": "Bearer not-a-real-token", "Content-Type": "application/json",
                                "Accept": "application/json, text/event-stream"}, data=INIT)
    ctx.check("token rác -> 401 invalid_token", r.status_code == 401 and "invalid_token" in r.headers.get("WWW-Authenticate", ""),
              f"HTTP {r.status_code} {r.headers.get('WWW-Authenticate')!r}")

    if ctx.admin is not None and ctx.admin.authenticated:
        r = anon.raw("POST", headers={"Cookie": ctx.admin.cookie_header, "Content-Type": "application/json",
                                      "Accept": "application/json, text/event-stream"}, data=INIT)
        ctx.check("chỉ có cookie phiên (không bearer) -> 401", r.status_code == 401, f"HTTP {r.status_code}")
    else:
        ctx.skip("cookie-only bị từ chối", "không có phiên đăng nhập")

    for method in ("PUT", "PATCH"):
        r = anon.raw(method, headers={"Authorization": "Bearer x"})
        ctx.check(f"{method} /mcp không phải 5xx", r.status_code < 500, f"HTTP {r.status_code}")

    token = ctx.token_for(["orca:read"])
    if not token:
        ctx.skip("kiểm tra có token", "không có PAT (đăng nhập admin hoặc ORCA_MCP_TOKEN)")
        return

    c = ctx.client(token, origin="https://evil.example")
    r = c.raw("POST", headers={"Authorization": f"Bearer {token}", "Origin": "https://evil.example",
                               "Content-Type": "application/json", "Accept": "application/json, text/event-stream"},
              data=INIT)
    ctx.check("Origin lạ -> 403 forbidden_origin", r.status_code == 403, f"HTTP {r.status_code}: {r.text[:120]}")

    if ctx.cfg.allowed_origin:
        r = c.raw("POST", headers={"Authorization": f"Bearer {token}", "Origin": ctx.cfg.allowed_origin,
                                   "Content-Type": "application/json", "Accept": "application/json, text/event-stream"},
                  data=INIT)
        ctx.check(f"Origin {ctx.cfg.allowed_origin} (trong allow-list) được chấp nhận", r.status_code == 200,
                  f"HTTP {r.status_code}")
    else:
        ctx.skip("Origin hợp lệ được chấp nhận", "đặt ORCA_MCP_ALLOWED_ORIGIN để kiểm tra")

    good = ctx.client(token)
    # initialize trước, sau đó mọi lời gọi không-initialize không session phải bị chặn.
    r = good.raw("POST", headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json",
                                  "Accept": "application/json, text/event-stream"},
                 data=json.dumps({"jsonrpc": "2.0", "id": 2, "method": "tools/list"}))
    ctx.check("tools/list không session -> 400 (-32600)", r.status_code == 400 and "-32600" in r.text,
              f"HTTP {r.status_code}: {r.text[:120]}")

    r = good.raw("POST", headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json",
                                  "Accept": "application/json, text/event-stream"}, data="{not json")
    ctx.check("JSON hỏng -> 4xx, không 5xx", 400 <= r.status_code < 500, f"HTTP {r.status_code}: {r.text[:120]}")

    big = '{"jsonrpc":"2.0","id":3,"method":"ping","params":{"pad":"' + "x" * (2 * 1024 * 1024) + '"}}'
    r = good.raw("POST", headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json",
                                  "Accept": "application/json, text/event-stream"}, data=big)
    ctx.check("body 2 MiB -> 413", r.status_code == 413, f"HTTP {r.status_code}")

    r = good.raw("GET", headers={"Authorization": f"Bearer {token}", "Accept": "text/event-stream"})
    ctx.check("GET stream không session -> 4xx", 400 <= r.status_code < 500, f"HTTP {r.status_code}")

    if ctx.admin is not None and ctx.admin.authenticated:
        r = ctx.admin.request("POST", "/v1/auth/mcp-tokens", bearer=token, cookies=False,
                              json_body={"name": ctx.name("mint"), "scopes": ["orca:read"], "expires_in_days": 1})
        ctx.check("PAT không tự tạo được PAT (401/403)", r.status_code in (401, 403), f"HTTP {r.status_code}: {r.text[:120]}")
    else:
        ctx.skip("PAT không tạo được PAT", "không có phiên đăng nhập")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
