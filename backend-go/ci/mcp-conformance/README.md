# MCP conformance (BE-MCP-SOL-015)

Three tiers, from most to least trusted:

| Tier | What | Blocks a PR? | How |
|---|---|---|---|
| 1 | Go, in-process: real `/mcp` handler, official Go SDK client, raw HTTP for negatives. Initialize, version negotiation, ping, tools, resources, prompts, resume across two replicas, cancel, auth/origin/cookie negatives, kill switch, metrics and trace chain, rollout flags, `FuzzJSONRPCDecode` | yes | `./run-go-conformance.sh` |
| 1b | Python reference client with the **official `mcp` SDK** (an implementation independent of the Go server SDK): initialize, tools/list, tools/call, progress, resumability, OAuth discovery (RFC 9728 / RFC 8414) | **no (non-blocking) until it has run green on a real dev stack in CI** | `python run_reference_client.py` against a running stack, or `./run-python-tier-local.sh` |
| 2 | MCP Inspector CLI / official conformance suite | - | **not implemented** (existence of a headless mode unverified) |

## Python tier

```sh
pip install -r requirements.txt            # mcp==2.2.0, httpx2==2.13.1
MCP_CONF_BASE_URL=http://localhost:8081 \  # public gateway origin, no trailing slash
MCP_CONF_TOKEN=<dev PAT> \                 # created via /v1/auth/mcp-tokens
python run_reference_client.py
```

`MCP_CONF_PATH` (default `/mcp`) overrides the endpoint path. Exit code 0 only if
every check passes; a step that cannot apply to the target (the streaming tool
`slow_progress` exists only in the dev server) is reported as SKIPPED, not hidden.

### SDK API facts (verified by running against mcp 2.2.0)

- The SDK is 2.x: the transport is `mcp.client.streamable_http.streamable_http_client(url, *, http_client=, terminate_on_close=)`
  (the 1.x name `streamablehttp_client` and its `headers=` argument are gone). It yields a tuple whose first two
  items are the read/write streams for `ClientSession`; the HTTP client is `httpx2`
  (bearer and other headers go on the `httpx2.AsyncClient`).
- `ClientSession.initialize()`, `list_tools()`, `call_tool()`, `send_ping()` and
  `send_request(request, result_type, progress_callback=, metadata=ClientMessageMetadata(on_resumption_token_update=))` exist and were run.
- Protocol version negotiated with our server: `2025-06-18` (the only one `mcpserver.SupportedProtocolVersions` lists).
- The client logs `Unknown SSE event: prime` at start: the Go SDK sends a priming SSE event; harmless.

### What is NOT done through the SDK, and why

- **Cut + resume** uses plain `httpx2`: the SDK's resumption (`ClientMessageMetadata.resumption_token`) re-issues a
  request with a new JSON-RPC id, so it cannot be mapped to the original request's replayed response, and cancelling an
  SDK call sends `notifications/cancelled`, which would stop the tool. The SDK part of the step only proves that event ids are exposed.
- **OAuth discovery** is checked with plain `httpx2` GETs. The SDK's own OAuth client (`mcp.client.auth`) and the
  DCR/authorize/token flow are **not exercised** here; the full flow is covered by Go tests in auth-service and api-gateway.

## What has actually been run

Tier 1 and tier 1b were run on 2026-10-02 against the test-only `cmd/mcpconformance-devserver` (real `/mcp`
handler, static token, two stub tools), **not** against the compose dev stack (auth-service, mcp-service, NATS,
Postgres), because that stack was not started. The CI workflow runs tier 1 always; tier 1b runs against the same
dev server and is `continue-on-error`.
