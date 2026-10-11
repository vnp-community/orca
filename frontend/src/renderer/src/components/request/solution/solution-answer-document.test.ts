import { describe, expect, it } from 'vitest'
import { readAnswerDocument } from './solution-answer-document'

describe('readAnswerDocument', () => {
  it('reads answer_markdown and confidence from the backend schema', () => {
    const doc = readAnswerDocument(
      JSON.stringify({
        schema_version: 1,
        kind: 'answer',
        answer_markdown: 'Trả lời **ngắn**',
        confidence: 0.7
      })
    )
    expect(doc.answerMarkdown).toBe('Trả lời **ngắn**')
    expect(doc.confidencePercent).toBe(70)
    expect(doc.rawText).toBeNull()
  })

  it('keeps accepting the legacy answer/response keys and camelCase', () => {
    expect(readAnswerDocument('{"answer":"old"}').answerMarkdown).toBe('old')
    expect(readAnswerDocument('{"response":"older"}').answerMarkdown).toBe('older')
    expect(readAnswerDocument('{"answerMarkdown":"camel"}').answerMarkdown).toBe('camel')
  })

  it('prefers answer_markdown over legacy keys', () => {
    expect(readAnswerDocument('{"answer":"a","answer_markdown":"b"}').answerMarkdown).toBe('b')
  })

  it('reads citations with excerpts, string citations and aliases', () => {
    const doc = readAnswerDocument(
      JSON.stringify({
        answer_markdown: 'x',
        citations: [{ ref: 'src/a.ts:10', excerpt: 'const a = 1' }, 'docs/b.md', { ref: '' }]
      })
    )
    expect(doc.citations).toEqual([
      { ref: 'src/a.ts:10', excerpt: 'const a = 1' },
      { ref: 'docs/b.md' }
    ])
    expect(readAnswerDocument('{"sources":["s"]}').citations).toEqual([{ ref: 's' }])
  })

  it('reads assumptions, limitations and the suggested follow-up', () => {
    const doc = readAnswerDocument(
      JSON.stringify({
        answer_markdown: 'x',
        assumptions: ['A1', { text: 'A2' }],
        limitations: ['L1'],
        suggested_follow_up: { type: 'task', title: 'Do it' }
      })
    )
    expect(doc.assumptions).toEqual(['A1', 'A2'])
    expect(doc.limitations).toEqual(['L1'])
    expect(doc.followUp).toEqual({ type: 'task', title: 'Do it' })
  })

  it('normalises confidence given on a 0-100 scale and ignores junk', () => {
    expect(readAnswerDocument('{"confidence":85}').confidencePercent).toBe(85)
    expect(readAnswerDocument('{"confidence":"high"}').confidencePercent).toBeNull()
    expect(readAnswerDocument('{"confidence":-1}').confidencePercent).toBeNull()
    expect(readAnswerDocument('{"confidence":250}').confidencePercent).toBe(100)
  })

  it('returns the raw text when the content is not a JSON object', () => {
    expect(readAnswerDocument('plain **markdown**').rawText).toBe('plain **markdown**')
    expect(readAnswerDocument('{broken').rawText).toBe('{broken')
    expect(readAnswerDocument('[1,2]').rawText).toBe('[1,2]')
  })

  it('returns an empty document for empty content', () => {
    const doc = readAnswerDocument(undefined)
    expect(doc.answerMarkdown).toBeNull()
    expect(doc.rawText).toBeNull()
    expect(doc.citations).toEqual([])
  })
})
