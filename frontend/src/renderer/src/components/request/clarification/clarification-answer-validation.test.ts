import { describe, expect, it } from 'vitest'
import {
  MAX_FILE_TEXT_BYTES,
  MAX_TEXT_LENGTH,
  canSubmit,
  dueState,
  isNumberAnswer,
  isNumberText,
  missingRequired,
  questionIdFromServerMessage,
  serverReasonText,
  toAnswerPayload,
  validateAnswer
} from './clarification-answer-validation'
import type {
  Clarification,
  ClarificationQuestion
} from '../../../../../shared/request-artifact-types'

const q = (over: Partial<ClarificationQuestion>): ClarificationQuestion => ({
  id: 'q',
  seq: 1,
  questionKey: 'q',
  kind: 'text',
  prompt: 'p',
  required: true,
  ...over
})
const options = [
  { value: 'a', label: 'A' },
  { value: 'b', label: 'B' }
]

describe('validateAnswer', () => {
  it('text: required, blank after trim, and length cap', () => {
    expect(validateAnswer(q({}), '   ')).toMatchObject({ ok: false })
    expect(validateAnswer(q({ required: false }), '')).toEqual({ ok: true })
    expect(validateAnswer(q({}), 'x'.repeat(MAX_TEXT_LENGTH))).toEqual({ ok: true })
    expect(validateAnswer(q({}), 'x'.repeat(MAX_TEXT_LENGTH + 1))).toMatchObject({ ok: false })
  })
  it('single/multi choice must come from the options', () => {
    expect(validateAnswer(q({ kind: 'single_choice', options }), 'a')).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'single_choice', options }), 'z')).toMatchObject({ ok: false })
    expect(validateAnswer(q({ kind: 'multi_choice', options }), ['a', 'b'])).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'multi_choice', options }), ['a', 'z'])).toMatchObject({
      ok: false
    })
    expect(validateAnswer(q({ kind: 'multi_choice', options, required: false }), [])).toEqual({
      ok: true
    })
    expect(validateAnswer(q({ kind: 'multi_choice', options }), [])).toMatchObject({ ok: false })
  })
  it('boolean accepts false as an answer', () => {
    expect(validateAnswer(q({ kind: 'boolean' }), false)).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'boolean' }), 'yes')).toMatchObject({ ok: false })
  })
  it('file: text only, UTF-8 byte cap, no NUL', () => {
    const file = (text: string, mime = 'text/plain') => ({
      filename: 'a.txt',
      mime,
      size: text.length,
      text
    })
    expect(validateAnswer(q({ kind: 'file' }), file('hello'))).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'file' }), file('a\u0000b'))).toMatchObject({
      reasonKey: expect.stringContaining('fileBinary')
    })
    expect(validateAnswer(q({ kind: 'file' }), file('x', 'image/png'))).toMatchObject({ ok: false })
    expect(
      validateAnswer(q({ kind: 'file' }), file('é'.repeat(MAX_FILE_TEXT_BYTES / 2 + 1)))
    ).toMatchObject({ reasonKey: expect.stringContaining('fileTooLarge') })
  })
})

describe('number text (valueType: number)', () => {
  const hours = q({ id: 'h', questionKey: 'type_fields.time_box_hours', valueType: 'number' })

  it('is only a number question for kind text with valueType number', () => {
    expect(isNumberText(hours)).toBe(true)
    expect(isNumberText(q({ valueType: 'string' }))).toBe(false)
    expect(isNumberText(q({}))).toBe(false)
    expect(isNumberText(q({ kind: 'boolean', valueType: 'number' }))).toBe(false)
  })

  it('accepts plain decimals and rejects letters, long sentences and separators', () => {
    for (const ok of ['8', ' 8 ', '2.5', '.5', '8.', '+3', '-1', '0']) {
      expect(isNumberAnswer(ok), ok).toBe(true)
    }
    for (const bad of [
      'MTX-13',
      'eight',
      '1,5',
      '1 000',
      '8h',
      '1e3',
      '',
      'a long sentence about hours'
    ]) {
      expect(isNumberAnswer(bad), bad).toBe(false)
    }
  })

  it('blocks a non-number early with a number-specific reason, and lets a number through', () => {
    expect(validateAnswer(hours, 'MTX-13')).toEqual({
      ok: false,
      reasonKey: 'auto.components.request.clarification.invalidNumber'
    })
    expect(validateAnswer(hours, '8')).toEqual({ ok: true })
    expect(validateAnswer(hours, '   ')).toMatchObject({
      ok: false,
      reasonKey: expect.stringContaining('required')
    })
  })

  it('leaves other value types to the service (no early rules beyond the existing ones)', () => {
    expect(validateAnswer(q({ valueType: 'list' }), 'step one\nstep two')).toEqual({ ok: true })
    expect(validateAnswer(q({ valueType: 'string' }), 'MTX-13')).toEqual({ ok: true })
    expect(validateAnswer(q({}), 'MTX-13')).toEqual({ ok: true })
  })

  it('sends the number as a trimmed JSON string, which is what a text question accepts', () => {
    const c: Clarification = {
      id: 'c1',
      displayId: 'CL-1',
      requestId: 'r',
      source: 'readiness',
      status: 'open',
      round: 1,
      version: 2,
      questions: [hours, q({ id: 'free', seq: 2, valueType: 'string' })]
    }
    const p = toAnswerPayload(
      c,
      new Map<string, unknown>([
        ['h', ' 8 '],
        ['free', ' keep spaces ']
      ]),
      new Set()
    )
    expect(p.answers).toEqual([
      { questionId: 'h', valueJson: '"8"', acceptDefault: false },
      { questionId: 'free', valueJson: '" keep spaces "', acceptDefault: false }
    ])
  })
})

describe('server validation messages', () => {
  const qs = [
    { id: 'q1', questionKey: 'type_fields.time_box_hours' },
    { id: 'q2', questionKey: 'goal' }
  ]

  it('finds the question by id or by the service key', () => {
    expect(
      questionIdFromServerMessage(
        'REQUEST_CLARIFICATION_INVALID_ANSWER: question_id=q2 too vague',
        qs
      )
    ).toBe('q2')
    expect(
      questionIdFromServerMessage(
        'REQUEST_CLARIFICATION_INVALID_ANSWER: question "type_fields.time_box_hours": the answer does not fit time_box_hours (number)',
        qs
      )
    ).toBe('q1')
  })

  it('returns null for a message that names no known question', () => {
    expect(
      questionIdFromServerMessage('REQUEST_CLARIFICATION_INVALID_ANSWER: question "nope": x', qs)
    ).toBeNull()
    expect(questionIdFromServerMessage('REQUEST_CLARIFICATION_INCOMPLETE: missing', qs)).toBeNull()
    expect(questionIdFromServerMessage('question_id=zzz', qs)).toBeNull()
  })

  it('shows only the reason inline', () => {
    expect(
      serverReasonText(
        'REQUEST_CLARIFICATION_INVALID_ANSWER: question "goal": an answer is required'
      )
    ).toBe('an answer is required')
    expect(serverReasonText('REQUEST_CLARIFICATION_INVALID_ANSWER: question_id=q1 too vague')).toBe(
      'question_id=q1 too vague'
    )
    expect(serverReasonText('plain')).toBe('plain')
  })
})

describe('canSubmit / toAnswerPayload', () => {
  const c: Clarification = {
    id: 'c1',
    displayId: 'CL-1',
    requestId: 'r',
    source: 'readiness',
    status: 'open',
    round: 1,
    version: 4,
    questions: [
      q({ id: 'a', suggestedDefault: 'yes' }),
      q({ id: 'b', kind: 'boolean', seq: 2 }),
      q({ id: 'c', seq: 3, required: false })
    ]
  }
  it('needs every required question answered or its default accepted', () => {
    expect(canSubmit(c, new Map(), new Set())).toBe(false)
    expect(missingRequired(c, new Map(), new Set())).toEqual(['a', 'b'])
    expect(canSubmit(c, new Map([['b', true]]), new Set(['a']))).toBe(true)
  })
  it('does not accept a default for a question that has none', () => {
    expect(canSubmit(c, new Map([['a', 'x']]), new Set(['b']))).toBe(false)
  })
  it('builds the payload: JSON values, acceptDefault with empty valueJson, optional skipped', () => {
    const p = toAnswerPayload(
      c,
      new Map<string, unknown>([
        ['b', false],
        ['c', '  ']
      ]),
      new Set(['a'])
    )
    expect(p).toEqual({
      clarificationId: 'c1',
      complete: true,
      expectedVersion: 4,
      answers: [
        { questionId: 'a', valueJson: '', acceptDefault: true },
        { questionId: 'b', valueJson: 'false', acceptDefault: false }
      ]
    })
  })
})

describe('dueState', () => {
  const now = Date.parse('2026-10-07T00:00:00Z')
  it('reports none, overdue and days left', () => {
    expect(dueState(undefined, now)).toEqual({ kind: 'none' })
    expect(dueState('2026-10-06T00:00:00Z', now)).toEqual({ kind: 'overdue' })
    expect(dueState('2026-10-09T00:00:00Z', now)).toEqual({ kind: 'due', days: 2 })
    expect(dueState('garbage', now)).toEqual({ kind: 'none' })
  })
})
