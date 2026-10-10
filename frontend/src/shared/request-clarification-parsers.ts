/**
 * Clarification parsers — FE-REQ-TASK-036-01
 *
 * @module shared/request-clarification-parsers
 */

import type {
  Clarification,
  ClarificationQuestion,
  ClarificationQuestionKind,
  ClarificationSource,
  ClarificationStatus
} from './request-artifact-types'
import { jsonish, num, oneOf, pick, rec, str, strings } from './request-artifact-wire-fields'

const KIND_ALIASES: Record<string, ClarificationQuestionKind> = {
  text: 'text',
  single: 'single_choice',
  single_choice: 'single_choice',
  multi: 'multi_choice',
  multi_choice: 'multi_choice',
  file: 'file',
  confirm: 'boolean',
  boolean: 'boolean'
}

function parseQuestion(raw: unknown): ClarificationQuestion | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {
    return null
  }
  const kindRaw = String(o.kind ?? '').toLowerCase()
  const options = jsonish(pick(o, 'options', 'optionsJson', 'options_json'))
  const q: ClarificationQuestion = {
    id,
    seq: num(o.seq) ?? 0,
    questionKey: str(pick(o, 'questionKey', 'question_key')) ?? id,
    kind: KIND_ALIASES[kindRaw] ?? 'text',
    prompt: str(o.prompt) ?? '',
    required: o.required !== false
  }
  const reason = str(o.reason)
  if (reason) {
    q.reason = reason
  }
  if (Array.isArray(options)) {
    q.options = options.flatMap((x) => {
      if (typeof x === 'string') {
        return [{ value: x, label: x }]
      }
      const r = rec(x)
      // Why: the service sends options as {id,label}; older fixtures used {value,label}.
      const value = r ? str(pick(r, 'value', 'id')) : undefined
      return r && value ? [{ value, label: str(r.label) ?? value }] : []
    })
  }
  const valueType = String(pick(o, 'valueType', 'value_type') ?? '').toLowerCase()
  if (
    valueType === 'string' ||
    valueType === 'number' ||
    valueType === 'boolean' ||
    valueType === 'list'
  ) {
    q.valueType = valueType
  }
  const def = jsonish(pick(o, 'suggestedDefault', 'suggestedDefaultJson', 'suggested_default_json'))
  if (def !== undefined && def !== null && def !== '') {
    q.suggestedDefault = def
  }
  const ans = jsonish(pick(o, 'answer', 'answerJson', 'answer_json'))
  if (ans !== undefined && ans !== null && ans !== '') {
    q.answer = ans
  }
  return q
}

export function parseClarification(raw: unknown): Clarification | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {
    return null
  }
  const questions = (Array.isArray(o.questions) ? o.questions : [])
    .map(parseQuestion)
    .filter((q): q is ClarificationQuestion => q !== null)
    .sort((a, b) => a.seq - b.seq)
  const c: Clarification = {
    id,
    displayId: str(pick(o, 'displayId', 'display_id')) ?? id,
    requestId: str(pick(o, 'requestId', 'request_id')) ?? '',
    source: oneOf<ClarificationSource>(
      o.source,
      ['readiness', 'solution_open_question', 'plan_assumption', 'task_blocked'],
      'unknown'
    ),
    status: oneOf<ClarificationStatus>(
      o.status,
      ['open', 'answered', 'expired', 'cancelled'],
      'unknown'
    ),
    round: num(o.round) ?? 1,
    questions,
    version: num(o.version) ?? 0
  }
  const sourceRef = str(pick(o, 'sourceRef', 'source_ref'))
  if (sourceRef) {
    c.sourceRef = sourceRef
  }
  const resume = str(pick(o, 'resumeStatus', 'resume_status'))
  if (resume) {
    c.resumeStatus = resume
  }
  const due = str(pick(o, 'dueAt', 'due_at'))
  if (due) {
    c.dueAt = due
  }
  const assignees = strings(pick(o, 'assigneeIds', 'assignee_ids'))
  if (assignees.length > 0) {
    c.assigneeIds = assignees
  }
  return c
}
