"""Vòng đời PAT: tạo (REST và WS), liệt kê, dùng trên /mcp, thu hồi, kiểm tra bị chặn sau thu hồi.

Spec: adapter/httpgateway/auth_mcp_token_routes.go, adapter/mcptokens/service.go, docs/guides/mcp/personal-access-tokens.md.
Thu hồi có hiệu lực trong ~1 phút (cache xác thực), nên suite này chờ tối đa ORCA_MCP_REVOKE_WAIT_S giây.
"""
from __future__ import annotations

import time

from mcp_check_framework import Context, run_single

SUITE = "token-lifecycle"


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return
    admin = ctx.need_admin()
    if admin is None:
        return
    if not ctx.cfg.allow_writes:
        ctx.skip("vòng đời PAT", "ORCA_ALLOW_WRITES=false")
        return

    name = ctx.name("lifecycle")
    resp = admin.request("POST", "/v1/auth/mcp-tokens",
                         json_body={"name": name, "scopes": ["orca:read"], "expires_in_days": 1})
    ctx.check("POST /v1/auth/mcp-tokens -> 201", resp.status_code == 201, f"HTTP {resp.status_code}: {resp.text[:160]}")
    if resp.status_code != 201:
        return
    body = resp.json()
    secret, token = body.get("secret"), body.get("token") or {}
    jti = token.get("id")
    ctx.check("trả secret một lần + id", bool(secret and jti), str(body)[:160])
    ctx.check("Cache-Control: no-store trên phản hồi chứa secret",
              "no-store" in resp.headers.get("Cache-Control", ""), f"Cache-Control={resp.headers.get('Cache-Control')!r}")
    ctx.check("token trả về đúng scope và status active",
              token.get("scopes") == ["orca:read"] and token.get("status") == "active", str(token)[:160])
    if not (secret and jti):
        return
    ctx.cleanup(f"revoke {jti}", lambda: admin.request("DELETE", f"/v1/auth/mcp-tokens/{jti}"))

    lst = admin.request("GET", "/v1/auth/mcp-tokens")
    ctx.check("GET /v1/auth/mcp-tokens -> 200 và có token vừa tạo",
              lst.status_code == 200 and any(t.get("id") == jti for t in (lst.json().get("tokens") or [])),
              f"HTTP {lst.status_code}: {lst.text[:160]}")
    ctx.check("danh sách KHÔNG chứa secret", secret not in lst.text, "secret lộ trong danh sách")

    # --- dùng token ---------------------------------------------------------------
    client = ctx.client(secret)
    reply = client.open_session()
    ctx.check("token mới initialize được trên /mcp", reply.status == 200, f"HTTP {reply.status}: {reply.text[:160]}")
    if client.session_id:
        ctx.cleanup("đóng session", client.delete_session)

    # --- tham số sai --------------------------------------------------------------
    for label, payload in (
        ("scope không tồn tại", {"name": ctx.name("bad"), "scopes": ["orca:bogus"], "expires_in_days": 1}),
        ("tên rỗng", {"name": "", "scopes": ["orca:read"], "expires_in_days": 1}),
        ("thời hạn quá dài", {"name": ctx.name("long"), "scopes": ["orca:read"], "expires_in_days": 100000}),
    ):
        r = admin.request("POST", "/v1/auth/mcp-tokens", json_body=payload)
        ctx.check(f"tạo PAT với {label} -> 4xx", 400 <= r.status_code < 500, f"HTTP {r.status_code}: {r.text[:120]}")
        if r.status_code == 201:  # lỡ tạo được thì dọn
            extra = (r.json().get("token") or {}).get("id")
            if extra:
                admin.request("DELETE", f"/v1/auth/mcp-tokens/{extra}")

    if ctx.user is not None:
        r = ctx.user.request("POST", "/v1/auth/mcp-tokens",
                             json_body={"name": ctx.name("adm"), "scopes": ["orca:admin"], "expires_in_days": 1})
        ctx.check("người dùng thường xin scope orca:admin -> bị từ chối", r.status_code in (400, 403), f"HTTP {r.status_code}")
        if r.status_code == 201:
            extra = (r.json().get("token") or {}).get("id")
            if extra:
                ctx.user.request("DELETE", f"/v1/auth/mcp-tokens/{extra}")
    else:
        ctx.skip("user thường xin orca:admin", "đặt ORCA_USER_EMAIL/ORCA_USER_PASSWORD")

    # --- tương đương qua WebSocket --------------------------------------------------
    rpc = ctx.ws(admin)
    if rpc is not None:
        msg = rpc.invoke("mcp.token.list", [], timeout=ctx.cfg.timeout)
        ctx.check("mcp.token.list (WS) thấy token tạo qua REST", msg.get("type") == "result" and jti in str(msg.get("result")),
                  str(msg)[:200])

    # --- thu hồi ------------------------------------------------------------------
    r = admin.request("DELETE", f"/v1/auth/mcp-tokens/{jti}")
    ctx.check("DELETE /v1/auth/mcp-tokens/{id} -> 204", r.status_code == 204, f"HTTP {r.status_code}: {r.text[:120]}")
    r = admin.request("DELETE", f"/v1/auth/mcp-tokens/{jti}")
    ctx.check("thu hồi lần hai không 5xx", r.status_code < 500, f"HTTP {r.status_code}")

    if ctx.cfg.revoke_wait_s <= 0:
        ctx.skip("token bị chặn sau thu hồi", "ORCA_MCP_REVOKE_WAIT_S=0")
        return
    probe = ctx.client(secret)
    deadline, status = time.time() + ctx.cfg.revoke_wait_s, 0
    while time.time() < deadline:
        status = probe.initialize().status
        if status in (401, 403):
            break
        if probe.session_id:
            probe.delete_session()
            probe.session_id = None
        time.sleep(3)
    ctx.check(f"token đã thu hồi bị chặn trong {ctx.cfg.revoke_wait_s:.0f}s", status in (401, 403), f"HTTP cuối={status}")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
