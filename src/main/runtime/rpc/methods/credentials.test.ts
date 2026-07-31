import { describe, expect, it, vi } from 'vitest'
// ── Mock WebCredentialStore + isWebCredentialMode ────────────────────────────
const {
  mockIsWebCredentialMode,
  mockSetToken,
  mockDeleteToken,
  mockHasToken,
  mockGetConfig,
  mockListServices
} = vi.hoisted(() => ({
  mockIsWebCredentialMode: vi.fn().mockReturnValue(true),
  mockSetToken: vi.fn().mockResolvedValue(undefined),
  mockDeleteToken: vi.fn().mockResolvedValue(undefined),
  mockHasToken: vi.fn().mockResolvedValue(false),
  mockGetConfig: vi.fn().mockResolvedValue(null),
  mockListServices: vi.fn().mockResolvedValue([])
}))

vi.mock('../../../credentials', () => ({
  isWebCredentialMode: mockIsWebCredentialMode,
  getWebCredentialStore: () => ({
    setToken: mockSetToken,
    deleteToken: mockDeleteToken,
    hasToken: mockHasToken,
    getConfig: mockGetConfig,
    listServices: mockListServices
  })
}))

import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { CREDENTIAL_METHODS } from './credentials'

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeDispatcher() {
  const runtime = { getRuntimeId: () => 'test' } as unknown as OrcaRuntimeService
  return new RpcDispatcher({ runtime, methods: CREDENTIAL_METHODS })
}

describe('credentials.set', () => {
  it('stores token in WebCredentialStore when in web mode', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.set', { service: 'bitbucket', token: 'app-password-xyz' })
    )

    expect(mockSetToken).toHaveBeenCalledWith('bitbucket', 'app-password-xyz', undefined)
    expect(response).toMatchObject({ ok: true, result: { success: true } })
  })

  it('stores config alongside token', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const dispatcher = makeDispatcher()

    await dispatcher.dispatch(
      makeRequest('credentials.set', {
        service: 'bitbucket',
        token: 'my-token',
        config: { email: 'user@example.com', apiBaseUrl: 'https://api.bitbucket.org' }
      })
    )

    expect(mockSetToken).toHaveBeenCalledWith('bitbucket', 'my-token', {
      email: 'user@example.com',
      apiBaseUrl: 'https://api.bitbucket.org'
    })
  })

  it('throws when not in web mode', async () => {
    mockIsWebCredentialMode.mockReturnValue(false)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.set', { service: 'bitbucket', token: 'tok' })
    )

    expect(response).toMatchObject({ ok: false })
    // error is an object with {code, message} in dispatcher format
    const resp = response as { ok: false; error: { message?: string; code?: string } }
    expect(resp.error?.message ?? '').toContain('Web Server mode')
  })

  it('rejects empty token with validation error', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.set', { service: 'bitbucket', token: '' })
    )

    expect(response).toMatchObject({ ok: false })
  })
})

describe('credentials.revoke', () => {
  it('deletes token from WebCredentialStore', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.revoke', { service: 'gitea' })
    )

    expect(mockDeleteToken).toHaveBeenCalledWith('gitea')
    expect(response).toMatchObject({ ok: true, result: { success: true } })
  })

  it('does not throw if token does not exist (deleteToken is idempotent)', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockDeleteToken.mockResolvedValueOnce(undefined) // success even if not present
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.revoke', { service: 'azure-devops' })
    )

    expect(response).toMatchObject({ ok: true })
  })
})

describe('credentials.status', () => {
  it('returns { configured: false } when no token stored', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockHasToken.mockResolvedValueOnce(false)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.status', { service: 'bitbucket' })
    )

    expect(response).toMatchObject({
      ok: true,
      result: { configured: false, mode: 'web' }
    })
  })

  it('returns { configured: true } when token is set', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockHasToken.mockResolvedValueOnce(true)
    mockGetConfig.mockResolvedValueOnce({ email: 'user@bitbucket.org', apiBaseUrl: 'https://api.bitbucket.org' })
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.status', { service: 'bitbucket' })
    )

    expect(response).toMatchObject({
      ok: true,
      result: { configured: true, mode: 'web' }
    })
  })

  it('does NOT include token value in status response', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockHasToken.mockResolvedValueOnce(true)
    mockGetConfig.mockResolvedValueOnce({ email: 'x@x.com', secret: 'should-not-appear' })
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.status', { service: 'bitbucket' })
    )

    const result = (response as { result: Record<string, unknown> }).result
    // 'secret' is not in SAFE_CONFIG_FIELDS for bitbucket → must not appear
    expect(JSON.stringify(result)).not.toContain('should-not-appear')
  })

  it('only includes safe config fields in response', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockHasToken.mockResolvedValueOnce(true)
    mockGetConfig.mockResolvedValueOnce({
      email: 'user@b.org',
      apiBaseUrl: 'https://api.bb.org',
      unsafeField: 'raw-token'
    })
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.status', { service: 'bitbucket' })
    )

    const result = (response as { result: { config?: Record<string, unknown> } }).result
    expect(result.config).toEqual({ email: 'user@b.org', apiBaseUrl: 'https://api.bb.org' })
    expect(result.config).not.toHaveProperty('unsafeField')
  })

  it('returns { mode: electron } when not in web mode', async () => {
    mockIsWebCredentialMode.mockReturnValue(false)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(
      makeRequest('credentials.status', { service: 'linear' })
    )

    expect(response).toMatchObject({ ok: true, result: { configured: false, mode: 'electron' } })
  })
})

describe('credentials.list', () => {
  it('returns empty list when no credentials stored', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockListServices.mockResolvedValueOnce([])
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(makeRequest('credentials.list'))

    expect(response).toMatchObject({ ok: true, result: { services: [], mode: 'web' } })
  })

  it('returns list of configured service names', async () => {
    mockIsWebCredentialMode.mockReturnValue(true)
    mockListServices.mockResolvedValueOnce(['bitbucket', 'linear'])
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(makeRequest('credentials.list'))

    expect(response).toMatchObject({ ok: true, result: { services: ['bitbucket', 'linear'] } })
  })

  it('returns { mode: electron } when not in web mode', async () => {
    mockIsWebCredentialMode.mockReturnValue(false)
    const dispatcher = makeDispatcher()

    const response = await dispatcher.dispatch(makeRequest('credentials.list'))

    expect(response).toMatchObject({ ok: true, result: { services: [], mode: 'electron' } })
  })
})
