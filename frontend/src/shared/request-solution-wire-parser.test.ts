import { describe, expect, it } from 'vitest'
import { parseSolution } from './request-wire-parsers'

describe('parseSolution tolerant of CONTRACT SolutionView', () => {
  it('maps contract statuses and the options object', () => {
    const s = parseSolution({
      id: 's1',
      requestId: 'r',
      kind: 'solution',
      status: 'approved',
      version: 2,
      createdAt: '2026-10-07T00:00:00Z',
      options: {
        options: [
          { id: 'opt-0', title: 'A', estimated_effort: '2d' },
          { id: 'opt-1', title: 'B' }
        ],
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
    const s = parseSolution({
      kind: 'diagnosis',
      status: 'proposed',
      options: { root_cause: 'x' },
      chosenOption: -1
    })
    expect(s.status).toBe('ready')
    expect(s.options).toBeUndefined()
    expect(JSON.parse(s.content!)).toEqual({ root_cause: 'x' })
    expect(s.chosenOptionId).toBeUndefined()
  })
})

describe('parseSolution open questions', () => {
  const base = { id: 's1', requestId: 'r', kind: 'solution', status: 'proposed' }

  it('reads {id, text, blocking} from open_questions', () => {
    const s = parseSolution({
      ...base,
      options: {
        options: [{ id: 'opt-1', title: 'A' }],
        open_questions: [
          { id: 'Q-1', text: 'Which DB?', blocking: true },
          { id: 'Q-2', text: 'Keep the old endpoint?', blocking: false }
        ]
      }
    })
    expect(s.openQuestions).toEqual([
      { id: 'Q-1', text: 'Which DB?', blocking: true },
      { id: 'Q-2', text: 'Keep the old endpoint?', blocking: false }
    ])
  })

  it('accepts camelCase and the legacy string[] shape (never blocking)', () => {
    expect(
      parseSolution({
        ...base,
        options: { options: [], openQuestions: [{ id: 'Q-7', text: 'x', blocking: true }] }
      }).openQuestions
    ).toEqual([{ id: 'Q-7', text: 'x', blocking: true }])
    expect(
      parseSolution({
        ...base,
        options: { options: [], open_questions: ['first', '  ', 'second'] }
      }).openQuestions
    ).toEqual([
      { id: 'Q-1', text: 'first', blocking: false },
      { id: 'Q-3', text: 'second', blocking: false }
    ])
  })

  it('fills a missing id, treats a non-true blocking as false and drops unreadable items', () => {
    const s = parseSolution({
      ...base,
      options: {
        options: [],
        open_questions: [{ text: 'no id', blocking: 'yes' }, { id: 'Q-2' }, null, 7, ['x']]
      }
    })
    expect(s.openQuestions).toEqual([{ id: 'Q-1', text: 'no id', blocking: false }])
  })

  it('is absent when the payload has none, and also read for document kinds', () => {
    expect(
      parseSolution({ ...base, options: { options: [{ id: 'opt-1', title: 'A' }] } }).openQuestions
    ).toBeUndefined()
    expect(
      parseSolution({ ...base, options: [{ id: 'o', title: 'A' }] }).openQuestions
    ).toBeUndefined()
    expect(
      parseSolution({ ...base, options: { options: [], open_questions: 'nope' } }).openQuestions
    ).toBeUndefined()
    const d = parseSolution({
      ...base,
      kind: 'diagnosis',
      options: { root_cause: 'x', open_questions: [{ id: 'Q-1', text: 'why?', blocking: true }] }
    })
    expect(d.openQuestions).toEqual([{ id: 'Q-1', text: 'why?', blocking: true }])
    expect(JSON.parse(d.content!).root_cause).toBe('x')
  })
})

describe('parseSolution revision fields (CR-REQ-041)', () => {
  const base = { id: 's1', requestId: 'r', kind: 'solution', status: 'proposed' }

  it('leaves both undefined when an older gateway omits them', () => {
    const s = parseSolution(base)
    expect(s.revisionFeedback).toBeUndefined()
    expect(s.seq).toBeUndefined()
  })

  it('reads the feedback and the version number', () => {
    const s = parseSolution({ ...base, revisionFeedback: 'Cover the rollback path', seq: 3 })
    expect(s.revisionFeedback).toBe('Cover the rollback path')
    expect(s.seq).toBe(3)
  })

  it('keeps an empty feedback string (the first proposal) as written', () => {
    expect(parseSolution({ ...base, revisionFeedback: '', seq: 1 }).revisionFeedback).toBe('')
  })

  it.each([
    ['a string', '2'],
    ['zero (a dry run)', 0],
    ['negative', -1],
    ['fractional', 1.5],
    ['NaN', Number.NaN],
    ['Infinity', Number.POSITIVE_INFINITY],
    ['null', null],
    ['an object', { n: 1 }]
  ])('treats seq %s as absent', (_name, value) => {
    expect(parseSolution({ ...base, seq: value }).seq).toBeUndefined()
  })

  it.each([5, null, ['x'], { a: 1 }])('treats a non-string feedback (%j) as absent', (value) => {
    expect(parseSolution({ ...base, revisionFeedback: value }).revisionFeedback).toBeUndefined()
  })

  it('does not throw on junk', () => {
    for (const junk of [undefined, null, 5, 'x', []]) {
      expect(() => parseSolution(junk)).not.toThrow()
    }
  })
})
