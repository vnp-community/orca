"""Auth: /auth/*, /v1/auth/cli-tokens, /v1/users/me/paired-devices, pairing public route.

Spec: specs/backend-go/tdd/services/auth-service.md, api-gateway.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

import base64
import os

from check_framework import Context, run_single
from orca_api_session import ApiSession, find_value, json_body

SUITE = "auth"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    anon = ctx.anon

    # --- GET /auth/config (public) ---
    r = anon.get("/auth/config")
    ctx.expect("GET /auth/config", r, 200)
    body = json_body(r) or {}
    ctx.assert_true("auth/config có providers[] và localEnabled",
                    isinstance(body.get("providers"), list) and isinstance(body.get("localEnabled"), bool),
                    f"body={body}")

    # --- POST /auth/local ---
    ctx.expect("POST /auth/local sai mật khẩu -> 401",
               anon.post("/auth/local", json={"email": "nobody@invalid.test", "password": "x"}), 401, 403)
    ctx.expect("POST /auth/local body hỏng -> 400",
               anon.post("/auth/local", data="not-json", headers={"Content-Type": "application/json"}), 400)
    admin = ctx.need_admin()
    if admin is None:
        return
    ctx.assert_true("POST /auth/local đăng nhập admin (cookie orca_session)", admin.authenticated)
    ctx.assert_true("login trả user có id/email/role",
                    bool(admin.user and admin.user.get("id") and admin.user.get("email")
                         and admin.user.get("role")), f"user={admin.user}")

    # --- GET /auth/me ---
    r = admin.get("/auth/me")
    ctx.expect("GET /auth/me (có cookie)", r, 200)
    ctx.assert_true("auth/me email khớp", (json_body(r) or {}).get("email") == ctx.cfg.admin_email,
                    f"body={json_body(r)}")
    ctx.expect("GET /auth/me (không cookie) -> 401", anon.get("/auth/me"), 401)

    # --- POST /auth/cli-token ---
    r = anon.post("/auth/cli-token", json={"email": ctx.cfg.admin_email, "password": ctx.cfg.admin_password})
    ctx.expect("POST /auth/cli-token", r, 200)
    ctx.assert_true("cli-token trả jwt", bool((json_body(r) or {}).get("jwt")), f"body={json_body(r)}")
    ctx.expect("POST /auth/cli-token sai mật khẩu -> 401",
               anon.post("/auth/cli-token", json={"email": ctx.cfg.admin_email, "password": "wrong-" + os.urandom(3).hex()}),
               401)

    # --- refresh / logout trên phiên riêng (không làm hỏng phiên admin dùng chung) ---
    sess = ApiSession(ctx.cfg, "refresh-probe")
    if sess.login(ctx.cfg.admin_email, ctx.cfg.admin_password).status_code == 200:
        r = sess.post("/auth/refresh")
        ctx.expect("POST /auth/refresh (có refresh cookie)", r, 200)
        sess._absorb_cookies(r)
        ctx.expect("GET /auth/me sau refresh", sess.get("/auth/me"), 200)
        ctx.expect("POST /auth/logout", sess.post("/auth/logout"), 200)
        ctx.expect("GET /auth/me sau logout -> 401", sess.get("/auth/me"), 401)
    else:
        ctx.skip("refresh/logout", "login phiên phụ thất bại (rate-limit?)")
    ctx.expect("POST /auth/refresh không cookie -> 401", anon.post("/auth/refresh"), 401)
    ctx.expect("POST /auth/logout không cookie -> 200", anon.post("/auth/logout"), 200)

    # --- SSO (không cần IdP thật: chỉ kiểm tra gateway xử lý, không 5xx) ---
    ctx.expect("GET /auth/sso/{provider} provider lạ", anon.get("/auth/sso/unknown-provider",
               template="/auth/sso/{provider}"), "no5xx")
    ctx.expect("GET /auth/callback thiếu code/state", anon.get("/auth/callback"), "no5xx")

    # --- CLI tokens (đăng nhập) ---
    if ctx.need_writes("cli-tokens"):
        r = admin.post("/v1/auth/cli-tokens/")
        ctx.expect("POST /v1/auth/cli-tokens", r, 201)
        r = admin.get("/v1/auth/cli-tokens/")
        ctx.expect("GET /v1/auth/cli-tokens", r, 200)
        tokens = (json_body(r) or {}).get("tokens") or []
        jti = tokens[0].get("jti") if tokens else None
        if jti:
            ctx.expect("DELETE /v1/auth/cli-tokens/{id}",
                       admin.delete(f"/v1/auth/cli-tokens/{jti}", template="/v1/auth/cli-tokens/{id}"), "2xx")
        else:
            ctx.skip("DELETE /v1/auth/cli-tokens/{id}", "không có token để thu hồi")
        ctx.expect("DELETE cli-token không tồn tại", admin.delete(f"/v1/auth/cli-tokens/{ZERO_UUID}",
                   template="/v1/auth/cli-tokens/{id}"), "4xx", "2xx")

    # --- Paired devices ---
    if ctx.need_writes("paired-devices"):
        r = admin.post("/v1/users/me/paired-devices/pairing-sessions")
        ctx.expect("POST /v1/users/me/paired-devices/pairing-sessions", r, 201)
        token = find_value(json_body(r), "pairing_token", "pairingToken", "token")
        ctx.expect("GET /v1/users/me/paired-devices", admin.get("/v1/users/me/paired-devices/"), 200)
        tpl = "/v1/paired-devices/pairing-sessions/{token}/complete"
        ctx.expect("POST pairing complete token sai -> 4xx",
                   anon.post("/v1/paired-devices/pairing-sessions/bogus-token/complete", template=tpl,
                             json={"mobilePublicKey": base64.b64encode(os.urandom(32)).decode(),
                                   "deviceLabel": "api-test"}), "4xx")
        if token:
            r = anon.post(f"/v1/paired-devices/pairing-sessions/{token}/complete", template=tpl,
                          json={"mobilePublicKey": base64.b64encode(os.urandom(32)).decode(),
                                "deviceLabel": ctx.name("device")})
            ctx.expect("POST pairing complete (token hợp lệ)", r, 200)
            device_id = find_value(json_body(r), "device_id", "deviceId", "id")
            if device_id:
                ctx.expect("DELETE /v1/users/me/paired-devices/{deviceId}",
                           admin.delete(f"/v1/users/me/paired-devices/{device_id}",
                                        template="/v1/users/me/paired-devices/{deviceId}"), "2xx")
        else:
            ctx.skip("pairing complete (token hợp lệ)", "response không có pairing token")
        ctx.expect("DELETE paired-device không tồn tại", admin.delete(
            f"/v1/users/me/paired-devices/{ZERO_UUID}", template="/v1/users/me/paired-devices/{deviceId}"),
            "4xx", "2xx")


if __name__ == "__main__":
    run_single(SUITE, run)
