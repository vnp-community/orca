"""Notification (/v1/notifications/*, /api/push-*, /api/trace-stream), Usage (/v1/usage/*),
AI provider (/v1/ai-providers/*).

Spec: specs/backend-go/tdd/services/notification-service.md, usage-service.md, ai-provider-service.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

import uuid
from datetime import datetime, timedelta, timezone

import requests

from check_framework import Context, run_single
from orca_api_session import EXERCISED
from orca_api_session import find_value, json_body

SUITE = "notifications+usage+ai"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def _check_trace_sse(ctx: Context) -> None:
    """SSE công khai: kết nối được, đúng Content-Type, nhận dòng ': connected'."""
    EXERCISED.add(("GET", "/api/trace-stream"))
    try:
        with requests.get(ctx.cfg.base_url + "/api/trace-stream", stream=True, timeout=ctx.cfg.timeout,
                          verify=ctx.cfg.verify_tls, headers={"Accept": "text/event-stream"}) as resp:
            ok_type = "text/event-stream" in resp.headers.get("Content-Type", "")
            first = next(resp.iter_lines(decode_unicode=True), "")
            ctx.assert_true("GET /api/trace-stream là SSE và gửi ': connected'",
                            resp.status_code == 200 and ok_type and first.startswith(":"),
                            f"status={resp.status_code} type={resp.headers.get('Content-Type')} first={first!r}")
    except requests.RequestException as exc:
        ctx.assert_true("GET /api/trace-stream là SSE", False, str(exc))


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    uid = (admin.user or {}).get("id", "")

    # ---------------- notifications ----------------
    ctx.expect("GET /v1/notifications", admin.get("/v1/notifications/", params={"limit": 5}), 200)
    ctx.expect("GET /v1/notifications?unread_only", admin.get("/v1/notifications/", params={"unread_only": "true"}),
               200)
    r = admin.get("/v1/notifications/unread-count")
    ctx.expect("GET /v1/notifications/unread-count", r, 200)
    ctx.expect("GET /v1/notifications/vapid-public-key", admin.get("/v1/notifications/vapid-public-key"),
               200, "handled")
    ctx.expect("GET /api/vapid-public-key (công khai)", ctx.anon.get("/api/vapid-public-key", auth=False),
               200, "handled")
    _check_trace_sse(ctx)
    ctx.expect("GET /v1/notifications/stream không upgrade WS -> 4xx", admin.get("/v1/notifications/stream"), "4xx")
    if ctx.need_writes("notification writes"):
        ctx.expect("POST /v1/notifications/read-all", admin.post("/v1/notifications/read-all"), "2xx")
        ctx.expect("POST /v1/notifications/{id}/read không tồn tại", admin.post(
                   f"/v1/notifications/{ZERO_UUID}/read", template="/v1/notifications/{id}/read"), "handled")
        sub = {"endpoint": f"https://push.example.test/{uuid.uuid4()}", "p256dh_key": "BPubKey" + "A" * 80,
               "auth_key": "AuthKey" + "A" * 16, "device_label": ctx.name("browser")}
        ctx.expect("POST /v1/notifications/subscribe", admin.post("/v1/notifications/subscribe", json=sub),
                   "2xx", "handled")
        ctx.expect("POST /api/push-subscribe (có cookie)", admin.post("/api/push-subscribe", json=sub),
                   "2xx", "handled")
        ctx.expect("POST /api/push-subscribe thiếu endpoint -> 400", ctx.anon.post(
                   "/api/push-subscribe", json={}, auth=False), 400)
        ctx.expect("POST /api/push-unsubscribe", ctx.anon.post("/api/push-unsubscribe",
                   json={"endpoint": sub["endpoint"]}, auth=False), "no5xx")

    # ---------------- usage ----------------
    today = datetime.now(timezone.utc).strftime("%Y-%m-%d")
    ctx.expect("GET /v1/usage/daily", admin.get("/v1/usage/daily", params={"user_id": uid, "day": today,
               "provider": "claude"}), 200)
    ctx.expect("GET /v1/usage/daily day sai định dạng -> 400", admin.get("/v1/usage/daily",
               params={"user_id": uid, "day": "31/12/2026"}), 400)
    ctx.expect("GET /v1/usage/sessions", admin.get("/v1/usage/sessions", params={"user_id": uid, "page_size": 5}),
               200)
    if ctx.need_writes("usage writes"):
        start = datetime.now(timezone.utc) - timedelta(minutes=5)
        sid = str(uuid.uuid4())
        rec = {"id": sid, "provider": "claude", "worktree_id": ZERO_UUID, "input_tokens": 100,
               "output_tokens": 50, "cache_read_tokens": 10, "cache_write_tokens": 5, "cost_usd": 0.01,
               "started_at": start.isoformat().replace("+00:00", "Z"),
               "ended_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
               "request_id": str(uuid.uuid4())}
        ctx.expect("POST /v1/usage/sessions", admin.post("/v1/usage/sessions", json=rec), "2xx")
        ctx.expect("POST /v1/usage/sessions idempotent (cùng request_id)", admin.post("/v1/usage/sessions",
                   json=rec), "2xx", 409)
        r = admin.get("/v1/usage/sessions", params={"user_id": uid, "page_size": 50})
        ctx.assert_true("session usage vừa ghi có trong danh sách", sid in r.text, r.text[:200])
        ctx.expect("POST /v1/usage/sessions body hỏng -> 400", admin.post(
                   "/v1/usage/sessions", data="{bad", headers={"Content-Type": "application/json"}), 400)

    # ---------------- AI providers ----------------
    ctx.expect("GET /v1/ai-providers/resolve", admin.get("/v1/ai-providers/resolve",
               params={"user_id": uid, "model_hint": "claude"}), "handled")
    ctx.expect("GET /v1/ai-providers/usage-today", admin.get("/v1/ai-providers/usage-today",
               params={"account_id": ZERO_UUID}), "handled")
    if ctx.need_writes("ai-provider writes"):
        r = admin.post("/v1/ai-providers/accounts", json={
            "type": "anthropic", "label": ctx.name("acct"), "model_hint": "claude-sonnet",
            "base_url": "https://api.anthropic.com", "quota_limit_day": 1000, "models": ["claude-sonnet"],
            "is_default": False})
        ctx.expect("POST /v1/ai-providers/accounts", r, "2xx", "handled")
        acct = find_value(json_body(r), "id", "account_id", "accountId") if r.status_code < 300 else None
        ctx.expect("POST accounts type sai -> 400", admin.post(
                   "/v1/ai-providers/accounts", json={"type": "bogus", "label": "x"}), 400)
        if acct:
            ctx.expect("GET /v1/ai-providers/usage-today (account thật)", admin.get(
                       "/v1/ai-providers/usage-today", params={"account_id": acct}), 200)
            ctx.expect("POST /v1/ai-providers/accounts/{id}/rotate-key", admin.post(
                       f"/v1/ai-providers/accounts/{acct}/rotate-key",
                       template="/v1/ai-providers/accounts/{id}/rotate-key"), "2xx", "handled")
        ctx.expect("POST rotate-key account không tồn tại -> 404", admin.post(
                   f"/v1/ai-providers/accounts/{ZERO_UUID}/rotate-key",
                   template="/v1/ai-providers/accounts/{id}/rotate-key"), 404, 400)


if __name__ == "__main__":
    run_single(SUITE, run)
