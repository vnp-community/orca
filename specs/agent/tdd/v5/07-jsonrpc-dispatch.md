# TDD-AG-07: JSON-RPC Dispatch & MCP Protocol

**Document:** TDD-AG-07
**Version:** 2.0
**Date:** 2026-07-28
**Domain:** JSON-RPC 2.0 dispatch, MCP tools/list and tools/call handlers
**Source:** `src/relay/agent-rpc-dispatch.ts` — `createRpcDispatcher()`, route() in agent-rpc-dispatch.ts
**HLD Ref:** C3.8
**ADR:** ADR-005

---

## 1. JSON-RPC Method Router

```javascript
async function dispatchRpc(ws, rpc) {
  let response;
  try {
    switch (rpc.method) {
      case 'tools/list':
        response = await handleToolsList(rpc);
        break;
      case 'tools/call':
        response = await handleToolsCall(rpc);
        break;
      default:
        response = {
          jsonrpc: '2.0', id: rpc.id,
          error: { code: -32601, message: `Method not found: ${rpc.method}` },
        };
    }
  } catch (err) {
    response = {
      jsonrpc: '2.0', id: rpc.id,
      error: { code: -32603, message: `Internal error: ${err.message}` },
    };
  }

  ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify(response)));
}
```

---

## 2. tools/list Handler

```javascript
async function handleToolsList(rpc) {
  return {
    jsonrpc: '2.0',
    id: rpc.id,
    result: {
      tools: discoveredTools.map(t => ({
        name: t.name,
        description: t.description,
        inputSchema: t.inputSchema,
      })),
    },
  };
}
```

**MCP spec**: Response format compatible với `ModelContextProtocol tools/list` response.

---

## 3. tools/call Handler

```javascript
async function handleToolsCall(rpc) {
  const { name, arguments: params = {} } = rpc.params || {};

  const tool = discoveredTools.find(t => t.name === name);
  if (!tool) {
    return {
      jsonrpc: '2.0', id: rpc.id,
      error: { code: -32601, message: `Tool not found: ${name}` },
    };
  }

  try {
    const result = await tool.handler(params);

    // Format output theo MCP content array
    const text = [
      result.stdout || '',
      result.stderr ? `[stderr]\n${result.stderr}` : '',
    ].filter(Boolean).join('\n').trim();

    return {
      jsonrpc: '2.0', id: rpc.id,
      result: {
        content: [{ type: 'text', text: text || '(no output)' }],
        isError: (result.exitCode || 0) !== 0,
        exitCode: result.exitCode,
        meta: result.meta,
      },
    };
  } catch (err) {
    return {
      jsonrpc: '2.0', id: rpc.id,
      error: { code: -32603, message: err.message },
    };
  }
}
```

---

## 4. MCP Protocol Compliance

Agent tuân theo [Model Context Protocol](https://spec.modelcontextprotocol.io/) cho tools:

| MCP Method | Agent Support |
|------------|--------------|
| `tools/list` | ✅ Full |
| `tools/call` | ✅ Full |
| `resources/list` | ❌ Not implemented |
| `resources/read` | ❌ Not implemented |
| `prompts/list` | ❌ Not implemented |
| `sampling/createMessage` | ❌ Not implemented |

---

## 5. Error Codes

| Code | JSON-RPC Standard | Usage |
|------|------------------|-------|
| -32700 | Parse error | Malformed JSON in frame (log + ignore, not returned) |
| -32600 | Invalid Request | Currently unused |
| -32601 | Method not found | Unknown RPC method; Tool not found |
| -32603 | Internal error | Tool handler throws; uncaught exception |

---

## 6. v5.0 — Additional Methods (TDD-AG-09, TDD-AG-10, TDD-AG-11)

> See also §9 below for `fs.watch`/`fs.unwatch` and the `pty.*` methods, plus the `makeNotifier` push-notification mechanism added 2026-08-03.

```javascript
// Will be added to dispatchRpc():
case 'ai.provider.writeCredential':
  response = await handleAIWriteCredential(rpc);
  break;
case 'ai.provider.readCredential':
  response = await handleAIReadCredential(rpc);
  break;
case 'ai.provider.healthCheck':
  response = await handleAIHealthCheck(rpc);
  break;
case 'git.exec':
  response = await handleGitExec(rpc);
  break;
case 'git.execStream':
  // Streaming — sends multiple frames
  await handleGitExecStream(ws, rpc);
  return;  // No single response frame
case 'fs.readDir':
  response = await handleFsReadDir(rpc);
  break;
case 'fs.readFile':
  response = await handleFsReadFile(rpc);
  break;
case 'fs.grep':
  response = await handleFsGrep(rpc);
  break;
case 'preflight.check':
  response = await handlePreflightCheck(rpc);
  break;
```

---

## 7. Streaming Protocol (v5.0)

For `git.execStream` (push/pull streaming), agent gửi nhiều frames:

```javascript
// Frame 1..N: streaming data
ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
  jsonrpc: '2.0', id: rpc.id,
  result: { type: 'stream.chunk', content: line }
})));

// Final frame: stream end
ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
  jsonrpc: '2.0', id: rpc.id,
  result: { type: 'stream.end', exitCode: 0 }
})));
```

---

## 8. Test Coverage

```
tests/unit/
├── jsonrpc-dispatch.test.js
│   ├── tools/list → returns discoveredTools list
│   ├── tools/call: found tool → handler called with params
│   ├── tools/call: unknown tool → error -32601
│   ├── tools/call: handler throws → error -32603
│   ├── unknown method → error -32601 "Method not found"
│   └── response sent via ws.send(encodeFrame(...))
├── tools-list-format.test.js
│   ├── each tool has name, description, inputSchema
│   └── only discoveredTools (not all TOOL_DEFINITIONS) returned
└── tools-call-mcp-format.test.js
    ├── success: content=[{type:'text', text: stdout}]
    ├── isError=false when exitCode=0
    ├── isError=true when exitCode≠0
    └── stderr included in text with [stderr] prefix
```

**Target:** ≥ 20 tests

---

## 9. Addendum (2026-08-03) — Push Notifications, PTY Streaming, `fs.watch`

**Status: ✅ IMPLEMENTED** — `src/relay/agent-rpc-dispatch.ts`'s `route()`, `src/relay/pty-agent-bridge.ts`, `src/relay/fs-agent-extensions.ts`

This adds real-time server-push on top of the request/response surface above, using the one-way notification mechanism from TDD-AG-02 §5 (`makeNotifier`). `route()` builds one notifier per inbound RPC and passes it into the two handlers below that need to push.

### 9.1 `makeNotifier`

```typescript
// src/relay/agent-rpc-dispatch.ts
function makeNotifier(ws: WebSocket, state: WireState): (method: string, params: Record<string, unknown>) => void
```

### 9.2 PTY methods (`pty-agent-bridge.ts`)

The agent's PTY surface is six request/response methods, unchanged by this addendum, plus two new push notifications:

| Method | Direction | Params → Result |
|--------|-----------|------------------|
| `pty.create` | request/response | `{cols?, rows?, cwd?, env?, shellOverride?}` → `{id, cols, rows, cwd, shell}` |
| `pty.write` | request/response | `{id, data}` → ack |
| `pty.resize` | request/response | `{id, cols, rows}` → ack |
| `pty.destroy` | request/response | `{id, graceful?}` → ack |
| `pty.scrollback` | request/response | `{id, lines?}` → on-demand read of the last 500 buffered lines; now redundant with live streaming but kept for reconnect catch-up |
| `pty.sendSignal` | request/response | `{id, signal}` → ack; `signal` allowlisted to `SIGTERM`/`SIGKILL`/`SIGINT`/`SIGHUP`/`SIGTSTP` |
| `pty.data` | **notification** (new) | `{id, data}` — pushed on every `term.onData` chunk |
| `pty.exit` | **notification** (new) | `{id, exitCode, signal}` — pushed once from `term.onExit`; previously PTY exit was never reported except implicitly via the next failed op on that id |

`case 'pty.create'` in `route()` now resolves `handlePtyCreate(rpc.id, rpc.params ?? {}, log, makeNotifier(ws, state))` — the notifier is the 4th argument, after `log`. Inside the handler, `term.onData` both appends to the scrollback ring (unchanged) **and** calls `notify('pty.data', { id: ptyId, data })`.

Cleanup fix: `cleanupAgentPtys(log)` (kills all `AGENT_PTY_MAP` entries) was documented as required on session termination but was never wired anywhere. It is now called from `agent-session.ts`'s `stop()` — see TDD-AG-04 §8.

### 9.3 `fs.watch` / `fs.unwatch` (`fs-agent-extensions.ts`)

New cases in `route()`:

```typescript
case 'fs.watch': {
  const { handleFsWatch } = await import('./fs-agent-extensions')
  return (await handleFsWatch(rpc.id, rpc.params ?? {}, config, makeNotifier(ws, state))) as JsonRpcResponse
}
case 'fs.unwatch': {
  const { handleFsUnwatch } = await import('./fs-agent-extensions')
  return (await handleFsUnwatch(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
}
```

| Method | Params → Result |
|--------|------------------|
| `fs.watch` | `{path}` → `{ok: true, path}`; starts (or refcounts an existing) `fs.watch()` on `path`, pushing `fs.changed` notifications |
| `fs.unwatch` | `{path}` → `{ok: true}`; decrements the refcount, closes the underlying watcher at zero |
| `fs.changed` | **notification** — `{path, eventType, filename}` pushed per native `fs.watch` callback |

Refcounting (`AGENT_WATCH_MAP: Map<absPath, {watcher, refCount}>`) matters because multiple Orca-side user processes can share one physical Dev Server and independently watch the same path — one caller's `fs.unwatch` must not tear down another's subscription.

**Known v1 gap:** Node's `fs.watch(path, { recursive: true })` is only honored on macOS and Windows. On Linux (all 3 current Dev Servers) it silently watches the top-level directory only — deeper changes aren't pushed. This is a completeness gap, not a correctness bug: `DevServerFilesystemProvider.watch()` (main-process side) falls back to Phase-1 polling for anything push can't cover.

Cleanup fix (mirrors §9.2): `cleanupAgentWatches()` closes all active watchers and is now wired into `agent-session.ts`'s `stop()`, so a dropped WebSocket doesn't leak watchers.
