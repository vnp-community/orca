// src/relay/__tests__/agent-rpc-dispatch.test.ts
import { describe, it, expect, vi } from 'vitest'
import { createRpcDispatcher, formatMcpResult, makeError } from '../agent-rpc-dispatch'
import type { ToolDefinition, ToolResult } from '../agent-tool-registry'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { createWireState, HEADER_SIZE } from '../agent-wire'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import { AgentErrorCode } from '../../shared/agent-wire-protocol'

// ─── Mock WebSocket ──────────────────────────────────────────────────────────
class MockWs {
  readyState = 1  // WebSocket.OPEN
  sent: Buffer[] = []
  send = vi.fn((data: Buffer) => { this.sent.push(data) })
}

function lastResponseJson(ws: MockWs): Record<string, unknown> {
  const last = ws.sent.at(-1)!
  return JSON.parse(last.subarray(HEADER_SIZE).toString('utf8'))
}

// ─── Mock tools ──────────────────────────────────────────────────────────────
const echoTool: ToolDefinition = {
  name: 'echo',
  binary: null,
  description: 'Echo the msg param',
  inputSchema: { type: 'object', properties: { msg: { type: 'string', description: 'message' } } },
  async handler(params): Promise<ToolResult> {
    return { stdout: String(params.msg ?? ''), stderr: '', exitCode: 0 }
  },
}

const failTool: ToolDefinition = {
  name: 'fails',
  binary: null,
  description: 'Throws an error',
  inputSchema: { type: 'object', properties: {} },
  async handler(): Promise<ToolResult> { throw new Error('handler kaboom') },
}

const exitOneTool: ToolDefinition = {
  name: 'exitone',
  binary: null,
  description: 'Returns exitCode=1',
  inputSchema: { type: 'object', properties: {} },
  async handler(): Promise<ToolResult> {
    return { stdout: '', stderr: 'error output', exitCode: 1 }
  },
}

const mockConfig = { workDir: '/tmp', toolEnv: {} } as unknown as AgentConfig
const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

// ─── tests/list ─────────────────────────────────────────────────────────────
describe('tools/list', () => {
  it('returns all registered tools', async () => {
    const d  = createRpcDispatcher([echoTool, failTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 1, method: 'tools/list' })
    const resp = lastResponseJson(ws) as any
    expect(resp.result.tools).toHaveLength(2)
  })

  it('tool entry has name, description, inputSchema', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 1, method: 'tools/list' })
    const resp = lastResponseJson(ws) as any
    const t = resp.result.tools[0]
    expect(t.name).toBe('echo')
    expect(t.description).toBeTruthy()
    expect(t.inputSchema).toBeDefined()
  })

  it('returns empty tools array when no tools registered', async () => {
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 2, method: 'tools/list' })
    const resp = lastResponseJson(ws) as any
    expect(resp.result.tools).toHaveLength(0)
  })

  it('response id matches request id', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 99, method: 'tools/list' })
    expect(lastResponseJson(ws).id).toBe(99)
  })
})

// ─── tools/call ──────────────────────────────────────────────────────────────
describe('tools/call', () => {
  it('calls handler with parsed arguments', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 2, method: 'tools/call',
      params: { name: 'echo', arguments: { msg: 'hello-world' } },
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.result.content[0].text).toContain('hello-world')
  })

  it('returns MethodNotFound (-32601) for unknown tool name', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 3, method: 'tools/call',
      params: { name: 'nonexistent' },
    })
    expect((lastResponseJson(ws) as any).error.code).toBe(-32601)
  })

  it('returns ServerError (-32000) when handler throws — no crash', async () => {
    const d  = createRpcDispatcher([failTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 4, method: 'tools/call',
      params: { name: 'fails', arguments: {} },
    })
    expect((lastResponseJson(ws) as any).error.code).toBe(-32000)
  })

  it('sets isError=true when exitCode != 0', async () => {
    const d  = createRpcDispatcher([exitOneTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 5, method: 'tools/call',
      params: { name: 'exitone', arguments: {} },
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.result.isError).toBe(true)
  })

  it('passes null id correctly in response', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: null, method: 'tools/call',
      params: { name: 'echo', arguments: { msg: 'x' } },
    })
    expect(lastResponseJson(ws).id).toBeNull()
  })
})

// ─── unknown method ───────────────────────────────────────────────────────────
describe('unknown method', () => {
  it('returns MethodNotFound (-32601)', async () => {
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 5, method: 'nonexistent.method' })
    expect((lastResponseJson(ws) as any).error.code).toBe(-32601)
  })

  it('response id matches request id for unknown method', async () => {
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 77, method: 'bad' })
    expect(lastResponseJson(ws).id).toBe(77)
  })
})

// ─── ws.readyState guard ─────────────────────────────────────────────────────
describe('ws.readyState guard', () => {
  it('does NOT call ws.send when readyState != 1 (CLOSED)', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    ws.readyState = 3  // CLOSED
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 6, method: 'tools/list' })
    expect(ws.send).not.toHaveBeenCalled()
  })

  it('DOES call ws.send when readyState == 1 (OPEN)', async () => {
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    ws.readyState = 1
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 7, method: 'tools/list' })
    expect(ws.send).toHaveBeenCalledOnce()
  })
})

// ─── formatMcpResult ─────────────────────────────────────────────────────────
describe('formatMcpResult', () => {
  it('content[0].type is "text"', () => {
    const r = formatMcpResult(1, { stdout: 'out', stderr: '', exitCode: 0 }) as any
    expect(r.result.content[0].type).toBe('text')
  })

  it('isError=false when exitCode=0', () => {
    const r = formatMcpResult(1, { stdout: 'x', stderr: '', exitCode: 0 }) as any
    expect(r.result.isError).toBe(false)
  })

  it('isError=true when exitCode=1', () => {
    const r = formatMcpResult(1, { stdout: '', stderr: 'err', exitCode: 1 }) as any
    expect(r.result.isError).toBe(true)
  })

  it('includes "[stderr]" prefix when stderr is non-empty', () => {
    const r = formatMcpResult(1, { stdout: 'out', stderr: 'err msg', exitCode: 0 }) as any
    expect(r.result.content[0].text).toContain('[stderr]')
    expect(r.result.content[0].text).toContain('err msg')
  })

  it('uses "(no output)" when both stdout and stderr are empty', () => {
    const r = formatMcpResult(1, { stdout: '', stderr: '', exitCode: 0 }) as any
    expect(r.result.content[0].text).toBe('(no output)')
  })

  it('preserves exitCode in result', () => {
    const r = formatMcpResult(1, { stdout: '', stderr: '', exitCode: 42 }) as any
    expect(r.result.exitCode).toBe(42)
  })

  it('includes meta when provided', () => {
    const r = formatMcpResult(1, { stdout: '', stderr: '', exitCode: 0, meta: { path: '/foo' } }) as any
    expect(r.result.meta?.path).toBe('/foo')
  })
})

// ─── makeError ───────────────────────────────────────────────────────────────
describe('makeError', () => {
  it('returns valid JSON-RPC error structure', () => {
    const e = makeError(1, -32601, 'Not found') as any
    expect(e.jsonrpc).toBe('2.0')
    expect(e.id).toBe(1)
    expect(e.error.code).toBe(-32601)
    expect(e.error.message).toBe('Not found')
  })

  it('includes data field when provided', () => {
    const e = makeError(1, -32000, 'err', { detail: 'x' }) as any
    expect(e.error.data?.detail).toBe('x')
  })

  it('does not include data field when not provided', () => {
    const e = makeError(1, -32000, 'err') as any
    expect('data' in e.error).toBe(false)
  })
})

// ─── dispatch() — trace resume (CR-TRACE-000) ────────────────────────────────
describe('dispatch() — trace resume', () => {
  it('resumes agent:rpc span id from params._trace.id', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'tools/list', params: { _trace: { id: 'resumed-rpc-1' } },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')
    expect(start?.id).toBe('resumed-rpc-1')
  })

  it('generates a new span id when params._trace is absent', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const d  = createRpcDispatcher([echoTool], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), { jsonrpc: '2.0', id: 1, method: 'tools/list' })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')
    expect(start?.id).toBeTruthy()
    expect(start?.id).not.toBe('resumed-rpc-1')
  })
})

// ─── agent.exec — extractTraceFields (CR-TRACE-015) ──────────────────────────
describe('agent.exec — extractTraceFields (CR-TRACE-015)', () => {
  it('start event includes binary/argsCount/hasEnvOverride/timeoutMs, not session/cmd', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: ['hi'], cwd: '/tmp', env: { FOO: 'bar' }, timeoutMs: 5000 },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.binary).toBe('echo')
    expect(start.fields.argsCount).toBe(1)
    expect(start.fields.hasEnvOverride).toBe(true)
    expect(start.fields.timeoutMs).toBe(5000)
    expect(start.fields.session).toBeUndefined()
  })

  it('ok event includes exitCode and timedOut from the agent.exec result', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: process.execPath, args: ['-e', 'process.exit(0)'] },
    })
    unregister()
    const ok = events.find(e => e.flow === 'agent:rpc' && e.level === 'ok')!
    expect(ok.fields.exitCode).toBe(0)
    expect(ok.fields.timedOut).toBe(false)
  })

  it('agent.spawn (interactive) still uses the legacy session/cmd bucket, unaffected', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.spawn',
      params: { taskId: 'task-1', modelId: 'unknown-model-xyz', userId: 'u1', accountId: 'a1' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    // Legacy 'agent.' bucket (session/binary/cmd) still applies — unaffected by
    // the new dedicated agent.exec bucket, which is checked BEFORE this one.
    expect(start.fields.session).toBe('task-1')
    // argsCount/hasEnvOverride only exist on the new agent.exec bucket, never
    // set by the legacy 'agent.' bucket used for agent.spawn.
    expect(start.fields.argsCount).toBeUndefined()
    expect(start.fields.hasEnvOverride).toBeUndefined()
  })

  it('agent.exec without env param → hasEnvOverride is false, not undefined-vs-false ambiguity', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: ['hi'] },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.hasEnvOverride).toBe(false)
  })
})

// ─── agent.exec — stepId / parentTraceId (CR-TRACE-017) ──────────────────────
describe('agent.exec — stepId / parentTraceId (CR-TRACE-017)', () => {
  it('surfaces stepId when StepExecutors sends it', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp', stepId: 'step-42' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.stepId).toBe('step-42')
  })

  it('surfaces parentTraceId when present (forward-compat with future backend change)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp', parentTraceId: 'root-abc123' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.parentTraceId).toBe('root-abc123')
  })

  it('omits stepId/parentTraceId cleanly for non-workflow agent.exec callers (Profile/Task Graph)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.stepId).toBeUndefined()
    expect(start.fields.parentTraceId).toBeUndefined()
  })
})

// ─── shell.exec (CR-TRACE-017 gap closed — specs/agent/api/gaps-and-findings.md #1) ──
describe('shell.exec', () => {
  it('executes the script and returns stdout/exitCode', async () => {
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'shell.exec', params: { script: 'echo hi' },
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.error).toBeUndefined()
    expect(resp.result.exitCode).toBe(0)
    expect(resp.result.stdout.trim()).toBe('hi')
  })

  it('returns InvalidParams when script is missing', async () => {
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'shell.exec', params: {},
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.error.code).toBe(AgentErrorCode.InvalidParams)
  })
})

// ─── notification.send (CR-TRACE-017 gap closed — specs/agent/api/gaps-and-findings.md #1) ──
describe('notification.send', () => {
  it('acknowledges the notification without erroring', async () => {
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'notification.send', params: { message: 'hi' },
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.error).toBeUndefined()
    expect(resp.result.ok).toBe(true)
    expect(typeof resp.result.delivered).toBe('boolean')
  })

  it('returns InvalidParams when message is missing', async () => {
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'notification.send', params: {},
    })
    const resp = lastResponseJson(ws) as any
    expect(resp.error.code).toBe(AgentErrorCode.InvalidParams)
  })
})

// ─── ai.complete — extractTraceFields (CR-TRACE-018) ─────────────────────────
describe('ai.complete — extractTraceFields (CR-TRACE-018)', () => {
  it('surfaces model/taskId/promptLength on the agent:rpc dispatch span', async () => {
    // No provider API key in env → handleAIComplete fails fast (no network
    // call) — irrelevant here since the agent:rpc 'start' event is emitted by
    // dispatch() BEFORE route() runs, from extractTraceFields() alone.
    vi.stubEnv('ANTHROPIC_API_KEY', '')
    vi.stubEnv('OPENAI_API_KEY', '')
    vi.stubEnv('GOOGLE_API_KEY', '')
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'ai.complete',
      params: { prompt: 'hello world', taskId: 'task-77', model: 'claude-opus-4-5' },
    })
    unregister()
    vi.unstubAllEnvs()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.model).toBe('claude-opus-4-5')
    expect(start.fields.taskId).toBe('task-77')
    expect(start.fields.promptLength).toBe('hello world'.length)
  })
})

// ─── agent.exec — taskId (CR-TRACE-018, forward-compat) ──────────────────────
describe('agent.exec — taskId (CR-TRACE-018, forward-compat)', () => {
  it('surfaces taskId when present in params', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp', taskId: 'task-99' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.taskId).toBe('task-99')
  })

  it('omits taskId cleanly when absent (current ProfileAwareAgentSpawner behavior)', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    // Matches today's real backend payload: ProfileAwareAgentSpawner.spawn()
    // only sends { binary, args, cwd, env, timeoutMs } — no top-level taskId.
    await dispatcher.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: [], cwd: '/tmp' },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.taskId).toBeUndefined()
  })
})

// ─── case 'agent.exec' — agentOrch:spawn (CR-TRACE-002) ──────────────────────
describe("case 'agent.exec' — agentOrch:spawn", () => {
  it('emits agentOrch:spawn span with ok() containing exitCode on success', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: process.execPath, args: ['-e', 'process.exit(0)'] },
    })
    unregister()
    const ok = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'ok')
    expect(ok?.fields.exitCode).toBe(0)
  })

  it('emits fail() when binary is missing', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec', params: {},
    })
    unregister()
    const fail = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'fail')
    expect(fail?.fields.err).toBe('binary is required')
  })

  it('emits fail() with timeout field when subprocess times out', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const d  = createRpcDispatcher([], mockConfig, mockLog)
    const ws = new MockWs()
    await d.dispatch(ws as any, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: process.execPath, args: ['-e', 'setTimeout(() => {}, 5000)'], timeoutMs: 1000 },
    })
    unregister()
    const fail = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'fail')
    expect(fail?.fields.err).toContain('timeout')
  }, 10_000)
})
