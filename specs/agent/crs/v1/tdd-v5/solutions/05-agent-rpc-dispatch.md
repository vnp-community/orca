# SOL-05: agent-rpc-dispatch.ts — JSON-RPC Router

**TDD Ref:** TDD-AG-06, TDD-AG-07  
**File:** `src/relay/agent-rpc-dispatch.ts` [NEW]  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## Full Implementation

```typescript
// src/relay/agent-rpc-dispatch.ts

import type WebSocket from 'ws'
import type { ToolDefinition, ToolResult } from './agent-tool-registry'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from './agent-wire'
import { encodeDataFrame } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ─── JSON-RPC Types ──────────────────────────────────────────────────────────

interface JsonRpcRequest {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly method: string
  readonly params?: Record<string, unknown>
}

interface JsonRpcSuccess {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly result: unknown
}

interface JsonRpcError {
  readonly jsonrpc: '2.0'
  readonly id: string | number | null
  readonly error: { code: number; message: string; data?: unknown }
}

type JsonRpcResponse = JsonRpcSuccess | JsonRpcError

// ─── RpcDispatcher ───────────────────────────────────────────────────────────

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
      let response: JsonRpcResponse
      try {
        response = await route(rpc, tools, config, log, ws, state)
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`RPC dispatch error for method=${rpc.method}: ${msg}`)
        response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`)
      }

      // Send response (only if ws still open)
      if (ws.readyState === 1 /* OPEN */) {
        ws.send(encodeDataFrame(state, JSON.stringify(response)))
      }
    },
  }
}

// ─── Route ───────────────────────────────────────────────────────────────────

async function route(
  rpc: JsonRpcRequest,
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse> {

  switch (rpc.method) {
    // ── MCP: tools/list ──────────────────────────────────────────────────────
    case 'tools/list':
      return {
        jsonrpc: '2.0', id: rpc.id,
        result: {
          tools: tools.map(t => ({
            name: t.name,
            description: t.description,
            inputSchema: t.inputSchema,
          })),
        },
      }

    // ── MCP: tools/call ──────────────────────────────────────────────────────
    case 'tools/call': {
      const params = rpc.params ?? {}
      const name   = typeof params.name === 'string' ? params.name : ''
      const args   = (typeof params.arguments === 'object' && params.arguments !== null)
        ? params.arguments as Record<string, unknown>
        : {}

      const tool = tools.find(t => t.name === name)
      if (!tool) {
        return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Tool not found: ${name}`)
      }

      log.info(`tools/call: ${name} params=${JSON.stringify(args).slice(0, 100)}`)
      const result = await tool.handler(args, config)
      return formatMcpResult(rpc.id, result)
    }

    // ── v5.0: git.exec ───────────────────────────────────────────────────────
    case 'git.exec': {
      const { handleGitExec } = await import('./git-handler')
      return handleGitExec(rpc.id, rpc.params ?? {}, config, log)
    }

    // ── v5.0: git.execStream ─────────────────────────────────────────────────
    case 'git.execStream': {
      const { handleGitExecStream } = await import('./git-handler')
      // Streaming: sends multiple frames, no single response
      void handleGitExecStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
      return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
    }

    // ── v5.0: fs.readDir ─────────────────────────────────────────────────────
    case 'fs.readDir': {
      const { handleFsReadDir } = await import('./fs-agent-extensions')
      return handleFsReadDir(rpc.id, rpc.params ?? {}, config)
    }

    // ── v5.0: fs.readFile ────────────────────────────────────────────────────
    case 'fs.readFile': {
      const { handleFsReadFile } = await import('./fs-agent-extensions')
      return handleFsReadFile(rpc.id, rpc.params ?? {}, config)
    }

    // ── v5.0: fs.grep ────────────────────────────────────────────────────────
    case 'fs.grep': {
      const { handleFsGrep } = await import('./fs-agent-extensions')
      return handleFsGrep(rpc.id, rpc.params ?? {}, config)
    }

    // ── v5.0: ai.provider.writeCredential ────────────────────────────────────
    case 'ai.provider.writeCredential': {
      const { handleWriteCredential } = await import('./agent-credential-store')
      return handleWriteCredential(rpc.id, rpc.params ?? {}, config, log)
    }

    // ── v5.0: ai.provider.readCredential ─────────────────────────────────────
    case 'ai.provider.readCredential': {
      const { handleReadCredential } = await import('./agent-credential-store')
      return handleReadCredential(rpc.id, rpc.params ?? {}, config, log)
    }

    // ── v5.0: ai.provider.healthCheck ────────────────────────────────────────
    case 'ai.provider.healthCheck': {
      const { handleHealthCheck } = await import('./agent-credential-store')
      return handleHealthCheck(rpc.id, rpc.params ?? {}, config, log)
    }

    // ── v5.0: preflight.check ────────────────────────────────────────────────
    case 'preflight.check': {
      const { handlePreflightCheck } = await import('./fs-agent-extensions')
      return handlePreflightCheck(rpc.id, rpc.params ?? {}, config)
    }

    // ── Unknown method ───────────────────────────────────────────────────────
    default:
      return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Method not found: ${rpc.method}`)
  }
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatMcpResult(id: string | number | null, result: ToolResult): JsonRpcSuccess {
  const text = [
    result.stdout || '',
    result.stderr ? `[stderr]\n${result.stderr}` : '',
  ].filter(Boolean).join('\n').trim()

  return {
    jsonrpc: '2.0', id,
    result: {
      content: [{ type: 'text', text: text || '(no output)' }],
      isError: result.exitCode !== 0,
      exitCode: result.exitCode,
      meta: result.meta,
    },
  }
}

function makeError(
  id: string | number | null,
  code: number,
  message: string,
  data?: unknown
): JsonRpcError {
  return { jsonrpc: '2.0', id, error: { code, message, ...(data !== undefined && { data }) } }
}

export { makeError, formatMcpResult }
```

---

## Lưu ý

**Dynamic imports cho v5.0 extensions**: Dùng `await import('./git-handler')` để các module v5.0 chưa tồn tại không làm fail build. Khi v5.0 module được tạo → import sẽ resolve. Nếu chưa tạo → lỗi runtime chỉ xảy ra khi method được call (không fail startup).

**Streaming return**: `git.execStream` trả về `{ type: 'stream.started' }` ngay lập tức, rồi `handleGitExecStream` tự gửi frames bất đồng bộ. Caller (Orca Server) phải handle stream pattern.

---

## Definition of Done

- [x] `src/relay/agent-rpc-dispatch.ts` created
- [x] `tsc` passes
- [x] `src/relay/__tests__/agent-rpc-dispatch.test.ts` — ≥ 20 tests
  - tools/list, tools/call found/not-found
  - unknown method → MethodNotFound
  - MCP format verification (content, isError, exitCode)
  - handler throws → ServerError response (không crash agent)
  - ws.send called once per dispatch
