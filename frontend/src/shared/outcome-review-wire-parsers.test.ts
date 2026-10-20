import { describe, expect, it } from 'vitest'
import {
  DECISION_VALUES,
  DIAGRAM_STATUSES,
  REVIEW_CHANGES,
  REVIEW_PHASES,
  REVIEW_SECTIONS,
  REVIEW_STATES,
  isKnown
} from './outcome-review-enums'
import { parseOutcomeReview, parseReviewModel } from './outcome-review-wire-parsers'
import {
  parseArtifactContent,
  parseDecideOutcome,
  parseDecisionList,
  parseGenerateOutcome,
  parseGetReview,
  parseModelDiff,
  parseRegenerateOutcome,
  parseReviewEstimate,
  parseRuns
} from './outcome-review-response-parsers'

const item = {
  key: 'arch:CT-02>SVC-05',
  section: 'architecture',
  change: 'added',
  origin: 'agent',
  title: 'Gateway calls bug management',
  severity: 'deviation',
  needsDecision: true,
  source: 'sad-text',
  reviewState: 'needs_review',
  expected: false,
  evidence: [
    { kind: 'kg', ref: 'CT-02', snapshot: 'snap-4' },
    { kind: 'source', ref: 'SAD', doc: 'SAD.md', section: '5.1' }
  ]
}

describe('parseOutcomeReview', () => {
  it('reads a whole review and gives left-out fields neutral values', () => {
    const r = parseOutcomeReview({
      reviewId: 'v1',
      requestId: 'r1',
      phase: 'delivered',
      state: 'ready',
      headKey: 'abc',
      version: 3,
      model: {
        schemaVersion: 1,
        phase: 'delivered',
        items: [item],
        kg: { available: true, snapshot: 'snap-4' },
        verdict: { architecture: { designChecked: true, reviewedRatio: 0.5 } }
      }
    })
    expect(r?.version).toBe(3)
    expect(r?.model.items[0]).toMatchObject({
      key: item.key,
      needsDecision: true,
      source: 'sad-text',
      expected: false,
      advisory: false,
      unverified: false,
      containers: []
    })
    expect(r?.model.items[0].evidence[1]).toMatchObject({
      kind: 'source',
      doc: 'SAD.md',
      section: '5.1'
    })
    expect(r?.model.kg).toEqual({ available: true, snapshot: 'snap-4', gaps: [], conflicts: [] })
    expect(r?.model.verdict.architecture).toMatchObject({
      designChecked: true,
      reviewedRatio: 0.5,
      newEdges: 0
    })
    expect(r?.model.verdict.flow.designChecked).toBe(false)
    expect(r?.model.diagrams).toEqual([])
    expect(r?.model.warnings).toEqual([])
  })

  it('has no review without an id, and never throws on garbage', () => {
    expect(parseOutcomeReview({ phase: 'design' })).toBeNull()
    expect(parseOutcomeReview(null)).toBeNull()
    expect(parseOutcomeReview('x')).toBeNull()
    expect(parseReviewModel(undefined).items).toEqual([])
    expect(parseReviewModel({ items: 'nope', diagrams: 5, warnings: [1, 'a'] }).warnings).toEqual([
      'a'
    ])
  })

  it('keeps an enum it does not know as the raw string and never folds it into a neighbour', () => {
    const r = parseOutcomeReview({
      reviewId: 'v1',
      phase: 'post-mortem',
      state: 'archived',
      model: {
        phase: 'post-mortem',
        state: 'archived',
        items: [
          {
            key: 'k',
            section: 'security',
            change: 'moved',
            origin: 'oracle',
            severity: 'catastrophic',
            source: 'wiki'
          }
        ],
        diagrams: [{ section: 'flow', variant: 'sketch', status: 'blurry' }]
      }
    })
    expect(r?.phase).toBe('post-mortem')
    expect(isKnown(REVIEW_PHASES, r?.phase)).toBe(false)
    expect(isKnown(REVIEW_STATES, r?.state)).toBe(false)
    const it0 = r?.model.items[0]
    expect(it0).toMatchObject({
      section: 'security',
      change: 'moved',
      origin: 'oracle',
      severity: 'catastrophic',
      source: 'wiki'
    })
    expect(isKnown(REVIEW_SECTIONS, it0?.section)).toBe(false)
    expect(isKnown(REVIEW_CHANGES, it0?.change)).toBe(false)
    expect(isKnown(DIAGRAM_STATUSES, r?.model.diagrams[0].status)).toBe(false)
  })

  it('recognises every value of the closed sets', () => {
    expect(isKnown(REVIEW_PHASES, 'design') && isKnown(REVIEW_PHASES, 'delivered')).toBe(true)
    expect(
      isKnown(DECISION_VALUES, 'accept_exception') &&
        isKnown(DECISION_VALUES, 'send_back') &&
        isKnown(DECISION_VALUES, 'dismiss')
    ).toBe(true)
    expect(isKnown(REVIEW_PHASES, '')).toBe(false)
    expect(isKnown(REVIEW_PHASES, undefined)).toBe(false)
  })
})

describe('answers of request.review.*', () => {
  it('get: a Request with no review is found:false, not an error', () => {
    expect(parseGetReview({ found: false })).toEqual({ found: false, review: null })
    expect(parseGetReview({ found: true, review: { phase: 'design' } })).toEqual({
      found: false,
      review: null
    })
    expect(parseGetReview({ found: true, review: { reviewId: 'v1' } }).review?.reviewId).toBe('v1')
  })

  it('estimate keeps lists as lists and the regeneration estimate optional', () => {
    const e = parseReviewEstimate({
      phase: 'design',
      currentInputsDigest: 'd',
      contextGaps: ['no_dev_server'],
      regeneration: { agentRuns: 2, diagramsToDraw: 3, costKnown: false, confirmRequired: false }
    })
    expect(e).toMatchObject({
      phase: 'design',
      executionFinished: false,
      reviewExists: false,
      contextGaps: ['no_dev_server'],
      warnings: []
    })
    expect(e?.regeneration).toMatchObject({ agentRuns: 2, diagramsToDraw: 3, costKnown: false })
    expect(e?.regeneration?.costUsd).toBeUndefined()
    expect(parseReviewEstimate(null)).toBeNull()
  })

  it('generate needs a review; a run and a draw error are optional', () => {
    expect(parseGenerateOutcome({ created: true })).toBeNull()
    const g = parseGenerateOutcome({
      review: { reviewId: 'v1' },
      created: true,
      run: { runId: 'u1', kind: 'context', status: 'running' },
      drawError: 'REQUEST_REVIEW_SKILL_MISSING',
      drawingsPending: 2
    })
    expect(g).toMatchObject({
      created: true,
      reused: false,
      drawingsPending: 2,
      drawError: 'REQUEST_REVIEW_SKILL_MISSING'
    })
    expect(g?.run?.status).toBe('running')
    expect(g?.drawRun).toBeUndefined()
  })

  it('runs drop entries without an id', () => {
    expect(
      parseRuns({
        runs: [
          { runId: 'u1', status: 'failed', errorCode: 'REQUEST_REVIEW_RESULT_INVALID' },
          { status: 'running' }
        ]
      })
    ).toHaveLength(1)
    expect(parseRuns(undefined)).toEqual([])
  })

  it('decisions: history, orphans and the open count', () => {
    const list = parseDecisionList({
      decisions: [
        {
          decisionId: 'd1',
          itemKey: 'k',
          decision: 'send_back',
          childRequestId: 'c1',
          active: true
        },
        { itemKey: 'x' }
      ],
      openDecisions: 2
    })
    expect(list.openDecisions).toBe(2)
    expect(list.decisions).toHaveLength(1)
    expect(list.decisions[0]).toMatchObject({
      decision: 'send_back',
      childRequestId: 'c1',
      active: true,
      orphaned: false
    })
    expect(
      parseDecideOutcome({
        decision: { decisionId: 'd1' },
        reviewVersion: 5,
        reviewState: 'ready',
        openDecisions: 1
      })
    ).toMatchObject({ reviewVersion: 5, openDecisions: 1 })
    expect(parseDecideOutcome({})).toBeNull()
  })

  it('artifact: content only when sent, not-modified flag kept', () => {
    const a = parseArtifactContent({
      artifact: {
        artifactId: 'a1',
        section: 'architecture',
        variant: 'delta',
        viewBox: '0 0 800 400',
        revision: 2
      },
      mediaType: 'text/html',
      content: '<svg/>'
    })
    expect(a).toMatchObject({ notModified: false, mediaType: 'text/html', content: '<svg/>' })
    expect(a?.artifact).toMatchObject({ artifactId: 'a1', viewBox: '0 0 800 400', revision: 2 })
    const same = parseArtifactContent({ artifact: { artifactId: 'a1' }, notModified: true })
    expect(same?.notModified).toBe(true)
    expect(same?.content).toBeUndefined()
    expect(parseArtifactContent({ notModified: true })).toBeNull()
  })

  it('model diff and regenerate', () => {
    expect(parseModelDiff({ addedNodes: ['a'], removedEdges: ['b>c'] })).toMatchObject({
      addedNodes: ['a'],
      removedEdges: ['b>c'],
      changedNodes: []
    })
    expect(parseModelDiff(null)).toBeUndefined()
    expect(parseRegenerateOutcome({ replayed: true, skipped: 'draw_disabled' })).toEqual({
      replayed: true,
      skipped: 'draw_disabled'
    })
  })
})
