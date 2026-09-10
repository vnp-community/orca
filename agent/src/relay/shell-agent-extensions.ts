// src/relay/shell-agent-extensions.ts
// Shell RPC handlers for Orca Dev Agent v5.0: shell.eval, shell.exec.
// Split out of fs-agent-extensions.ts to stay under the repo's 300-line max-lines ratchet.

import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const fsTracer = createTracer('agent:fs')

// ─── shell.eval ───────────────────────────────────────────────────────────────
// Executes a short shell command and returns stdout + stderr.
// Used internally by devServer.browseDir to resolve '~' → real home directory.
// SECURITY: only reachable via the authenticated agent relay, never from browser.

export async function handleShellEval(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig
): Promise<object> {
  const command = typeof params.command === 'string' ? params.command : ''
  const timeoutMs = typeof params.timeout === 'number' ? Math.min(params.timeout, 10_000) : 5_000
  const span = fsTracer.start({ method: 'shell.eval', cmd: command.slice(0, 80) })

  if (!command) {
    span.fail('missing param: command', { method: 'shell.eval' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: command' }
    }
  }

  return new Promise((resolve) => {
    let stdout = ''
    let stderr = ''
    const child = spawn('sh', ['-c', command], { env: process.env })
    const timer = setTimeout(() => {
      child.kill()
      span.fail('timed out', { cmd: command.slice(0, 80), timeoutMs })
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: 'shell.eval timed out' }
      })
    }, timeoutMs)

    child.stdout.on('data', (d: Buffer) => {
      stdout += d.toString()
    })
    child.stderr.on('data', (d: Buffer) => {
      stderr += d.toString()
    })
    child.on('close', (code) => {
      clearTimeout(timer)
      span.ok({ exitCode: code ?? 0, stdoutLen: stdout.length })
      resolve({ jsonrpc: '2.0', id, result: { stdout, stderr, exitCode: code ?? 0 } })
    })
    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err, { cmd: command.slice(0, 80) })
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: err.message }
      })
    })
  })
}

// ─── shell.exec ───────────────────────────────────────────────────────────────
// Executes a shell script for workflow 'shell' steps and returns captured
// stdout/stderr/exitCode. Distinct from shell.eval above (no env/timeout
// params, internal-only '~' resolution) — shell.exec is the RPC method
// StepExecutors.executeShell() calls via relay.call('shell.exec',
// { script, env, traceId }); it had no agent-side handler until this fix
// (specs/agent/api/gaps-and-findings.md #1 — confirmed by the previously
// MethodNotFound-asserting test in agent-rpc-dispatch.test.ts).
// SECURITY: only reachable via the authenticated agent relay, never from browser.

const SHELL_EXEC_DEFAULT_TIMEOUT_MS = 300_000 // 5 minutes, matches agent.exec's default
const SHELL_EXEC_MAX_TIMEOUT_MS = 300_000
const SHELL_EXEC_MAX_OUTPUT_BYTES = 4 * 1024 * 1024 // matches agent.execNonInteractive's cap

export async function handleShellExec(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  notify?: (method: string, params: Record<string, unknown>) => void
): Promise<object> {
  const script = typeof params.script === 'string' ? params.script : ''
  const traceId = typeof params.traceId === 'string' ? params.traceId : undefined
  const extraEnv =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : {}
  const timeoutMs =
    typeof params.timeoutMs === 'number'
      ? Math.min(Math.max(params.timeoutMs, 1_000), SHELL_EXEC_MAX_TIMEOUT_MS)
      : SHELL_EXEC_DEFAULT_TIMEOUT_MS
  const span = fsTracer.start({ method: 'shell.exec', scriptLen: script.length, traceId })

  if (!script) {
    span.fail('missing param: script', { method: 'shell.exec' })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: script' }
    }
  }

  return new Promise((resolve) => {
    let stdout = ''
    let stderr = ''
    let truncated = false
    const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
    const child = spawn('sh', ['-c', script], { env: spawnEnv })
    const timer = setTimeout(() => {
      try {
        child.kill('SIGKILL')
      } catch {
        /* ignore */
      }
      span.fail('timed out', { timeoutMs })
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: 'shell.exec timed out' }
      })
    }, timeoutMs)

    child.stdout.on('data', (d: Buffer) => {
      const chunk = d.toString('utf8')
      if (stdout.length < SHELL_EXEC_MAX_OUTPUT_BYTES) {
        stdout += chunk
      } else {
        truncated = true
      }
      notify?.('shell.exec.output', { traceId, stream: 'stdout', data: chunk })
    })
    child.stderr.on('data', (d: Buffer) => {
      const chunk = d.toString('utf8')
      if (stderr.length < SHELL_EXEC_MAX_OUTPUT_BYTES) {
        stderr += chunk
      } else {
        truncated = true
      }
      notify?.('shell.exec.output', { traceId, stream: 'stderr', data: chunk })
    })
    child.on('close', (code) => {
      clearTimeout(timer)
      span.ok({ exitCode: code ?? 0, stdoutLen: stdout.length, truncated })
      resolve({
        jsonrpc: '2.0',
        id,
        result: { stdout, stderr, exitCode: code ?? 0, ...(truncated ? { truncated: true } : {}) }
      })
    })
    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err)
      resolve({
        jsonrpc: '2.0',
        id,
        error: { code: AgentErrorCode.ServerError, message: err.message }
      })
    })
  })
}
