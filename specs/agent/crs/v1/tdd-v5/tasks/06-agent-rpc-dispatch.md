# TASK-06: Create src/relay/agent-rpc-dispatch.ts

**Phase:** 3  
**SOL Ref:** SOL-05  
**Estimated time:** 2h  
**Precondition:** TASK-04 (agent-wire.ts), TASK-05 (agent-tool-registry.ts) hoàn thành  

---

## Tạo file mới: `src/relay/agent-rpc-dispatch.ts`

**QUAN TRỌNG:**
- Import `AgentErrorCode` từ `'../shared/agent-wire-protocol'` (đã có sẵn)
- Dùng dynamic `await import('./git-handler')` cho v5.0 methods — không fail nếu module chưa tồn tại khi startup
- `ws.readyState === 1` để check OPEN trước khi `ws.send()`

### Imports

```typescript
import type WebSocket from 'ws'
import type { ToolDefinition, ToolResult } from './agent-tool-registry'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from './agent-wire'
import { encodeDataFrame } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
```

### RpcDispatcher interface + createRpcDispatcher()

```typescript
export interface RpcDispatcher {
  dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void>
}

export function createRpcDispatcher(
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): RpcDispatcher
```

### Route switch — methods cần implement

| Method | Handler |
|--------|---------|
| `tools/list` | Trả về tất cả tools theo MCP format |
| `tools/call` | Gọi `tool.handler(args, config)` → MCP result |
| `git.exec` | `await import('./git-handler').then(m => m.handleGitExec(...))` |
| `git.execStream` | `await import('./git-handler').then(m => m.handleGitExecStream(...))` |
| `fs.readDir` | `await import('./fs-agent-extensions').then(m => m.handleFsReadDir(...))` |
| `fs.readFile` | `await import('./fs-agent-extensions').then(m => m.handleFsReadFile(...))` |
| `fs.grep` | `await import('./fs-agent-extensions').then(m => m.handleFsGrep(...))` |
| `ai.provider.writeCredential` | `await import('./agent-credential-store').then(m => m.handleWriteCredential(...))` |
| `ai.provider.readCredential` | `await import('./agent-credential-store').then(m => m.handleReadCredential(...))` |
| `ai.provider.healthCheck` | `await import('./agent-credential-store').then(m => m.handleHealthCheck(...))` |
| `preflight.check` | `await import('./fs-agent-extensions').then(m => m.handlePreflightCheck(...))` |
| `*` | `MethodNotFound (-32601)` |

### MCP result format (bắt buộc)

```typescript
function formatMcpResult(id: string | number | null, result: ToolResult): object {
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
```

### Error handling

- Mọi exception trong dispatch → catch → `ServerError (-32000)` response, log error
- Không để exception propagate ra ngoài `dispatch()`

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-rpc-dispatch" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/agent-rpc-dispatch.ts` created
- [x] `createRpcDispatcher()` exported
- [x] Tất cả 11 methods trong route switch
- [x] `formatMcpResult()` exported (needed by tests)
- [x] Dynamic imports cho v5.0 modules
- [x] Exception catch trong `dispatch()` → ServerError response
- [x] `pnpm run typecheck:node` passes
