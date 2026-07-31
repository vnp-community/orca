# TDD-AG-06: Tool Handlers — Extend agent-exec-handler.ts (v2.1)

**Document:** TDD-AG-06
**Version:** 2.1
**Date:** 2026-07-28
**Source files:**
- `src/relay/agent-exec-handler.ts` ← [REUSE existing — already in codebase!]
- `src/relay/agent-tool-registry.ts` ← handlers use runCommandCapture() from above
**HLD Ref:** C3.8

---

## 1. Reuse Existing agent-exec-handler.ts

`src/relay/agent-exec-handler.ts` **đã tồn tại** trong codebase với:
- `runCommandCapture()` → spawn with shell=false, timeout SIGTERM
- Windows-safe spawn logic
- Binary resolution from PATH

**Agent KHÔNG cần viết lại** — chỉ import và dùng.

Signature extension cần thêm `env` parameter:

```typescript
// src/relay/agent-exec-handler.ts (EXTEND, không phải rewrite)

// Existing type (approximate):
export interface CommandResult {
  stdout: string
  stderr: string
  exitCode: number | null
}

// Add env support to existing runCommandCapture():
export async function runCommandCapture(
  binary: string,
  args: string[],
  options: {
    cwd?: string
    timeout?: number
    env?: NodeJS.ProcessEnv  // ← ADD này nếu chưa có
  } = {}
): Promise<CommandResult> {
  // existing implementation...
}
```

---

## 2. MCP Result Formatting

Handlers trong `agent-tool-registry.ts` trả về `ToolResult`.  
`agent-rpc-dispatch.ts` format thành MCP content format:

```typescript
// src/relay/agent-rpc-dispatch.ts (format function)

function formatMcpResult(result: ToolResult, rpcId: unknown): JsonRpcResponse {
  const text = [
    result.stdout || '',
    result.stderr ? `[stderr]\n${result.stderr}` : '',
  ].filter(Boolean).join('\n').trim()

  return {
    jsonrpc: '2.0',
    id: rpcId,
    result: {
      content: [{ type: 'text', text: text || '(no output)' }],
      isError: (result.exitCode ?? 0) !== 0,
      exitCode: result.exitCode,
      meta: result.meta,
    },
  }
}
```

---

## 3. agent-rpc-dispatch.ts — JSON-RPC Router

```typescript
// src/relay/agent-rpc-dispatch.ts

import WebSocket from 'ws'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from './agent-wire'
import { encodeDataFrame } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

interface JsonRpcRequest {
  jsonrpc: '2.0'
  id: string | number
  method: string
  params?: Record<string, unknown>
}

export interface RpcDispatcher {
  dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void>
}

export function createRpcDispatcher(
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): RpcDispatcher {
  return {
    async dispatch(ws, state, rpc) {
      let response: object
      try {
        response = await route(rpc, tools, config, log)
      } catch (err: any) {
        response = {
          jsonrpc: '2.0', id: rpc.id,
          error: { code: AgentErrorCode.ServerError, message: `Internal error: ${err.message}` },
        }
      }
      ws.send(encodeDataFrame(state, JSON.stringify(response)))
    },
  }
}

async function route(
  rpc: JsonRpcRequest,
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  switch (rpc.method) {
    case 'tools/list':
      return {
        jsonrpc: '2.0', id: rpc.id,
        result: {
          tools: tools.map(t => ({ name: t.name, description: t.description, inputSchema: t.inputSchema })),
        },
      }

    case 'tools/call': {
      const { name, arguments: params = {} } = (rpc.params ?? {}) as { name?: string; arguments?: Record<string, unknown> }
      const tool = tools.find(t => t.name === name)
      if (!tool) {
        return { jsonrpc: '2.0', id: rpc.id, error: { code: AgentErrorCode.MethodNotFound, message: `Tool not found: ${name}` } }
      }
      log.info(`tools/call: ${name} params=${JSON.stringify(params).slice(0, 100)}`)
      const result = await tool.handler(params, config)
      return formatMcpResult(result, rpc.id)
    }

    default:
      return { jsonrpc: '2.0', id: rpc.id, error: { code: AgentErrorCode.MethodNotFound, message: `Method not found: ${rpc.method}` } }
  }
}

function formatMcpResult(result: import('./agent-tool-registry').ToolResult, id: unknown): object {
  const text = [result.stdout || '', result.stderr ? `[stderr]\n${result.stderr}` : ''].filter(Boolean).join('\n').trim()
  return {
    jsonrpc: '2.0', id,
    result: {
      content: [{ type: 'text', text: text || '(no output)' }],
      isError: (result.exitCode ?? 0) !== 0,
      exitCode: result.exitCode,
      meta: result.meta,
    },
  }
}
```

---

## 4. Test Coverage

```typescript
// src/relay/__tests__/agent-rpc-dispatch.test.ts
describe('rpcDispatcher', () => {
  it('tools/list: returns all discovered tools', async () => { ... })
  it('tools/call: found tool → handler called with params', async () => { ... })
  it('tools/call: unknown tool → MethodNotFound error', async () => { ... })
  it('tools/call: handler throws → ServerError response', async () => { ... })
  it('unknown method → MethodNotFound', async () => { ... })
  it('response sent via ws.send(encodeDataFrame(...))', async () => { ... })
  it('MCP format: content=[{type:text}], isError=false when exitCode=0', async () => { ... })
  it('MCP format: isError=true when exitCode≠0', async () => { ... })
  it('MCP format: stderr included with [stderr] prefix', async () => { ... })
})
```

**Target:** ≥ 20 tests
