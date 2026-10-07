/**
 * merge-review-report-into-body.test.ts — FE-CV-TASK-090-07
 */

import { describe, it, expect } from 'vitest'
import { mergeReviewReportIntoBody, bodyHasReviewReport } from './merge-review-report-into-body'
import type { ReviewReportModel } from './review-report-model-parser'

const MODEL: ReviewReportModel = {
  title: 'Test',
  description: '',
  summary: { overallRisk: 'LOW', testedBehavior: [], untestedBehavior: [], checklistItems: [] },
  sections: [],
  overlay: [],
  headCommit: null,
  generatedAt: null,
  modelVersion: 1,
}

describe('mergeReviewReportIntoBody', () => {
  it('appends to empty body', () => {
    const { body, replaced } = mergeReviewReportIntoBody('', MODEL)
    expect(replaced).toBe(false)
    expect(body).toContain('<!-- orca-review-report:begin -->')
    expect(body).toContain('<!-- orca-review-report:end -->')
  })

  it('appends to existing text', () => {
    const { body, replaced } = mergeReviewReportIntoBody('Existing body', MODEL)
    expect(replaced).toBe(false)
    expect(body).toContain('Existing body')
    expect(body).toContain('<!-- orca-review-report:begin -->')
  })

  it('replaces existing report block', () => {
    const first = mergeReviewReportIntoBody('', MODEL)
    const second = mergeReviewReportIntoBody(first.body, MODEL)
    expect(second.replaced).toBe(true)
    // Only one begin and one end
    const beginCount = (second.body.match(/orca-review-report:begin/g) ?? []).length
    const endCount = (second.body.match(/orca-review-report:end/g) ?? []).length
    expect(beginCount).toBe(1)
    expect(endCount).toBe(1)
  })

  it('is idempotent', () => {
    const first = mergeReviewReportIntoBody('Preamble', MODEL)
    const second = mergeReviewReportIntoBody(first.body, MODEL)
    const third = mergeReviewReportIntoBody(second.body, MODEL)
    expect(second.body).toBe(third.body)
  })

  it('preserves CRLF line endings', () => {
    const crlf = 'Existing\r\nBody\r\n'
    const { body } = mergeReviewReportIntoBody(crlf, MODEL)
    expect(body).toContain('\r\n')
  })

  it('preserves LF-only endings', () => {
    const lf = 'Existing\nBody\n'
    const { body } = mergeReviewReportIntoBody(lf, MODEL)
    expect(body).not.toContain('\r\n')
  })

  it('handles GitLab MR body same as GitHub PR body', () => {
    const gitlabBody = '## Description\nSome MR description\n'
    const { body } = mergeReviewReportIntoBody(gitlabBody, MODEL)
    expect(body).toContain('## Description')
    expect(body).toContain('<!-- orca-review-report:begin -->')
  })
})

describe('bodyHasReviewReport', () => {
  it('returns false for empty body', () => {
    expect(bodyHasReviewReport('')).toBe(false)
  })

  it('returns true after inserting', () => {
    const { body } = mergeReviewReportIntoBody('', MODEL)
    expect(bodyHasReviewReport(body)).toBe(true)
  })
})
