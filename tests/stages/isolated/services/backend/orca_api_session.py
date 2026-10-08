"""HTTP session cho api-gateway: đăng nhập cookie, gửi request, ghi nhận độ phủ route."""
from __future__ import annotations

import time
import uuid
from http.cookiejar import DefaultCookiePolicy
from typing import Any

import requests

from env_config import Config

SESSION_COOKIE = "orca_session"
REFRESH_COOKIE = "orca_refresh"

# Mọi (method, path-template) đã được gọi — run_all.py dùng để báo cáo độ phủ.
EXERCISED: set[tuple[str, str]] = set()


class ApiSession:
    """Một danh tính (hoặc ẩn danh) gọi api-gateway.

    Cookie orca_session được đặt Secure=true nên `requests` không gửi lại qua
    http:// — ta tự gắn header Cookie để chạy được cả môi trường dev không TLS.
    """

    def __init__(self, cfg: Config, label: str = "anon"):
        self.cfg = cfg
        self.label = label
        self.http = requests.Session()
        # Cookie do ta tự quản: jar tự giữ cookie sẽ làm request ẩn danh bị nhiễm session.
        self.http.cookies.set_policy(DefaultCookiePolicy(allowed_domains=[]))
        self.cookies: dict[str, str] = {}
        self.bearer: str | None = None
        self.user: dict[str, Any] | None = None

    # ---- đăng nhập -----------------------------------------------------
    def login(self, email: str, password: str) -> requests.Response:
        resp = self.request("POST", "/auth/local", json={"email": email, "password": password},
                            template="/auth/local")
        if resp.status_code == 200:
            self._absorb_cookies(resp)
            try:
                self.user = resp.json()
            except ValueError:
                self.user = None
        return resp

    def _absorb_cookies(self, resp: requests.Response) -> None:
        for name in (SESSION_COOKIE, REFRESH_COOKIE):
            val = resp.cookies.get(name)
            if val:
                self.cookies[name] = val

    @property
    def authenticated(self) -> bool:
        return SESSION_COOKIE in self.cookies

    # ---- request -------------------------------------------------------
    def request(self, method: str, path: str, *, template: str | None = None,
                json: Any = None, params: dict | None = None, data: Any = None,
                headers: dict | None = None, auth: bool = True,
                raw_cookie: str | None = None) -> requests.Response:
        """Gửi request. `template` là dạng path có {param} để tính độ phủ."""
        hdrs = {"Accept": "application/json", "X-Request-Id": str(uuid.uuid4())}
        if auth and self.cookies:
            hdrs["Cookie"] = "; ".join(f"{k}={v}" for k, v in self.cookies.items())
        if raw_cookie is not None:
            hdrs["Cookie"] = raw_cookie
        if auth and self.bearer:
            hdrs["Authorization"] = f"Bearer {self.bearer}"
        if headers:
            hdrs.update(headers)
        if self.cfg.delay_s:
            time.sleep(self.cfg.delay_s)
        EXERCISED.add((method.upper(), template or path.split("?", 1)[0]))
        try:
            return self.http.request(
                method, self.cfg.base_url + path, json=json, params=params, data=data,
                headers=hdrs, timeout=self.cfg.timeout, verify=self.cfg.verify_tls,
                allow_redirects=False,
            )
        except requests.RequestException as exc:  # trả về response giả để check ghi nhận lỗi mạng
            fake = requests.Response()
            fake.status_code = 599
            fake._content = str(exc).encode()
            fake.url = self.cfg.base_url + path
            return fake

    def get(self, path, **kw):
        return self.request("GET", path, **kw)

    def post(self, path, **kw):
        return self.request("POST", path, **kw)

    def put(self, path, **kw):
        return self.request("PUT", path, **kw)

    def patch(self, path, **kw):
        return self.request("PATCH", path, **kw)

    def delete(self, path, **kw):
        return self.request("DELETE", path, **kw)


def json_body(resp: requests.Response) -> Any:
    try:
        return resp.json()
    except ValueError:
        return None


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
        for item in data:
            found = find_value(item, *keys)
            if found not in (None, ""):
                return found
    return None
