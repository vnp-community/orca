// src/relay/shell-agent-extensions.ts
// Shell RPC handlers for Orca Dev Agent v5.0. Split out of
// fs-agent-extensions.ts (max-lines ratchet) — pure code move, no behavior
// change. Covers: shell.eval, shell.exec, shell.execStream.

import { spawn } from 'node:child_process'
import type WebSocket from 'ws'
import { encodeDataFrame } from 'orca-dev-agent-transport'
import type { WireState } from 'orca-dev-agent-transport'
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
  _config: AgentConfig
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
      if (stdout.length < SHELL_EXEC_MAX_OUTPUT_BYTES) {
        stdout += d.toString('utf8')
      } else {
        truncated = true
      }
    })
    child.stderr.on('data', (d: Buffer) => {
      if (stderr.length < SHELL_EXEC_MAX_OUTPUT_BYTES) {
        stderr += d.toString('utf8')
      } else {
        truncated = true
      }
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

// ─── shell.execStream ───────────────────────────────────────────────────────
// CR-TG-006: streaming sibling of handleShellExec above — same script/env/
// timeout setup, but delivers stdout/stderr incrementally via stream.chunk/
// stream.end wire frames (agent-git-handler.ts's handleGitExecStream
// convention) instead of a buffer-then-return response. Reused by the
// workflow 'shell' step executor for live output; StepExecutors.executeShell()'s
// relay.call('shell.exec', ...) contract and handleShellExec itself are
// untouched.
//
// Open Question 3 (chunking granularity) resolved consistently with
// handleAgentExecPromptStream in agent-print-mode-exec.ts: forward each raw
// stdout/stderr `data` event verbatim, no line-splitting — lower latency,
// and shell scripts commonly emit unterminated prompts/progress output that
// line-buffering would hold back.
//
// Open Question 1 (output-size cap) resolved: leave uncapped, relying on the
// caller/connection lifecycle — unlike handleShellExec, this handler never
// accumulates stdout/stderr into a buffer (each chunk is forwarded and
// discarded immediately), so SHELL_EXEC_MAX_OUTPUT_BYTES's memory-growth
// concern does not apply here; there is nothing to truncate.
export async function handleShellExecStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig
): Promise<void> {
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
  const span = fsTracer.start({ method: 'shell.execStream', scriptLen: script.length, traceId })

  if (!script) {
    span.fail('missing param: script', { method: 'shell.execStream' })
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: script' }
    })
    return
  }

  const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
  const child = spawn('sh', ['-c', script], { env: spawnEnv })

  function sendChunk(text: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      result: { type: 'stream.chunk', line: text, ...(source ? { source } : {}) }
    })
  }

  let settled = false
  const timer = setTimeout(() => {
    settled = true
    try {
      child.kill('SIGKILL')
    } catch {
      /* ignore */
    }
    span.fail('timed out', { timeoutMs })
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode: -1 } })
  }, timeoutMs)

  child.stdout.on('data', (d: Buffer) => sendChunk(d.toString('utf8')))
  child.stderr.on('data', (d: Buffer) => sendChunk(d.toString('utf8'), 'stderr'))
  child.on('close', (code) => {
    if (settled) {
      return
    }
    settled = true
    clearTimeout(timer)
    const exitCode = code ?? 0
    span.ok({ exitCode })
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } })
  })
  child.on('error', (err) => {
    if (settled) {
      return
    }
    settled = true
    clearTimeout(timer)
    span.fail(err)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    })
  })
}

function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}
