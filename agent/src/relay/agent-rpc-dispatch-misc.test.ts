// src/relay/agent-rpc-dispatch-misc.test.ts
// CR-STORAGE-008(a)/TASK-AG-STORAGE-007: dispatchMiscRpc's 'connection.teardown'
// case only — routing/response-shape contract, not cleanupAllPtys'/
// notifyDaemonSessionTeardown's own behavior (covered in
// agent-spawner.test.ts and pty-daemon-client.test.ts respectively).
//
// vm.sshDial / fs.*ViaHiddenTarget / git.statusViaHiddenTarget (TASK-AG-EVM-
// 006/007) live in agent-rpc-dispatch-hidden-target.test.ts; vm.exec/
// vm.provision/vm.cancelProvision (TASK-AG-EVM-001/002/003) live in
// agent-rpc-dispatch-vm.test.ts — both split out, not here.
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { createWireState } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { JsonRpcRequest } from './agent-rpc-dispatch'

const cleanupAllPtys = vi.fn((..._args: unknown[]) => {})
const notifyDaemonSessionTeardown = vi.fn(async (..._args: unknown[]) => {})

vi.mock('./agent-spawner', () => ({
  cleanupAllPtys: (...args: unknown[]) => cleanupAllPtys(...args)
}))
vi.mock('./pty-daemon-client', () => ({
  notifyDaemonSessionTeardown: (...args: unknown[]) => notifyDaemonSessionTeardown(...args)
}))

class MockWs {
  readyState = 1
  send = vi.fn()
}

const LOG: AgentLogger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
  debug: vi.fn()
}

beforeEach(() => {
  cleanupAllPtys.mockReset()
  notifyDaemonSessionTeardown.mockReset().mockImplementation(async () => {})
})

describe('dispatchMiscRpc — connection.teardown', () => {
  it('kills agent.spawn PTYs and notifies the terminal-PTY daemon, then returns {ok: true}', async () => {
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = { jsonrpc: '2.0', id: 1, method: 'connection.teardown' }

    const response = await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )

    expect(cleanupAllPtys).toHaveBeenCalledWith(LOG)
    expect(notifyDaemonSessionTeardown).toHaveBeenCalledWith(LOG)
    expect(response).toEqual({ jsonrpc: '2.0', id: 1, result: { ok: true } })
  })

  it('returns a ServerError response if cleanupAllPtys throws, instead of leaving the request unanswered', async () => {
    cleanupAllPtys.mockImplementation(() => {
      throw new Error('boom')
    })
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = { jsonrpc: '2.0', id: 2, method: 'connection.teardown' }

    const response = await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )

    expect(response).toMatchObject({
      jsonrpc: '2.0',
      id: 2,
      error: { message: expect.stringContaining('connection.teardown failed') }
    })
  })
})

// ─── shell.execStream (CR-TG-006) ──────────────────────────────────────────
// Routing/response-shape contract only — handleShellExecStream's own
// stream.chunk/stream.end behavior is covered in
// __tests__/shell-agent-extensions.test.ts. This also exercises dispatchMiscRpc's
// `state` parameter (previously unused/`_state`, now live again).
describe('dispatchMiscRpc — shell.execStream', () => {
  it('returns { type: "stream.started" } synchronously, without awaiting the handler', async () => {
    let handlerResolved = false
    vi.doMock('./shell-agent-extensions', () => ({
      // A macrotask delay (not just a microtask `await Promise.resolve()`)
      // so the assertion below reliably observes the dispatch's return value
      // arriving first — dispatchMiscRpc's own `await import(...)` +
      // async-function-return machinery already costs a few microtask
      // ticks, so a same-tick microtask in the handler isn't a reliable
      // enough gap to prove "not awaited" against.
      handleShellExecStream: async () => {
        await new Promise((resolve) => setTimeout(resolve, 10))
        handlerResolved = true
      }
    }))
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 3,
      method: 'shell.execStream',
      params: { script: 'echo hi' }
    }

    const response = await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )

    expect(response).toEqual({ jsonrpc: '2.0', id: 3, result: { type: 'stream.started' } })
    // The dispatch call above must have returned before the fire-and-forget
    // handler's own promise settles.
    expect(handlerResolved).toBe(false)
    vi.doUnmock('./shell-agent-extensions')
  })

  it('returns a ServerError response if the dynamic import/handler setup throws', async () => {
    vi.doMock('./shell-agent-extensions', () => {
      throw new Error('module load failed')
    })
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 4,
      method: 'shell.execStream',
      params: { script: 'echo hi' }
    }

    const response = await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )

    expect(response).toMatchObject({
      jsonrpc: '2.0',
      id: 4,
      error: { message: expect.stringContaining('shell.execStream unavailable') }
    })
    vi.doUnmock('./shell-agent-extensions')
  })
})

// ─── agent.capabilities (CR-REQ-033 §2.6) ──────────────────────────────────
describe('dispatchMiscRpc — agent.capabilities', () => {
  it('routes agent.capabilities to the report builder and returns its result under the same id', async () => {
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 10,
      method: 'agent.capabilities',
      params: { tools: ['node'], refresh: true }
    }

    const response = (await dispatchMiscRpc(
      rpc,
      [],
      { toolPath: '/bin', workDir: '/tmp' } as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )) as any

    expect(response.jsonrpc).toBe('2.0')
    expect(response.id).toBe(10)
    expect(response.result).toBeDefined()
    expect(response.result.schemaVersion).toBe(1)
    expect(response.result.agent.protocolVersion).toBe(2)
  })

  it('returns InvalidParams with data.reason INVALID_CAPABILITY_PARAMS for tools given as a string', async () => {
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 11,
      method: 'agent.capabilities',
      params: { tools: 'invalid' }
    }

    const response = (await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )) as any

    expect(response.jsonrpc).toBe('2.0')
    expect(response.id).toBe(11)
    expect(response.error.code).toBe(-32602)
    expect(response.error.data.reason).toBe('INVALID_CAPABILITY_PARAMS')
  })

  it('returns InvalidParams TOO_MANY_ENV_NAMES for 65 envNames', async () => {
    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 12,
      method: 'agent.capabilities',
      params: { envNames: Array.from({ length: 65 }, (_, i) => `ENV_${i}`) }
    }

    const response = (await dispatchMiscRpc(
      rpc,
      [],
      {} as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )) as any

    expect(response.jsonrpc).toBe('2.0')
    expect(response.id).toBe(12)
    expect(response.error.code).toBe(-32602)
    expect(response.error.data.reason).toBe('TOO_MANY_ENV_NAMES')
  })

  it('result matches the golden contract keys', async () => {
    const fs = await import('node:fs')
    const path = await import('node:path')
    const goldenPath = path.join(import.meta.dirname, '__fixtures__', 'agent-capabilities-golden.json')
    const golden = JSON.parse(fs.readFileSync(goldenPath, 'utf8'))

    const { dispatchMiscRpc } = await import('./agent-rpc-dispatch-misc')
    const rpc: JsonRpcRequest = {
      jsonrpc: '2.0',
      id: 13,
      method: 'agent.capabilities',
      params: { refresh: true }
    }

    const response = (await dispatchMiscRpc(
      rpc,
      [],
      { toolPath: '/bin', workDir: '/tmp' } as AgentConfig,
      LOG,
      new MockWs() as never,
      createWireState()
    )) as any

    const actual = response.result
    const expected = golden.full

    // Compare top-level core keys
    const coreKeys = ['schemaVersion', 'probedAt', 'partial', 'agent', 'tools', 'claude', 'host', 'env']
    expect(Object.keys(actual)).toEqual(expect.arrayContaining(coreKeys))
    // Compare nested sections
    expect(Object.keys(actual.agent).sort()).toEqual(Object.keys(expected.agent).sort())
    expect(Object.keys(actual.claude).sort()).toEqual(expect.arrayContaining(['auth', 'installed']))
    expect(Object.keys(actual.host).sort()).toEqual(expect.arrayContaining(['platform', 'arch', 'nodeVersion', 'cpuCount']))
  })
})

