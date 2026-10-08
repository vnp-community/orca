/**
 * Clarification answer validation — FE-REQ-TASK-036-03
 *
 * Pure. The server stays the source of truth; this only blocks obviously bad
 * input early to save a round trip.
 *
 * @module components/request/clarification/clarification-answer-validation
 */

import type { AnswerPayload, Clarification, ClarificationQuestion } from '../../../../../shared/request-artifact-types'

export const MAX_TEXT_LENGTH = 4000
export const MAX_FILE_TEXT_BYTES = 64 * 1024

export type FileAnswer = { filename: string; mime: string; size: number; text: string }
export type AnswerDraft = ReadonlyMap<string, unknown>
export type ValidationResult = { ok: true } | { ok: false; reasonKey: string }

const T = 'auto.components.request.clarification.'

function utf8Bytes(s: string): number {
  return new TextEncoder().encode(s).length
}

export function isTextFile(mime: string, text: string): boolean {
  if (text.includes('\u0000')) {return false}
  // Why: v1 backend only accepts text attachments; binary mime types are refused up front.
  return mime === '' || mime.startsWith('text/') || /(json|xml|yaml|csv|javascript|markdown)/.test(mime)
}

export function validateAnswer(question: ClarificationQuestion, value: unknown): ValidationResult {
  const empty = value === undefined || value === null || (typeof value === 'string' && value.trim() === '') || (Array.isArray(value) && value.length === 0)
  if (empty) {return question.required ? { ok: false, reasonKey: `${T}required` } : { ok: true }}
  switch (question.kind) {
    case 'text':
      return typeof value === 'string' && value.length <= MAX_TEXT_LENGTH ? { ok: true } : { ok: false, reasonKey: `${T}textTooLong` }
    case 'single_choice':
      return typeof value === 'string' && (question.options ?? []).some((o) => o.value === value) ? { ok: true } : { ok: false, reasonKey: `${T}invalidChoice` }
    case 'multi_choice': {
      const allowed = new Set((question.options ?? []).map((o) => o.value))
      return Array.isArray(value) && value.every((v) => typeof v === 'string' && allowed.has(v)) ? { ok: true } : { ok: false, reasonKey: `${T}invalidChoice` }
    }
    case 'boolean':
      return typeof value === 'boolean' ? { ok: true } : { ok: false, reasonKey: `${T}invalidChoice` }
    case 'file': {
      const f = value as Partial<FileAnswer>
      if (typeof f.text !== 'string' || typeof f.filename !== 'string') {return { ok: false, reasonKey: `${T}fileBinary` }}
      if (!isTextFile(f.mime ?? '', f.text)) {return { ok: false, reasonKey: `${T}fileBinary` }}
      return utf8Bytes(f.text) <= MAX_FILE_TEXT_BYTES ? { ok: true } : { ok: false, reasonKey: `${T}fileTooLarge` }
    }
  }
}

/** A question counts as answered when it has a valid value or its default was accepted. */
export function canSubmit(clarification: Clarification, draft: AnswerDraft, acceptedDefaults: ReadonlySet<string>): boolean {
  return clarification.questions.every((q) => {
    if (acceptedDefaults.has(q.id) && q.suggestedDefault !== undefined) {return true}
    return validateAnswer(q, draft.get(q.id)).ok
  })
}

export function missingRequired(clarification: Clarification, draft: AnswerDraft, acceptedDefaults: ReadonlySet<string>): string[] {
  return clarification.questions
    .filter((q) => !(acceptedDefaults.has(q.id) && q.suggestedDefault !== undefined) && !validateAnswer(q, draft.get(q.id)).ok)
    .map((q) => q.id)
}

export function toAnswerPayload(clarification: Clarification, draft: AnswerDraft, acceptedDefaults: ReadonlySet<string>): AnswerPayload {
  const answers: AnswerPayload['answers'] = []
  for (const q of clarification.questions) {
    if (acceptedDefaults.has(q.id) && q.suggestedDefault !== undefined) {
      answers.push({ questionId: q.id, valueJson: '', acceptDefault: true })
      continue
    }
    const v = draft.get(q.id)
    if (v === undefined || v === null || (typeof v === 'string' && v.trim() === '')) {continue}
    answers.push({ questionId: q.id, valueJson: JSON.stringify(v), acceptDefault: false })
  }
  return { clarificationId: clarification.id, answers, complete: true, expectedVersion: clarification.version }
}

/** Days until `dueAt`; null when absent. Negative = overdue. Clock injected for tests. */
export function dueState(dueAt: string | undefined, now: number): { kind: 'none' } | { kind: 'overdue' } | { kind: 'due'; days: number } {
  if (!dueAt) {return { kind: 'none' }}
  const t = Date.parse(dueAt)
  if (Number.isNaN(t)) {return { kind: 'none' }}
  if (t <= now) {return { kind: 'overdue' }}
  return { kind: 'due', days: Math.ceil((t - now) / 86_400_000) }
}
