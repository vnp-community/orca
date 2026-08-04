// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ kind: 'local' }),
}))
vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) },
}))
vi.mock('../CredentialInput', () => ({
  CredentialInput: ({ onEncrypted, onClear }: any) => (
    <button
      data-testid="mock-credential-input"
      onClick={() => onEncrypted('blob123', 'iv456')}
    >
      Provide Credential
    </button>
  ),
}))
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

// Mock span exposes id/step/ok/fail so security-sensitive tests can assert the
// tracer NEVER receives apiKey/encryptedBlob/iv — only accountId/provider/blobLength.
const { credSpan, uiAiProviderWriteCredFlowStart } = vi.hoisted(() => {
  const credSpan = { id: 'write-cred-span-id', step: vi.fn(), ok: vi.fn(), fail: vi.fn() }
  return {
    credSpan,
    uiAiProviderWriteCredFlowStart: vi.fn(() => credSpan),
  }
})
vi.mock('../../../../../shared/trace/tracers', () => ({
  Tracers: {
    uiAiProviderWriteCredFlow: { start: uiAiProviderWriteCredFlowStart },
  }
}))

import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)
import { ProviderForm } from '../ProviderForm'

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
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith(
      expect.anything(), 'aiProvider.create', expect.any(Object)
    ))
  })

  it('update account → calls aiProvider.update with target', async () => {
    const existing = { id: 'acc1', provider: 'openai', label: 'Prod', model: 'gpt-4', scope: 'server', devServerId: 'srv1', quotaLimitDay: 0 } as any
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm account={existing} onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith(
      expect.anything(), 'aiProvider.update', expect.objectContaining({ accountId: 'acc1' })
    ))
  })

  it('calls aiProvider.writeCredential with traceId when new credential provided', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    // Trigger credential encryption
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith(
      expect.anything(),
      'aiProvider.writeCredential',
      expect.objectContaining({ encryptedBlob: 'blob123', iv: 'iv456', traceId: 'write-cred-span-id' })
    ))
  })

  it('does NOT call writeCredential when no new credential', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))
    expect(mockRpc).not.toHaveBeenCalledWith(
      expect.anything(), 'aiProvider.writeCredential', expect.any(Object)
    )
    expect(uiAiProviderWriteCredFlowStart).not.toHaveBeenCalled()
  })

  it('shows "Add AI Provider" title for new account', () => {
    render(<ProviderForm onClose={vi.fn()} />)
    expect(screen.getByText('Add AI Provider')).toBeInTheDocument()
  })

  it('shows "Edit AI Provider" title when editing', () => {
    const existing = { id: 'acc1', provider: 'openai', label: 'P', model: '', scope: 'user', devServerId: '', quotaLimitDay: 0 } as any
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
    expect(fields).toEqual({ accountId: 'new-acc', provider: 'anthropic', blobLength: 'blob123'.length })
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
      ...uiAiProviderWriteCredFlowStart.mock.calls.map(c => c[0]),
      ...credSpan.ok.mock.calls.map(c => c[0]),
      ...credSpan.fail.mock.calls.map(c => c[1]),
    ].filter(Boolean)

    for (const fields of allFieldObjects) {
      expect(Object.keys(fields)).not.toContain('apiKey')
      expect(Object.keys(fields)).not.toContain('encryptedBlob')
      expect(Object.keys(fields)).not.toContain('iv')
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
