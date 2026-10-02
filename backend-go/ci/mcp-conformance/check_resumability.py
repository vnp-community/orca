"""Streamable HTTP resumability: cut a streaming tools/call, resume with Last-Event-ID.

Uses plain httpx2 for the cut and the resume. Reason (verified against mcp 2.2.0):
the SDK offers ClientMessageMetadata.resumption_token, but a resumed request is a
NEW JSON-RPC request id, so the SDK cannot map the replayed response of the ORIGINAL
id; and cancelling the SDK call sends notifications/cancelled, which would stop the
tool. A dropped connection (no cancel) is what a real network cut looks like.
"""
from __future__ import annotations

import json

from conformance_support import Config, Result

ACCEPT = "application/json, text/event-stream"
PROTO = "2025-06-18"


def parse_sse_line(line: str, cur: dict) -> dict | None:
    """Accumulate one SSE line; return the event when a blank line completes it."""
    if line == "":
        if cur:
            ev = dict(cur)
            cur.clear()
            return ev
        return None
    if ":" in line:
        k, _, v = line.partition(":")
        cur[k] = v[1:] if v.startswith(" ") else v
    return None


async def read_events(resp, limit: int | None) -> list[dict]:
    events: list[dict] = []
    cur: dict = {}
    async for line in resp.aiter_lines():
        ev = parse_sse_line(line, cur)
        if ev and ("data" in ev):
            events.append(ev)
            if limit is not None and len(events) >= limit:
                break
    return events


async def run(cfg: Config) -> list[Result]:
    out: list[Result] = []
    async with cfg.client(headers={"Accept": ACCEPT, "Content-Type": "application/json"}) as http:
        r = await http.post(cfg.mcp_url, json={"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
            "protocolVersion": PROTO, "capabilities": {}, "clientInfo": {"name": "ref-resume", "version": "1"}}})
        sid = r.headers.get("mcp-session-id")
        if r.status_code != 200 or not sid:
            return [Result("resumability: open a session", False, f"status={r.status_code}", via="raw")]
        sess = {"Mcp-Session-Id": sid, "MCP-Protocol-Version": PROTO}
        await http.post(cfg.mcp_url, headers=sess, json={"jsonrpc": "2.0", "method": "notifications/initialized"})
        tools = await http.post(cfg.mcp_url, headers=sess, json={"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
        if "slow_progress" not in tools.text:
            return [Result("resumability: streaming tool available", True, via="raw",
                           detail="SKIPPED: the target has no `slow_progress` tool (only the dev server provides it)")]

        call = {"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": {
            "name": "slow_progress", "arguments": {"steps": 10}, "_meta": {"progressToken": "p1"}}}
        async with http.stream("POST", cfg.mcp_url, headers=sess, json=call) as resp:
            first = await read_events(resp, 4)
        # leaving the `with` closes the connection without any cancellation message
        ids = [e["id"] for e in first if "id" in e]
        out.append(Result("tools/call streams SSE events with ids", len(ids) >= 3, f"{len(ids)} id(s) in first leg", via="raw"))
        if not ids:
            return out

        async with http.stream("GET", cfg.mcp_url, headers={**sess, "Last-Event-ID": ids[-1]}) as resp:
            ok_status = resp.status_code == 200
            rest = await read_events(resp, None) if ok_status else []
        final = [e for e in rest if '"id":7' in e.get("data", "").replace(" ", "")]
        out.append(Result("GET with Last-Event-ID resumes the cut stream", ok_status, f"status={resp.status_code}", via="raw"))
        out.append(Result("resumed stream ends with the response to the original request", len(final) == 1,
                          f"{len(rest)} event(s) replayed", via="raw"))
        replay_ids = [e["id"] for e in rest if "id" in e]
        out.append(Result("replay contains no event from before the cut", not (set(replay_ids) & set(ids)), via="raw"))
        if final:
            try:
                body = json.loads(final[0]["data"])
                out.append(Result("replayed response is a successful JSON-RPC result", "result" in body, via="raw"))
            except ValueError:
                out.append(Result("replayed response is valid JSON", False, via="raw"))

        gap = await http.get(cfg.mcp_url, headers={**sess, "Last-Event-ID": "not-an-event-id"})
        out.append(Result("a Last-Event-ID that was never issued is a clean client error (not 5xx)",
                          gap.status_code < 500, f"status={gap.status_code}", via="raw"))
        await http.delete(cfg.mcp_url, headers=sess)
    return out
