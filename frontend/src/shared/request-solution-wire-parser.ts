/**
 * Solution wire parser (CR-REQ-018): options may be a plain array (earlier drafts) or an object with `options` and a
 * recommendation, and statuses of the contract map to the UI ones.
 *
 * @module shared/request-solution-wire-parser
 */

import type { Solution, SolutionKind, SolutionOption, SolutionStatus } from './request-types'
import { parseSolutionOpenQuestions } from './request-solution-open-questions'
import { safeArr, safeEnum, safeOptNum, safeOptStr, safeRaw, safeStr } from './request-wire-scalars'

const SOLUTION_KINDS = new Set<SolutionKind>(['solution', 'diagnosis', 'findings', 'answer'])
const SOLUTION_STATUSES = new Set<SolutionStatus>([
  'generating',
  'ready',
  'chosen',
  'rejected',
  'superseded'
])
function parseSolutionOption(raw: unknown, index: number): SolutionOption {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const KNOWN = ['id', 'title', 'summary', 'pros', 'cons', 'estimatedEffort']
  return {
    // Fall back to String(index) when id is missing, as spec requires
    id: typeof r.id === 'string' && r.id ? r.id : String(index),
    title: safeStr(r.title),
    summary: safeOptStr(r.summary),
    pros: safeArr(r.pros, (x) => safeStr(x)),
    cons: safeArr(r.cons, (x) => safeStr(x)),
    estimatedEffort: safeOptStr(r.estimatedEffort) ?? safeOptStr(r.estimated_effort),
    raw: safeRaw(r, ...KNOWN, 'estimated_effort')
  }
}

// CONTRACT SolutionView statuses -> UI statuses (older drafts already used the UI names).
const SOLUTION_STATUS_ALIASES: Record<string, string> = {
  draft: 'generating',
  proposed: 'ready',
  approved: 'chosen'
}

function recommendedOptionId(rec: unknown): string | undefined {
  const r = rec && typeof rec === 'object' ? (rec as Record<string, unknown>) : { id: rec }
  return [r.optionId, r.option_id, r.id].find((v): v is string => typeof v === 'string' && v !== '')
}

// 0 is what the backend shows for a dry run; a version number has to be a whole number from 1.
function parseSolutionSeq(value: unknown): number | undefined {
  const seq = safeOptNum(value)
  return seq !== undefined && Number.isInteger(seq) && seq >= 1 ? seq : undefined
}

export function parseSolution(raw: unknown): Solution {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  // CONTRACT: options is an object { options[], recommendation, ... } for kind=solution
  // and an AI document for other kinds; older drafts sent a plain array.
  const optObj =
    r.options && typeof r.options === 'object' && !Array.isArray(r.options)
      ? (r.options as Record<string, unknown>)
      : undefined
  const optList = Array.isArray(r.options)
    ? r.options
    : Array.isArray(optObj?.options)
      ? optObj.options
      : undefined
  let options = optList?.map((item, i) => parseSolutionOption(item, i))
  const recId = recommendedOptionId(optObj?.recommendation)
  if (options && recId) {
    options = options.map((o) =>
      o.id === recId ? { ...o, raw: { ...o.raw, recommended: true } } : o
    )
  }
  const statusRaw =
    typeof r.status === 'string' ? (SOLUTION_STATUS_ALIASES[r.status] ?? r.status) : r.status
  let chosenOptionId = safeOptStr(r.chosenOptionId)
  if (!chosenOptionId && typeof r.chosenOption === 'number' && r.chosenOption >= 0 && options) {
    chosenOptionId = options[r.chosenOption]?.id
  }
  const openQuestions = parseSolutionOpenQuestions(optObj)
  let content = safeOptStr(r.content)
  content ??= optObj && !optList ? JSON.stringify(optObj) : undefined
  return {
    id: safeStr(r.id),
    requestId: safeStr(r.requestId),
    kind: safeEnum(r.kind, SOLUTION_KINDS, 'unknown'),
    status: safeEnum(statusRaw, SOLUTION_STATUSES, 'unknown'),
    content,
    options,
    ...(openQuestions ? { openQuestions } : {}),
    chosenOptionId,
    rejectionReason: safeOptStr(r.rejectionReason),
    generatedAt: safeOptStr(r.generatedAt) ?? safeOptStr(r.createdAt),
    reviewedAt: safeOptStr(r.reviewedAt),
    reviewedById: safeOptStr(r.reviewedById),
    version: safeOptNum(r.version),
    revisionFeedback: safeOptStr(r.revisionFeedback),
    seq: parseSolutionSeq(r.seq)
  }
}
