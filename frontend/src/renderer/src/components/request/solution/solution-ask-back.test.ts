import { describe, expect, it } from 'vitest'
import {
  ASK_BACK_PROMPT_MAX,
  ASK_BACK_REASON_MAX,
  buildAskBackQuestions,
  defaultAskBackSelection
} from './solution-ask-back'

describe('buildAskBackQuestions', () => {
  it('keys each question open_question:<id> as a required text question', () => {
    expect(
      buildAskBackQuestions([{ questionId: 'Q-1', prompt: ' Which DB? ', reason: ' needed ' }])
    ).toEqual([
      {
        questionKey: 'open_question:Q-1',
        kind: 'text',
        prompt: 'Which DB?',
        reason: 'needed',
        required: true
      }
    ])
  })

  it('clips to the backend limits by code point and drops a repeated key', () => {
    const [q, ...rest] = buildAskBackQuestions([
      {
        questionId: 'Q-1',
        prompt: '😀'.repeat(ASK_BACK_PROMPT_MAX + 5),
        reason: 'r'.repeat(ASK_BACK_REASON_MAX + 5)
      },
      { questionId: 'Q-1', prompt: 'again', reason: 'again' }
    ])
    expect([...q.prompt]).toHaveLength(ASK_BACK_PROMPT_MAX)
    expect(q.reason).toHaveLength(ASK_BACK_REASON_MAX)
    expect(rest).toEqual([])
  })
})

describe('defaultAskBackSelection', () => {
  const qs = [
    { id: 'Q-1', text: 'a', blocking: true },
    { id: 'Q-2', text: 'b', blocking: true },
    { id: 'Q-3', text: 'c', blocking: false }
  ]
  it('prefers the questions the server refused on, else every blocking one', () => {
    expect(defaultAskBackSelection(qs, ['Q-2', 'Q-9'])).toEqual(['Q-2'])
    expect(defaultAskBackSelection(qs, [])).toEqual(['Q-1', 'Q-2'])
    expect(defaultAskBackSelection(qs, ['Q-3'])).toEqual(['Q-1', 'Q-2'])
  })
})
