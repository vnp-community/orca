/**
 * Ask-back payload — CONTRACT-request-ui-api 2.6 (câu hỏi chặn của Solution), CR-REQ-028 §2.5
 *
 * One `clarification.request` carries every blocking question the reporter must answer.
 * Limits mirror the backend: prompt <= 1000, reason required and <= 500.
 *
 * @module components/request/solution/solution-ask-back
 */

import type { SolutionOpenQuestion } from '../../../../../shared/request-types'

export const ASK_BACK_PROMPT_MAX = 1000
export const ASK_BACK_REASON_MAX = 500

export type AskBackItem = { questionId: string; prompt: string; reason: string }

export type AskBackQuestion = {
  questionKey: string
  kind: 'text'
  prompt: string
  reason: string
  required: true
}

function clip(text: string, max: number): string {
  const chars = [...text.trim()]
  return chars.length <= max ? chars.join('') : chars.slice(0, max).join('')
}

/** Prompt is the AI's own question text; the user edits the reason, not the question. */
export function askBackPrompt(question: SolutionOpenQuestion): string {
  return clip(question.text, ASK_BACK_PROMPT_MAX)
}

export function clipAskBackReason(text: string): string {
  return clip(text, ASK_BACK_REASON_MAX)
}

export function buildAskBackQuestions(items: AskBackItem[]): AskBackQuestion[] {
  const seen = new Set<string>()
  const out: AskBackQuestion[] = []
  for (const item of items) {
    const questionKey = `open_question:${item.questionId}`
    if (seen.has(questionKey)) {
      continue
    }
    seen.add(questionKey)
    out.push({
      questionKey,
      kind: 'text',
      prompt: clip(item.prompt, ASK_BACK_PROMPT_MAX),
      reason: clip(item.reason, ASK_BACK_REASON_MAX),
      required: true
    })
  }
  return out
}

/**
 * Questions to tick by default: the ones the server just refused on, else every blocking one
 * (the UI cannot tell which already have a recorded answer).
 */
export function defaultAskBackSelection(
  questions: SolutionOpenQuestion[],
  refusedIds: string[]
): string[] {
  const blocking = questions.filter((q) => q.blocking).map((q) => q.id)
  const refused = blocking.filter((id) => refusedIds.includes(id))
  return refused.length > 0 ? refused : blocking
}
