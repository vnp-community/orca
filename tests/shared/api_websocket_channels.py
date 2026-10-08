"""WebSocket /ws — giao thức invoke/result/error mà frontend dùng (wscompat).

Tên kênh được quét từ mã Go (Registry.Register/RegisterStream) nên luôn khớp backend hiện tại.
Mặc định chỉ probe kênh CHỈ-ĐỌC (list/get/status/is/has...) với args rỗng; đặt ORCA_WS_PROBE_ALL=true
để probe mọi kênh (có thể thay đổi dữ liệu). Cần `pip install websocket-client`.
Spec: specs/backend-go/tdd/services/api-gateway.md (WS bridge), wscompat/envelope.go.
"""
from __future__ import annotations

import json
import re
import time
import uuid
from pathlib import Path

from check_framework import Context, run_single
from orca_api_session import SESSION_COOKIE

try:
    import websocket  # websocket-client
except ImportError:  # pragma: no cover
    websocket = None

SUITE = "websocket"
_WSCOMPAT = (Path(__file__).resolve().parents[2] / "backend-go" / "services" / "api-gateway"
             / "internal" / "adapter" / "wscompat")
_REG = re.compile(r'\.(Register|RegisterStream)\(\s*"([A-Za-z0-9_.:-]+)"')
_READONLY = re.compile(r"^(list|get|is|has|exists|status|count|peek|read|query|find)([A-Z0-9_]|$)")
# Kênh lấy ảnh/luồng nặng hoặc phụ thuộc phần cứng cục bộ — không probe tự động.
_SKIP = re.compile(r"^(terminal\.|browser\.|emulator\.|files\.watch|ephemeralVm\.)")


def discover_channels() -> tuple[list[str], list[str]]:
    """Trả về (kênh request/response, kênh stream) quét từ mã nguồn."""
    plain: set[str] = set()
    streams: set[str] = set()
    if _WSCOMPAT.is_dir():
        for f in _WSCOMPAT.glob("*.go"):
            if f.name.endswith("_test.go"):
                continue
            for kind, name in _REG.findall(f.read_text(encoding="utf-8")):
                (streams if kind == "RegisterStream" else plain).add(name)
    return sorted(plain), sorted(streams)


class WsRpc:
    def __init__(self, url: str, cookie: str | None, timeout: float, verify_tls: bool):
        opts = {} if verify_tls else {"sslopt": {"cert_reqs": 0}}
        headers = [f"Cookie: {cookie}"] if cookie else []
        self.ws = websocket.create_connection(url, header=headers, timeout=timeout, **opts)

    def invoke(self, channel: str, args: list | None = None, timeout: float = 30.0) -> dict:
        rid = str(uuid.uuid4())
        self.ws.settimeout(timeout)
        self.ws.send(json.dumps({"id": rid, "type": "invoke", "channel": channel, "args": args or []}))
        deadline = time.time() + timeout
        while time.time() < deadline:
            raw = self.ws.recv()
            msg = json.loads(raw) if raw else {}
            if msg.get("id") == rid and msg.get("type") in ("result", "error"):
                return msg
        raise TimeoutError(channel)

    def close(self) -> None:
        try:
            self.ws.close()
        except Exception:
            pass


def run(ctx: Context) -> None:
    if websocket is None:
        ctx.skip("WebSocket /ws", "thiếu thư viện websocket-client (pip install -r requirements.txt)")
        return
    admin = ctx.need_admin()
    if admin is None:
        return
    cookie = f"{SESSION_COOKIE}={admin.cookies[SESSION_COOKIE]}"

    # --- xác thực tại thời điểm upgrade (đóng 4401 nếu không có session) ---
    try:
        anon = WsRpc(ctx.cfg.ws_url, None, ctx.cfg.timeout, ctx.cfg.verify_tls)
        try:
            anon.ws.settimeout(ctx.cfg.timeout)
            # Server đóng 4401: recv() trả '' hoặc ném WebSocketConnectionClosedException tuỳ phiên bản.
            closed = not anon.ws.recv()
        except Exception:
            closed = True
        anon.close()
        ctx.assert_true("/ws không cookie -> server đóng kết nối (4401)", closed, "kết nối vẫn mở")
    except Exception:
        ctx.assert_true("/ws không cookie -> bị từ chối ở bước handshake", True)

    try:
        rpc = WsRpc(ctx.cfg.ws_url, cookie, ctx.cfg.timeout, ctx.cfg.verify_tls)
    except Exception as exc:
        ctx.assert_true("/ws kết nối với session cookie", False, str(exc))
        return
    ctx.assert_true("/ws kết nối với session cookie", True)

    try:
        msg = rpc.invoke("nonexistent.channel.apitest", [], timeout=15)
        ctx.assert_true("kênh không tồn tại -> error 'not yet implemented'",
                        msg["type"] == "error" and "not yet implemented" in msg.get("message", ""), str(msg)[:200])

        plain, streams = discover_channels()
        if not plain:
            ctx.skip("probe kênh", "không tìm thấy mã nguồn wscompat để quét danh sách kênh")
            return
        probe_all = ctx.cfg.ws_probe_all
        targets = [c for c in plain if not _SKIP.match(c)
                   and (probe_all or _READONLY.match(c.rsplit(".", 1)[-1]))]
        ctx.state["ws_channels_total"] = len(plain) + len(streams)
        print(f"[info] websocket: {len(plain)} kênh + {len(streams)} stream; probe {len(targets)} kênh "
              f"({'tất cả' if probe_all else 'chỉ-đọc'})")
        for ch in targets:
            try:
                msg = rpc.invoke(ch, [])
            except TimeoutError:
                ctx.assert_true(f"ws {ch}", False, "không phản hồi trong thời hạn")
                rpc = WsRpc(ctx.cfg.ws_url, cookie, ctx.cfg.timeout, ctx.cfg.verify_tls)
                continue
            except Exception as exc:
                ctx.assert_true(f"ws {ch}", False, f"mất kết nối: {exc}")
                rpc = WsRpc(ctx.cfg.ws_url, cookie, ctx.cfg.timeout, ctx.cfg.verify_tls)
                continue
            text = msg.get("message", "") if msg["type"] == "error" else ""
            # args rỗng -> lỗi validate là hợp lệ; chỉ coi là hỏng khi chưa cài đặt/panic/lỗi nội bộ.
            bad = re.search(r"not yet implemented|panic|nil pointer|internal error", text, re.I)
            ctx.assert_true(f"ws {ch}", not bad, text[:200])
    finally:
        rpc.close()


if __name__ == "__main__":
    run_single(SUITE, run)
