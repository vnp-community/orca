import { describe, expect, it } from 'vitest'
import { parseSolution } from './request-wire-parsers'

describe('parseSolution tolerant of CONTRACT SolutionView', () => {
  it('maps contract statuses and the options object', () => {
    const s = parseSolution({
      id: 's1', requestId: 'r', kind: 'solution', status: 'approved', version: 2, createdAt: '2026-10-07T00:00:00Z',
      options: {
        options: [{ id: 'opt-0', title: 'A', estimated_effort: '2d' }, { id: 'opt-1', title: 'B' }],
        recommendation: { option_id: 'opt-1' }
      },
      chosenOption: 1
    })
    expect(s.status).toBe('chosen')
    expect(s.options).toHaveLength(2)
    expect(s.options![0].estimatedEffort).toBe('2d')
    expect(s.options![1].raw?.recommended).toBe(true)
    expect(s.chosenOptionId).toBe('opt-1')
    expect(s.generatedAt).toBe('2026-10-07T00:00:00Z')
  })

  it('maps draft/proposed and keeps document options as content', () => {
    expect(parseSolution({ kind: 'diagnosis', status: 'draft' }).status).toBe('generating')
    const s = parseSolution({ kind: 'diagnosis', status: 'proposed', options: { root_cause: 'x' }, chosenOption: -1 })
    expect(s.status).toBe('ready')
    expect(s.options).toBeUndefined()
    expect(JSON.parse(s.content!)).toEqual({ root_cause: 'x' })
    expect(s.chosenOptionId).toBeUndefined()
  })
})
