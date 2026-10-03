// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react'
import { createRef } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CredentialInput, type CredentialInputHandle } from '../CredentialInput'

afterEach(() => cleanup())

const LONG_KEY = 'sk-ant-api03-this-key-is-much-longer-than-ten-characters-0123456789'

function renderInput(
  props: Partial<Parameters<typeof CredentialInput>[0]> = {},
  ref = createRef<CredentialInputHandle>()
) {
  const onChange = vi.fn()
  const utils = render(
    <CredentialInput
      ref={ref}
      provider="anthropic"
      hasExisting={false}
      onChange={onChange}
      {...props}
    />
  )
  return { ...utils, ref, onChange }
}

describe('CredentialInput', () => {
  it('ollama provider → input not rendered and parent told there is no value', () => {
    const { container, onChange } = renderInput({ provider: 'ollama' })
    expect(container.firstChild).toBeNull()
    expect(onChange).toHaveBeenCalledWith(false)
  })

  it('shows label for anthropic provider', () => {
    renderInput()
    expect(screen.getByText(/Anthropic API Key/)).toBeInTheDocument()
  })

  it('keeps a key longer than 10 characters intact while typing (no mid-typing clear)', () => {
    const { ref } = renderInput()
    const input = screen.getByTestId('credential-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: LONG_KEY } })
    expect(input.value).toBe(LONG_KEY)
    expect(ref.current?.take()).toBe(LONG_KEY)
  })

  it('reports only whether a value exists, never the value', () => {
    const { onChange } = renderInput()
    const input = screen.getByTestId('credential-input')
    fireEvent.change(input, { target: { value: 'a' } })
    fireEvent.change(input, { target: { value: 'ab' } })
    fireEvent.change(input, { target: { value: '' } })
    expect(onChange.mock.calls).toEqual([[true], [false]])
    expect(JSON.stringify(onChange.mock.calls)).not.toContain('ab')
  })

  it('take() trims, clears the field, and returns null the second time', () => {
    const { ref, onChange } = renderInput()
    const input = screen.getByTestId('credential-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: `  ${LONG_KEY}\n` } })
    let first: string | null = null
    act(() => {
      first = ref.current?.take() ?? null
    })
    expect(first).toBe(LONG_KEY)
    expect(input.value).toBe('')
    expect(ref.current?.take()).toBeNull()
    expect(onChange).toHaveBeenCalledWith(true)
  })

  it('take() returns null when nothing was entered', () => {
    const { ref } = renderInput()
    expect(ref.current?.take()).toBeNull()
  })

  it('never persists the key to web storage', () => {
    const local = vi.spyOn(Storage.prototype, 'setItem')
    renderInput()
    fireEvent.change(screen.getByTestId('credential-input'), { target: { value: LONG_KEY } })
    expect(local).not.toHaveBeenCalled()
    local.mockRestore()
  })

  it('does not claim client-side encryption; states the real transport', () => {
    renderInput()
    fireEvent.change(screen.getByTestId('credential-input'), { target: { value: LONG_KEY } })
    const note = screen.getByTestId('credential-transport-note')
    expect(note).toHaveTextContent(/TLS/)
    expect(note).toHaveTextContent(/encrypted at rest/)
    expect(note.textContent).not.toMatch(/in browser|end-to-end/i)
  })

  it('existing credential: shows "Leave blank" hint until a value is typed', () => {
    renderInput({ hasExisting: true })
    expect(screen.getByText(/Leave blank to keep existing/)).toBeInTheDocument()
    fireEvent.change(screen.getByTestId('credential-input'), { target: { value: 'x' } })
    expect(screen.queryByText(/Leave blank to keep existing/)).toBeNull()
  })

  it('switching providers back and forth keeps hook order valid (no crash)', () => {
    const onChange = vi.fn()
    const ref = createRef<CredentialInputHandle>()
    const { rerender } = render(
      <CredentialInput ref={ref} provider="openai" hasExisting={false} onChange={onChange} />
    )
    expect(() => {
      rerender(
        <CredentialInput ref={ref} provider="ollama" hasExisting={false} onChange={onChange} />
      )
      rerender(
        <CredentialInput ref={ref} provider="anthropic" hasExisting={false} onChange={onChange} />
      )
    }).not.toThrow()
    expect(screen.getByTestId('credential-input')).toBeInTheDocument()
  })
})
