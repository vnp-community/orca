import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { RelayDispatcher } from './dispatcher'
import { encodeJsonRpcFrame, MessageType, type JsonRpcRequest } from './protocol'
import { registerCodeIntelHandlers, toRelayThrowable } from './codeintel-relay-handlers'
import { CodeIntelError } from './codeintel-errors'
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

describe('registerCodeIntelHandlers (RelayDispatcher / Part B)', () => {
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

  function sendRequest(method: string, id: number, params: any = {}): void {
    const req: JsonRpcRequest = { jsonrpc: '2.0', id, method, params }
    dispatcher.feed(encodeJsonRpcFrame(req, id, 0))
  }

  function findResponseFor(id: number): { id: number; result?: unknown; error?: { code: number; message: string; data?: any } } | undefined {
    for (const buf of written) {
      const frame = decodeFirstFrame(buf)
      if (frame.type !== MessageType.Regular) {
        continue
      }
      try {
        const msg = JSON.parse(frame.payload.toString('utf-8'))
        if (msg.id === id && ('result' in msg || 'error' in msg)) {
          return msg
        }
      } catch {
        continue
      }
    }
    return undefined
  }

  it('toRelayThrowable preserves error code and data payload', () => {
    const err = new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'Invalid param', { reason: 'base_required' })
    const throwable = toRelayThrowable(err)
    expect(throwable.message).toBe('Invalid param')
    expect(throwable.code).toBe(-32602)
    expect(throwable.data).toEqual({
      code: 'CODEINTEL_INVALID_PARAMS',
      reason: 'base_required'
    })
  })

  it('registers all methods defined in CODEINTEL_METHODS', () => {
    for (const methodName of Object.keys(CODEINTEL_METHODS)) {
      expect((dispatcher as any).requestHandlers.has(methodName)).toBe(true)
    }
  })

  it('returns -32601 Method Not Found for unknown codeintel methods', async () => {
    sendRequest('codeintel.nonexistentMethod', 99)
    await new Promise(r => setTimeout(r, 100))

    const resp = findResponseFor(99)
    expect(resp?.error?.code).toBe(-32601)
  })

  it('handles invalid params by returning an error with code -32602', async () => {
    sendRequest('codeintel.status', 1, { workspaceRoot: 123 })
    await new Promise(r => setTimeout(r, 100))

    const resp = findResponseFor(1)
    expect(resp?.error?.code).toBe(-32602)
  })
})
