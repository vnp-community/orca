// @vitest-environment happy-dom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { Solution } from '../../../../../shared/request-types'
import { AnswerView } from './AnswerView'

afterEach(cleanup)

function answer(content: string | undefined): Solution {
  return { id: 's1', requestId: 'r1', kind: 'answer', status: 'ready', content }
}

describe('AnswerView', () => {
  it('shows answer_markdown from the backend document instead of "No data"', () => {
    render(
      <AnswerView
        solution={answer(
          JSON.stringify({
            schema_version: 1,
            kind: 'answer',
            answer_markdown: 'Trả lời **ngắn**',
            confidence: 0.7,
            citations: [{ ref: 'src/a.ts:1', excerpt: 'let a' }]
          })
        )}
      />
    )
    expect(screen.getByText('ngắn').tagName).toBe('STRONG')
    expect(screen.queryByText('No data')).toBeNull()
    expect(screen.getByTestId('answer-confidence').textContent).toContain('70%')
    expect(screen.getByText('src/a.ts:1')).toBeTruthy()
    expect(screen.getByText('let a')).toBeTruthy()
  })

  it('still reads the legacy answer key', () => {
    render(<AnswerView solution={answer('{"answer":"legacy text"}')} />)
    expect(screen.getByText('legacy text')).toBeTruthy()
  })

  it('renders non-JSON content as Markdown', () => {
    render(<AnswerView solution={answer('just **text**')} />)
    expect(screen.getByText('text').tagName).toBe('STRONG')
  })

  it('shows assumptions, limitations and follow-up when present', () => {
    render(
      <AnswerView
        solution={answer(
          JSON.stringify({
            answer_markdown: 'x',
            assumptions: ['Assume A'],
            limitations: ['Limit B'],
            suggested_follow_up: { type: 'task', title: 'Follow C' }
          })
        )}
      />
    )
    expect(screen.getByText('Assume A')).toBeTruthy()
    expect(screen.getByText('Limit B')).toBeTruthy()
    expect(screen.getByTestId('answer-follow-up').textContent).toContain('Follow C')
  })

  it('does not render raw HTML or javascript: links from the answer', () => {
    const { container } = render(
      <AnswerView
        solution={answer(
          JSON.stringify({
            answer_markdown: '<script>alert(1)</script> [x](javascript:alert(1))'
          })
        )}
      />
    )
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('a')).toBeNull()
  })

  it('marks the Answer section as having no data when the document has no answer text', () => {
    render(<AnswerView solution={answer('{"schema_version":1}')} />)
    expect(screen.getAllByText('No data').length).toBeGreaterThan(0)
  })
})
