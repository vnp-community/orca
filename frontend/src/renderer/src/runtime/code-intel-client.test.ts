import { beforeEach, describe, expect, it, vi } from 'vitest'

const storeState = { maybeTriggerConnectivityPollAfterRpcFailure: vi.fn() }
vi.mock('@/store', () => ({ useAppStore: { getState: () => storeState } }))

const resolveSelector = vi.fn()
vi.mock('../lib/code-intel-worktree-selector', () => ({
  resolveCodeIntelSelector: (...args: unknown[]) => resolveSelector(...args)
}))

import { createCodeIntelClient } from './code-intel-client'
import type { CodeIntelBridgeApi } from '../../../shared/code-intel-bridge'

const READY = { state: 'ready', worktreeId: 'r::/wt', projectId: 'p-1', environmentId: 'env-1' } as const

function makeClient(response: unknown = { ok: true, result: { x: 1 } }) {
  const call = vi.fn().mockResolvedValue(response)
  const bridge: CodeIntelBridgeApi = { call, subscribeEvents: vi.fn() }
  return { client: createCodeIntelClient(bridge), call }
}

beforeEach(() => {
  vi.clearAllMocks()
  resolveSelector.mockReturnValue(READY)
})

describe('code-intel client call()', () => {
  it('adds projectId/worktreeId and routes to the resolved environment', async () => {
    const { client, call } = makeClient()
    const r = await client.call('r::/wt', 'codeIntel.status', { refresh: true }, {})
    expect(r).toEqual({ ok: true, result: { x: 1 } })
    expect(call).toHaveBeenCalledWith({
      environmentId: 'env-1',
      method: 'codeIntel.status',
      params: { refresh: true, projectId: 'p-1', worktreeId: 'r::/wt' }
    })
  })

  it('an explicit environmentId overrides the selector', async () => {
    const { client, call } = makeClient()
    await client.call('r::/wt', 'codeIntel.status', {}, { environmentId: null })
    expect(call.mock.calls[0][0].environmentId).toBeNull()
  })

  it('unsupported selector (no projectId) makes no network call', async () => {
    resolveSelector.mockReturnValue({ state: 'unsupported', reason: 'no-project' })
    const { client, call } = makeClient()
    const r = await client.call('r::/wt', 'codeIntel.status', {}, {})
    expect(call).not.toHaveBeenCalled()
    expect(r).toMatchObject({ ok: false, error: { kind: 'unsupported' } })
  })

  it('settings.* skips the selector and does not inject ids', async () => {
    resolveSelector.mockReturnValue({ state: 'unsupported', reason: 'unknown-worktree' })
    const { client, call } = makeClient()
    const r = await client.call('', 'codeIntel.settings.get', {}, { environmentId: 'env-9' })
    expect(r.ok).toBe(true)
    expect(call).toHaveBeenCalledWith({ environmentId: 'env-9', method: 'codeIntel.settings.get', params: {} })
  })

  it.each(['__proto__', 'constructor', 'prototype'])('rejects forbidden key %s', async (key) => {
    const { client, call } = makeClient()
    const params = JSON.parse(`{"${key}": {"a": 1}}`)
    const r = await client.call('r::/wt', 'codeIntel.status', params, {})
    expect(call).not.toHaveBeenCalled()
    expect(r).toMatchObject({ ok: false, error: { kind: 'validation' } })
  })

  it('rejects array params', async () => {
    const { client, call } = makeClient()
    const r = await client.call('r::/wt', 'codeIntel.status', [1], {})
    expect(call).not.toHaveBeenCalled()
    expect(r).toMatchObject({ ok: false, error: { kind: 'validation' } })
  })

  it('rejects params over the per-method byte limit locally', async () => {
    const { client, call } = makeClient()
    const big = { blob: 'x'.repeat(17 * 1024) }
    const r = await client.call('r::/wt', 'codeIntel.status', big, {})
    expect(call).not.toHaveBeenCalled()
    expect(r).toMatchObject({ ok: false, error: { kind: 'validation' } })
  })

  it('allows reviewState.save up to 256 KiB', async () => {
    const { client, call } = makeClient()
    const r = await client.call('r::/wt', 'codeIntel.reviewState.save', { blob: 'x'.repeat(100 * 1024) }, {})
    expect(r.ok).toBe(true)
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('drops the result when the signal is aborted', async () => {
    const { client } = makeClient()
    const ctrl = new AbortController()
    ctrl.abort()
    const r = await client.call('r::/wt', 'codeIntel.status', {}, { signal: ctrl.signal })
    expect(r).toMatchObject({ ok: false, error: { message: 'aborted' } })
  })

  it('classifies an ok:false envelope', async () => {
    const { client } = makeClient({
      ok: false,
      error: { code: 'internal', message: 'CODEINTEL_DISABLED: off' }
    })
    const r = await client.call('r::/wt', 'codeIntel.status', {}, {})
    expect(r).toMatchObject({ ok: false, error: { kind: 'disabled', code: 'CODEINTEL_DISABLED' } })
  })

  it('offline triggers a connectivity poll for the environment', async () => {
    const { client } = makeClient({ ok: false, error: { code: 'connection_refused', message: 'down' } })
    await client.call('r::/wt', 'codeIntel.status', {}, { environmentId: 'env-1' })
    expect(storeState.maybeTriggerConnectivityPollAfterRpcFailure).toHaveBeenCalledWith(expect.any(Error), {
      kind: 'environment',
      environmentId: 'env-1'
    })
  })

  it('a thrown transport error becomes a classified failure', async () => {
    const call = vi.fn().mockRejectedValue(new Error('CODEINTEL_TOOL_FAILED: boom'))
    const client = createCodeIntelClient({ call, subscribeEvents: vi.fn() })
    const r = await client.call('r::/wt', 'codeIntel.status', {}, {})
    expect(r).toMatchObject({ ok: false, error: { kind: 'tool-failed' } })
  })
})

describe('code-intel client callEnvelope()', () => {
  const envelopeResult = {
    worktreeId: 'r::/wt',
    view: 'structure',
    sources: [],
    headCommit: null,
    stale: false,
    truncated: false,
    totalCount: 0,
    etag: '"e"',
    fromCache: false,
    generatedAt: 'now',
    data: [1]
  }

  it('parses the envelope and data', async () => {
    const { client } = makeClient({ ok: true, result: envelopeResult })
    const r = await client.callEnvelope('r::/wt', 'codeIntel.structure', {}, (d) => d as number[], {})
    expect(r.ok && r.envelope.data).toEqual([1])
  })

  it.each(['codeIntel.status', 'codeIntel.reviewState.get', 'codeIntel.c4.save', 'codeIntel.settings.set', 'codeIntel.dismissFinding'])(
    'refuses %s (no envelope channel)',
    async (method) => {
      const { client, call } = makeClient()
      await expect(client.callEnvelope('r::/wt', method, {}, (d) => d, {})).rejects.toThrow(/does not return an envelope/)
      expect(call).not.toHaveBeenCalled()
    }
  )

  it('a non-object result is a failure, not a crash', async () => {
    const { client } = makeClient({ ok: true, result: 'garbage' })
    const r = await client.callEnvelope('r::/wt', 'codeIntel.structure', {}, (d) => d, {})
    expect(r.ok).toBe(false)
  })
})
