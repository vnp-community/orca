/**
 * Review samples for the Outcome Review tests. Numbers and names are made up for the tests; none is real project data.
 *
 * @module components/request/outcome-review/outcome-review.fixture
 */

import type {
  DesignVerdict,
  OutcomeReview,
  OutcomeReviewModel,
  OutcomeReviewRun,
  ReviewItem
} from '../../../../../shared/outcome-review-types'

export function makeItem(over: Partial<ReviewItem> = {}): ReviewItem {
  return {
    key: 'arch:CT-02>SVC-05',
    section: 'architecture',
    change: 'added',
    origin: 'agent',
    title: 'Gateway calls bug management',
    detail: '',
    severity: 'deviation',
    needsDecision: false,
    advisory: false,
    unverified: false,
    containers: [],
    protocol: 'http',
    evidence: [],
    before: '',
    after: '',
    ...over
  }
}

export function makeVerdict(over: Partial<DesignVerdict> = {}): DesignVerdict {
  return {
    containersChanged: 0,
    newEdges: 0,
    removedEdges: 0,
    unplannedEdges: 0,
    newCalls: 0,
    removedCalls: 0,
    reordered: 0,
    unplannedCalls: 0,
    reportedDeviations: 0,
    withEvidence: 0,
    advisory: 0,
    designChecked: false,
    reviewedRatio: 0,
    ...over
  }
}

export function makeModel(over: Partial<OutcomeReviewModel> = {}): OutcomeReviewModel {
  return {
    schemaVersion: 1,
    phase: 'delivered',
    subject: { baseRef: 'main', heads: [{ branch: 'feat/one', commit: 'abc1234', taskId: 't1' }] },
    inputsDigest: 'in',
    factsDigest: 'fa',
    state: 'ready',
    verdict: {
      requirements: { delivered: 2, partial: 1, notCovered: 0, unknown: 0, planned: 0, total: 3 },
      architecture: makeVerdict({ designChecked: true, reviewedRatio: 1 }),
      flow: makeVerdict({ designChecked: true, reviewedRatio: 1 }),
      quality: { gate: 'pass' },
      release: { risk: 'low', flags: [] }
    },
    items: [],
    kg: { available: true, snapshot: 'snap-000004', gaps: [], conflicts: [] },
    diagrams: [],
    warnings: [],
    truncated: false,
    ...over
  }
}

export function makeReview(
  over: Omit<Partial<OutcomeReview>, 'model'> & { model?: Partial<OutcomeReviewModel> } = {}
): OutcomeReview {
  const { model, ...rest } = over
  const base: OutcomeReview = {
    reviewId: 'v1',
    requestId: 'r1',
    phase: 'delivered',
    state: 'ready',
    headKey: 'abc',
    version: 3,
    createdAt: '2026-10-20T10:00:00Z',
    updatedAt: '2026-10-20T10:05:00Z',
    model: makeModel({ phase: rest.phase ?? 'delivered', state: rest.state ?? 'ready', ...model })
  }
  return { ...base, ...rest }
}

export function makeRun(over: Partial<OutcomeReviewRun> = {}): OutcomeReviewRun {
  return {
    runId: 'u1',
    reviewId: 'v1',
    kind: 'context',
    status: 'succeeded',
    scope: 'delivered',
    startedAt: '2026-10-20T10:00:00Z',
    finishedAt: '2026-10-20T10:02:00Z',
    costActual: '0.4200 USD',
    ...over
  }
}
