import { describe, expect, it } from 'vitest'
import {
  MAX_FILE_TEXT_BYTES, MAX_TEXT_LENGTH, canSubmit, dueState, missingRequired, toAnswerPayload, validateAnswer
} from './clarification-answer-validation'
import type { Clarification, ClarificationQuestion } from '../../../../../shared/request-artifact-types'

const q = (over: Partial<ClarificationQuestion>): ClarificationQuestion => ({
  id: 'q', seq: 1, questionKey: 'q', kind: 'text', prompt: 'p', required: true, ...over
})
const options = [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }]

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
    expect(validateAnswer(q({ kind: 'multi_choice', options }), ['a', 'z'])).toMatchObject({ ok: false })
    expect(validateAnswer(q({ kind: 'multi_choice', options, required: false }), [])).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'multi_choice', options }), [])).toMatchObject({ ok: false })
  })
  it('boolean accepts false as an answer', () => {
    expect(validateAnswer(q({ kind: 'boolean' }), false)).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'boolean' }), 'yes')).toMatchObject({ ok: false })
  })
  it('file: text only, UTF-8 byte cap, no NUL', () => {
    const file = (text: string, mime = 'text/plain') => ({ filename: 'a.txt', mime, size: text.length, text })
    expect(validateAnswer(q({ kind: 'file' }), file('hello'))).toEqual({ ok: true })
    expect(validateAnswer(q({ kind: 'file' }), file('a\u0000b'))).toMatchObject({ reasonKey: expect.stringContaining('fileBinary') })
    expect(validateAnswer(q({ kind: 'file' }), file('x', 'image/png'))).toMatchObject({ ok: false })
    expect(validateAnswer(q({ kind: 'file' }), file('é'.repeat(MAX_FILE_TEXT_BYTES / 2 + 1)))).toMatchObject({ reasonKey: expect.stringContaining('fileTooLarge') })
  })
})

describe('canSubmit / toAnswerPayload', () => {
  const c: Clarification = {
    id: 'c1', displayId: 'CL-1', requestId: 'r', source: 'readiness', status: 'open', round: 1, version: 4,
    questions: [q({ id: 'a', suggestedDefault: 'yes' }), q({ id: 'b', kind: 'boolean', seq: 2 }), q({ id: 'c', seq: 3, required: false })]
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
    const p = toAnswerPayload(c, new Map<string, unknown>([['b', false], ['c', '  ']]), new Set(['a']))
    expect(p).toEqual({
      clarificationId: 'c1', complete: true, expectedVersion: 4,
      answers: [{ questionId: 'a', valueJson: '', acceptDefault: true }, { questionId: 'b', valueJson: 'false', acceptDefault: false }]
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
