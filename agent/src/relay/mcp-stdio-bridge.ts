// src/relay/mcp-stdio-bridge.ts
// Minimal MCP client over stdio (newline-delimited JSON-RPC) for mcp.callTool.
// Why our own client instead of an SDK: the agent bundle is self-contained and
// only needs initialize + tools/call; a dependency would ship far more.
// One child process per (server, projectDir), reused across calls and closed
// after it has been idle, so a workflow's frequent polls don't respawn Python.

import { spawn, type ChildProcess } from 'node:child_process'

export type McpServerSpec = {
  command: string
  args: string[]
  env?: Record<string, string>
}

export type McpToolCallOutcome =
  | { kind: 'result'; isError: boolean; payload: unknown }
  | {
      kind: 'failure'
      code: 'MCP_SERVER_NOT_INSTALLED' | 'MCP_SERVER_FAILED' | 'MCP_TIMEOUT'
      message: string
    }

type Pending = {
  resolve: (msg: Record<string, unknown>) => void
  reject: (err: Error) => void
  timer: NodeJS.Timeout
}

const PROTOCOL_VERSION = '2025-06-18'
export const DEFAULT_IDLE_MS = 10 * 60_000

class BridgeError extends Error {
  constructor(
    readonly code: 'MCP_SERVER_NOT_INSTALLED' | 'MCP_SERVER_FAILED' | 'MCP_TIMEOUT',
    message: string
  ) {
    super(message)
  }
}

/** One live MCP server process. */
export class McpStdioSession {
  private child: ChildProcess | null = null
  private buffer = ''
  private nextId = 1
  private readonly pending = new Map<number, Pending>()
  private ready: Promise<void> | null = null
  private stderrTail = ''
  closed = false

  constructor(
    private readonly spec: McpServerSpec,
    private readonly cwd: string,
    private readonly baseEnv: NodeJS.ProcessEnv
  ) {}

  private start(): Promise<void> {
    const child = spawn(this.spec.command, this.spec.args, {
      cwd: this.cwd,
      env: { ...this.baseEnv, ...this.spec.env },
      stdio: ['pipe', 'pipe', 'pipe'],
      windowsHide: true
    })
    this.child = child
    child.stdout?.setEncoding('utf8')
    child.stdout?.on('data', (chunk: string) => this.onData(chunk))
    child.stderr?.setEncoding('utf8')
    child.stderr?.on('data', (chunk: string) => {
      this.stderrTail = (this.stderrTail + chunk).slice(-2000)
    })
    const spawnFailed = new Promise<never>((_, reject) => {
      child.once('error', (err: NodeJS.ErrnoException) => {
        const code = err.code === 'ENOENT' ? 'MCP_SERVER_NOT_INSTALLED' : 'MCP_SERVER_FAILED'
        this.fail(new BridgeError(code, `${this.spec.command}: ${err.message}`))
        reject(new BridgeError(code, `${this.spec.command}: ${err.message}`))
      })
    })
    child.once('exit', (code, signal) => {
      this.fail(
        new BridgeError(
          'MCP_SERVER_FAILED',
          `MCP server exited (code ${code ?? 'null'}, signal ${signal ?? 'none'}): ${this.stderrTail.trim().slice(-500)}`
        )
      )
    })
    const handshake = (async () => {
      await this.request(
        'initialize',
        {
          protocolVersion: PROTOCOL_VERSION,
          capabilities: {},
          clientInfo: { name: 'orca-dev-server-agent', version: '1' }
        },
        30_000
      )
      this.write({ jsonrpc: '2.0', method: 'notifications/initialized' })
    })()
    return Promise.race([handshake, spawnFailed])
  }

  private onData(chunk: string): void {
    this.buffer += chunk
    let nl = this.buffer.indexOf('\n')
    while (nl >= 0) {
      const line = this.buffer.slice(0, nl).trim()
      this.buffer = this.buffer.slice(nl + 1)
      nl = this.buffer.indexOf('\n')
      if (!line) {
        continue
      }
      let msg: Record<string, unknown>
      try {
        msg = JSON.parse(line) as Record<string, unknown>
      } catch {
        continue // servers may log non-JSON lines; they are not protocol messages
      }
      const id = typeof msg.id === 'number' ? msg.id : null
      const waiter = id === null ? undefined : this.pending.get(id)
      if (waiter && ('result' in msg || 'error' in msg)) {
        this.pending.delete(id as number)
        clearTimeout(waiter.timer)
        waiter.resolve(msg)
      }
    }
  }

  private write(msg: Record<string, unknown>): void {
    this.child?.stdin?.write(`${JSON.stringify(msg)}\n`)
  }

  private request(
    method: string,
    params: Record<string, unknown>,
    timeoutMs: number
  ): Promise<Record<string, unknown>> {
    const id = this.nextId++
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        reject(new BridgeError('MCP_TIMEOUT', `${method} did not answer within ${timeoutMs} ms`))
      }, timeoutMs)
      this.pending.set(id, { resolve, reject, timer })
      this.write({ jsonrpc: '2.0', id, method, params })
    })
  }

  private fail(err: Error): void {
    this.closed = true
    for (const [id, waiter] of this.pending) {
      clearTimeout(waiter.timer)
      waiter.reject(err)
      this.pending.delete(id)
    }
  }

  async callTool(
    name: string,
    args: Record<string, unknown>,
    timeoutMs: number
  ): Promise<McpToolCallOutcome> {
    try {
      this.ready ??= this.start()
      await this.ready
      const msg = await this.request('tools/call', { name, arguments: args }, timeoutMs)
      if (msg.error && typeof msg.error === 'object') {
        const e = msg.error as { message?: string }
        return {
          kind: 'result',
          isError: true,
          payload: { error: { code: 'MCP_PROTOCOL_ERROR', message: e.message ?? 'protocol error' } }
        }
      }
      return { kind: 'result', ...parseToolResult(msg.result) }
    } catch (err) {
      if (err instanceof BridgeError) {
        if (err.code === 'MCP_TIMEOUT') {
          this.close() // the server may still be busy with the call; a fresh process is safer than a confused one
        }
        return { kind: 'failure', code: err.code, message: err.message }
      }
      return {
        kind: 'failure',
        code: 'MCP_SERVER_FAILED',
        message: err instanceof Error ? err.message : String(err)
      }
    }
  }

  close(): void {
    this.fail(new BridgeError('MCP_SERVER_FAILED', 'session closed'))
    if (this.child && this.child.exitCode === null) {
      this.child.kill('SIGTERM')
    }
    this.child = null
  }
}

/** tools/call result -> parsed JSON of its text content (tdd-agent returns one JSON text block). */
export function parseToolResult(result: unknown): { isError: boolean; payload: unknown } {
  const r = (result ?? {}) as {
    isError?: boolean
    content?: { type?: string; text?: string }[]
    structuredContent?: unknown
  }
  const isError = r.isError === true
  if (r.structuredContent !== undefined) {
    return { isError, payload: r.structuredContent }
  }
  const text = (r.content ?? [])
    .filter((c) => c.type === 'text')
    .map((c) => c.text ?? '')
    .join('')
  try {
    return { isError, payload: JSON.parse(text) }
  } catch {
    return { isError, payload: text }
  }
}

/** Sessions keyed by server + projectDir, closed when idle. */
export class McpSessionPool {
  private readonly sessions = new Map<
    string,
    { session: McpStdioSession; idle: NodeJS.Timeout | null }
  >()

  constructor(
    private readonly baseEnv: NodeJS.ProcessEnv,
    private readonly idleMs = DEFAULT_IDLE_MS,
    private readonly factory = (spec: McpServerSpec, cwd: string, env: NodeJS.ProcessEnv) =>
      new McpStdioSession(spec, cwd, env)
  ) {}

  async call(
    server: string,
    spec: McpServerSpec,
    projectDir: string,
    tool: string,
    args: Record<string, unknown>,
    timeoutMs: number
  ): Promise<McpToolCallOutcome> {
    const key = `${server}\u0000${projectDir}`
    let entry = this.sessions.get(key)
    if (!entry || entry.session.closed) {
      entry = { session: this.factory(spec, projectDir, this.baseEnv), idle: null }
      this.sessions.set(key, entry)
    }
    if (entry.idle) {
      clearTimeout(entry.idle)
      entry.idle = null
    }
    const outcome = await entry.session.callTool(tool, args, timeoutMs)
    const current = entry
    current.idle = setTimeout(() => {
      current.session.close()
      if (this.sessions.get(key) === current) {
        this.sessions.delete(key)
      }
    }, this.idleMs)
    current.idle.unref?.()
    return outcome
  }

  size(): number {
    return this.sessions.size
  }

  closeAll(): void {
    for (const { session, idle } of this.sessions.values()) {
      if (idle) {
        clearTimeout(idle)
      }
      session.close()
    }
    this.sessions.clear()
  }
}
