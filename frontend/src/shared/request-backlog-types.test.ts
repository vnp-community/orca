import { describe, expect, it } from 'vitest'
import {
  BACKLOG_RPC_BY_VIEW,
  normalizeNextPageToken,
  parseRequestBacklogPage,
  parseTaskBacklogPage
} from './request-backlog-types'

const fullRow = {
  requestId: 'r1', number: 7, title: 'Fix', type: 'bug', sourceProvider: 'github', sourceRef: 'o/r#1',
  sourceUrl: 'https://x/y', returnedFromStage: 'plan', returnedCategory: 'infeasible',
  returnReason: 'no', returnedBy: 'u1', returnedAt: '2026-01-01T00:00:00Z', parentRequestIds: ['p']
}

describe('request-backlog-types', () => {
  it('parses a full request page', () => {
    const page = parseRequestBacklogPage({ requestRows: [fullRow], nextPageToken: 'n' })
    expect(page.items[0]).toMatchObject({ requestId: 'r1', number: 7, type: 'bug', returnedCategory: 'infeasible' })
    expect(page.nextPageToken).toBe('n')
  })

  it('survives missing fields, unknown enums, and rows without an id', () => {
    const page = parseRequestBacklogPage({
      requestRows: [{ requestId: 'r2', returnedCategory: 'x', returnedFromStage: 'zz' }, { title: 'no id' }]
    })
    expect(page.items).toHaveLength(1)
    expect(page.items[0]).toMatchObject({
      returnedCategory: 'unknown', returnedFromStage: 'unknown', sourceProvider: 'unknown',
      parentRequestIds: [], type: null
    })
    expect(parseRequestBacklogPage(null).items).toEqual([])
  })

  it('keeps only http(s) source URLs', () => {
    const urls = ['javascript:alert(1)', 'https://x/y', 'not a url'].map(
      (sourceUrl) => parseRequestBacklogPage({ requestRows: [{ ...fullRow, sourceUrl }] }).items[0].sourceUrl
    )
    expect(urls).toEqual([undefined, 'https://x/y', undefined])
  })

  it('reads camelCase and snake_case identically', () => {
    const snake = {
      request_rows: [{
        request_id: 'r1', number: 7, title: 'Fix', type: 'bug', source_provider: 'github', source_ref: 'o/r#1',
        source_url: 'https://x/y', returned_from_stage: 'plan', returned_category: 'infeasible',
        return_reason: 'no', returned_by: 'u1', returned_at: '2026-01-01T00:00:00Z', parent_request_ids: ['p']
      }],
      next_page_token: ''
    }
    expect(parseRequestBacklogPage(snake).items).toEqual(parseRequestBacklogPage({ requestRows: [fullRow] }).items)
  })

  it('normalizes empty page tokens and null groups', () => {
    expect(normalizeNextPageToken('')).toBeNull()
    expect(normalizeNextPageToken(undefined)).toBeNull()
    expect(parseTaskBacklogPage({ groups: null, nextPageToken: '' })).toEqual({ items: [], nextPageToken: null })
  })

  it('parses task groups, dropping bad numbers and unknown gate status', () => {
    const page = parseTaskBacklogPage({
      groups: [{
        requestId: 'r1', planTaskId: 'pl', gateStatus: 'weird',
        tasks: [{ taskId: 't1', title: 'A', estimatedHours: -3, failedAttempts: 'x' }, { title: 'no id' }]
      }]
    })
    expect(page.items[0].gateStatus).toBe('unknown')
    expect(page.items[0].tasks).toEqual([
      expect.objectContaining({ taskId: 't1', estimatedHours: null, failedAttempts: 0, blockedByTaskIds: [] })
    ])
  })

  it('maps views to the CR-016 channels', () => {
    expect(BACKLOG_RPC_BY_VIEW).toEqual({
      requests: 'backlog.requests', tasks: 'backlog.tasks', execute: 'backlog.execute'
    })
  })
})
