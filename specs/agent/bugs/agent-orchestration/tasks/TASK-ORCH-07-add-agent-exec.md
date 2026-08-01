# TASK-ORCH-07: Add agent.exec RPC Handler (Non-interactive Subprocess)

**Task ID:** TASK-ORCH-07  
**Priority:** 🔴 HIGH  
**Bugs fixed:** TG-001  
**Estimated effort:** Medium  
**Dependencies:** None (standalone)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-rpc-dispatch.ts`

**Problem:** The task graph step executor needs to run agent commands non-interactively (capture stdout/stderr, return exit code) rather than streaming to a PTY. There is no `agent.exec` RPC method — the task graph step silently fails or uses `shell.eval` as a workaround.

**Difference from `agent.spawn`:**
| | `agent.spawn` | `agent.exec` |
|---|---|---|
| Output | Streamed via `agent.output` notifications | Captured, returned in response |
| Terminal | PTY (full terminal) | No TTY (`stdio: pipe`) |
| Use case | Interactive AI session | Task graph step, automation |
| Response | Immediate `spawn.accepted`, then async | Waits for completion, returns all output |

---

## Implementation

### Add `case 'agent.exec'` to `src/relay/agent-rpc-dispatch.ts`

Add after `case 'agent.sendInput'` in the switch statement:

```typescript
// ── agent.exec ────────────────────────────────────────────────────────────────
// TG-001: Non-interactive subprocess execution for task graph steps.
// Runs a binary with args, captures stdout/stderr, returns exit code.
// Distinct from agent.spawn: no PTY, no streaming, blocks until completion.
case 'agent.exec': {
  try {
    const { spawn } = await import('node:child_process')
    const p = rpc.params ?? {}

    // Required params
    const binary = typeof p.binary === 'string' ? p.binary.trim() : ''
    if (!binary) {
      return makeError(rpc.id, AgentErrorCode.InvalidParams, 'agent.exec: binary is required')
    }

    // Optional params with safe defaults
    const args      = Array.isArray(p.args)
      ? (p.args as unknown[]).filter(a => typeof a === 'string') as string[]
      : []
    const cwd       = typeof p.cwd === 'string' ? p.cwd : config.workDir
    const stdin     = typeof p.stdin === 'string' ? p.stdin : null
    const extraEnv  = (p.env && typeof p.env === 'object' && !Array.isArray(p.env))
      ? p.env as Record<string, string>
      : {}
    const timeoutMs = typeof p.timeoutMs === 'number'
      ? Math.min(Math.max(p.timeoutMs, 1_000), 5 * 60_000)
      : 300_000  // 5 min default, 5 min max

    log.info(`agent.exec: binary=${binary} args=${JSON.stringify(args)} cwd=${cwd} timeout=${timeoutMs}ms`)

    type ExecResult = { stdout: string; stderr: string; exitCode: number | null; timedOut: boolean }
    const result = await new Promise<ExecResult>((resolve) => {
      let stdout = '', stderr = '', timedOut = false, settled = false

      const spawnEnv = {
        ...process.env,
        ...extraEnv,
        PATH: process.env.PATH ?? '/usr/local/bin:/usr/bin:/bin',
      } as NodeJS.ProcessEnv

      const child = spawn(binary, args, {
        cwd,
        env:   spawnEnv,
        stdio: ['pipe', 'pipe', 'pipe'],
      })

      const finish = (r: ExecResult): void => {
        if (settled) return
        settled = true
        clearTimeout(timer)
        resolve(r)
      }

      const timer = setTimeout(() => {
        timedOut = true
        try { child.kill('SIGKILL') } catch { /* ignore */ }
        finish({ stdout, stderr, exitCode: null, timedOut: true })
      }, timeoutMs)

      child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
      child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
      child.on('error', (err) => {
        finish({ stdout, stderr: err.message, exitCode: null, timedOut: false })
      })
      child.on('close', (code) => {
        finish({ stdout, stderr, exitCode: code, timedOut: false })
      })

      if (stdin !== null) child.stdin?.end(stdin)
      else child.stdin?.end()
    })

    log.info(`agent.exec: done exitCode=${result.exitCode} timedOut=${result.timedOut} outLen=${result.stdout.length}`)
    return { jsonrpc: '2.0', id: rpc.id, result }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`)
  }
}
```

---

## Wire Protocol

```json
// Request:
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "agent.exec",
  "params": {
    "binary": "claude",
    "args": ["--output-format", "json", "--no-chat", "-p", "What is 2+2?"],
    "cwd": "/home/user/project",
    "env": { "ANTHROPIC_API_KEY": "sk-ant-..." },
    "timeoutMs": 60000
  }
}

// Response (success):
{
  "jsonrpc": "2.0",
  "id": 10,
  "result": {
    "stdout": "{\"content\": \"4\"}",
    "stderr": "",
    "exitCode": 0,
    "timedOut": false
  }
}

// Response (timeout):
{
  "jsonrpc": "2.0",
  "id": 10,
  "result": {
    "stdout": "partial output...",
    "stderr": "",
    "exitCode": null,
    "timedOut": true
  }
}
```

---

## Validation Rules

| Param | Type | Constraint |
|-------|------|-----------|
| `binary` | string | Required, non-empty |
| `args` | string[] | Optional, each element must be string |
| `cwd` | string | Optional, defaults to `config.workDir` |
| `stdin` | string | Optional, piped to stdin if present |
| `env` | object | Optional, merged with process.env |
| `timeoutMs` | number | Optional, 1000–300000ms (clamped) |

---

## Unit Tests to Add

File: `src/relay/__tests__/agent-rpc-dispatch.test.ts`

```typescript
describe('agent.exec', () => {
  it('captures stdout from successful command', async () => {
    const r = await dispatch({ id: 1, method: 'agent.exec', params: {
      binary: 'echo', args: ['hello world']
    }}) as any
    expect(r.result.stdout).toContain('hello world')
    expect(r.result.exitCode).toBe(0)
    expect(r.result.timedOut).toBe(false)
  })

  it('captures exit code from failing command', async () => {
    const r = await dispatch({ id: 2, method: 'agent.exec', params: {
      binary: 'false'
    }}) as any
    expect(r.result.exitCode).not.toBe(0)
  })

  it('returns error when binary is empty', async () => {
    const r = await dispatch({ id: 3, method: 'agent.exec', params: { binary: '' }}) as any
    expect(r.error.message).toContain('binary is required')
  })

  it('respects timeoutMs', async () => {
    const r = await dispatch({ id: 4, method: 'agent.exec', params: {
      binary: 'sleep', args: ['10'], timeoutMs: 100
    }}) as any
    expect(r.result.timedOut).toBe(true)
    expect(r.result.exitCode).toBeNull()
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-rpc-dispatch
npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-exec-handler.ts: handleAgentExec(id, params, config, log) — non-interactive Claude/Codex/Gemini run dùng child_process.spawn. Binary existence check trước spawn. 10s–10m timeout.  
**Tests:** Verified via agent-rpc-dispatch.ts routing to handleAgentExec.  
