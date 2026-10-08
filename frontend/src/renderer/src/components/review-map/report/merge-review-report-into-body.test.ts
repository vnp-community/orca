import { describe, expect, it } from 'vitest'
import { bodyHasReviewReport, mergeReviewReportIntoBody } from './merge-review-report-into-body'
import { REVIEW_REPORT_END_MARKER, REVIEW_REPORT_START_MARKER } from './review-report-markdown'

const block = (text: string) => `${REVIEW_REPORT_START_MARKER}\n${text}\n${REVIEW_REPORT_END_MARKER}`

describe('mergeReviewReportIntoBody', () => {
  it('uses the report alone for an empty description', () => {
    expect(mergeReviewReportIntoBody('', block('A'))).toBe(block('A'))
  })

  it('appends after the user text with a blank line', () => {
    expect(mergeReviewReportIntoBody('My text\n', block('A'))).toBe(`My text\n\n${block('A')}`)
  })

  it('replaces the existing block and keeps surrounding user text', () => {
    const body = `before\n\n${block('OLD')}\n\nafter`
    const merged = mergeReviewReportIntoBody(body, block('NEW'))
    expect(merged).toBe(`before\n\n${block('NEW')}\n\nafter`)
    expect(merged).not.toContain('OLD')
  })

  it('is idempotent', () => {
    const once = mergeReviewReportIntoBody('x', block('A'))
    expect(mergeReviewReportIntoBody(once, block('A'))).toBe(once)
  })

  it('collapses two pasted blocks into one', () => {
    const body = `${block('ONE')}\nmiddle\n${block('TWO')}`
    const merged = mergeReviewReportIntoBody(body, block('NEW'))
    expect(merged.split(REVIEW_REPORT_START_MARKER)).toHaveLength(2)
    expect(merged).toContain('NEW')
  })

  it('preserves CRLF line endings', () => {
    const merged = mergeReviewReportIntoBody('line1\r\nline2', block('A'))
    expect(merged).toContain('line1\r\nline2\r\n\r\n')
    expect(merged.replace(/\r\n/g, '')).not.toContain('\n')
  })

  it('treats a lone start marker as no block', () => {
    const body = `x ${REVIEW_REPORT_START_MARKER} y`
    expect(bodyHasReviewReport(body)).toBe(false)
    expect(mergeReviewReportIntoBody(body, block('A'))).toBe(`${body}\n\n${block('A')}`)
  })
})
