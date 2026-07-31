// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
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

import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)
import { ProviderForm } from '../ProviderForm'

afterEach(() => cleanup())
beforeEach(() => mockRpc.mockReset())

describe('ProviderForm', () => {
  it('create account → calls aiProvider.create', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc', provider: 'anthropic' })
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith('aiProvider.create', expect.any(Object)))
  })

  it('update account → calls aiProvider.update', async () => {
    const existing = { id: 'acc1', provider: 'openai', label: 'Prod', model: 'gpt-4', scope: 'server', devServerId: 'srv1', quotaLimitDay: 0 } as any
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm account={existing} onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith('aiProvider.update', expect.objectContaining({ accountId: 'acc1' })))
  })

  it('calls aiProvider.writeCredential when new credential provided', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    mockRpc.mockResolvedValueOnce({})
    render(<ProviderForm onClose={vi.fn()} />)
    // Trigger credential encryption
    fireEvent.click(screen.getByTestId('mock-credential-input'))
    await act(async () => {})
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith('aiProvider.writeCredential', expect.objectContaining({
      encryptedBlob: 'blob123', iv: 'iv456'
    })))
  })

  it('does NOT call writeCredential when no new credential', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-acc' })
    render(<ProviderForm onClose={vi.fn()} />)
    fireEvent.click(screen.getByTestId('save-provider-btn'))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))
    expect(mockRpc).not.toHaveBeenCalledWith('aiProvider.writeCredential', expect.any(Object))
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
})
