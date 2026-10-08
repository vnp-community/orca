import { describe, expect, it } from 'vitest'
import { createCodeIntelBridge } from '../../../shared/code-intel-bridge'
import { parseCodeIntelErrorMessage } from '../../../shared/code-intel-error-codes'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { ALL_CODE_INTEL_CHANNELS, createFakeCodeIntelBackend } from './code-intel-fake-backend'

const SEL = { projectId: 'project-1', worktreeId: 'worktree-1' }
const codeOf = async (p: Promise<unknown>): Promise<string | null> => {
  try {
    await p
    return null
  } catch (e) {
    return parseCodeIntelErrorMessage((e as Error).message).code
  }
}

describe('createFakeCodeIntelBackend', () => {
  it('covers the 46 contract channels', () => {
    expect(ALL_CODE_INTEL_CHANNELS).toHaveLength(46)
  })

  it('rejects unknown keys, identity params and extra positional args with INVALID_PARAMS', async () => {
    const b = createFakeCodeIntelBackend()
    expect(await codeOf(b.call('codeIntel.status', { ...SEL, tenantId: 't' }))).toBe('CODEINTEL_INVALID_PARAMS')
    expect(await codeOf(b.call('codeIntel.status', { ...SEL, cypher: 'MATCH' }))).toBe('CODEINTEL_INVALID_PARAMS')
    expect(await codeOf(b.call('codeIntel.status', SEL, {}))).toBe('CODEINTEL_INVALID_PARAMS')
    expect(await codeOf(b.call('codeIntel.status', { projectId: 'p' }))).toBe('CODEINTEL_INVALID_PARAMS')
    expect(await codeOf(b.call('codeIntel.settings.set', { bogus: 1 }))).toBe('CODEINTEL_INVALID_PARAMS')
  })

  it('puts the error code on the message', async () => {
    const b = createFakeCodeIntelBackend()
    b.failNext('codeIntel.status', 'CODEINTEL_TIMEOUT: slow | {"retryAfterMs":5}')
    await expect(b.call('codeIntel.status', SEL)).rejects.toThrow(/^CODEINTEL_TIMEOUT: slow/)
    await expect(b.call('codeIntel.status', SEL)).resolves.toMatchObject({ overall: 'READY' })
  })

  it('gates by flag level and always answers settings.get', async () => {
    const b = createFakeCodeIntelBackend({ role: 'admin' })
    b.setSettings({ codeIntelEnabled: false })
    expect(await codeOf(b.call('codeIntel.status', SEL))).toBe('CODEINTEL_DISABLED')
    expect(await codeOf(b.call('codeIntel.quality.gate', SEL))).toBe('CODEINTEL_DISABLED')
    await expect(b.call('codeIntel.settings.get', {})).resolves.toMatchObject({
      effective: { codeIntelEnabled: false, qualityGateEnabled: false }
    })
    b.setSettings({ codeIntelEnabled: true, qualityGateEnabled: false })
    await expect(b.call('codeIntel.status', SEL)).resolves.toBeTruthy()
    expect(await codeOf(b.call('codeIntel.quality.gate', SEL))).toBe('CODEINTEL_QUALITY_GATE_DISABLED')
    b.setSettings({ qualityGateEnabled: true })
    b.setChannelData('codeIntel.quality.summary', { ok: true })
    expect(await codeOf(b.call('codeIntel.quality.summary', SEL))).toBe('CODEINTEL_AI_REVIEW_DISABLED')
    b.setSettings({ aiReviewEnabled: true })
    await expect(b.call('codeIntel.quality.summary', SEL)).resolves.toEqual({ ok: true })
  })

  it('settings.set needs admin and updates effective flags', async () => {
    const b = createFakeCodeIntelBackend()
    expect(await codeOf(b.call('codeIntel.settings.set', { codeIntelEnabled: false }))).toBe('CODEINTEL_NOT_AUTHORIZED')
    b.setRole('admin')
    const s = (await b.call('codeIntel.settings.set', { codeIntelEnabled: false })) as {
      effective: { codeIntelEnabled: boolean }
    }
    expect(s.effective.codeIntelEnabled).toBe(false)
  })

  it('reviewState.save checks expectedVersion', async () => {
    const b = createFakeCodeIntelBackend()
    const save = (expectedVersion: number) =>
      b.call('codeIntel.reviewState.save', { ...SEL, baseCommit: 'a', headCommit: 'b', readingProgress: { version: 1, entries: {}, lastFocusedKey: null }, expectedVersion })
    await expect(save(0)).resolves.toMatchObject({ version: 1 })
    await expect(save(0)).rejects.toThrow(/CODEINTEL_VERSION_CONFLICT.*"currentVersion":1/)
    await expect(save(1)).resolves.toMatchObject({ version: 2 })
  })

  it('wraps envelope channels flat and reports unimplemented channels', async () => {
    const b = createFakeCodeIntelBackend()
    b.setErd({ tables: [] })
    const env = (await b.call('codeIntel.erd', SEL)) as Record<string, unknown>
    expect(env).toMatchObject({ data: { tables: [] }, stale: false, truncated: false, fromCache: false })
    expect(typeof env.etag).toBe('string')
    await expect(b.call('codeIntel.routes', SEL)).rejects.toThrow(/is not yet implemented/)
  })

  it('rejects oversized params', async () => {
    const b = createFakeCodeIntelBackend()
    expect(await codeOf(b.call('codeIntel.status', { ...SEL, note: 'x'.repeat(17 * 1024) }))).toBe('CODEINTEL_PAYLOAD_TOO_LARGE')
  })

  it('pushes frames with event, counts streams and drops with resync', () => {
    const b = createFakeCodeIntelBackend()
    const frames: Record<string, unknown>[] = []
    let closed = 0
    b.subscribe({ onEvent: (f) => frames.push(f as Record<string, unknown>), onClose: () => closed++ })
    expect(b.streamCount()).toBe(1)
    b.pushChanged()
    b.pushProgress('worktree-1', 40)
    b.pushQuality({ event: 'quality.finished', runId: 'r', status: 'succeeded' })
    expect(frames.map((f) => f.event)).toEqual(['changed', 'reindexProgress', 'quality.finished'])
    b.dropStream()
    expect(frames.at(-1)).toMatchObject({ event: 'changed', resync: true })
    expect(closed).toBe(1)
    expect(b.streamCount()).toBe(0)
  })

  it('plugs into createCodeIntelBridge and normalizes push frames', async () => {
    const b = createFakeCodeIntelBackend()
    const bridge = createCodeIntelBridge(b.bridgeDeps)
    const res = await bridge.call({ environmentId: 'env-1', method: CODE_INTEL_RPC_METHODS.SETTINGS_GET, params: {} })
    expect(res.ok).toBe(true)
    const events: unknown[] = []
    bridge.subscribeEvents('env-1', { onEvent: (e) => events.push(e), onClose: () => {}, onUnsupported: () => {} })
    b.pushChanged('worktree-1', 'commit')
    expect(events[0]).toMatchObject({ event: 'changed', worktreeId: 'worktree-1' })
    const bad = await bridge.call({ environmentId: 'env-1', method: 'codeIntel.nope', params: {} })
    expect(bad).toMatchObject({ ok: false, error: { code: 'method_not_found' } })
  })

  it('reset restores defaults and records calls', async () => {
    const b = createFakeCodeIntelBackend()
    await b.call('codeIntel.status', SEL)
    expect(b.callsTo('codeIntel.status')).toHaveLength(1)
    b.setSettings({ codeIntelEnabled: false })
    b.reset()
    expect(b.calls).toHaveLength(0)
    expect(b.getSettings().effective.codeIntelEnabled).toBe(true)
  })
})
