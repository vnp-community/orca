"""Khung chạy kiểm tra MCP: ghi nhận PASS/FAIL/SKIP, đăng nhập, cấp PAT tạm, WebSocket, dọn dẹp."""
from __future__ import annotations

import json
import time
import traceback
import uuid
from dataclasses import dataclass, field
from typing import Any, Callable

import requests

from mcp_env_config import McpConfig, load_config
from mcp_http_client import McpClient

try:
    import websocket  # websocket-client (tuỳ chọn)
except ImportError:  # pragma: no cover
    websocket = None

SESSION_COOKIE = "orca_session"
ALL_SCOPES = ["orca:read", "orca:write", "orca:exec"]


@dataclass
class Outcome:
    suite: str
    name: str
    status: str  # PASS | FAIL | SKIP
    detail: str = ""


class RestSession:
    """Phiên REST bằng cookie (cookie Secure nên tự gắn header để chạy được trên http dev)."""

    def __init__(self, cfg: McpConfig):
        self.cfg = cfg
        self.cookies: dict[str, str] = {}

    @property
    def authenticated(self) -> bool:
        return SESSION_COOKIE in self.cookies

    @property
    def cookie_header(self) -> str:
        return "; ".join(f"{k}={v}" for k, v in self.cookies.items())

    def request(self, method: str, path: str, *, json_body: Any = None, bearer: str | None = None,
                cookies: bool = True) -> requests.Response:
        headers = {"Accept": "application/json", "X-Request-Id": str(uuid.uuid4())}
        if cookies and self.cookies:
            headers["Cookie"] = self.cookie_header
        if bearer:
            headers["Authorization"] = f"Bearer {bearer}"
        try:
            return requests.request(method, self.cfg.base_url + path, json=json_body, headers=headers,
                                    timeout=self.cfg.timeout, verify=self.cfg.verify_tls, allow_redirects=False)
        except requests.RequestException as exc:
            fake = requests.Response()
            fake.status_code = 599
            fake._content = str(exc).encode()
            fake.url = self.cfg.base_url + path
            return fake

    def login(self, email: str, password: str) -> requests.Response:
        resp = self.request("POST", "/auth/local", json_body={"email": email, "password": password}, cookies=False)
        if resp.status_code == 200:
            for name in (SESSION_COOKIE, "orca_refresh"):
                if resp.cookies.get(name):
                    self.cookies[name] = resp.cookies.get(name)
        return resp


class WsRpc:
    """Giao thức invoke/result/error của /ws (cùng định dạng frontend dùng)."""

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


def find_value(data: Any, *keys: str) -> Any:
    """Tìm đệ quy giá trị đầu tiên (không rỗng) của một trong các key."""
    if isinstance(data, dict):
        for k in keys:
            if k in data and data[k] not in (None, ""):
                return data[k]
        for v in data.values():
            found = find_value(v, *keys)
            if found not in (None, ""):
                return found
    elif isinstance(data, list):
        for v in data:
            found = find_value(v, *keys)
            if found not in (None, ""):
                return found
    return None


@dataclass
class Context:
    cfg: McpConfig
    admin: RestSession | None = None
    user: RestSession | None = None
    state: dict[str, Any] = field(default_factory=dict)
    results: list[Outcome] = field(default_factory=list)
    cleanups: list[tuple[str, Callable[[], None]]] = field(default_factory=list)
    suite: str = ""
    _tokens: dict[tuple[str, ...], str] = field(default_factory=dict)

    # ---- ghi nhận ------------------------------------------------------
    def _record(self, name: str, status: str, detail: str = "") -> bool:
        self.results.append(Outcome(self.suite, name, status, detail))
        mark = {"PASS": "  ok ", "FAIL": " FAIL", "SKIP": " skip"}[status]
        print(f"[{mark}] {self.suite}: {name}" + (f"  -> {detail}" if detail and status != "PASS" else ""))
        return status == "PASS"

    def check(self, name: str, cond: bool, detail: str = "") -> bool:
        return self._record(name, "PASS" if cond else "FAIL", "" if cond else detail)

    def skip(self, name: str, reason: str) -> None:
        self._record(name, "SKIP", reason)

    def cleanup(self, label: str, fn: Callable[[], None]) -> None:
        self.cleanups.append((label, fn))

    def run_cleanups(self) -> None:
        for label, fn in reversed(self.cleanups):
            try:
                fn()
            except Exception as exc:  # dọn dẹp không được làm hỏng báo cáo
                print(f"[warn] cleanup {label}: {exc}")
        self.cleanups.clear()
        # PAT tạm vừa bị thu hồi ở trên: suite sau phải tạo token mới, không dùng lại bản cache.
        self._tokens.clear()

    def name(self, tag: str) -> str:
        return f"{self.cfg.prefix}-{tag}-{uuid.uuid4().hex[:8]}"

    # ---- tình trạng MCP ------------------------------------------------
    @property
    def mcp_enabled(self) -> bool:
        return bool(self.state.get("mcp_enabled"))

    def need_mcp(self) -> bool:
        if not self.mcp_enabled:
            self.skip("MCP endpoint", "MCP đang tắt (MCP_ENABLED=false) — đặt ORCA_MCP_EXPECT_ENABLED=true để coi là lỗi")
            return False
        return True

    def need_admin(self) -> RestSession | None:
        if self.admin is None or not self.admin.authenticated:
            self.skip("yêu cầu phiên đăng nhập", "không đăng nhập được (kiểm tra ORCA_ADMIN_*)")
            return None
        return self.admin

    # ---- PAT tạm -------------------------------------------------------
    def token_for(self, scopes: list[str]) -> str | None:
        """Trả PAT có đúng `scopes` (tạo qua REST bằng phiên admin, tự thu hồi khi kết thúc).

        Không có tài khoản đăng nhập thì dùng ORCA_MCP_TOKEN (không biết scope chính xác).
        """
        key = tuple(sorted(scopes))
        if key in self._tokens:
            return self._tokens[key]
        if self.admin is not None and self.admin.authenticated:
            resp = self.admin.request("POST", "/v1/auth/mcp-tokens", json_body={
                "name": self.name("pat"), "scopes": scopes, "expires_in_days": 1})
            body = resp.json() if resp.status_code == 201 else {}
            secret = body.get("secret")
            jti = (body.get("token") or {}).get("id")
            if secret and jti:
                admin = self.admin
                self.cleanup(f"revoke PAT {jti}", lambda: admin.request("DELETE", f"/v1/auth/mcp-tokens/{jti}"))
                self._tokens[key] = secret
                return secret
            print(f"[warn] không tạo được PAT {scopes}: {resp.status_code} {resp.text[:120]}")
        if self.cfg.mcp_token:
            self._tokens[key] = self.cfg.mcp_token
            return self.cfg.mcp_token
        return None

    def client(self, token: str | None, **kw: Any) -> McpClient:
        return McpClient(self.cfg.mcp_url, token, self.cfg.timeout, self.cfg.verify_tls, **kw)

    def session_client(self, scopes: list[str]) -> McpClient | None:
        """Client đã initialize xong với PAT `scopes`; tự đóng session khi kết thúc."""
        token = self.token_for(scopes)
        if not token:
            self.skip(f"PAT {scopes}", "không có tài khoản đăng nhập hay ORCA_MCP_TOKEN")
            return None
        c = self.client(token)
        reply = c.open_session()
        if reply.status != 200 or not c.session_id:
            self.check(f"initialize với PAT {scopes}", False, f"HTTP {reply.status}: {reply.text[:160]}")
            return None
        self.cleanup("đóng session MCP", c.delete_session)
        return c

    # ---- WebSocket -----------------------------------------------------
    def ws(self, session: RestSession) -> WsRpc | None:
        if websocket is None:
            self.skip("WebSocket /ws", "thiếu websocket-client (pip install -r requirements.txt)")
            return None
        try:
            rpc = WsRpc(self.cfg.ws_url, f"{SESSION_COOKIE}={session.cookies[SESSION_COOKIE]}",
                        self.cfg.timeout, self.cfg.verify_tls)
        except Exception as exc:
            self.check("/ws kết nối với session cookie", False, str(exc))
            return None
        self.cleanup("đóng /ws", rpc.close)
        return rpc


def build_context() -> Context:
    cfg = load_config()
    ctx = Context(cfg=cfg)
    if cfg.has_admin_credentials:
        s = RestSession(cfg)
        resp = s.login(cfg.admin_email, cfg.admin_password)
        if resp.status_code == 200 and s.authenticated:
            ctx.admin = s
        else:
            print(f"[warn] đăng nhập admin thất bại: {resp.status_code} {resp.text[:120]}")
    if cfg.user_email and cfg.user_password:
        u = RestSession(cfg)
        resp = u.login(cfg.user_email, cfg.user_password)
        if resp.status_code == 200 and u.authenticated:
            ctx.user = u
    # MCP có được mount không? (/.well-known luôn 404 khi MCP_ENABLED=false)
    try:
        probe = requests.get(cfg.base_url + "/.well-known/oauth-protected-resource", timeout=cfg.timeout,
                             verify=cfg.verify_tls, allow_redirects=False)
        ctx.state["mcp_enabled"] = probe.status_code != 404
    except requests.RequestException as exc:
        ctx.state["mcp_enabled"] = False
        ctx.state["unreachable"] = str(exc)[:160]
    return ctx


def run_suite(ctx: Context, name: str, fn: Callable[[Context], None]) -> None:
    ctx.suite = name
    try:
        fn(ctx)
    except Exception:  # lỗi lập trình/mạng trong suite -> FAIL có traceback
        ctx._record("suite crashed", "FAIL", traceback.format_exc(limit=3))
    finally:
        ctx.run_cleanups()


def summarize(ctx: Context) -> int:
    counts = {"PASS": 0, "FAIL": 0, "SKIP": 0}
    for r in ctx.results:
        counts[r.status] += 1
    print("\n" + "=" * 70)
    print(f"PASS={counts['PASS']}  FAIL={counts['FAIL']}  SKIP={counts['SKIP']}  (mcp={ctx.cfg.mcp_url})")
    for r in ctx.results:
        if r.status == "FAIL":
            print(f"  FAIL {r.suite}: {r.name}\n       {r.detail}")
    return 1 if counts["FAIL"] else 0


def run_single(suite_name: str, fn: Callable[[Context], None]) -> int:
    """Chạy một suite độc lập (python check_mcp_xxx.py)."""
    ctx = build_context()
    if "unreachable" in ctx.state:
        ctx.suite = suite_name
        ctx.check("gateway truy cập được", False, ctx.state["unreachable"])
        return summarize(ctx)
    if not ctx.mcp_enabled and ctx.cfg.expect_enabled:
        ctx.suite = suite_name
        ctx.check("MCP được mount", False, "/.well-known/oauth-protected-resource trả 404 (MCP_ENABLED=false?)")
        return summarize(ctx)
    run_suite(ctx, suite_name, fn)
    return summarize(ctx)
