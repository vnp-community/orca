import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getGrokAccountStatus } from './runtime-grok-accounts-client'
import type { GrokAccountStatus } from '../../../shared/rate-limit-types'

const grokGetStatus = vi.fn()
const runtimeCall = vi.fn()

const fakeStatus = { signedIn: true } as unknown as GrokAccountStatus

beforeEach(() => {
  grokGetStatus.mockReset().mockResolvedValue(fakeStatus)
  runtimeCall.mockReset()
  vi.stubGlobal('window', {
    api: {
      grokAccounts: { getStatus: grokGetStatus },
      // Why: regression guard — this used to route through
      // window.api.runtime.call (a real network call to backend-go on the
      // web build, which has no grokAccounts.* channels and always threw
      // "not yet implemented"). Asserting this is never called catches a
      // regression back to that indirection.
      runtime: { call: runtimeCall }
    }
  })
})

describe('runtime-grok-accounts-client', () => {
  it('getGrokAccountStatus calls window.api.grokAccounts.getStatus directly, not runtime.call', async () => {
    const result = await getGrokAccountStatus()
    expect(grokGetStatus).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
    expect(result).toBe(fakeStatus)
  })
})
