"""OAuth discovery: RFC 9728 protected-resource metadata and RFC 8414 AS metadata."""
from __future__ import annotations

import httpx2

from conformance_support import Config, Result


async def run(cfg: Config) -> list[Result]:
    out: list[Result] = []
    async with httpx2.AsyncClient(timeout=15) as anon:  # discovery documents are public: no token
        ch = await anon.post(cfg.mcp_url, json={"jsonrpc": "2.0", "id": 1, "method": "ping"})
        www = ch.headers.get("www-authenticate", "")
        out.append(Result("unauthenticated /mcp answers 401 with a Bearer resource_metadata challenge",
                          ch.status_code == 401 and www.startswith("Bearer") and "resource_metadata=" in www,
                          f"status={ch.status_code}", via="raw"))

        prm_url = cfg.base_url + "/.well-known/oauth-protected-resource"
        prm = await anon.get(prm_url)
        if prm.status_code != 200:
            return out + [Result("protected-resource metadata is public", False, f"status={prm.status_code}", via="raw")]
        doc = prm.json()
        out.append(Result("protected-resource metadata (RFC 9728) names the resource and its authorization server",
                          doc.get("resource") == cfg.mcp_url and bool(doc.get("authorization_servers")),
                          f"resource={doc.get('resource')}", via="raw"))
        out.append(Result("protected-resource metadata also served at the path-suffixed URL",
                          (await anon.get(prm_url + cfg.mcp_path)).status_code == 200, via="raw"))
        out.append(Result("bearer_methods_supported is header only", doc.get("bearer_methods_supported") == ["header"], via="raw"))

        issuer = doc["authorization_servers"][0].rstrip("/")
        asm = await anon.get(issuer + "/.well-known/oauth-authorization-server")
        if asm.status_code != 200:
            return out + [Result("authorization-server metadata is public", False, f"status={asm.status_code} at {issuer}", via="raw")]
        a = asm.json()
        out.append(Result("authorization-server metadata (RFC 8414) issuer matches", a.get("issuer", "").rstrip("/") == issuer, via="raw"))
        out.append(Result("endpoints present: authorize, token, revoke",
                          all(a.get(k) for k in ("authorization_endpoint", "token_endpoint", "revocation_endpoint")), via="raw"))
        out.append(Result("PKCE S256 is the only challenge method", a.get("code_challenge_methods_supported") == ["S256"], via="raw"))
        out.append(Result("public clients only (token_endpoint_auth_methods_supported == [none])",
                          a.get("token_endpoint_auth_methods_supported") == ["none"], via="raw"))
        out.append(Result("scopes advertised by the resource are advertised by the AS",
                          set(doc.get("scopes_supported", [])) <= set(a.get("scopes_supported", [])), via="raw"))
    return out
