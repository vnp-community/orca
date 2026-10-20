// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import {
  MARKDOWN_COLLAPSE_LENGTH,
  RequestMarkdownContent,
  isSafeMarkdownHref
} from './RequestMarkdownContent'

afterEach(cleanup)

describe('RequestMarkdownContent', () => {
  it('does not execute or inject raw HTML', () => {
    const { container } = render(
      <RequestMarkdownContent content="hello <script>window.__x=1<\/script> <img src=x onerror=alert(1)>" />
    )
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
  })

  it('links only http(s)', () => {
    render(
      <RequestMarkdownContent content="[bad](javascript:alert(1)) [file](file:///etc/passwd) [ok](https://example.com)" />
    )
    const links = screen.getAllByRole('link')
    expect(links).toHaveLength(1)
    expect(links[0]).toHaveAttribute('href', 'https://example.com')
    expect(isSafeMarkdownHref('javascript:alert(1)')).toBe(false)
    expect(isSafeMarkdownHref('http://a.b')).toBe(true)
  })

  it('collapses long content and opens the full text in a sheet', () => {
    const tail = 'ENDMARK'
    render(<RequestMarkdownContent content={'a'.repeat(MARKDOWN_COLLAPSE_LENGTH + 10) + tail} />)
    expect(screen.queryByText(new RegExp(tail))).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'View full' }))
    expect(screen.getByText(new RegExp(tail))).toBeInTheDocument()
  })
})

describe('RequestMarkdownContent gfm option', () => {
  const TABLE = '| a | b |\n|---|---|\n| 1 | 2 |'
  const TASKS = '- [ ] 1.1 write the test\n- [x] 1.2 write the code'

  it('shows a table and task list checkboxes when on, read only and named by state', () => {
    const { container } = render(<RequestMarkdownContent gfm content={`${TABLE}\n\n${TASKS}`} />)
    expect(container.querySelector('table')).not.toBeNull()
    const boxes = screen.getAllByRole('checkbox')
    expect(boxes).toHaveLength(2)
    boxes.forEach((box) => expect(box).toBeDisabled())
    expect(boxes[0]).not.toBeChecked()
    expect(boxes[0]).toHaveAccessibleName('Not done')
    expect(boxes[1]).toBeChecked()
    expect(boxes[1]).toHaveAccessibleName('Done')
  })

  it('renders as before when off: no table, no checkbox', () => {
    const { container } = render(<RequestMarkdownContent content={`${TABLE}\n\n${TASKS}`} />)
    expect(container.querySelector('table')).toBeNull()
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0)
    expect(container.textContent).toContain('[ ] 1.1 write the test')
  })

  it('still drops raw HTML and unsafe links when on', () => {
    const { container } = render(
      <RequestMarkdownContent
        gfm
        content={
          '<script>x()</script>\n\n[bad](javascript:alert(1)) [ok](https://example.com)\n\ntext <b>bold</b> end'
        }
      />
    )
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('b')).toBeNull()
    expect(container.textContent).not.toContain('<script>')
    expect(screen.getAllByRole('link')).toHaveLength(1)
  })

  it('does not show an HTML comment, which the Source view is for', () => {
    const { container } = render(
      <RequestMarkdownContent gfm content={'<!-- orca:begin plan -->\n\nvisible'} />
    )
    expect(container.textContent).not.toContain('orca:begin')
    expect(container.textContent).toContain('visible')
  })
})

describe('solution sources', () => {
  const files = readdirSync(__dirname).filter((f) => /\.(ts|tsx)$/.test(f) && !/\.test\./.test(f))
  it.each(files)(
    '%s has no hex colours, emoji, raw tailwind colours or dangerouslySetInnerHTML',
    (f) => {
      const src = readFileSync(join(__dirname, f), 'utf8')
      expect(src).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
      expect(src).not.toMatch(/\p{Extended_Pictographic}/u)
      expect(src).not.toMatch(
        /\b(text|bg|border)-(red|green|blue|yellow|amber|orange|gray|slate)-\d/
      )
      expect(src).not.toMatch(/dangerouslySetInnerHTML/)
      expect(src).not.toMatch(/max-lines/)
    }
  )
})
