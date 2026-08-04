# TASK-AG-002.2: Resume agent:rpc tracer + agentOrch:spawn span for agent.exec

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-002](../solutions/SOL-AG-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-002.1](./TASK-AG-002.1-agentorch-tracers-registration.md)
**Estimated time:** 2h
**Status:** ✅ Done (2026-08-03) — implemented exactly as specced; `agent.exec` case body was untouched by the concurrent PTY-daemon/fs.watch work (which only touched the `pty.*`/`fs.watch` case arms elsewhere in this file). 23/23 existing tests pass, `pnpm run typecheck:node` clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

`agent-rpc-dispatch.ts` là file high-fan-in bị nhiều task khác cùng mở rộng (015.1, 017.1, 018.2) — target đúng symbol, KHÔNG explore/impact cả file:

```bash
codegraph explore "extractTraceFields"
codegraph explore "createRpcDispatcher"
```

Cả 2 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
gitnexus_impact({ target: "createRpcDispatcher", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

Có **2 con đường "spawn agent" song song**: `agent.exec` (`agent-rpc-dispatch.ts` case inline, `child_process.spawn`, không PTY — target thật của `ProfileAwareAgentSpawner.spawn()`) và `agent.spawn` (`agent-spawner.ts`, `node-pty`, interactive — xem [TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md)). Task này sửa `agent-rpc-dispatch.ts`: (1) fix `extractTraceFields()` cho nhóm `agent.*` thiếu field `binary`, (2) resume tracer hạ tầng `agent:rpc` từ `params._trace.id` — điểm đòn bẩy cao nhất vì áp dụng cho MỌI method (kể cả `git.*`, `pty.*`), (3) thêm domain span `agentOrch:spawn` cho case `'agent.exec'`.

## File: `src/relay/agent-rpc-dispatch.ts` [MODIFY]

### Import mới

```typescript
import { Tracers } from '../shared/trace/tracers'   // [NEW]
```

### `extractTraceFields()` — sửa nhánh `agent.*`

```typescript
// (giữ nguyên str/num/truncPath/truncCmd và các nhánh fs./git./github./gitlab./ai.provider./tools/call)

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId'] ?? p['taskId']),      // [MODIFIED] agent.exec/agent.spawn dùng taskId, không phải sessionId
      binary:  str(p['binary'] ?? p['model'] ?? p['modelId']),  // [NEW] khớp params thật của agent.exec (p.binary) và agent.spawn (p.model)
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}
```

### Resume extraction (mới)

```typescript
// ─── Trace resume extraction ────────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested tại params._trace.id (CR-TRACE-000 §3.3).
function extractResume(params: Record<string, unknown>): { id: string } | undefined {  // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}
```

### `dispatch()` — resume `agent:rpc`

```typescript
export function createRpcDispatcher(
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): RpcDispatcher {
  return {
    async dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void> {
      const ctxFields = extractTraceFields(rpc.method, rpc.params ?? {})
      const span = rpcTracer.start(
        { method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields },
        extractResume(rpc.params ?? {})                                            // [MODIFIED]
      )
      let response: JsonRpcResponse
      try {
        response = await route(rpc, tools, config, log, ws, state)
        if ('error' in response) {
          span.fail(response.error.message, { method: rpc.method, code: response.error.code })
        } else {
          span.ok({ method: rpc.method })
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`RPC dispatch unhandled error method=${rpc.method}: ${msg}`)
        span.fail(msg, { method: rpc.method, phase: 'dispatch' })
        response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`)
      }

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeDataFrame(state, JSON.stringify(response)))
      }
    },
  }
}
```

### Case `'agent.exec'` — domain span `agentOrch:spawn`

```typescript
    // ── v5.0: agent.exec ─────────────────────────────────────────────────────
    // TG-001/BL-AG-01: Non-interactive subprocess execution — target thật của
    // ProfileAwareAgentSpawner.spawn() (Orca Server) theo relay.call('agent.exec', ...).
    case 'agent.exec': {
      const p       = rpc.params ?? {}
      const binary  = typeof p.binary === 'string' ? p.binary : ''
      const span = Tracers.agentOrchSpawn.start(                                    // [NEW]
        { binary, taskId: typeof p.taskId === 'string' ? p.taskId : undefined },
        extractResume(p)
      )
      try {
        const { spawn } = await import('node:child_process')
        const args    = Array.isArray(p.args) ? (p.args as unknown[]).map(String) : []
        const cwd     = typeof p.cwd      === 'string' ? p.cwd                    : config.workDir
        const stdin   = typeof p.stdin    === 'string' ? p.stdin                  : null
        const extraEnv = (p.env && typeof p.env === 'object' && !Array.isArray(p.env))
          ? p.env as Record<string, string>
          : {}
        const timeoutMs = typeof p.timeoutMs === 'number'
          ? Math.min(Math.max(p.timeoutMs, 1_000), 5 * 60_000)
          : 300_000

        if (!binary) {
          span.fail('binary is required')                                          // [NEW]
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'agent.exec: binary is required')
        }

        span.step('subprocess-spawn', { binary, cwd })                             // [NEW]
        const result = await new Promise<{
          stdout: string; stderr: string; exitCode: number | null; timedOut: boolean
        }>((resolve) => {
          let stdout = '', stderr = '', timedOut = false, settled = false
          const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
          const child = spawn(binary, args, { cwd, env: spawnEnv, stdio: ['pipe', 'pipe', 'pipe'] })

          const finish = (r: typeof result): void => {
            if (settled) return
            settled = true
            clearTimeout(timer)
            resolve(r)
          }
          const timer = setTimeout(() => {
            timedOut = true
            try { child.kill('SIGKILL') } catch { /* ignore */ }
            finish({ stdout, stderr, exitCode: null, timedOut })
          }, timeoutMs)

          child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
          child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
          child.on('error', (err) => {
            finish({ stdout, stderr: err.message, exitCode: null, timedOut })
          })
          child.on('close', (code) => { finish({ stdout, stderr, exitCode: code, timedOut }) })

          if (stdin !== null) child.stdin?.end(stdin)
          else child.stdin?.end()
        })

        log.info(`agent.exec: binary=${binary} exitCode=${result.exitCode} timedOut=${result.timedOut}`)
        if (result.timedOut) {
          span.fail(`timeout after ${timeoutMs}ms`, { binary })                     // [NEW]
        } else if (result.exitCode !== 0) {
          span.fail(`exit code ${result.exitCode}`, { binary, exitCode: result.exitCode ?? -1 })  // [NEW]
        } else {
          span.ok({ binary, exitCode: result.exitCode ?? 0 })                       // [NEW]
        }
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        span.fail(err, { binary })                                                  // [NEW]
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`)
      }
    }
```

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-rpc-dispatch" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `extractTraceFields()` nhánh `agent.*` trích `binary` từ `p['binary'] ?? p['model'] ?? p['modelId']` và `session` từ `p['sessionId'] ?? p['taskId']`
- [ ] `dispatch()` trong `agent-rpc-dispatch.ts` resume tracer hạ tầng `agent:rpc` từ `params._trace.id` cho MỌI method (không riêng agent.*)
- [ ] Case `'agent.exec'` phát `agentOrch:spawn` span: `ok()` chứa `{binary, exitCode}`, `fail()` khi timeout hoặc exitCode ≠ 0 hoặc binary rỗng
- [ ] `env`/API key KHÔNG bao giờ xuất hiện trong field của `agentOrch:spawn` — chỉ `binary`, `taskId`, `exitCode`
- [ ] `import { Tracers } from '../shared/trace/tracers'` thêm đúng vị trí đầu file
- [ ] `pnpm run typecheck:node` pass
