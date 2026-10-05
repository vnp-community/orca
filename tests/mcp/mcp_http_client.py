"""Client MCP tối giản qua streamable HTTP (JSON-RPC 2.0), đủ để kiểm tra server.

Tự viết thay vì dùng SDK để quan sát được status HTTP, header và các lỗi giao thức
(-32600/-32601/-32700) mà SDK thường che đi.
"""
from __future__ import annotations

import json
import uuid
from dataclasses import dataclass, field
from typing import Any

import requests

PROTOCOL_VERSION = "2025-06-18"  # khớp mcpserver.SupportedProtocolVersions


@dataclass
class RpcReply:
    """Một phản hồi HTTP cho một lời gọi JSON-RPC."""
    status: int
    headers: dict[str, str]
    message: dict[str, Any] | None   # message JSON-RPC khớp id (nếu có)
    text: str

    @property
    def result(self) -> Any:
        return (self.message or {}).get("result")

    @property
    def error(self) -> dict[str, Any] | None:
        return (self.message or {}).get("error")

    @property
    def tool_is_error(self) -> bool:
        r = self.result
        return isinstance(r, dict) and bool(r.get("isError"))

    @property
    def tool_text(self) -> str:
        r = self.result
        if isinstance(r, dict):
            for c in r.get("content") or []:
                if isinstance(c, dict) and c.get("type") == "text":
                    return str(c.get("text", ""))
        return ""

    @property
    def structured(self) -> Any:
        r = self.result
        return r.get("structuredContent") if isinstance(r, dict) else None


def _utf8(resp: requests.Response) -> str:
    return resp.content.decode("utf-8", errors="replace") if resp.content else ""


def _parse_body(resp: requests.Response, rid: Any) -> dict[str, Any] | None:
    ctype = resp.headers.get("Content-Type", "")
    # SSE không khai báo charset nên requests đoán latin-1 và làm hỏng tiếng Việt; JSON-RPC luôn là UTF-8.
    text = resp.content.decode("utf-8", errors="replace") if resp.content else ""
    candidates: list[Any] = []
    if "text/event-stream" in ctype:
        for line in text.splitlines():
            if line.startswith("data:"):
                try:
                    candidates.append(json.loads(line[5:].strip()))
                except ValueError:
                    pass
    else:
        try:
            body = resp.json()
        except ValueError:
            return None
        candidates = body if isinstance(body, list) else [body]
    for c in candidates:
        if isinstance(c, dict) and (rid is None or c.get("id") == rid):
            return c
    return candidates[0] if candidates and isinstance(candidates[0], dict) else None


@dataclass
class McpClient:
    url: str
    token: str | None
    timeout: float = 20.0
    verify_tls: bool = True
    origin: str | None = None
    session_id: str | None = None
    protocol_version: str | None = None
    extra_headers: dict[str, str] = field(default_factory=dict)
    _next_id: int = 0

    def _headers(self, with_session: bool = True) -> dict[str, str]:
        h = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream",
             "X-Request-Id": str(uuid.uuid4())}
        if self.token:
            h["Authorization"] = f"Bearer {self.token}"
        if self.origin:
            h["Origin"] = self.origin
        if with_session and self.session_id:
            h["Mcp-Session-Id"] = self.session_id
        if with_session and self.protocol_version:
            h["MCP-Protocol-Version"] = self.protocol_version
        h.update(self.extra_headers)
        return h

    def _send(self, method: str, **kw: Any) -> requests.Response:
        try:
            return requests.request(method, self.url, timeout=self.timeout, verify=self.verify_tls,
                                    allow_redirects=False, **kw)
        except requests.RequestException as exc:  # lỗi mạng -> status 599 để check ghi nhận FAIL
            fake = requests.Response()
            fake.status_code = 599
            fake._content = str(exc).encode()
            fake.url = self.url
            return fake

    def rpc(self, method: str, params: dict | None = None, *, with_session: bool = True,
            raw_body: bytes | str | None = None) -> RpcReply:
        self._next_id += 1
        rid = self._next_id
        if raw_body is None:
            payload: dict[str, Any] = {"jsonrpc": "2.0", "id": rid, "method": method}
            if params is not None:
                payload["params"] = params
            raw_body = json.dumps(payload)
        resp = self._send("POST", data=raw_body, headers=self._headers(with_session))
        sid = resp.headers.get("Mcp-Session-Id")
        if sid and method == "initialize":
            self.session_id = sid
        return RpcReply(resp.status_code, dict(resp.headers), _parse_body(resp, rid), _utf8(resp))

    def notify(self, method: str, params: dict | None = None) -> RpcReply:
        payload: dict[str, Any] = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            payload["params"] = params
        resp = self._send("POST", data=json.dumps(payload), headers=self._headers())
        return RpcReply(resp.status_code, dict(resp.headers), None, _utf8(resp))

    def initialize(self, version: str = PROTOCOL_VERSION) -> RpcReply:
        reply = self.rpc("initialize", {
            "protocolVersion": version, "capabilities": {},
            "clientInfo": {"name": "orca-mcp-selftest", "version": "1.0"},
        }, with_session=False)
        res = reply.result
        if isinstance(res, dict):
            self.protocol_version = res.get("protocolVersion")
        return reply

    def open_session(self) -> RpcReply:
        """initialize + notifications/initialized."""
        reply = self.initialize()
        if reply.status == 200 and self.session_id:
            self.notify("notifications/initialized")
        return reply

    def call_tool(self, name: str, arguments: dict | None = None) -> RpcReply:
        return self.rpc("tools/call", {"name": name, "arguments": arguments or {}})

    def list_tools(self) -> list[dict[str, Any]]:
        tools: list[dict[str, Any]] = []
        cursor = None
        for _ in range(50):  # chặn vòng lặp vô hạn nếu server trả cursor lặp
            reply = self.rpc("tools/list", {"cursor": cursor} if cursor else {})
            res = reply.result
            if not isinstance(res, dict):
                break
            tools += [t for t in res.get("tools") or [] if isinstance(t, dict)]
            cursor = res.get("nextCursor")
            if not cursor:
                break
        return tools

    def delete_session(self) -> int:
        return self._send("DELETE", headers=self._headers()).status_code

    def raw(self, method: str, *, headers: dict[str, str] | None = None, data: Any = None) -> requests.Response:
        """Gửi request thô (không tự gắn Authorization/session) để thử các trường hợp biên."""
        return self._send(method, headers=headers or {}, data=data)
