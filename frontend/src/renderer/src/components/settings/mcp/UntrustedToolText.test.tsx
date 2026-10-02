// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { UNTRUSTED_TEXT_LIMIT, UntrustedToolText } from './UntrustedToolText'

afterEach(cleanup)

describe('UntrustedToolText', () => {
  it('renders markup as text, never as elements', () => {
    const { container } = render(
      <UntrustedToolText text="<img src=x onerror=alert(1)> [click](javascript:alert(1))" />
    )
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('a')).toBeNull()
    expect(container.textContent).toContain('<img src=x onerror=alert(1)>')
  })

  it('visualizes bidi and zero-width characters', () => {
    const sneaky = 'ok\u202Eevil\u200B'
    const { container } = render(<UntrustedToolText text={sneaky} />)
    expect(container.textContent).toContain('\\u{202E}')
    expect(container.textContent).toContain('\\u{200B}')
  })

  it('truncates long text with a Show more toggle', () => {
    const { container } = render(<UntrustedToolText text={'x'.repeat(UNTRUSTED_TEXT_LIMIT + 50)} />)
    expect(container.querySelector('pre')?.textContent).toHaveLength(UNTRUSTED_TEXT_LIMIT + 1)
    fireEvent.click(screen.getByRole('button', { name: 'Show more' }))
    expect(container.querySelector('pre')?.textContent).toHaveLength(UNTRUSTED_TEXT_LIMIT + 50)
  })
})
