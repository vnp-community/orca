"""Khám phá OAuth: /.well-known/oauth-protected-resource (RFC 9728) và authorization-server.

Spec: backend-go/services/api-gateway/internal/adapter/mcpserver/{handler,metadata}.go.
"""
from __future__ import annotations

import requests

from mcp_check_framework import Context, run_single

SUITE = "discovery"


def _get(ctx: Context, path: str) -> requests.Response:
    try:
        return requests.get(ctx.cfg.base_url + path, timeout=ctx.cfg.timeout, verify=ctx.cfg.verify_tls,
                            allow_redirects=False, headers={"Accept": "application/json"})
    except requests.RequestException as exc:
        fake = requests.Response()
        fake.status_code = 599
        fake._content = str(exc).encode()
        return fake


def run(ctx: Context) -> None:
    if not ctx.need_mcp():
        return
    for path in ("/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"):
        resp = _get(ctx, path)
        ctx.check(f"GET {path} -> 200", resp.status_code == 200, f"HTTP {resp.status_code}")
        if resp.status_code != 200:
            continue
        md = resp.json()
        ctx.check(f"{path}: resource trỏ tới /mcp", str(md.get("resource", "")).rstrip("/").endswith("/mcp"),
                  f"resource={md.get('resource')!r}")
        ctx.check(f"{path}: có authorization_servers", bool(md.get("authorization_servers")), str(md)[:160])
        ctx.check(f"{path}: bearer_methods_supported có 'header'",
                  "header" in (md.get("bearer_methods_supported") or []), str(md.get("bearer_methods_supported")))
        scopes = md.get("scopes_supported") or []
        ctx.check(f"{path}: scopes_supported chứa orca:read", "orca:read" in scopes, f"scopes={scopes}")

    resp = _get(ctx, "/.well-known/oauth-authorization-server")
    if resp.status_code == 404:
        ctx.skip("authorization-server metadata", "gateway không phục vụ (auth-service là AS riêng)")
    else:
        ctx.check("GET /.well-known/oauth-authorization-server -> 200", resp.status_code == 200, f"HTTP {resp.status_code}")
        if resp.status_code == 200:
            md = resp.json()
            for key in ("issuer", "authorization_endpoint", "token_endpoint"):
                ctx.check(f"authorization-server có {key}", bool(md.get(key)), str(md)[:160])
            methods = md.get("code_challenge_methods_supported")
            if methods is not None:
                ctx.check("PKCE: hỗ trợ S256", "S256" in methods, f"methods={methods}")


if __name__ == "__main__":
    raise SystemExit(run_single(SUITE, run))
