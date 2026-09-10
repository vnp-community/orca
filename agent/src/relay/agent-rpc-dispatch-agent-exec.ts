// src/relay/agent-rpc-dispatch-agent-exec.ts
// agent.spawn/kill/sendInput/exec/execPrompt RPC methods — split out of
// agent-rpc-dispatch.ts's giant switch to keep each file under the oxlint
// max-lines budget.

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from 'orca-dev-agent-transport'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { Tracers } from '../shared/trace/tracers'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError, extractResume } from './agent-rpc-dispatch'

export async function dispatchAgentExecRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case 'agent.spawn': {
      try {
        const { handleAgentSpawn } = await import('./agent-spawner')
        // Fire-and-forget: streaming handler sends multiple frames asynchronously
        void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.kill ─────────────────────────────────────────────────────
    case 'agent.kill': {
      try {
        const { handleAgentKill } = await import('./agent-spawner')
        return (await handleAgentKill(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.kill unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.sendInput ────────────────────────────────────────────────
    // ORCH-001: Send data to a running agent PTY's stdin.
    // Used for graceful stop (Ctrl+C = '\x03') and interactive input.
    case 'agent.sendInput': {
      try {
        const { handleAgentSendInput } = await import('./agent-spawner')
        return (await handleAgentSendInput(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.sendInput unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.exec ─────────────────────────────────────────────────────
    // TG-001: Non-interactive subprocess execution (for task graph steps).
    // Returns captured stdout/stderr/exitCode instead of streaming.
    // Distinct from agent.spawn (interactive PTY) — no terminal allocation.
    // Called by:
    //   - StepExecutors.executeAgent() via relay.call('agent.exec', {...})
    //   - ProfileAwareAgentSpawner via relay.call('agent.exec', {...})
    case 'agent.exec': {
      const p = rpc.params ?? {}
      const binary = typeof p.binary === 'string' ? p.binary : ''
      const span = Tracers.agentOrchSpawn.start(
        { binary, taskId: typeof p.taskId === 'string' ? p.taskId : undefined },
        extractResume(p)
      )
      try {
        const { spawn } = await import('node:child_process')
        const args = Array.isArray(p.args) ? (p.args as unknown[]).map(String) : []
        const cwd = typeof p.cwd === 'string' ? p.cwd : config.workDir
        const stdin = typeof p.stdin === 'string' ? p.stdin : null
        const extraEnv =
          p.env && typeof p.env === 'object' && !Array.isArray(p.env)
            ? (p.env as Record<string, string>)
            : {}
        const timeoutMs =
          typeof p.timeoutMs === 'number'
            ? Math.min(Math.max(p.timeoutMs, 1_000), 5 * 60_000)
            : 300_000

        if (!binary) {
          span.fail('binary is required')
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'agent.exec: binary is required')
        }

        span.step('subprocess-spawn', { binary, cwd })
        const result = await new Promise<{
          stdout: string
          stderr: string
          exitCode: number | null
          timedOut: boolean
        }>((resolve) => {
          let stdout = '',
            stderr = '',
            timedOut = false,
            settled = false
          const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
          const child = spawn(binary, args, { cwd, env: spawnEnv, stdio: ['pipe', 'pipe', 'pipe'] })

          const finish = (r: typeof result): void => {
            if (settled) {
              return
            }
            settled = true
            clearTimeout(timer)
            resolve(r)
          }
          const timer = setTimeout(() => {
            timedOut = true
            try {
              child.kill('SIGKILL')
            } catch {
              /* ignore */
            }
            finish({ stdout, stderr, exitCode: null, timedOut })
          }, timeoutMs)

          child.stdout?.on('data', (d: Buffer) => {
            stdout += d.toString('utf8')
          })
          child.stderr?.on('data', (d: Buffer) => {
            stderr += d.toString('utf8')
          })
          child.on('error', (err) => {
            finish({ stdout, stderr: err.message, exitCode: null, timedOut })
          })
          child.on('close', (code) => {
            finish({ stdout, stderr, exitCode: code, timedOut })
          })

          if (stdin !== null) {
            child.stdin?.end(stdin)
          } else {
            child.stdin?.end()
          }
        })

        log.info(
          `agent.exec: binary=${binary} exitCode=${result.exitCode} timedOut=${result.timedOut}`
        )
        if (result.timedOut) {
          span.fail(`timeout after ${timeoutMs}ms`, { binary })
        } else if (result.exitCode !== 0) {
          span.fail(`exit code ${result.exitCode}`, { binary, exitCode: result.exitCode ?? -1 })
        } else {
          span.ok({ binary, exitCode: result.exitCode ?? 0 })
        }
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        span.fail(err, { binary })
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`)
      }
    }

    // ── agent.execPrompt ─────────────────────────────────────────────────────
    // One-shot, non-interactive AI-CLI invocation from a workflow/task-step
    // prompt request — distinct from agent.exec's generic "run this binary"
    // contract above (which agent.exec's real callers depend on unchanged).
    // Called by:
    //   - StepExecutors.executeAgent() via relay.call('agent.execPrompt', {...})
    //   - ProfileAwareAgentSpawner via relay.call('agent.execPrompt', {...})
    // See specs/agent/api/gaps-and-findings.md.
    case 'agent.execPrompt': {
      try {
        const { handleAgentExecPrompt } = await import('./agent-print-mode-exec')
        return (await handleAgentExecPrompt(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.execPrompt unavailable: ${msg}`)
      }
    }

    default:
      return null
  }
}
