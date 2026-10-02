// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { McpExternalServerSecretField } from './McpExternalServerSecretField'

afterEach(cleanup)

describe('McpExternalServerSecretField', () => {
  it('commits a short secret once, then clears the input', () => {
    const onCommit = vi.fn()
    render(<McpExternalServerSecretField label="API_KEY" onCommit={onCommit} onCancel={vi.fn()} />)
    const input = screen.getByTestId('mcp-secret-input') as HTMLInputElement
    expect(input.type).toBe('password')
    expect(input.getAttribute('autocomplete')).toBe('new-password')
    fireEvent.change(input, { target: { value: 'abc' } })
    fireEvent.click(screen.getByRole('button', { name: 'Set' }))
    expect(onCommit).toHaveBeenCalledTimes(1)
    expect(onCommit).toHaveBeenCalledWith('abc')
    expect(input.value).toBe('')
  })

  it('ignores an empty value, wipes on cancel, and states TLS + at-rest copy only', () => {
    const onCommit = vi.fn()
    const onCancel = vi.fn()
    render(<McpExternalServerSecretField label="K" onCommit={onCommit} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Set' }))
    expect(onCommit).not.toHaveBeenCalled()
    const input = screen.getByTestId('mcp-secret-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'typed' } })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(input.value).toBe('')
    expect(onCancel).toHaveBeenCalled()
    const text = document.body.textContent ?? ''
    expect(text).toContain('encrypted at rest')
    expect(text.toLowerCase()).not.toContain('end-to-end')
  })

  it('wipes the DOM value on unmount', () => {
    const { unmount } = render(
      <McpExternalServerSecretField label="K" onCommit={vi.fn()} onCancel={vi.fn()} />
    )
    const input = screen.getByTestId('mcp-secret-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'left-behind' } })
    unmount()
    expect(input.value).toBe('')
  })
})
