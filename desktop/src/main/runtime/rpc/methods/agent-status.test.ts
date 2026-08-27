import { describe, expect, it, afterEach, vi } from 'vitest'

// Why: avoid importing RpcDispatcher — it eagerly imports the full
// ALL_RPC_METHODS aggregator (every namespace file), which transitively
// pulls in Electron-toolkit modules (app-icon.ts et al.) that this sandbox's
// electron CJS/ESM interop can't load standalone. Invoking the method
// handler directly keeps this suite scoped to agent-status.ts's own
// dependency graph (ipc/agent-hooks.ts only needs ipcMain for its own
// handler registration, unused by the enrichAgentStatusIpcPayload helper
// this file reuses).
vi.mock('electron', () => ({
  ipcMain: {
    removeHandler: vi.fn(),
    handle: vi.fn(),
    on: vi.fn(),
    removeAllListeners: vi.fn()
  }
}))

import type { RpcMethod } from '../core'
import { AGENT_STATUS_METHODS } from './agent-status'
import { agentHookServer } from '../../../agent-hooks/server'

function findMethod(name: string): RpcMethod {
  const method = AGENT_STATUS_METHODS.find((m) => m.name === name)
  if (!method || 'stream' in method) {
    throw new Error(`Expected a non-streaming method named ${name}`)
  }
  return method
}

function parseParams(method: RpcMethod, params: unknown): unknown {
  return method.params ? method.params.parse(params) : params
}

const ctx = { runtime: { getRuntimeId: () => 'test-runtime' } as never }

describe('agentStatus RPC methods (request/response subset)', () => {
  afterEach(() => {
    agentHookServer.dropStatusEntriesByTabPrefix('tab-1')
  })

  it('agentStatus.getSnapshot returns the same shape as agentHookServer.getStatusSnapshot()', async () => {
    const method = findMethod('agentStatus.getSnapshot')

    const result = await method.handler(undefined, ctx)

    expect(Array.isArray(result)).toBe(true)
  })

  it('agentStatus.getMigrationUnsupportedSnapshot returns an array', async () => {
    const method = findMethod('agentStatus.getMigrationUnsupportedSnapshot')

    const result = await method.handler(undefined, ctx)

    expect(Array.isArray(result)).toBe(true)
  })

  it('agentStatus.inferInterrupt rejects an unknown intent via zod validation', () => {
    const method = findMethod('agentStatus.inferInterrupt')

    expect(() =>
      parseParams(method, {
        paneKey: 'tab-1:leaf-1',
        baselineUpdatedAt: 1,
        baselineStateStartedAt: 1,
        baselinePrompt: '',
        intent: 'not-a-real-intent'
      })
    ).toThrow()
  })

  it('agentStatus.drop rejects an invalid paneKey via zod validation', () => {
    const method = findMethod('agentStatus.drop')

    expect(() => parseParams(method, { paneKey: '' })).toThrow()
  })

  it('agentStatus.dropByTabPrefix accepts a valid tabId and reports dropped', async () => {
    const method = findMethod('agentStatus.dropByTabPrefix')
    const params = parseParams(method, { tabId: 'tab-1' })

    const result = await method.handler(params, ctx)

    expect(result).toEqual({ dropped: true })
  })
})
