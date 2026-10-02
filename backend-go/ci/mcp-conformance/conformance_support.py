"""Shared config and result type for the MCP reference-client checks."""
from __future__ import annotations

import os
import sys
from dataclasses import dataclass, field

import httpx2


@dataclass
class Config:
    base_url: str  # public base URL, e.g. http://localhost:8081 (no trailing slash)
    token: str  # bearer token (a dev PAT, or the dev server's static token)
    mcp_path: str = "/mcp"

    @property
    def mcp_url(self) -> str:
        return self.base_url + self.mcp_path

    def client(self, **kw) -> httpx2.AsyncClient:
        headers = {"Authorization": f"Bearer {self.token}"}
        headers.update(kw.pop("headers", {}))
        return httpx2.AsyncClient(headers=headers, timeout=kw.pop("timeout", 30), **kw)

    @staticmethod
    def from_env() -> "Config":
        base = os.environ.get("MCP_CONF_BASE_URL", "").rstrip("/")
        token = os.environ.get("MCP_CONF_TOKEN", "")
        if not base or not token:
            sys.exit("MCP_CONF_BASE_URL and MCP_CONF_TOKEN are required (see README.md)")
        return Config(base_url=base, token=token, mcp_path=os.environ.get("MCP_CONF_PATH", "/mcp"))


@dataclass
class Result:
    name: str
    ok: bool
    detail: str = ""
    # "sdk" = exercised through the official SDK API; "raw" = plain HTTP because the
    # SDK has no high-level API for the step (see README, SDK assumptions).
    via: str = "sdk"
    notes: list[str] = field(default_factory=list)

    def line(self) -> str:
        return f"[{'PASS' if self.ok else 'FAIL'}] ({self.via}) {self.name}" + (f" - {self.detail}" if self.detail else "")
