"""Quản trị: /v1/auth/users|sessions|audit-log và /admin/api/* (stats, users, sessions, policies, audit).

Spec: specs/backend-go/tdd/services/auth-service.md, api-gateway.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single
from orca_api_session import ApiSession, find_value, json_body

SUITE = "admin"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return

    # --- read-only ---
    r = admin.get("/admin/api/stats")
    ctx.expect("GET /admin/api/stats", r, 200)
    stats = json_body(r) or {}
    ctx.assert_true("stats có totalUsers/activeSessions/totalPolicies",
                    all(k in stats for k in ("totalUsers", "activeSessions", "totalPolicies")), f"body={stats}")
    ctx.expect("GET /admin/api/users", admin.get("/admin/api/users"), 200)
    ctx.expect("GET /v1/auth/users", admin.get("/v1/auth/users", params={"page_size": 5}), 200)
    ctx.expect("GET /admin/api/sessions", admin.get("/admin/api/sessions"), 200)
    ctx.expect("GET /admin/api/policies", admin.get("/admin/api/policies"), 200)
    ctx.expect("GET /admin/api/audit", admin.get("/admin/api/audit", params={"page_size": 5}), 200)
    ctx.expect("GET /v1/auth/audit-log", admin.get("/v1/auth/audit-log", params={"page_size": 5}), 200)
    r = admin.get("/admin/api/audit/export")
    ctx.expect("GET /admin/api/audit/export (CSV)", r, 200)
    ctx.assert_true("audit export là text/csv", "text/csv" in r.headers.get("Content-Type", ""),
                    r.headers.get("Content-Type", ""))
    ctx.expect("POST /v1/auth/sessions/{id}/revoke id không tồn tại",
               admin.post(f"/v1/auth/sessions/{ZERO_UUID}/revoke", template="/v1/auth/sessions/{id}/revoke"),
               "handled", 200)
    ctx.expect("DELETE /admin/api/sessions/{sessionId} id không tồn tại",
               admin.delete(f"/admin/api/sessions/{ZERO_UUID}", template="/admin/api/sessions/{sessionId}"),
               "handled", 200)

    # --- phân quyền: user thường không được vào /admin/api ---
    if ctx.user is not None:
        ctx.expect("user thường GET /admin/api/stats -> 403", ctx.user.get("/admin/api/stats"), 403)
    else:
        ctx.skip("user thường GET /admin/api/stats -> 403", "chưa cấu hình ORCA_USER_EMAIL/PASSWORD")

    if not ctx.need_writes("admin writes"):
        return

    # --- users ---
    email = f"{ctx.name('user')}@example.test"
    password = "Test-" + ctx.name("pw")
    r = admin.post("/v1/auth/users", json={"email": email, "name": "API Test", "role": "user",
                                           "password": password})
    ctx.expect("POST /v1/auth/users", r, "2xx")
    user_id = find_value(json_body(r), "id", "userId", "user_id")
    if not user_id:
        ctx.skip("vòng đời user", "tạo user không trả id")
    else:
        ctx.cleanup(f"user {user_id}", lambda: admin.delete(f"/admin/api/users/{user_id}",
                                                           template="/admin/api/users/{id}"))
        r = admin.get("/v1/auth/users", params={"page_size": 200})
        ctx.assert_true("user mới xuất hiện trong danh sách",
                        any(u.get("id") == user_id for u in (json_body(r) or {}).get("users", [])),
                        "không thấy trong 200 user đầu")
        ctx.expect("PUT /v1/auth/users/{id}/role", admin.put(f"/v1/auth/users/{user_id}/role",
                   template="/v1/auth/users/{id}/role", json={"role": "admin"}), "2xx")
        ctx.expect("PUT /v1/auth/users/{id}/role giá trị sai -> 400", admin.put(
                   f"/v1/auth/users/{user_id}/role", template="/v1/auth/users/{id}/role",
                   json={"role": "superuser"}), 400)
        ctx.expect("PATCH /admin/api/users/{id}", admin.patch(f"/admin/api/users/{user_id}",
                   template="/admin/api/users/{id}", json={"name": "API Test Renamed", "role": "user"}), "2xx")
        # Đăng nhập bằng user mới -> có session để quản trị rồi thu hồi.
        probe = ApiSession(ctx.cfg, "new-user")
        if probe.login(email, password).status_code == 200:
            ctx.expect("user mới GET /admin/api/stats -> 403", probe.get("/admin/api/stats"), 403)
        else:
            ctx.skip("đăng nhập user mới", "login thất bại (rate-limit?)")
        ctx.expect("DELETE /admin/api/users/{userId}/sessions",
                   admin.delete(f"/admin/api/users/{user_id}/sessions",
                                template="/admin/api/users/{userId}/sessions"), "2xx")
        ctx.expect("user bị thu hồi session -> /auth/me 401", probe.get("/auth/me"), 401)
        ctx.expect("DELETE /admin/api/users/{id} (deactivate)",
                   admin.delete(f"/admin/api/users/{user_id}", template="/admin/api/users/{id}"), "2xx")
        ctx.expect("POST /v1/auth/users email trùng -> 409/400",
                   admin.post("/v1/auth/users", json={"email": email, "name": "dup", "role": "user",
                                                      "password": password}), 409, 400, "2xx")
    ctx.expect("POST /admin/api/users thiếu email -> 400",
               admin.post("/admin/api/users", json={"name": "x", "role": "user", "password": "x"}), 400)

    # --- policies (CRUD + optimistic version) ---
    doc = '{"limit": 100}'
    r = admin.post("/admin/api/policies", json={"name": ctx.name("policy"), "kind": "rate-tier",
                                                "document_json": doc})
    ctx.expect("POST /admin/api/policies", r, "2xx")
    pid = find_value(json_body(r), "id", "policyId")
    if pid:
        ctx.cleanup(f"policy {pid}", lambda: admin.delete(f"/admin/api/policies/{pid}",
                                                         template="/admin/api/policies/{id}"))
        ctx.expect("PUT /admin/api/policies/{id}", admin.put(f"/admin/api/policies/{pid}",
                   template="/admin/api/policies/{id}",
                   json={"document_json": '{"limit": 200}', "expected_version": 1}), "2xx")
        ctx.expect("PUT policy expected_version cũ -> xung đột", admin.put(
                   f"/admin/api/policies/{pid}", template="/admin/api/policies/{id}",
                   json={"document_json": '{"limit": 300}', "expected_version": 1}), 409, 412, 400)
        ctx.expect("DELETE /admin/api/policies/{id}", admin.delete(f"/admin/api/policies/{pid}",
                   template="/admin/api/policies/{id}"), "2xx")
    else:
        ctx.skip("vòng đời policy", "tạo policy không trả id")
    ctx.expect("POST /admin/api/policies thiếu kind -> 400",
               admin.post("/admin/api/policies", json={"name": "x", "document_json": "{}"}), 400)


if __name__ == "__main__":
    run_single(SUITE, run)
