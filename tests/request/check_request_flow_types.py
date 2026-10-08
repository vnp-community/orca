"""T2 (CR-REQ-025): vòng đời Request qua /ws cho 11 loại, trên stack dev có stub agent.

Mỗi loại: request.create -> chờ phân loại -> request.confirmType -> trạng thái kế tiếp phải khớp
`request.flow` (đường trạng thái chuẩn do request-service trả về, không chép bảng vào script).
Các giai đoạn sau xác nhận loại (Solution, Plan, thực thi) thêm vào khi các RPC tương ứng có thật.

Chạy: ORCA_REQUEST_PROJECT_ID=<id> python check_request_flow_types.py   (cần ORCA_ADMIN_* và `pip install websocket-client`).
Chưa chạy trên stack dev thật ở thời điểm viết (xem IMPLEMENTATION-NOTES của request-quality-rollout).
"""
from __future__ import annotations

import time
import uuid

from request_env_config import REQUEST_TYPES, marker, project_id, real_ai

from api_websocket_channels import WsRpc, websocket  # noqa: E402  (đường dẫn đặt bởi request_env_config)
from check_framework import Context, run_single  # noqa: E402
from orca_api_session import SESSION_COOKIE  # noqa: E402

SUITE = "request-flow-types"
POLL_TIMEOUT_S = 90.0


def result_of(msg: dict) -> dict:
    return msg.get("result") or {}


def wait_status(rpc: WsRpc, request_id: str, want: str, timeout: float = POLL_TIMEOUT_S) -> str:
    deadline = time.time() + timeout
    status = ""
    while time.time() < deadline:
        msg = rpc.invoke("request.get", [{"id": request_id}])
        status = result_of(msg).get("request", {}).get("status", "")
        if status == want:
            return status
        time.sleep(1.0)
    return status


def run(ctx: Context) -> None:
    if websocket is None:
        ctx.skip(SUITE, "thiếu thư viện websocket-client")
        return
    admin = ctx.need_admin()
    if admin is None:
        return
    pid = project_id()
    if not pid:
        ctx.skip(SUITE, "đặt ORCA_REQUEST_PROJECT_ID (project có dev server hoặc stub agent)")
        return
    if real_ai():
        ctx.skip(SUITE, "ORCA_REQUEST_E2E_REAL_AI=true thuộc T3, chạy riêng")
        return

    cookie = f"{SESSION_COOKIE}={admin.cookies[SESSION_COOKIE]}"
    rpc = WsRpc(ctx.cfg.ws_url, cookie, ctx.cfg.timeout, ctx.cfg.verify_tls)
    try:
        flag = rpc.invoke("request.flowSet", [{"enabled": True}])
        ctx.assert_true("bật request_flow_enabled cho tenant", flag["type"] == "result" and result_of(flag).get("enabled") is True,
                        str(flag)[:200])
        for typ in REQUEST_TYPES:
            size = "S" if typ in ("hotfix", "question", "docs", "task") else "M"
            flow = result_of(rpc.invoke("request.flow", [{"type": typ, "size": size}]))
            path = flow.get("statusPath") or []
            ctx.assert_true(f"{typ}: request.flow trả đường trạng thái", "awaiting_type_confirmation" in path, str(flow)[:200])
            if "awaiting_type_confirmation" not in path:
                continue
            created = rpc.invoke("request.create", [{
                "projectId": pid, "title": f"{ctx.cfg.prefix} {typ} {marker(typ, size)}", "body": "T2 kiểm tra luồng",
                "clientRequestId": str(uuid.uuid4()),
            }])
            rid = result_of(created).get("request", {}).get("id", "")
            ctx.assert_true(f"{typ}: tạo Request", bool(rid), str(created)[:200])
            if not rid:
                continue
            ctx.cleanup(f"huỷ {rid}", lambda rid=rid: rpc.invoke("request.cancel", [{"id": rid, "reason": "dọn dẹp T2"}]))
            ctx.assert_true(f"{typ}: AI đề xuất loại, chờ người xác nhận",
                            wait_status(rpc, rid, "awaiting_type_confirmation") == "awaiting_type_confirmation", "không tới awaiting_type_confirmation")
            confirmed = rpc.invoke("request.confirmType", [{
                "id": rid, "type": typ, "size": size, "urgency": "urgent" if typ == "hotfix" else "normal", "reason": "xác nhận trong T2",
            }])
            want = path[path.index("awaiting_type_confirmation") + 1]
            got = result_of(confirmed).get("request", {}).get("status", "")
            ctx.assert_true(f"{typ}: sau xác nhận loại là {want}", got == want, f"được {got!r}: {str(confirmed)[:200]}")
    finally:
        ctx.run_cleanups()
        rpc.close()


if __name__ == "__main__":
    run_single(SUITE, run)
