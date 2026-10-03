// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ kind: 'local' })
}))
vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))
const SECRET = 'sk-test-secret-value-1234567890'
vi.mock('../CredentialInput', async () => {
  const React = await import('react')
  return {
    CredentialInput: React.forwardRef(
      (
        { onChange }: { onChange: (hasValue: boolean) => void },
        ref: React.Ref<{ take: () => string }>
      ) => {
        React.useImperativeHandle(ref, () => ({ take: () => SECRET }))
        return (
          <button data-testid="mock-credential-input" onClick={() => onChange(true)}>
            Provide Credential
          </button>
        )
      }
    )
  }
})
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

// Mock span exposes id/step/ok/fail so security-sensitive tests can assert the
// tracer NEVER receives apiKey/encryptedBlob/iv — only accountId/provider/blobLength (bucketed).
const { credSpan, uiAiProviderWriteCredFlowStart } = vi.hoisted(() => {
  const credSpan = {
    id: 'write-cred-span-id',
    step: vi.fn(),
    ok: vi.fn((_fields?: Record<string, unknown>) => undefined),
    fail: vi.fn((_err?: unknown, _fields?: Record<string, unknown>) => undefined)
  }
  return {
    credSpan,
    uiAiProviderWriteCredFlowStart: vi.fn((_fields: Record<string, unknown>) => credSpan)
  }
})
vi.mock('../../../../../shared/trace/tracers', () => ({
  Tracers: {
    uiAiProviderWriteCredFlow: { start: uiAiProviderWriteCredFlowStart }
  }
}))

import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)
import { ProviderForm } from '../ProviderForm'
import type { AIProviderAccount } from '../../../types/ai-provider-types'

afterEach(() => cleanup())
beforeEach(() => {
  mockRpc.mockReset()
  uiAiProviderWriteCredFlowStart.mockClear()
  credSpan.ok.mockClear()
  credSpan.fail.mockClear()
})

describe('ProviderForm', () => {
  it('create account → calls aiProvider.create with target', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc', provider: 'anthropic' })
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() =>
      expect(mockRpc).toHaveBeenCalledWith(
        expect.anything(),
        'aiProvider.create',
        expect.any(Object)
      )
    )
  })

  it('update account → calls aiProvider.update with target', async () => {
    const existing = {
      id: 'acc1',
      provider: 'openai',
      label: 'Prod',
      model: 'gpt-4',
      scope: 'server',
      devServerId: 'srv1',
      quotaLimitDay: 0
    } as unknown as AIProviderAccount
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm account={existing} onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() =>
      expect(mockRpc).toHaveBeenCalledWith(
        expect.anything(),
        'aiProvider.update',
        expect.objectContaining({ accountId: 'acc1' })
      )
    )
  })

  it('calls aiProvider.writeCredential with traceId when new credential provided', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    // The user typed a key
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() =>
      expect(mockRpc).toHaveBeenCalledWith(
        expect.anything(),
        'aiProvider.writeCredential',
        expect.objectContaining({
          encryptedBlob: btoa(SECRET),
          iv: '',
          traceId: 'write-cred-span-id'
        })
      )
    )
  })

  it('does NOT call writeCredential when no new credential', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))
    expect(mockRpc).not.toHaveBeenCalledWith(
      expect.anything(),
      'aiProvider.writeCredential',
      expect.any(Object)
    )
    expect(uiAiProviderWriteCredFlowStart).not.toHaveBeenCalled()
  })

  it('shows "Add AI Provider" title for new account', () => {
    render(<ProviderForm onClose={vi.fn()} />)
    expect(screen.getByText('Add AI Provider')).toBeInTheDocument()
  })

  it('shows "Edit AI Provider" title when editing', () => {
    const existing = {
      id: 'acc1',
      provider: 'openai',
      label: 'P',
      model: '',
      scope: 'user',
      devServerId: '',
      quotaLimitDay: 0
    } as unknown as AIProviderAccount
    render(<ProviderForm account={existing} onClose={vi.fn()} />)
    expect(screen.getByText('Edit AI Provider')).toBeInTheDocument()
  })

  // --- TASK-FE-016.1: ui:aiProvider.writeCredential tracer coverage ---

  it('starts uiAiProviderWriteCredFlow span with accountId/provider/blobLength (SECURITY: no plaintext/blob content)', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))

    await waitFor(() => expect(uiAiProviderWriteCredFlowStart).toHaveBeenCalled())
    const fields = uiAiProviderWriteCredFlowStart.mock.calls[0][0]
    // Bucketed to a multiple of 64 so the span never reveals the exact key length.
    expect(fields).toEqual({ accountId: 'new-acc', provider: 'anthropic', blobLength: 64 })
    expect(JSON.stringify(fields)).not.toContain(SECRET)
    expect(JSON.stringify(fields)).not.toContain(btoa(SECRET))
  })

  it('SECURITY: no TraceFields passed to uiAiProviderWriteCredFlow ever contain apiKey/encryptedBlob/iv', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))

    await waitFor(() => expect(uiAiProviderWriteCredFlowStart).toHaveBeenCalled())

    const allFieldObjects = [
      ...uiAiProviderWriteCredFlowStart.mock.calls.map((c) => c[0]),
      ...credSpan.ok.mock.calls.map((c) => c[0]),
      ...credSpan.fail.mock.calls.map((c) => c[1])
    ].filter(Boolean)

    for (const fields of allFieldObjects) {
      expect(Object.keys(fields ?? {})).not.toContain('apiKey')
      expect(Object.keys(fields ?? {})).not.toContain('encryptedBlob')
      expect(Object.keys(fields ?? {})).not.toContain('iv')
    }
  })

  it('marks span ok with accountId after writeCredential succeeds', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))

    await waitFor(() => expect(credSpan.ok).toHaveBeenCalledWith({ accountId: 'new-acc' }))
  })

  it('marks span failed and re-throws when writeCredential RPC rejects', async () => {
    const err = new Error('relay timeout')
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockRejectedValueOnce(err)
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))

    await waitFor(() => expect(credSpan.fail).toHaveBeenCalledWith(err, { accountId: 'new-acc' }))
  })
})
