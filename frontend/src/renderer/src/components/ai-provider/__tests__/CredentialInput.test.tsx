// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, act, cleanup, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Mock crypto.subtle + store
vi.mock('../../../lib/credential-crypto', () => ({
  encryptCredential: vi.fn(async (plaintext: string) => ({
    encryptedBlob: btoa(`${plaintext  }-encrypted`),
    iv:            btoa('test-iv'),
  })),
}))
vi.mock('../../../store', () => ({
  useAppStore: (fn?: any) => fn
    ? fn({ auth: { sessionToken: 'test-session-token' } })
    : {},
}))

import { encryptCredential } from '../../../lib/credential-crypto'
const mockEncrypt = vi.mocked(encryptCredential)
import { CredentialInput } from '../CredentialInput'

afterEach(() => cleanup())

describe('CredentialInput', () => {
  it('ollama provider → input not rendered', () => {
    const { container } = render(
      <CredentialInput provider="ollama" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(container.firstChild).toBeNull()
  })

  it('shows label for anthropic provider', () => {
    render(
      <CredentialInput provider="anthropic" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(screen.getByText(/Anthropic API Key/)).toBeInTheDocument()
  })

  it('type <10 chars → NOT encrypted yet, onClear called', async () => {
    const onEncrypted = vi.fn()
    const onClear     = vi.fn()
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={onEncrypted} onClear={onClear} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), { target: { value: 'sk-short' } })
    await act(async () => {})
    expect(mockEncrypt).not.toHaveBeenCalled()
    expect(onEncrypted).not.toHaveBeenCalled()
    expect(onClear).toHaveBeenCalled()
  })

  it('type ≥10 chars → encrypts, calls onEncrypted', async () => {
    const onEncrypted = vi.fn()
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={onEncrypted} onClear={vi.fn()} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), {
      target: { value: 'sk-this-is-longer-than-10-chars' }
    })
    await act(async () => {})
    await waitFor(() => expect(onEncrypted).toHaveBeenCalled())
    const [blob, iv] = onEncrypted.mock.calls[0]
    expect(typeof blob).toBe('string')
    expect(typeof iv).toBe('string')
  })

  it('after encryption → rawValue cleared (input empty)', async () => {
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    const input = screen.getByTestId('credential-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-this-is-longer-than-10-chars' } })
    await act(async () => {})
    await waitFor(() => expect(input.value).toBe(''))
  })

  it('shows 🔒 lock icon when encrypted', async () => {
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), {
      target: { value: 'sk-this-is-a-valid-api-key-123' }
    })
    await act(async () => {})
    await waitFor(() => expect(screen.getByTestId('lock-icon')).toBeInTheDocument())
  })

  it('existing credential: shows "Leave blank" hint', () => {
    render(
      <CredentialInput provider="anthropic" hasExisting={true} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(screen.getByText(/Leave blank to keep existing/)).toBeInTheDocument()
  })
})
