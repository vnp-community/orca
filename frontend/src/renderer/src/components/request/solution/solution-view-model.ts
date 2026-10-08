/**
 * Solution View Model — CR-REQ-020-01
 *
 * Pure TypeScript helper functions for rendering Solution UI.
 * No React or Zustand imports — fully unit-testable.
 *
 * @module components/request/solution/solution-view-model
 */

import type { Solution, SolutionOption, Approval, ApprovalSubjectType, RequestStatus, RequestType } from '../../../../../shared/request-types'

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/** Minimum trimmed length for a rejection reason (shared with useApprovals.reject) */
export const REJECT_REASON_MIN_LENGTH = 10

/** Maximum feedback text length (CR-REQ-016: feedback <= 2000) */
export const FEEDBACK_MAX_LENGTH = 2000

// ---------------------------------------------------------------------------
// Solution presentation
// ---------------------------------------------------------------------------

export type SolutionAction = 'choose' | 'approve' | 'reject' | 'regenerate' | 'generatePlan'

export type SolutionPresentation = {
  bannerKey: string
  tone: 'neutral' | 'success' | 'warning' | 'info'
  actions: SolutionAction[]
  readOnly: boolean
}

// Statuses that indicate analysis is already past decision point
const POST_ANALYSIS_STATUSES = new Set<RequestStatus>([
  'planning',
  'awaiting_plan_approval',
  'executing',
  'completed',
  'cancelled'
])

type PresentationInput = {
  solution: Solution
  requestStatus: RequestStatus
  requestType: RequestType
  hasPendingApproval: boolean
}

export function getSolutionPresentation({
  solution,
  requestStatus,
  requestType,
  hasPendingApproval
}: PresentationInput): SolutionPresentation {
  const isReadOnly =
    POST_ANALYSIS_STATUSES.has(requestStatus) ||
    requestStatus === 'cancelled' ||
    requestType === 'hotfix'

  if (isReadOnly) {
    return {
      bannerKey: 'auto.components.request.SolutionPanel.banner.readOnly',
      tone: 'neutral',
      actions: [],
      readOnly: true
    }
  }

  switch (solution.status) {
    case 'generating':
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.generating',
        tone: 'info',
        actions: [],
        readOnly: false
      }

    case 'ready':
      if (hasPendingApproval) {
        return {
          bannerKey: 'auto.components.request.SolutionPanel.banner.awaitingApproval',
          tone: 'warning',
          actions: ['approve', 'reject'],
          readOnly: false
        }
      }
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.ready',
        tone: 'info',
        actions: ['choose', 'regenerate'],
        readOnly: false
      }

    case 'chosen':
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.chosen',
        tone: 'success',
        actions: ['generatePlan'],
        readOnly: false
      }

    case 'rejected':
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.rejected',
        tone: 'warning',
        actions: ['regenerate'],
        readOnly: false
      }

    case 'superseded':
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.superseded',
        tone: 'neutral',
        actions: [],
        readOnly: true
      }

    default:
      return {
        bannerKey: 'auto.components.request.SolutionPanel.banner.unknown',
        tone: 'neutral',
        actions: [],
        readOnly: false
      }
  }
}

// ---------------------------------------------------------------------------
// Approval subject type mapping
// ---------------------------------------------------------------------------

export function solutionApprovalSubject(
  kind: Solution['kind'],
  _requestType: RequestType
): ApprovalSubjectType | null {
  switch (kind) {
    case 'solution':
    case 'diagnosis':
      return 'solution'
    case 'findings':
      // hotfix has kind 'diagnosis' not 'findings', so this is safe
      return 'solution'
    case 'answer':
      return 'solution'
    default:
      return null
  }
}

// ---------------------------------------------------------------------------
// Comparison table
// ---------------------------------------------------------------------------

type ComparisonCriterion = 'summary' | 'pros' | 'cons' | 'effort' | 'risk'

export type ComparisonRow = {
  criterion: ComparisonCriterion
  cells: string[]
  differs: boolean
}

export function buildComparisonRows(options: SolutionOption[]): ComparisonRow[] {
  const criteria: ComparisonCriterion[] = ['summary', 'pros', 'cons', 'effort', 'risk']

  return criteria.map((criterion) => {
    const cells = options.map((opt) => {
      switch (criterion) {
        case 'summary':
          return opt.summary ?? ''
        case 'pros':
          return (opt.pros ?? []).join('; ')
        case 'cons':
          return (opt.cons ?? []).join('; ')
        case 'effort':
          return opt.estimatedEffort ?? ''
        case 'risk':
          // risk is not currently in SolutionOption type — forward compat
          return (opt.raw?.risk as string | undefined) ?? ''
      }
    })
    const differs = new Set(cells).size > 1
    return { criterion, cells, differs }
  })
}

// ---------------------------------------------------------------------------
// Approval gate check
// ---------------------------------------------------------------------------

type ApprovalCheck = {
  ok: boolean
  reasonKey?: 'needTwoOptions' | 'chooseOne'
}

export function canApproveSolution(params: {
  kind: Solution['kind']
  requestType: RequestType
  options: SolutionOption[] | undefined
  chosenOptionId: string | undefined
}): ApprovalCheck {
  const { kind, requestType, options, chosenOptionId } = params

  // Solution kind with change_request requires ≥2 options
  if (kind === 'solution' && requestType === 'change_request') {
    if (!options || options.length < 2) {
      return { ok: false, reasonKey: 'needTwoOptions' }
    }
  }

  // Solution kind requires a chosen option
  if (kind === 'solution' && !chosenOptionId) {
    return { ok: false, reasonKey: 'chooseOne' }
  }

  return { ok: true }
}

// ---------------------------------------------------------------------------
// Rejection reason validation
// ---------------------------------------------------------------------------

export function validateRejectReason(text: string): { ok: boolean; length: number } {
  // Count Unicode code points (not bytes / code units)
  const trimmed = text.trim()
  const length = [...trimmed].length
  return { ok: length >= REJECT_REASON_MIN_LENGTH, length }
}

export function clampFeedback(text: string): string {
  const chars = [...text]
  if (chars.length <= FEEDBACK_MAX_LENGTH) {return text}
  return chars.slice(0, FEEDBACK_MAX_LENGTH).join('')
}

// ---------------------------------------------------------------------------
// Pick pending approval
// ---------------------------------------------------------------------------

export function pickPendingApproval(approvals: Approval[], solution: Solution): Approval | null {
  // Prefer approval whose subjectId matches solution.id exactly
  const exact = approvals.find(
    (a) => a.status === 'pending' && a.subjectId === solution.id
  )
  if (exact) {return exact}

  // Fallback: any pending approval of the same subjectType (assumption noted)
  const fallback = approvals.find(
    (a) =>
      a.status === 'pending' &&
      a.subjectType === 'solution' // solutionApprovalSubject always returns 'solution'
  )
  return fallback ?? null
}
