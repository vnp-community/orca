import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getMiniMaxCredentialsStatus,
  saveMiniMaxCredentialsCookie,
  clearMiniMaxCredentialsCookie
} from './runtime-minimax-credentials-client'

const minimaxGetStatus = vi.fn()
const minimaxSaveCookie = vi.fn()
const minimaxClearCookie = vi.fn()
const runtimeCall = vi.fn()

const fakeStatus = { configured: true }

beforeEach(() => {
  minimaxGetStatus.mockReset().mockResolvedValue(fakeStatus)
  minimaxSaveCookie.mockReset().mockResolvedValue(fakeStatus)
  minimaxClearCookie.mockReset().mockResolvedValue(fakeStatus)
  runtimeCall.mockReset()
  vi.stubGlobal('window', {
    api: {
      minimaxCredentials: {
        getStatus: minimaxGetStatus,
        saveCookie: minimaxSaveCookie,
        clearCookie: minimaxClearCookie
      },
      // Why: regression guard — these used to route through
      // window.api.runtime.call (a real network call to backend-go on the
      // web build, which has no minimaxCredentials.* channels and always
      // threw "not yet implemented"). Asserting this is never called
      // catches a regression back to that indirection.
      runtime: { call: runtimeCall }
    }
  })
})

describe('runtime-minimax-credentials-client', () => {
  it('getMiniMaxCredentialsStatus calls window.api.minimaxCredentials.getStatus directly, not runtime.call', async () => {
    const result = await getMiniMaxCredentialsStatus()
    expect(minimaxGetStatus).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
    expect(result).toBe(fakeStatus)
  })

  it('saveMiniMaxCredentialsCookie calls window.api.minimaxCredentials.saveCookie directly with the cookie', async () => {
    await saveMiniMaxCredentialsCookie('cookie-value')
    expect(minimaxSaveCookie).toHaveBeenCalledWith('cookie-value')
    expect(runtimeCall).not.toHaveBeenCalled()
  })

  it('clearMiniMaxCredentialsCookie calls window.api.minimaxCredentials.clearCookie directly', async () => {
    await clearMiniMaxCredentialsCookie()
    expect(minimaxClearCookie).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
  })
})
