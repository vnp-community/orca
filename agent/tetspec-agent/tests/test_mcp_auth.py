"""HTTP transport authentication: token verification, the ASGI gate and an end-to-end streamable HTTP session."""
from __future__ import annotations

import json
import threading
import time

import httpx2
import pytest
import uvicorn
from mcp import Client
from mcp.client.streamable_http import streamable_http_client

from test_mcp_support import READ_TOOLS, anyio_backend, events, make_runtime  # noqa: F401
from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError
from tdd_agent.mcp import BearerAuthMiddleware, Identity, TokenAuth, build_http_app, serve

pytestmark = pytest.mark.anyio

TOKEN = "s3cr3t-token-value-0123456789"
WRONG = "not-the-token-but-also-long-0123456789"


class Downstream:
    def __init__(self):
        self.scopes = []

    async def __call__(self, scope, receive, send):
        self.scopes.append(scope["type"])
        if scope["type"] == "http":
            await send({"type": "http.response.start", "status": 200, "headers": []})
            await send({"type": "http.response.body", "body": b"ok"})


async def asgi(app, headers=None, *, scope_type="http", method="POST", path="/mcp") -> list[dict]:
    sent: list[dict] = []

    async def receive():
        return {"type": "http.request", "body": b"", "more_body": False}

    async def send(message):
        sent.append(message)

    scope = {"type": scope_type, "method": method, "path": path, "client": ("10.1.2.3", 4242),
             "headers": [(k.lower().encode(), v.encode()) for k, v in (headers or {}).items()]}
    await app(scope, receive, send)
    return sent


# ====================================================================== token verification
@pytest.mark.parametrize("header,reason", [(None, "missing"), ("", "missing"), ("   ", "missing"), ("Basic abc", "malformed"), ("Bearer", "malformed"),
                                           ("Bearer   ", "malformed"), (f"Token {TOKEN}", "malformed"), (f"Bearer {WRONG}", "invalid"),
                                           (f"Bearer {TOKEN[:-1]}", "invalid"), (f"Bearer {TOKEN}x", "invalid")])
def test_verify_rejects_missing_malformed_and_wrong_tokens(header, reason):
    identity, got = TokenAuth(TOKEN, "claude").verify(header)
    assert identity is None and got == reason


@pytest.mark.parametrize("header", [f"Bearer {TOKEN}", f"bearer {TOKEN}", f"BEARER   {TOKEN}  "])
def test_verify_accepts_the_token_and_maps_it_to_the_configured_actor(header):
    identity, reason = TokenAuth(TOKEN, "claude").verify(header)
    assert identity == Identity("claude", "http") and reason == "ok"
    assert TokenAuth(TOKEN).verify(header)[0] == Identity("")


def test_token_comes_from_the_configured_environment_variable():
    config = load_config(overrides={"mcp": {"token_env": "MY_MCP_TOKEN", "client_actor": "bob"}}, env={})
    auth = TokenAuth.from_config(config, {"MY_MCP_TOKEN": TOKEN})
    assert auth.actor == "bob" and auth.verify(f"Bearer {TOKEN}")[0] is not None
    for env in ({}, {"MY_MCP_TOKEN": ""}, {"TDD_AGENT_MCP_TOKEN": TOKEN}):
        with pytest.raises(ConfigError) as exc:
            TokenAuth.from_config(config, env)
        assert exc.value.code == "MCP_TOKEN_MISSING" and TOKEN not in str(exc.value)
    with pytest.raises(ConfigError):
        TokenAuth("")


def test_serving_over_http_without_a_token_or_with_an_unknown_transport_fails_before_binding(tmp_path, monkeypatch):
    rt = make_runtime(tmp_path)
    monkeypatch.delenv("TDD_AGENT_MCP_TOKEN", raising=False)
    with pytest.raises(ConfigError) as exc:
        serve(rt, "http")
    assert exc.value.code == "MCP_TOKEN_MISSING"
    with pytest.raises(ConfigError):
        serve(rt, "carrier-pigeon")
    with pytest.raises(ConfigError):
        build_http_app(rt, env={})


# ====================================================================== ASGI gate
async def test_requests_without_a_valid_token_are_rejected_logged_and_counted(tmp_path, events):
    rt = make_runtime(tmp_path)
    inner = Downstream()
    app = BearerAuthMiddleware(inner, TokenAuth(TOKEN, "claude"), rt.observer)
    for headers, reason in (({}, "missing"), ({"Authorization": f"Bearer {WRONG}"}, "invalid"), ({"Authorization": "Basic Zm9v"}, "malformed")):
        sent = await asgi(app, headers)
        start, body = sent
        assert start["status"] == 401 and dict(start["headers"])[b"www-authenticate"] == b'Bearer realm="tdd-agent"'
        payload = json.loads(body["body"])
        assert payload["error"]["code"] == "UNAUTHORIZED" and payload["error"]["details"] == {"reason": reason}
    assert inner.scopes == []
    rejected = events.named("mcp_auth_rejected")
    assert [e["reason"] for e in rejected] == ["missing", "invalid", "malformed"]
    assert all(e["level"] == "WARNING" and e["path"] == "/mcp" and e["method"] == "POST" and e["client"] == "10.1.2.3" for e in rejected)
    counters = {m["labels"]["reason"]: m["value"] for m in rt.operational.list_metrics("mcp_auth_rejections_total")}
    assert counters == {"missing": 1.0, "invalid": 1.0, "malformed": 1.0}


async def test_a_valid_token_reaches_the_app_and_lifespan_events_pass_through(tmp_path, events):
    rt = make_runtime(tmp_path)
    inner = Downstream()
    app = BearerAuthMiddleware(inner, TokenAuth(TOKEN), rt.observer)
    sent = await asgi(app, {"Authorization": f"Bearer {TOKEN}"})
    assert [m.get("status") for m in sent if m["type"] == "http.response.start"] == [200]
    await asgi(app, scope_type="lifespan")
    assert inner.scopes == ["http", "lifespan"] and events.named("mcp_auth_rejected") == []


async def test_neither_the_valid_nor_the_offered_token_is_ever_logged_or_stored(tmp_path, events):
    rt = make_runtime(tmp_path)
    app = BearerAuthMiddleware(Downstream(), TokenAuth(TOKEN), rt.observer)
    responses = []
    for header in (f"Bearer {WRONG}", f"Bearer {TOKEN}", f"Bearer {TOKEN}zz", "Bearer"):
        responses += await asgi(app, {"Authorization": header})
    blob = json.dumps(events.items, default=str) + json.dumps(rt.operational.list_metrics(), default=str) + repr(responses)
    assert TOKEN not in blob and WRONG not in blob and "Authorization" not in blob


# ====================================================================== end to end over streamable HTTP
@pytest.fixture
def http_endpoint(tmp_path):
    rt = make_runtime(tmp_path, approvals=True, client_actor="claude")
    app = build_http_app(rt, env={"TDD_AGENT_MCP_TOKEN": TOKEN})
    server = uvicorn.Server(uvicorn.Config(app, host="127.0.0.1", port=0, log_level="warning"))
    thread = threading.Thread(target=server.run, daemon=True)
    thread.start()
    deadline = time.monotonic() + 15
    while not server.started and time.monotonic() < deadline:
        time.sleep(0.02)
    assert server.started, "uvicorn did not start"
    port = server.servers[0].sockets[0].getsockname()[1]
    yield rt, f"http://127.0.0.1:{port}/mcp"
    server.should_exit = True
    thread.join(15)


async def test_http_session_needs_the_token_and_acts_as_the_configured_client(http_endpoint, events):
    rt, url = http_endpoint
    async with httpx2.AsyncClient() as bare:
        denied = await bare.post(url, json={"jsonrpc": "2.0", "id": 1, "method": "ping"})
        assert denied.status_code == 401 and denied.json()["error"]["code"] == "UNAUTHORIZED"
        wrong = await bare.post(url, headers={"Authorization": f"Bearer {WRONG}"}, json={"jsonrpc": "2.0", "id": 1, "method": "ping"})
        assert wrong.status_code == 401
    assert [e["reason"] for e in events.named("mcp_auth_rejected")] == ["missing", "invalid"]
    assert [s for s in rt.operational.list_spans() if s["kind"] == "tool"] == []

    async with httpx2.AsyncClient(headers={"Authorization": f"Bearer {TOKEN}"}) as authed:
        async with Client(streamable_http_client(url, http_client=authed)) as client:
            names = [t.name for t in (await client.list_tools()).tools]
            assert names == READ_TOOLS + ["approval_submit"], "the token maps to the allowlisted client, so both keys are present"
            result = await client.call_tool("kg_snapshots", {})
            assert not result.is_error and result.structured_content["snapshots"][0]["id"] == "snap-000001"
    spans = [s for s in rt.operational.list_spans() if s["kind"] == "tool"]
    assert [s["name"] for s in spans] == ["mcp:kg_snapshots"]
    assert spans[0]["attributes"]["actor"] == "claude" and spans[0]["attributes"]["transport"] == "http"
    blob = json.dumps(events.items, default=str) + json.dumps(rt.operational.list_spans(), default=str)
    assert TOKEN not in blob and WRONG not in blob
