import { beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({
  call: vi.fn(),
  setSupport: vi.fn(),
  state: { codeIntelSupportState: { state: 'disabled', effective: null }, activeWorktreeId: null } as Record<string, unknown>
}))
vi.mock('../../../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call: h.call }) }))
vi.mock('@/store', () => ({
  useAppStore: { getState: () => ({ ...h.state, setCodeIntelSupportState: h.setSupport }) }
}))

import { createCodeIntelSettingsApi } from './code-intel-settings-api'

beforeEach(() => {
  h.call.mockReset()
  h.setSupport.mockReset()
})

describe('createCodeIntelSettingsApi', () => {
  it('reads the tenant settings and refreshes the shared support state', async () => {
    const settings = { effective: { codeIntelEnabled: true, qualityGateEnabled: false, aiReviewEnabled: false }, tenant: {} }
    h.call.mockResolvedValue({ ok: true, result: settings })
    await expect(createCodeIntelSettingsApi().get()).resolves.toBe(settings)
    expect(h.call).toHaveBeenCalledWith('', 'codeIntel.settings.get', {}, { environmentId: null })
    // Why: with the Review tab closed nothing else would notice a re-enabled backend.
    expect(h.setSupport).toHaveBeenCalledWith(expect.objectContaining({ state: 'enabled' }))
  })

  it('sends only the changed field', async () => {
    h.call.mockResolvedValue({ ok: true, result: {} })
    await createCodeIntelSettingsApi().set({ qualityGateEnabled: true })
    expect(h.call).toHaveBeenCalledWith('', 'codeIntel.settings.set', { qualityGateEnabled: true }, expect.anything())
  })

  it('surfaces the error code so the card can show the permission message', async () => {
    h.call.mockResolvedValue({
      ok: false,
      error: { kind: 'forbidden', code: 'CODEINTEL_NOT_AUTHORIZED', message: 'nope', data: null, retryable: false }
    })
    await expect(createCodeIntelSettingsApi().set({ codeIntelEnabled: false })).rejects.toThrow(
      /^CODEINTEL_NOT_AUTHORIZED/
    )
  })
})
