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
