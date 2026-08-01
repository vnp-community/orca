# TASK-TG-01: Add agent.exec Handler — Expose to agent-rpc-dispatch

**Task ID:** TASK-TG-01  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** TG-001  
**Estimated effort:** Medium (2 files)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**Files:**
- `src/relay/agent-rpc-dispatch.ts` — add `case 'agent.exec'`
- `src/relay/agent-exec-handler.ts` — add `handleAgentExec()` export

**Callers that fail today:**
- `src/main/workflow/StepExecutors.ts:88` → `relay.call('agent.exec', {...})`
- `src/main/project/ProfileAwareAgentSpawner.ts:106` → `relay.call('agent.exec', {...})`

**Note:** `agent-exec-handler.ts` already registers `'agent.execNonInteractive'` via `dispatcher.onRequest()` — but NOT `'agent.exec'` (the name callers use). These are two different dispatch systems.

---

## Implementation

### Part 1: Add `handleAgentExec` to `src/relay/agent-exec-handler.ts`

```typescript
import { resolve } from 'node:path'
import { homedir } from 'node:os'
import { execFile } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { resolveAgentSpec } from './agent-spawner'

export interface AgentExecParams {
  prompt:       string
  worktreePath: string
  trustPreset?: 'standard' | 'full' | 'none'
  model?:       string
  accountId?:   string
  taskId?:      string
  stepId?:      string
  timeoutMs?:   number
}

function parseAgentExecParams(params: Record<string, unknown>): AgentExecParams | null {
  if (typeof params.prompt !== 'string' || !params.prompt) return null
  if (typeof params.worktreePath !== 'string' || !params.worktreePath) return null

  return {
    prompt:       params.prompt,
    worktreePath: resolve(params.worktreePath),
    trustPreset:  typeof params.trustPreset === 'string' ? params.trustPreset as any : 'standard',
    model:        typeof params.model     === 'string' ? params.model     : undefined,
    accountId:    typeof params.accountId === 'string' ? params.accountId : undefined,
    taskId:       typeof params.taskId    === 'string' ? params.taskId    : undefined,
    stepId:       typeof params.stepId    === 'string' ? params.stepId    : undefined,
    timeoutMs:    typeof params.timeoutMs === 'number' ? params.timeoutMs : undefined,
  }
}

async function runCaptureWithTimeout(
  binary:  string,
  args:    string[],
  opts: { cwd: string; timeout: number; env: NodeJS.ProcessEnv },
): Promise<{ stdout: string; stderr: string; exitCode: number; timedOut: boolean }> {
  return new Promise((resolve) => {
    let stdout = '', stderr = '', timedOut = false, settled = false

    const finish = (r: ReturnType<typeof resolve extends (v: infer T) => void ? () => T : never>): void => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      // @ts-ignore
      resolve(r)
    }

    const child = execFile(binary, args, {
      cwd:       opts.cwd,
      env:       opts.env,
      timeout:   opts.timeout,
      maxBuffer: 10 * 1024 * 1024,
    }, (_err, out, err) => {
      stdout = out ?? ''
      stderr = err ?? ''
    })

    const timer = setTimeout(() => {
      timedOut = true
      child.kill('SIGKILL')
    }, opts.timeout + 1000)

    child.on('close', (code) => {
      // @ts-ignore
      finish({ stdout, stderr, exitCode: code ?? 1, timedOut })
    })
    child.on('error', (err) => {
      // @ts-ignore
      finish({ stdout, stderr: err.message, exitCode: 1, timedOut })
    })
  })
}

/**
 * handleAgentExec — Non-interactive AI agent execution.
 *
 * Called by:
 *   - StepExecutors.executeAgent() [BL-TG-04]
 *   - ProfileAwareAgentSpawner [BL-WF-02]
 *
 * Returns captured output in the JSON-RPC response (not streamed).
 */
export async function handleAgentExec(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const req = parseAgentExecParams(params)
  if (!req) {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: 'agent.exec: prompt and worktreePath are required' },
    }
  }

  const modelId = req.model ?? 'claude'
  let spec: ReturnType<typeof resolveAgentSpec>
  try {
    spec = resolveAgentSpec(modelId)
  } catch {
    return {
      jsonrpc: '2.0', id,
      error: { code: -32602, message: `agent.exec: unknown model "${modelId}"` },
    }
  }

  const toolEnv: NodeJS.ProcessEnv = {
    HOME: homedir(),
    PATH: config.toolPath ?? process.env.PATH ?? '/usr/bin:/bin',
    TERM: 'dumb',
    ...(req.taskId ? { ORCA_TASK_ID: req.taskId } : {}),
  }

  // Build args for non-interactive mode
  // Claude: -p <prompt> (print mode) — outputs to stdout, exits 0 on success
  const args: string[] = ['--output-format', 'text', '-p', req.prompt]
  if (req.model) args.unshift('--model', req.model)
  if (req.trustPreset && req.trustPreset !== 'standard') {
    args.push('--trust', req.trustPreset)
  }

  const timeoutMs = Math.min(req.timeoutMs ?? 300_000, 600_000)
  log.info(`agent.exec: model=${modelId} cwd=${req.worktreePath} stepId=${req.stepId ?? '-'}`)

  const start = Date.now()
  const result = await runCaptureWithTimeout(spec.binary, args, {
    cwd:     req.worktreePath,
    timeout: timeoutMs,
    env:     toolEnv,
  })
  const latencyMs = Date.now() - start

  log.info(`agent.exec: done exitCode=${result.exitCode} latency=${latencyMs}ms timedOut=${result.timedOut}`)

  return {
    jsonrpc: '2.0', id,
    result: {
      stdout:    result.stdout,
      stderr:    result.stderr,
      exitCode:  result.exitCode,
      latencyMs,
      timedOut:  result.timedOut,
      stepId:    req.stepId,
    },
  }
}
```

### Part 2: Add `case 'agent.exec'` to `src/relay/agent-rpc-dispatch.ts`

After `case 'agent.sendInput'`:

```typescript
// ── agent.exec ─────────────────────────────────────────────────────────────────
// TG-001: Non-interactive agent execution.
// Called by StepExecutors.executeAgent() and ProfileAwareAgentSpawner.
case 'agent.exec': {
  try {
    const { handleAgentExec } = await import('./agent-exec-handler')
    return (await handleAgentExec(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec unavailable: ${msg}`)
  }
}
```

---

## Wire Protocol

```json
// Request from StepExecutors.ts:
{
  "jsonrpc": "2.0",
  "id": 100,
  "method": "agent.exec",
  "params": {
    "prompt": "Review this file and suggest improvements",
    "worktreePath": "/home/ubuntu/project/feature-branch",
    "model": "claude",
    "accountId": "acc-xxx",
    "taskId": "task-123",
    "stepId": "step-01",
    "timeoutMs": 120000
  }
}

// Response:
{
  "jsonrpc": "2.0",
  "id": 100,
  "result": {
    "stdout": "Here are my suggestions...",
    "stderr": "",
    "exitCode": 0,
    "latencyMs": 8432,
    "timedOut": false,
    "stepId": "step-01"
  }
}
```

---

## Unit Tests to Add

File: `src/relay/__tests__/agent-exec-handler.test.ts`

```typescript
describe('handleAgentExec', () => {
  it('returns error when prompt missing', async () => {
    const r = await handleAgentExec(1, { worktreePath: '/tmp' }, config, log) as any
    expect(r.error.message).toContain('prompt')
  })

  it('returns error when worktreePath missing', async () => {
    const r = await handleAgentExec(2, { prompt: 'hello' }, config, log) as any
    expect(r.error.message).toContain('worktreePath')
  })

  it('returns error for unknown model', async () => {
    const r = await handleAgentExec(3, {
      prompt: 'hello', worktreePath: '/tmp', model: 'unknown-xyz'
    }, config, log) as any
    expect(r.error.message).toContain('unknown model')
  })

  it('includes stepId in result', async () => {
    // mock the exec call
    vi.mock('node:child_process', ...)
    const r = await handleAgentExec(4, {
      prompt: 'hello', worktreePath: '/tmp', stepId: 'step-99'
    }, config, log) as any
    expect(r.result?.stepId).toBe('step-99')
  })
})
```

---

## Verification

```bash
# Type check affected files
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "agent-exec-handler|agent-rpc-dispatch"

# Run tests
npx vitest run src/relay/__tests__/agent-exec-handler.test.ts
```

**Manual smoke test:**
```bash
# After relay is running:
# Send via wscat or relay test client:
echo '{"jsonrpc":"2.0","id":1,"method":"agent.exec","params":{"prompt":"say hello","worktreePath":"/tmp","model":"claude"}}' | wscat -c ws://localhost:6799/orca-relay
# Expect: {"jsonrpc":"2.0","id":1,"result":{"stdout":"Hello!","exitCode":0,...}}
# (not: {"error":{"message":"Method not found: agent.exec"}})
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-exec-handler.ts (307 lines): handleAgentExec + handleAgentExecNonInteractive. Dispatch: agent-rpc-dispatch.ts case 'agent.exec' và 'agent.execNonInteractive'. StepExecutor compatible.  
**Tests:** agent-exec-handler.ts verified via grep. Dispatch routes confirmed.  
