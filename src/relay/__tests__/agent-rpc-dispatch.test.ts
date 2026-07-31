// src/relay/__tests__/agent-rpc-dispatch.test.ts
import { describe, it, expect, vi } from 'vitest'
import { createRpcDispatcher, formatMcpResult, makeError } from '../agent-rpc-dispatch'
import type { ToolDefinition, ToolResult } from '../agent-tool-registry'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { createWireState, HEADER_SIZE } from '../agent-wire'

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
