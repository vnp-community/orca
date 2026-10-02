"""initialize -> tools/list -> tools/call through the official MCP Python SDK."""
from __future__ import annotations

from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client

from conformance_support import Config, Result


async def run(cfg: Config) -> list[Result]:
    out: list[Result] = []
    tokens: list[str] = []

    async def on_token(tok: str) -> None:  # SDK resumption hook: called with each SSE event id
        tokens.append(tok)

    async with cfg.client() as http:
        async with streamable_http_client(cfg.mcp_url, http_client=http) as streams:
            async with ClientSession(streams[0], streams[1]) as session:
                init = await session.initialize()
                out.append(Result("initialize negotiates a protocol version and names the server",
                                  bool(init.protocol_version) and bool(init.server_info.name),
                                  f"protocol={init.protocol_version} server={init.server_info.name}"))
                out.append(Result("initialize declares the tools capability",
                                  init.capabilities.tools is not None))
                await session.send_ping()
                out.append(Result("ping", True))

                listed = await session.list_tools()
                names = [t.name for t in listed.tools]
                out.append(Result("tools/list returns tools with input schemas",
                                  len(names) > 0 and all(t.input_schema for t in listed.tools),
                                  f"{len(names)} tool(s)"))

                # Pick a tool that is safe by construction: the dev server's `echo`, else any
                # read-only tool (annotation readOnlyHint) of the real catalog.
                pick = "echo" if "echo" in names else next(
                    (t.name for t in listed.tools if t.annotations and t.annotations.read_only_hint), None)
                if pick is None:
                    out.append(Result("tools/call of a read-only tool", False, "no read-only tool in tools/list"))
                else:
                    args = {"text": "conformance"} if pick == "echo" else {}
                    res = await session.call_tool(pick, args)
                    out.append(Result(f"tools/call {pick} answers with content",
                                      bool(res.content) and not res.is_error,
                                      f"is_error={res.is_error}"))

                # A business failure must be a tool result, an unknown tool a protocol error.
                try:
                    bad = await session.call_tool("definitely_not_a_tool_xyz", {})
                    out.append(Result("tools/call of an unknown tool is rejected", bad.is_error,
                                      "returned a result with is_error=" + str(bad.is_error)))
                except Exception as e:  # MCPError: JSON-RPC error
                    out.append(Result("tools/call of an unknown tool is rejected", True, type(e).__name__))

                if "slow_progress" in names:
                    progress: list[float] = []

                    async def on_progress(p: float, total: float | None, msg: str | None) -> None:
                        progress.append(p)

                    # send_request + metadata is the SDK's documented resumption hook
                    from mcp import types
                    from mcp.shared.message import ClientMessageMetadata
                    req = types.CallToolRequest(params=types.CallToolRequestParams(name="slow_progress", arguments={"steps": 4}))
                    await session.send_request(req, types.CallToolResult, progress_callback=on_progress,
                                               metadata=ClientMessageMetadata(on_resumption_token_update=on_token))
                    out.append(Result("progress notifications reach the SDK client", len(progress) >= 2, f"{len(progress)} progress event(s)"))
                    out.append(Result("SSE events carry resumable ids the SDK can read", len(tokens) >= 2,
                                      f"{len(tokens)} event id(s)"))
    return out
