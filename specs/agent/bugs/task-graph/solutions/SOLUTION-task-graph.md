# SOLUTION: Task Graph Domain — Fix Bugs

**Domain:** task-graph  
**TDD Reference:** TDD-AG-07 (JSON-RPC Dispatch), TDD-AG-12 (Agent Spawner)  
**Files cần thay đổi:** `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-exec-handler.ts`  
**Tổng số bugs:** 1 (TG-001)

---

## BUG-TG-001 — Fix relay dispatch thiếu handler `agent.exec`

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `StepExecutors.ts` và `ProfileAwareAgentSpawner.ts` đều gọi `relay.call('agent.exec', {...})`, nhưng `agent-rpc-dispatch.ts` không có `case 'agent.exec'`.

### Phân tích hiện trạng

```
src/relay/agent-rpc-dispatch.ts:
  case 'agent.spawn':         ✅ (spawn interactive PTY agent)
  case 'agent.kill':          ✅
  case 'agent.sendInput':     ✅ (sau fix ORCH-001)
  case 'agent.exec':          ❌ MISSING ← BUG

src/relay/agent-exec-handler.ts:
  agent.execNonInteractive    ✅ (line ~140) — nhưng registered khác cách
```

### Phân biệt `agent.spawn` vs `agent.exec`

| | `agent.spawn` | `agent.exec` |
|---|---|---|
| Mode | Interactive (PTY) | Non-interactive (capture output) |
| Output | Stream via `agent.output` notifications | Return trong response |
| Use case | User terminal in browser | Task automation, workflow steps |
| Timeout | Indefinite (user controlled) | Fixed timeout (300s) |
| HLD | BL-AG-01 | BL-TG-04, BL-WF-02 |

### Fix: Đăng ký `agent.exec` trong dispatch

**Option A — Delegate đến `agent-exec-handler.ts` (khuyến nghị):**

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm case:

case 'agent.exec': {
  /**
   * Non-interactive agent execution — dùng cho workflow steps và task automation.
   * Theo BL-TG-04 và BL-WF-02.
   *
   * Params: { prompt, worktreePath, trustPreset, model?, accountId?, taskId?, stepId? }
   * Returns: { stdout, stderr, exitCode, latencyMs }
   */
  try {
    const { handleAgentExec } = await import('./agent-exec-handler')
    return (await handleAgentExec(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec unavailable: ${msg}`)
  }
}
```

**Implement `handleAgentExec` trong `agent-exec-handler.ts`:**

```typescript
// src/relay/agent-exec-handler.ts — thêm hàm:

export interface AgentExecParams {
  prompt:       string           // Prompt to send to agent
  worktreePath: string           // Working directory
  trustPreset?: 'standard' | 'full' | 'none'
  model?:       string           // Agent model (default: claude)
  accountId?:   string           // AI credential account
  taskId?:      string           // ORCA_TASK_ID context
  stepId?:      string           // Workflow step ID
  timeoutMs?:   number           // Max execution time (default: 300_000 = 5min)
}

/**
 * handleAgentExec: Chạy AI agent non-interactively, capture output.
 * Dùng cho BL-TG-04 (Task step execution) và BL-WF-02 (Workflow step).
 *
 * Khác với agent.spawn (interactive PTY):
 * - Không cần PTY (dùng execFile/spawn với pipe)
 * - Capture stdout/stderr và return trong response
 * - Có timeout cố định
 */
export async function handleAgentExec(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const req = parseAgentExecParams(params)
  if (!req) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: 'Invalid agent.exec params: prompt and worktreePath required' }
    }
  }

  // Resolve agent binary (default: claude)
  const modelId = req.model ?? 'claude'
  const spec    = resolveAgentSpec(modelId)
  if (!spec) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: `Unknown model: ${modelId}` }
    }
  }

  // Check binary available
  if (!isBinaryAvailable(spec.binary, config.toolPath)) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32603, message: `${spec.binary} not found in PATH` }
    }
  }

  // Load API credential
  const credStore = new AiCredStore(config)
  let toolEnv: NodeJS.ProcessEnv = { PATH: config.toolPath }
  if (spec.apiKeyEnv && req.accountId) {
    const apiKey = await credStore.readDecrypted(req.accountId)
    if (apiKey) {
      toolEnv = { ...toolEnv, [spec.apiKeyEnv]: apiKey }
    } else {
      log.warn(`agent.exec: no credential for accountId=${req.accountId}`)
    }
  }

  // Inject Orca context
  toolEnv = {
    ...toolEnv,
    ORCA_TASK_ID: req.taskId ?? '',
    HOME:         homedir(),
    TERM:         'xterm-256color',
  }

  // Build args for non-interactive mode
  // Claude: --print <prompt> (print mode — no PTY needed)
  const args: string[] = ['--print', req.prompt]
  if (req.model)       args.unshift('--model', req.model)
  if (req.trustPreset) args.push('--trust', req.trustPreset)

  const timeoutMs = req.timeoutMs ?? 300_000  // 5 minutes default

  log.info(`agent.exec: model=${modelId} worktreePath=${req.worktreePath} stepId=${req.stepId ?? 'none'}`)

  const start = Date.now()
  try {
    // runCommandCapture: existing function trong agent-exec-handler.ts
    const result = await runCommandCaptureWithTimeout(
      spec.binary,
      args,
      { cwd: req.worktreePath, timeout: timeoutMs, env: toolEnv }
    )

    const latencyMs = Date.now() - start
    log.info(`agent.exec: done exitCode=${result.exitCode} latency=${latencyMs}ms`)

    return {
      jsonrpc: '2.0', id,
      result: {
        stdout:    result.stdout,
        stderr:    result.stderr,
        exitCode:  result.exitCode,
        latencyMs,
        stepId:    req.stepId,
      }
    }
  } catch (err: unknown) {
    const msg     = err instanceof Error ? err.message : String(err)
    const latencyMs = Date.now() - start
    log.error(`agent.exec failed: ${msg}`)
    return {
      jsonrpc: '2.0', id,
      error: { code: -32603, message: `agent.exec failed: ${msg}`, data: { latencyMs } }
    }
  }
}

function parseAgentExecParams(params: Record<string, unknown>): AgentExecParams | null {
  if (typeof params.prompt !== 'string' || !params.prompt) return null
  if (typeof params.worktreePath !== 'string' || !params.worktreePath) return null

  return {
    prompt:       params.prompt,
    worktreePath: params.worktreePath,
    trustPreset:  (params.trustPreset as any) ?? 'standard',
    model:        typeof params.model     === 'string' ? params.model     : undefined,
    accountId:    typeof params.accountId === 'string' ? params.accountId : undefined,
    taskId:       typeof params.taskId    === 'string' ? params.taskId    : undefined,
    stepId:       typeof params.stepId    === 'string' ? params.stepId    : undefined,
    timeoutMs:    typeof params.timeoutMs === 'number' ? params.timeoutMs : undefined,
  }
}
```

### Helper: `runCommandCaptureWithTimeout`

Nếu chưa có trong `agent-exec-handler.ts`, thêm:

```typescript
// src/relay/agent-exec-handler.ts
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

interface CaptureResult {
  stdout:   string
  stderr:   string
  exitCode: number
}

async function runCommandCaptureWithTimeout(
  binary:  string,
  args:    string[],
  opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv }
): Promise<CaptureResult> {
  return new Promise((resolve, reject) => {
    const child = execFile(binary, args, {
      cwd:      opts.cwd,
      env:      opts.env,
      timeout:  opts.timeout,
      maxBuffer: 10 * 1024 * 1024,  // 10MB output limit
    }, (err, stdout, stderr) => {
      if (err && err.killed) {
        reject(new Error(`Command timed out after ${opts.timeout}ms`))
        return
      }
      const exitCode = err?.code ?? (err ? 1 : 0)
      resolve({
        stdout:   stdout ?? '',
        stderr:   stderr ?? '',
        exitCode: typeof exitCode === 'number' ? exitCode : 1,
      })
    })
  })
}
```

---

## Verification Plan

```bash
# 1. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json

# 2. Unit tests:
pnpm vitest run src/relay/__tests__/agent-exec-handler.test.ts

# 3. Integration test (manual):
# - Gọi relay.call('agent.exec', { prompt: 'say hello', worktreePath: '/tmp/test', model: 'claude' })
# - Expect: { result: { stdout: 'Hello!', exitCode: 0, latencyMs: N } }
# - Verify stdout captured (không stream về như agent.spawn)

# 4. End-to-end test:
# - StepExecutors.executeAgent() gọi relay.call('agent.exec', ...)
# - Verify không còn lỗi "Method not found: agent.exec"
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/relay/agent-rpc-dispatch.ts` | ADD `case 'agent.exec'` | TG-001 |
| `src/relay/agent-exec-handler.ts` | ADD `handleAgentExec`, `parseAgentExecParams`, `runCommandCaptureWithTimeout` | TG-001 |
| `src/relay/__tests__/agent-exec-handler.test.ts` | ADD tests cho `handleAgentExec` | TG-001 |

---

## ✅ Implementation Status (2026-08-01)

TG-001: agent.exec handler DONE (agent-exec-handler.ts). TG-002: ai.complete handler DONE (ai-complete-handler.ts).
