"""Bearer-token authentication for the HTTP transport.

The token comes from the environment variable named by ``[mcp] token_env`` and is
mapped to the single ``[mcp] client_actor`` identity.  A caller can never pick
its own actor.  Neither the token nor the ``Authorization`` header is ever
logged; rejections are logged with the reason only.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import logging
import os
from dataclasses import dataclass
from typing import Any, Awaitable, Callable, Mapping

from ..config import Config
from ..errors import ConfigError
from ..obs.observer import Observer

Scope = dict[str, Any]
Receive = Callable[[], Awaitable[dict[str, Any]]]
Send = Callable[[dict[str, Any]], Awaitable[None]]
ASGIApp = Callable[[Scope, Receive, Send], Awaitable[None]]

REASONS = ("missing", "malformed", "invalid")


@dataclass(frozen=True)
class Identity:
    """Who the authenticated client is.  ``actor`` is empty when no ``client_actor`` is configured."""
    actor: str
    channel: str = "http"


class TokenAuth:
    def __init__(self, token: str, actor: str = ""):
        if not token:
            raise ConfigError("an empty MCP bearer token is not allowed", code="MCP_TOKEN_MISSING")
        self._digest = hashlib.sha256(token.encode("utf-8")).digest()
        self.actor = actor

    @classmethod
    def from_config(cls, config: Config, env: Mapping[str, str] | None = None) -> "TokenAuth":
        name = str(config.get("mcp.token_env") or "")
        token = (os.environ if env is None else env).get(name, "") if name else ""
        if not token:
            raise ConfigError(f"the HTTP transport requires a bearer token in the environment variable {name or '[mcp] token_env'}",
                              code="MCP_TOKEN_MISSING", details={"token_env": name})
        return cls(token, str(config.get("mcp.client_actor") or ""))

    def verify(self, authorization: str | None) -> tuple[Identity | None, str]:
        """Return ``(identity, "ok")`` or ``(None, reason)`` with reason in ``REASONS``."""
        if not authorization or not authorization.strip():
            return None, "missing"
        parts = authorization.strip().split(None, 1)
        if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1].strip():
            return None, "malformed"
        candidate = hashlib.sha256(parts[1].strip().encode("utf-8")).digest()
        if not hmac.compare_digest(candidate, self._digest):
            return None, "invalid"
        return Identity(self.actor), "ok"


def _header(scope: Scope, name: bytes) -> str | None:
    for key, value in scope.get("headers") or ():
        if key.lower() == name:
            return value.decode("latin-1")
    return None


class BearerAuthMiddleware:
    """ASGI middleware: every HTTP request needs a valid bearer token, otherwise 401."""

    def __init__(self, app: ASGIApp, auth: TokenAuth, observer: Observer):
        self.app, self.auth, self.observer = app, auth, observer

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return
        identity, reason = self.auth.verify(_header(scope, b"authorization"))
        if identity is not None:
            await self.app(scope, receive, send)
            return
        client = scope.get("client")
        self.observer.event("mcp_auth_rejected", logging.WARNING, reason=reason, method=scope.get("method"), path=scope.get("path"),
                            client=client[0] if client else None)
        self.observer.metrics.store.incr_metric("mcp_auth_rejections_total", {"reason": reason})
        body = json.dumps({"error": {"code": "UNAUTHORIZED", "message": "a valid bearer token is required", "details": {"reason": reason}}}).encode()
        await send({"type": "http.response.start", "status": 401,
                    "headers": [(b"content-type", b"application/json"), (b"content-length", str(len(body)).encode()),
                                (b"www-authenticate", b'Bearer realm="tdd-agent"')]})
        await send({"type": "http.response.body", "body": body})
