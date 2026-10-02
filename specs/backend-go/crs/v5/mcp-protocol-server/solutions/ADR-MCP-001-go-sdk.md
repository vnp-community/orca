# ADR-MCP-001: Official Go SDK for the `/mcp` endpoint — version, hooks verified, gaps

> Status: accepted (2026-10-02) · Implements the D4 decision of the v5 README (official Go SDK; not reopened here).
> Code: `backend-go/services/api-gateway/internal/adapter/mcpserver` · Solutions: BE-MCP-SOL-002, BE-MCP-SOL-003.
> Supersedes the placeholder name `docs/adrs/v2/ADR-021-mcp-go-sdk-and-transport.md` used in SOL-003.

## Decision

`github.com/modelcontextprotocol/go-sdk` **v1.8.0** (latest at the time of writing, `go 1.25.0`, builds in the repo's Go 1.26 workspace). Used for JSON-RPC, the `initialize` handshake, version negotiation, `ping`, cancellation and the Streamable HTTP transport (`mcp.NewStreamableHTTPHandler`). The adapter only adds what must come from Orca: auth, Origin/body/rate limits, `ToolCatalog`/`ToolExecutor` ports, signed cursors, level validation.

The SDK implements spec revisions up to `2026-07-28` (a new sessionless protocol). Orca is narrowed to **`2025-06-18` only** through `ServerOptions.SupportedProtocolVersions` fed from `mcpserver.SupportedProtocolVersions` (the single place for version strings). Clients asking for anything else get `2025-06-18` back in `initialize`; an unsupported `MCP-Protocol-Version` header is a `400`.

## Spike results: the three hooks SOL-003 required

| Hook | SDK v1.8.0 | Status |
|---|---|---|
| (a) external `SessionStore` | **Missing.** `StreamableHTTPHandler` keeps `map[sessionID]*sessionInfo` in process memory (source comment: "Should we allow passing in a session store?"). `ServerOptions.GetSessionID` lets us choose ids; `SessionTimeout` gives in-memory idle expiry. | **Gap for BE-004.** Options: sticky routing per `Mcp-Session-Id` (cheap), or our own handler around the exported `mcp.StreamableServerTransport` with a durable store. The ports in `mcpserver.Deps` (`GetSessionID`, `EventStore`) are the seam. |
| (b) `EventStore` / SSE ids / `Last-Event-ID` | **Present.** `StreamableHTTPOptions.EventStore` (interface `mcp.EventStore`); the transport reads `Last-Event-ID` and replays. | Wired as `Deps.EventStore` (nil today = no resume). Not exercised until BE-004. |
| (c) per-request identity before the handler | **Partly.** The only way to put `auth.TokenInfo` into the context is `auth.RequireBearerToken` (no exported `ContextWithTokenInfo`). The SDK binds a session to `TokenInfo.UserID` and answers `403 session user mismatch` otherwise. `RequestExtra.TokenInfo` reaches method handlers. | Our `authenticate` middleware verifies first; `RequireBearerToken` then re-uses the verified `Principal` (tenant-qualified `UserID` = `tenant/user`, `Principal` carried in `TokenInfo.Extra`). A response wrapper rewrites the SDK `403` into `404` so a foreign session is indistinguishable from an unknown one. |

## Other gaps found (all patched in the adapter, tested in `conformance_test.go`)

1. **"Initialized" means `initialize` received, not `notifications/initialized`.** The SDK accepts `tools/list` between the two. Patched with a receiving middleware + `readySessions` (per-replica, same lifetime as the SDK session); error is JSON-RPC `-32600 "server not initialized"` (`ping`, `initialize`, `notifications/*` exempt).
2. **Session-less non-`initialize` POST** is answered by the SDK with error code `0` after creating a throwaway session. Patched with `requireInitializeFirst` (HTTP 400 + `-32600`).
3. **`logging/setLevel` accepts any string.** Patched: RFC 5424 levels only, else `-32602`.
4. **Tools are static, not per-request.** The SDK lists tools registered on the `Server` and paginates with its own non-signed cursor. We intercept `tools/list`/`tools/call` in a receiving middleware so the catalog is per principal and cursors are HMAC-signed, session-bound, 10-minute (`pagination_cursor.go`). Side effect: `ListToolsResult` serializes SDK cache fields (`ttlMs:0`, `cacheScope:""`) that 2025-06-18 clients ignore. Revisit with `ServerOptions.SetCacheable` if a strict client objects.
5. **Capabilities are inferred when tools exist.** We set `Capabilities` explicitly: `tools` (no `listChanged`) and `logging` only. BE-007 flips `listChanged`; BE-010/011 add `resources`/`prompts`.
6. **Body limit** is SDK-native (`MaxRequestBodyBytes`, plain-text 413). We additionally pre-check `Content-Length` to return the JSON-RPC `-32600` body specified by SOL-002.
7. **Accept without `text/event-stream`** is `400` in the SDK; SOL-003 wrote `406` ("from memory", never verified against the spec). Kept the SDK behavior.
8. **GET and DELETE work today** (standalone SSE stream, session termination) although SOL-003 said `405` until BE-004. They are subject to the same auth/Origin checks and a per-replica stream cap (`MCP_MAX_SSE_STREAMS_PER_USER/_TENANT`). No resumability, no cross-replica delivery, no heartbeat guarantees yet.
9. The SDK client (`mcp.StreamableClientTransport`) is used as the in-process test client. The Python reference client of BE-015 is **not** part of this change.

## Consequences

- One SDK dependency in `api-gateway/go.mod` (+ `jsonschema-go`, `segmentio/encoding`, `uritemplate`, `oauth2` indirect).
- Upgrading the SDK = re-run `mcpserver` conformance tests; the gaps above are the checklist of behavior we depend on being unchanged.
- `MCPGODEBUG` compatibility flags (`allowsessionsinstateless`, `nowrapinvalidparams`, ...) exist in this SDK and are scheduled for removal in 1.9.0; we use none.
