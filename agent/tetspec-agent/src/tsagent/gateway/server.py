"""MCP gateway (T7.3, FR-014, MECH-14, B14): JSON-RPC 2.0 over HTTP (Streamable HTTP, JSON mode) and stdio.

Protection: Bearer token (mandatory when binding beyond loopback), `Origin` check, request size limit, results cut at 30,000 characters
(read on with offset), review/baseline tools only when TSAGENT_MCP_REVIEW=1.
"""
import hmac
import json
import os
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

import jsonschema

from ..errors import ConfigError, TsagentError
from ..llm import make_client
from ..log import get_logger, mask_secrets
from ..store.db import Database
from .tools import TOOLS, tool_list

log = get_logger("tsagent.gateway")
MAX_TEXT = 30000
PROTOCOL = "2025-06-18"
LOOPBACK = ("127.0.0.1", "localhost", "::1")


class GatewayCtx:
    """Shared context: configuration, per-thread read connections, LLM client, background threads."""

    def __init__(self, cfg):
        self.cfg = cfg
        self._local = threading.local()
        self.threads = []
        self._client = None

    @property
    def db(self):
        d = getattr(self._local, "db", None)
        if d is None:
            d = Database(self.cfg.db, busy_timeout_ms=self.cfg.busy_timeout_ms)
            self._local.db = d
        return d

    def client(self):
        if self._client is None:
            self._client = make_client(self.cfg)
        return self._client


def _text(obj):
    text = obj if isinstance(obj, str) else json.dumps(obj, ensure_ascii=False, separators=(",", ":"), default=str)
    if len(text) > MAX_TEXT:
        text = text[:MAX_TEXT - 80] + f"\n...[cut at {MAX_TEXT} characters; narrow the query or use offset]"
    return text


def call_tool(ctx, name, args):
    t = TOOLS.get(name)
    if t is None or (t["reviewOnly"] and not ctx.cfg.mcp_review):
        hint = " (needs TSAGENT_MCP_REVIEW=1)" if t is not None else ""
        return {"content": [{"type": "text", "text": f"Error: no tool {name}{hint}"}], "isError": True}
    try:
        jsonschema.validate(args or {}, t["inputSchema"])
    except jsonschema.ValidationError as e:
        return {"content": [{"type": "text", "text": f"Error: invalid arguments: {e.message}"}], "isError": True}
    try:
        out = t["fn"](ctx, args or {})
        return {"content": [{"type": "text", "text": _text(out)}], "isError": False}
    except TsagentError as e:
        diags = getattr(e, "diagnostics", None)
        msg = str(e) + ("\n" + "\n".join(str(d) for d in diags[:30]) if diags else "")
        return {"content": [{"type": "text", "text": _text("Error: " + mask_secrets(msg))}], "isError": True}
    except Exception as e:  # programming error: report briefly, never leak details
        log.exception("tool %s failed", name)
        return {"content": [{"type": "text", "text": f"Internal error: {type(e).__name__}"}], "isError": True}


def handle(ctx, msg):
    """One JSON-RPC message -> response (None for a notification)."""
    if not isinstance(msg, dict) or msg.get("jsonrpc") != "2.0" or "method" not in msg:
        return {"jsonrpc": "2.0", "id": msg.get("id") if isinstance(msg, dict) else None,
                "error": {"code": -32600, "message": "Invalid Request"}}
    mid, method, params = msg.get("id"), msg["method"], msg.get("params") or {}
    if "id" not in msg:
        return None
    if method == "initialize":
        res = {"protocolVersion": params.get("protocolVersion") or PROTOCOL, "capabilities": {"tools": {"listChanged": False}},
               "serverInfo": {"name": "tsagent", "version": "2.0"}}
    elif method == "ping":
        res = {}
    elif method == "tools/list":
        res = {"tools": tool_list(ctx.cfg)}
    elif method == "tools/call":
        if not isinstance(params.get("name"), str):
            return {"jsonrpc": "2.0", "id": mid, "error": {"code": -32602, "message": "missing name"}}
        res = call_tool(ctx, params["name"], params.get("arguments") or {})
    else:
        return {"jsonrpc": "2.0", "id": mid, "error": {"code": -32601, "message": f"Method not found: {method}"}}
    return {"jsonrpc": "2.0", "id": mid, "result": res}


def allowed_origin(origin, host):
    if not origin:
        return True
    extra = [o.strip() for o in os.environ.get("TSAGENT_ALLOWED_ORIGINS", "").split(",") if o.strip()]
    if origin in extra:
        return True
    h = urlparse(origin).hostname or ""
    return h in LOOPBACK or (h and h == (host or "").split(":")[0])


def make_handler(ctx):
    class Handler(BaseHTTPRequestHandler):
        server_version = "tsagent"

        def log_message(self, fmt, *args):  # never write headers (they carry the token) to stderr
            log.debug("http %s", fmt % args)

        def _send(self, code, obj=None, extra=None):
            body = b"" if obj is None else json.dumps(obj, ensure_ascii=False).encode("utf-8")
            self.send_response(code)
            if obj is not None:
                self.send_header("Content-Type", "application/json; charset=utf-8")
            for k, v in (extra or {}).items():
                self.send_header(k, v)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            if body:
                self.wfile.write(body)

        def _authorized(self):
            if not ctx.cfg.token:
                return True
            got = self.headers.get("Authorization", "")
            return got.startswith("Bearer ") and hmac.compare_digest(got[7:].encode(), ctx.cfg.token.encode())

        def do_GET(self):
            if urlparse(self.path).path == "/health":
                ok = True
                try:
                    ctx.db.one("SELECT 1")
                except Exception:
                    ok = False
                return self._send(200 if ok else 503, {"status": "ok" if ok else "db-error"})
            return self._send(405, {"error": "only POST /mcp"}, {"Allow": "POST"})

        def do_DELETE(self):
            return self._send(405, {"error": "sessions are not supported"})

        def do_POST(self):
            if urlparse(self.path).path not in ("/mcp", "/"):
                return self._send(404, {"error": "not found"})
            if not allowed_origin(self.headers.get("Origin"), self.headers.get("Host")):
                return self._send(403, {"error": "Origin refused"})
            if not self._authorized():
                return self._send(401, {"error": "Bearer token required"}, {"WWW-Authenticate": "Bearer"})
            try:
                n = int(self.headers.get("Content-Length", "0"))
            except ValueError:
                return self._send(400, {"error": "Content-Length sai"})
            if n > ctx.cfg.max_body_kb * 1024:
                left = min(n, 16 * 1024 * 1024)  # drain the body (capped) so the client gets 413 instead of a broken connection
                while left > 0:
                    chunk = self.rfile.read(min(65536, left))
                    if not chunk:
                        break
                    left -= len(chunk)
                self.close_connection = True
                return self._send(413, {"error": f"request larger than {ctx.cfg.max_body_kb} KB"}, {"Connection": "close"})
            raw = self.rfile.read(n)
            try:
                msg = json.loads(raw.decode("utf-8"))
            except (ValueError, UnicodeDecodeError):
                return self._send(400, {"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "Parse error"}})
            if isinstance(msg, list):
                out = [r for r in (handle(ctx, m) for m in msg) if r is not None]
                return self._send(200, out) if out else self._send(202)
            res = handle(ctx, msg)
            return self._send(200, res) if res is not None else self._send(202)
    return Handler


def make_server(cfg, host="127.0.0.1", port=8766):
    if host not in LOOPBACK and not cfg.token:
        raise ConfigError("binding beyond loopback needs TSAGENT_TOKEN")
    ctx = GatewayCtx(cfg)
    srv = ThreadingHTTPServer((host, port), make_handler(ctx))
    srv.daemon_threads = True
    srv.ctx = ctx
    return srv


def serve_http(cfg, host="127.0.0.1", port=8766):
    srv = make_server(cfg, host, port)
    log.info("tsagent MCP at http://%s:%d/mcp", host, srv.server_address[1])
    try:
        srv.serve_forever()
    finally:
        srv.server_close()


def serve_stdio(cfg, stdin=None, stdout=None):
    ctx = GatewayCtx(cfg)
    stdin, stdout = stdin or sys.stdin, stdout or sys.stdout
    for line in stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except ValueError:
            res = {"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "Parse error"}}
        else:
            res = handle(ctx, msg)
        if res is not None:
            stdout.write(json.dumps(res, ensure_ascii=False) + "\n")
            stdout.flush()
