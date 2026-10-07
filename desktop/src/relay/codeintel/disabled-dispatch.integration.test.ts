import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createRpcDispatcher } from '../agent-rpc-dispatch'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { buildCapabilities } from '../agent-session-capabilities'
import { createWireState, HEADER_SIZE } from 'orca-dev-agent-transport'
import { createFakeCli } from './fake-codeintel-cli'

class MockWs {
  readyState = 1
  sent: Buffer[] = []
  send = vi.fn((data: Buffer, cb?: (err?: Error) => void) => {
    this.sent.push(data)
    if (cb) cb()
  })
}

function decodeResponse(ws: MockWs): any {
  const last = ws.sent.at(-1)
  if (!last) throw new Error('No frame sent')
  return JSON.parse(last.subarray(HEADER_SIZE).toString('utf8'))
}

describe('disabled-dispatch.integration', () => {
  let tmpDir: string
  let savedEnv: NodeJS.ProcessEnv

  const mockLog: AgentLogger = {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn()
  }

  beforeEach(() => {
    savedEnv = { ...process.env }
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'disabled-dispatch-test-'))
  })

  afterEach(() => {
    process.env = savedEnv
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    } catch {}
  })

  describe('When ORCA_CODEINTEL_DISABLED=1 (Full Kill Switch)', () => {
    it('rejects codeintel.symbol with -32000 and reason codeintel_disabled', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '1'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-symbol-1',
        method: 'codeintel.symbol',
        params: { workspaceRoot: tmpDir, symbol: 'TestSymbol' }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-symbol-1')
      expect(res.error).toBeDefined()
      expect(res.error.code).toBe(-32000)
      expect(res.error.data).toEqual({
        code: 'CODEINTEL_TOOL_UNAVAILABLE',
        reason: 'codeintel_disabled'
      })
    })

    it('rejects codeintel.reindex with -32000 and reason codeintel_disabled', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '1'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-reindex-1',
        method: 'codeintel.reindex',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-reindex-1')
      expect(res.error).toBeDefined()
      expect(res.error.code).toBe(-32000)
      expect(res.error.data).toEqual({
        code: 'CODEINTEL_TOOL_UNAVAILABLE',
        reason: 'codeintel_disabled'
      })
    })

    it('rejects quality.run with -32000 and reason codeintel_disabled', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '1'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-quality-1',
        method: 'quality.run',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-quality-1')
      expect(res.error).toBeDefined()
      expect(res.error.code).toBe(-32000)
      expect(res.error.data).toEqual({
        code: 'CODEINTEL_TOOL_UNAVAILABLE',
        reason: 'codeintel_disabled'
      })
    })

    it('allows codeintel.status and reports warning codeintel_disabled', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '1'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-status-1',
        method: 'codeintel.status',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-status-1')
      expect(res.error).toBeUndefined()
      expect(res.result).toBeDefined()
      expect(res.result.warnings).toContain('codeintel_disabled')
      expect(res.result.tools.gitnexus.available).toBe(false)
      expect(res.result.tools.codegraph.available).toBe(false)
    })

    it('removes codeintel capabilities from handshake', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '1'
        }
      } as any

      const caps = await buildCapabilities(config, mockLog)
      expect(caps).not.toContain('codeintel')
      expect(caps).not.toContain('codeintel.gitnexus')
      expect(caps).not.toContain('codeintel.codegraph')
      expect(caps).not.toContain('quality')
    })
  })

  describe('Selective switches: reindex and quality', () => {
    it('rejects codeintel.reindex when ORCA_CODEINTEL_REINDEX=off', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '0',
          ORCA_CODEINTEL_REINDEX: 'off'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-reindex-off',
        method: 'codeintel.reindex',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-reindex-off')
      expect(res.error).toBeDefined()
      expect(res.error.code).toBe(-32000)
      expect(res.error.data).toEqual({
        code: 'CODEINTEL_TOOL_UNAVAILABLE',
        reason: 'reindex_disabled'
      })
    })

    it('rejects quality.run when ORCA_QUALITY_RUN=off', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '0',
          ORCA_QUALITY_RUN: 'off'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-quality-off',
        method: 'quality.run',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-quality-off')
      expect(res.error).toBeDefined()
      expect(res.error.code).toBe(-32000)
      expect(res.error.data).toEqual({
        code: 'CODEINTEL_TOOL_UNAVAILABLE',
        reason: 'quality_disabled'
      })
    })
  })

  describe('When kill switch is OFF (Enabled)', () => {
    it('codeintel.status returns normally without codeintel_disabled warning', async () => {
      const config: AgentConfig = {
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '0'
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-status-on',
        method: 'codeintel.status',
        params: { workspaceRoot: tmpDir }
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-status-on')
      expect(res.error).toBeUndefined()
      expect(res.result).toBeDefined()
      expect(res.result.warnings ?? []).not.toContain('codeintel_disabled')
    })

    it('executes through method handler rather than disabled gate when enabled', async () => {
      // Create fake cli to verify execution
      const binDir = path.join(tmpDir, 'bin')
      fs.mkdirSync(binDir, { recursive: true })
      createFakeCli({
        toolName: 'gitnexus',
        dir: binDir,
        delayMs: 10,
        stdout: JSON.stringify({ symbols: [{ name: 'TestSymbol', path: 'src/index.ts' }] })
      })

      const config: AgentConfig = {
        toolPath: binDir,
        toolEnv: {
          ORCA_CODEINTEL_DISABLED: '0',
          PATH: `${binDir}:${process.env.PATH}`
        }
      } as any

      const ws = new MockWs()
      const wireState = createWireState()
      const dispatcher = createRpcDispatcher([], config, mockLog)

      // Test with missing workspaceRoot to verify params validator runs (proves it passed disabled gate)
      await dispatcher.dispatch(ws as any, wireState, {
        jsonrpc: '2.0',
        id: 'req-symbol-validator',
        method: 'codeintel.symbol',
        params: {}
      })

      const res = decodeResponse(ws)
      expect(res.id).toBe('req-symbol-validator')
      // It should NOT be codeintel_disabled, but CODEINTEL_INVALID_PARAMS
      expect(res.error).toBeDefined()
      expect(res.error.data?.code).toBe('CODEINTEL_INVALID_PARAMS')
      expect(res.error.data?.reason).not.toBe('codeintel_disabled')
    })
  })
})
