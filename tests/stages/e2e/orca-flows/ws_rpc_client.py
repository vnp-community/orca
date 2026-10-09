"""Minimal RFC 6455 client for the gateway's `/ws` invoke protocol (stdlib only, so the suite needs no pip install).

Wire format: {"id","type":"invoke","channel","args":[...]} -> {"type":"result"|"error","id",...}; pushes carry no id.
"""
from __future__ import annotations

import base64
import collections
import json
import os
import queue
import re
import socket
import ssl
import struct
import threading
import time
from urllib.parse import urlparse


def retry_after_seconds(exc: Exception, fallback: float) -> float:
    """Wait the gateway's `{"retryAfterMs":N}` trailer asks for (plus a little slack); fall back to a fixed backoff."""
    match = re.search(r'"retryAfterMs":\s*(\d+)', str(exc))
    return min(10.0, int(match.group(1)) / 1000.0 + 0.05) if match else fallback


class RpcError(Exception):
    """A gateway `error` message; `message` keeps the raw text so tests can match a code inside it."""

    def __init__(self, message: str):
        super().__init__(message)
        self.message = message

    def has_code(self, code: str) -> bool:
        return code in self.message


class WsRpcClient:
    def __init__(self, url: str, cookie: str = "", origin: str = "", default_timeout: float = 20.0):
        self.url, self.cookie, self.origin, self.default_timeout = url, cookie, origin, default_timeout
        self._sock: socket.socket | None = None
        self._pending: dict[str, queue.Queue] = {}
        self._lock = threading.Lock()
        self._next_id = 0
        self.pushes: collections.deque = collections.deque(maxlen=200)
        self._closed = threading.Event()
        self.rate_limit_retries = 0  # how often the service throttled us; reported so a too-tight limit is visible

    # ---- connection -----------------------------------------------------
    def connect(self) -> None:
        u = urlparse(self.url)
        port = u.port or (443 if u.scheme == "wss" else 80)
        raw = socket.create_connection((u.hostname, port), timeout=10)
        self._sock = ssl.create_default_context().wrap_socket(raw, server_hostname=u.hostname) if u.scheme == "wss" else raw
        key = base64.b64encode(os.urandom(16)).decode()
        headers = [
            f"GET {u.path or '/'}{('?' + u.query) if u.query else ''} HTTP/1.1", f"Host: {u.hostname}:{port}",
            "Upgrade: websocket", "Connection: Upgrade", f"Sec-WebSocket-Key: {key}", "Sec-WebSocket-Version: 13",
        ]
        if self.origin:
            headers.append(f"Origin: {self.origin}")
        if self.cookie:
            headers.append(f"Cookie: {self.cookie}")
        self._sock.sendall(("\r\n".join(headers) + "\r\n\r\n").encode())
        head = b""
        while b"\r\n\r\n" not in head:
            chunk = self._sock.recv(1)
            if not chunk:
                raise ConnectionError("gateway closed the connection during the WebSocket handshake")
            head += chunk
        status_line = head.split(b"\r\n", 1)[0].decode(errors="replace")
        if " 101 " not in status_line + " ":
            raise ConnectionError(f"WebSocket upgrade refused: {status_line}")
        self._sock.settimeout(None)
        threading.Thread(target=self._reader, daemon=True).start()

    def close(self) -> None:
        self._closed.set()
        try:
            if self._sock:
                self._send_frame(0x8, b"")
                self._sock.close()
        except OSError:
            pass

    # ---- framing --------------------------------------------------------
    def _send_frame(self, opcode: int, payload: bytes) -> None:
        header = bytes([0x80 | opcode])
        n = len(payload)
        if n < 126:
            header += bytes([0x80 | n])
        elif n < 65536:
            header += bytes([0x80 | 126]) + struct.pack(">H", n)
        else:
            header += bytes([0x80 | 127]) + struct.pack(">Q", n)
        mask = os.urandom(4)
        masked = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
        with self._lock:
            self._sock.sendall(header + mask + masked)

    def _read_exact(self, n: int) -> bytes:
        buf = b""
        while len(buf) < n:
            chunk = self._sock.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("connection closed")
            buf += chunk
        return buf

    def _reader(self) -> None:
        message = b""
        try:
            while not self._closed.is_set():
                b1, b2 = self._read_exact(2)
                fin, opcode, length = b1 & 0x80, b1 & 0x0F, b2 & 0x7F
                if length == 126:
                    length = struct.unpack(">H", self._read_exact(2))[0]
                elif length == 127:
                    length = struct.unpack(">Q", self._read_exact(8))[0]
                mask = self._read_exact(4) if b2 & 0x80 else b""
                payload = self._read_exact(length)
                if mask:
                    payload = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
                if opcode == 0x9:
                    self._send_frame(0xA, payload)
                elif opcode == 0x8:
                    break
                elif opcode in (0x0, 0x1, 0x2):
                    message += payload
                    if fin:
                        self._dispatch(message)
                        message = b""
        except (OSError, ConnectionError):
            pass
        finally:
            for q in list(self._pending.values()):
                q.put({"type": "error", "message": "CONNECTION_CLOSED: the gateway closed the WebSocket"})

    def _dispatch(self, raw: bytes) -> None:
        try:
            msg = json.loads(raw.decode())
        except ValueError:
            return
        q = self._pending.get(str(msg.get("id", "")))
        if q is not None and msg.get("type") in ("result", "error"):
            q.put(msg)
        else:
            self.pushes.append(msg)

    # ---- API ------------------------------------------------------------
    def call(self, channel: str, *args, timeout: float | None = None, retry_rate_limit: bool = True):
        """Invoke a channel. A REQUEST_RATE_LIMITED reply is retried with backoff (the limiter is a pacing device, not a failure)
        unless `retry_rate_limit=False`, which tests of the limiter itself use."""
        for attempt in range(10):
            try:
                return self._call_once(channel, args, timeout)
            except RpcError as exc:
                if not retry_rate_limit or not exc.has_code("REQUEST_RATE_LIMITED") or attempt == 9:
                    raise
                self.rate_limit_retries += 1
                time.sleep(retry_after_seconds(exc, 0.6 + 0.2 * attempt))

    def _call_once(self, channel: str, args, timeout: float | None):
        with self._lock:
            self._next_id += 1
            rid = str(self._next_id)
        q: queue.Queue = queue.Queue()
        self._pending[rid] = q
        try:
            self._send_frame(0x1, json.dumps({"id": rid, "type": "invoke", "channel": channel, "args": list(args)}).encode())
            try:
                msg = q.get(timeout=timeout or self.default_timeout)
            except queue.Empty:
                raise TimeoutError(f"no reply to {channel} within {timeout or self.default_timeout}s") from None
        finally:
            self._pending.pop(rid, None)
        if msg.get("type") == "error":
            raise RpcError(str(msg.get("message", "")))
        return msg.get("result")
