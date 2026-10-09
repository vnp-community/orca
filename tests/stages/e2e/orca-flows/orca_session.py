"""Authenticated access to the gateway: HTTP login (cookie session) and WebSocket RPC with the same cookies."""
from __future__ import annotations

import json
import urllib.error
import urllib.request
from http.cookiejar import CookieJar

from orca_stack_config import StackConfig
from ws_rpc_client import WsRpcClient


class HttpReply:
    def __init__(self, status: int, body: bytes, headers):
        self.status, self.body, self.headers = status, body, headers

    @property
    def text(self) -> str:
        return self.body.decode(errors="replace")

    def json(self):
        return json.loads(self.text)


def http_request(method: str, url: str, body: dict | None = None, opener=None, timeout: float = 15.0) -> HttpReply:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json"} if data else {})
    open_ = opener.open if opener else urllib.request.urlopen
    try:
        with open_(req, timeout=timeout) as resp:
            return HttpReply(resp.status, resp.read(), resp.headers)
    except urllib.error.HTTPError as err:
        return HttpReply(err.code, err.read(), err.headers)


class OrcaSession:
    def __init__(self, cfg: StackConfig):
        self.cfg = cfg
        self.jar = CookieJar()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.jar))
        self.user: dict = {}
        self._ws: WsRpcClient | None = None

    def login(self) -> HttpReply:
        reply = http_request("POST", f"{self.cfg.gateway_url}/auth/local",
                             {"email": self.cfg.admin_email, "password": self.cfg.admin_password}, self.opener)
        if reply.status == 200:
            self.user = reply.json()
        return reply

    @property
    def cookie_header(self) -> str:
        return "; ".join(f"{c.name}={c.value}" for c in self.jar)

    def get_authed(self, path: str) -> HttpReply:
        # http.cookiejar drops cookies for the bare host "localhost", so send the session cookie explicitly.
        req = urllib.request.Request(self.cfg.gateway_url + path, headers={"Cookie": self.cookie_header})
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return HttpReply(resp.status, resp.read(), resp.headers)
        except urllib.error.HTTPError as err:
            return HttpReply(err.code, err.read(), err.headers)

    def ws(self) -> WsRpcClient:
        if self._ws is None:
            client = WsRpcClient(self.cfg.ws_url, cookie=self.cookie_header, origin=self.cfg.frontend_url)
            client.connect()
            self._ws = client
        return self._ws

    def close(self) -> None:
        if self._ws:
            self._ws.close()
