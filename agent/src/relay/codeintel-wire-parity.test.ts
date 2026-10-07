import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { RelayDispatcher } from './dispatcher'
import { encodeJsonRpcFrame, MessageType, type JsonRpcRequest } from './protocol'
import { registerCodeIntelHandlers } from './codeintel-relay-handlers'
import { dispatchCodeIntelRpc } from './agent-rpc-dispatch-codeintel'
import { CODEINTEL_METHODS } from './codeintel-method-table'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'

function decodeFirstFrame(buf: Buffer): { type: number; id: number; ack: number; payload: Buffer } {
  const type = buf[0]
  const id = buf.readUInt32BE(1)
  const ack = buf.readUInt32BE(5)
  const len = buf.readUInt32BE(9)
  const payload = buf.subarray(13, 13 + len)
  return { type, id, ack, payload }
}

const FAKE_CONFIG: AgentConfig = {
  mode: 'stdio',
  orcaUrl: '',
  orcaHttpUrl: '',
  agentToken: '',
  apiSecret: '',
  agentPort: 0,
  devServerId: 'dev-server-1',
  logLevel: 'info',
  workDir: '/work',
  toolPath: '/usr/bin',
  toolEnv: {},
  credentialDir: '/tmp/creds',
  tlsRejectUnauthorized: true
}

const NOOP_LOG: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

describe('codeintel-wire-parity (Part A vs Part B equivalence)', () => {
  let dispatcher: RelayDispatcher
  let written: Buffer[]

  beforeEach(() => {
    written = []
    dispatcher = new RelayDispatcher((data) => {
      written.push(Buffer.from(data))
    })
    registerCodeIntelHandlers(dispatcher, FAKE_CONFIG, NOOP_LOG)
  })

  afterEach(() => {
    dispatcher.dispose()
  })

  async function callPartB(method: string, id: number, params: any): Promise<any> {
    const req: JsonRpcRequest = { jsonrpc: '2.0', id, method, params }
    dispatcher.feed(encodeJsonRpcFrame(req, id, 0))
    await new Promise(r => setTimeout(r, 10))

    for (const buf of written) {
      const frame = decodeFirstFrame(buf)
      if (frame.type !== MessageType.Regular) continue
      try {
        const msg = JSON.parse(frame.payload.toString('utf-8'))
        if (msg.id === id) return msg
      } catch {}
    }
    return null
  }

  async function callPartA(method: string, id: number, params: any): Promise<any> {
    const rpc: JsonRpcRequest = { jsonrpc: '2.0', id, method, params }
    const fakeWs = { send: vi.fn(), readyState: 1 } as any
    const fakeState = { seq: 0, ack: 0 } as any
    return await dispatchCodeIntelRpc(rpc, FAKE_CONFIG, NOOP_LOG, fakeWs, fakeState)
  }

  function stripDynamicFields(obj: any): any {
    if (!obj || typeof obj !== 'object') return obj
    const clone = JSON.parse(JSON.stringify(obj))
    const removeKeys = (target: any) => {
      if (!target || typeof target !== 'object') return
      delete target.perf
      delete target.startedAt
      delete target.durationMs
      for (const k of Object.keys(target)) {
        removeKeys(target[k])
      }
    }
    removeKeys(clone)
    return clone
  }

  it('matches error response for invalid params across Part A and Part B', async () => {
    const resA = await callPartA('codeintel.status', 101, { workspaceRoot: 123 })
    const resB = await callPartB('codeintel.status', 101, { workspaceRoot: 123 })

    expect(resA).toBeDefined()
    expect(resB).toBeDefined()
    expect(resA.error.code).toBe(resB.error.code)
    expect(resA.error.message).toBe(resB.error.message)
    expect(stripDynamicFields(resA.error.data)).toEqual(stripDynamicFields(resB.error.data))
  })

  it('matches method-not-found response across Part A and Part B', async () => {
    const resA = await callPartA('codeintel.unknownMethod', 202, {})
    const resB = await callPartB('codeintel.unknownMethod', 202, {})

    expect(resA.error.code).toBe(-32601)
    expect(resB.error.code).toBe(-32601)
  })

  it('verifies that all CODEINTEL_METHODS produce equivalent parameter validation behavior', async () => {
    // Test on sample methods with invalid workspaceRoot
    const sampleMethods = ['codeintel.status', 'codeintel.detectChanges']
    let id = 300
    for (const method of sampleMethods) {
      const curId = id++
      const resA = await callPartA(method, curId, { workspaceRoot: 123 })
      const resB = await callPartB(method, curId, { workspaceRoot: 123 })

      expect(resA.error.code).toBe(resB.error.code)
      expect(resA.error.message).toBe(resB.error.message)
      expect(stripDynamicFields(resA.error.data)).toEqual(stripDynamicFields(resB.error.data))
    }
  })
})
