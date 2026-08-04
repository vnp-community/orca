// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CredentialInputForm, type CredentialField } from './CredentialInputForm'

// TASK-FE-014.2: mock spans expose id/step/ok/fail so tests can assert both
// the span lifecycle calls AND — critically — that no credential value ever
// reaches the fields passed to start()/ok()/fail() (security constraint,
// CR-TRACE-014 §4).
const { credentialSpans, uiRemoteIntegrationCredentialStoreFlowStart } = vi.hoisted(() => {
  const credentialSpans: {
    id: string
    step: ReturnType<typeof vi.fn>
    ok: ReturnType<typeof vi.fn>
    fail: ReturnType<typeof vi.fn>
  }[] = []
  const uiRemoteIntegrationCredentialStoreFlowStart = vi.fn(() => {
    const span = { id: `cred-span-${credentialSpans.length}`, step: vi.fn(), ok: vi.fn(), fail: vi.fn() }
    credentialSpans.push(span)
    return span
  })
  return { credentialSpans, uiRemoteIntegrationCredentialStoreFlowStart }
})

vi.mock('../../../../shared/trace/tracers', () => ({
  Tracers: {
    uiRemoteIntegrationCredentialStoreFlow: { start: uiRemoteIntegrationCredentialStoreFlowStart }
  }
}))

const credentialsSet = vi.fn()
const credentialsRevoke = vi.fn()

const FIELDS: CredentialField[] = [
  { key: 'token', label: 'Personal Access Token', placeholder: 'ghp_...', type: 'password', required: true },
  { key: 'username', label: 'Username', placeholder: 'octocat', type: 'text', required: false }
]

function setup(overrides?: { onSaved?: () => void; onRevoked?: () => void; isConfigured?: boolean }) {
  const onSaved = overrides?.onSaved ?? vi.fn()
  const onRevoked = overrides?.onRevoked ?? vi.fn()
  render(
    <CredentialInputForm
      service="bitbucket"
      fields={FIELDS}
      isConfigured={overrides?.isConfigured ?? true}
      onSaved={onSaved}
      onRevoked={onRevoked}
    />
  )
  return { onSaved, onRevoked }
}

describe('CredentialInputForm tracing (TASK-FE-014.2)', () => {
  beforeEach(() => {
    credentialSpans.length = 0
    uiRemoteIntegrationCredentialStoreFlowStart.mockClear()
    credentialsSet.mockReset()
    credentialsRevoke.mockReset()
    // Why: only stub `window.api` (contextBridge surface) and `confirm` —
    // replacing the whole `window` object would desync happy-dom's `document`
    // global from the object @testing-library/react renders into.
    ;(window as unknown as { api: unknown }).api = {
      credentials: {
        set: credentialsSet,
        revoke: credentialsRevoke
      }
    }
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('handleSave success: starts span({ service, op: "set" }) before the IPC call, then ok({ service })', async () => {
    credentialsSet.mockResolvedValueOnce(undefined)
    const { onSaved } = setup()

    fireEvent.change(screen.getByPlaceholderText('ghp_...'), { target: { value: 'super-secret-token' } })
    await act(async () => {
      fireEvent.click(screen.getByText('Save'))
    })

    expect(uiRemoteIntegrationCredentialStoreFlowStart).toHaveBeenCalledWith({
      service: 'bitbucket',
      op: 'set'
    })
    expect(uiRemoteIntegrationCredentialStoreFlowStart.mock.invocationCallOrder[0]).toBeLessThan(
      credentialsSet.mock.invocationCallOrder[0]
    )
    expect(credentialSpans[0].ok).toHaveBeenCalledWith({ service: 'bitbucket' })
    expect(onSaved).toHaveBeenCalled()
  })

  it('handleSave reject: span.fail(err, { service }) without leaking token/config', async () => {
    const err = new Error('network down')
    credentialsSet.mockRejectedValueOnce(err)
    setup()

    fireEvent.change(screen.getByPlaceholderText('ghp_...'), { target: { value: 'super-secret-token' } })
    await act(async () => {
      fireEvent.click(screen.getByText('Save'))
    })

    expect(credentialSpans[0].fail).toHaveBeenCalledWith(err, { service: 'bitbucket' })
    const failCallFields = credentialSpans[0].fail.mock.calls[0][1] as Record<string, unknown>
    expect(Object.keys(failCallFields)).toEqual(['service'])
    expect(Object.values(failCallFields)).not.toContain('super-secret-token')
  })

  it('handleSave: no field passed to start()/ok()/fail() ever contains the token or config value', async () => {
    credentialsSet.mockResolvedValueOnce(undefined)
    setup()

    fireEvent.change(screen.getByPlaceholderText('ghp_...'), { target: { value: 'ghp_TOTALLY_SECRET_123' } })
    fireEvent.change(screen.getByPlaceholderText('octocat'), { target: { value: 'octo-user' } })
    await act(async () => {
      fireEvent.click(screen.getByText('Save'))
    })

    const allFieldObjects = [
      uiRemoteIntegrationCredentialStoreFlowStart.mock.calls[0][0],
      credentialSpans[0].ok.mock.calls[0]?.[0]
    ].filter(Boolean) as Record<string, unknown>[]

    for (const fields of allFieldObjects) {
      expect(JSON.stringify(fields)).not.toContain('ghp_TOTALLY_SECRET_123')
      expect(JSON.stringify(fields)).not.toContain('octo-user')
      expect(fields).not.toHaveProperty('token')
      expect(fields).not.toHaveProperty('config')
      expect(fields).not.toHaveProperty('encryptedBlob')
    }
  })

  it('handleRevoke success: span with op "revoke" then ok()', async () => {
    credentialsRevoke.mockResolvedValueOnce(undefined)
    const { onRevoked } = setup({ isConfigured: true })

    await act(async () => {
      fireEvent.click(screen.getByText('Revoke'))
    })

    expect(uiRemoteIntegrationCredentialStoreFlowStart).toHaveBeenCalledWith({
      service: 'bitbucket',
      op: 'revoke'
    })
    expect(credentialSpans[0].ok).toHaveBeenCalledWith({ service: 'bitbucket' })
    expect(onRevoked).toHaveBeenCalled()
  })

  it('handleRevoke reject: span.fail(err, { service }) without leaking credential data', async () => {
    const err = new Error('revoke failed')
    credentialsRevoke.mockRejectedValueOnce(err)
    setup({ isConfigured: true })

    await act(async () => {
      fireEvent.click(screen.getByText('Revoke'))
    })

    expect(credentialSpans[0].fail).toHaveBeenCalledWith(err, { service: 'bitbucket' })
  })

  it('handleSave validate fail (missing required field): does NOT create a span', async () => {
    setup()
    // Leave the required 'token' field empty.
    await act(async () => {
      fireEvent.click(screen.getByText('Save'))
    })

    expect(uiRemoteIntegrationCredentialStoreFlowStart).not.toHaveBeenCalled()
    expect(credentialsSet).not.toHaveBeenCalled()
  })
})
