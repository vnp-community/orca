/**
 * Tests for request-wire-parsers.ts
 */

import { describe, it, expect } from 'vitest'
import {
  parseRequest,
  parseSolution,
  parseApproval,
  parseBacklogItem,
  parseRequestEvent,
  parseRequestTypeHistoryEntry
} from './request-wire-parsers'

describe('parseRequest', () => {
  it('parses a full valid request', () => {
    const raw = {
      id: 'req-1',
      projectId: 'proj-1',
      number: 42,
      title: 'Fix bug',
      type: 'bug',
      status: 'analyzing',
      size: 'L',
      urgency: 'urgent',
      confidence: 0.85,
      createdAt: '2024-01-01T00:00:00Z',
      updatedAt: '2024-01-02T00:00:00Z'
    }
    const result = parseRequest(raw)
    expect(result.id).toBe('req-1')
    expect(result.type).toBe('bug')
    expect(result.status).toBe('analyzing')
    expect(result.size).toBe('L')
    expect(result.urgency).toBe('urgent')
    expect(result.confidence).toBe(0.85)
  })

  it('maps unknown enum values to "unknown"', () => {
    const raw = { id: 'x', projectId: 'p', number: 1, title: 't', type: 'alien_type', status: 'flying', createdAt: '', updatedAt: '' }
    const result = parseRequest(raw)
    expect(result.type).toBe('unknown')
    expect(result.status).toBe('unknown')
  })

  it('handles empty object without throwing', () => {
    expect(() => parseRequest({})).not.toThrow()
    const result = parseRequest({})
    expect(result.id).toBe('')
    expect(result.type).toBe('unknown')
    expect(result.status).toBe('unknown')
  })

  it('handles null without throwing', () => {
    expect(() => parseRequest(null)).not.toThrow()
  })

  it('sets links array when present', () => {
    const raw = {
      id: 'r1', projectId: 'p', number: 1, title: 't',
      type: 'bug', status: 'submitted', createdAt: '', updatedAt: '',
      links: [{ id: 'l1', requestId: 'r1', relatedRequestId: 'r2', reason: 'child', createdAt: '' }]
    }
    const result = parseRequest(raw)
    expect(result.links).toHaveLength(1)
    expect(result.links![0].reason).toBe('child')
  })

  it('leaves links undefined when absent', () => {
    const result = parseRequest({ id: 'r', projectId: 'p', number: 1, title: 't', type: 'bug', status: 'submitted', createdAt: '', updatedAt: '' })
    expect(result.links).toBeUndefined()
  })

  it('maps null slice in links to empty array item gracefully', () => {
    const raw = { id: 'r', projectId: 'p', number: 1, title: 't', type: 'bug', status: 'submitted', createdAt: '', updatedAt: '', links: [null] }
    expect(() => parseRequest(raw)).not.toThrow()
  })
})

describe('parseSolution', () => {
  it('parses a solution with options', () => {
    const raw = {
      id: 's1', requestId: 'r1', kind: 'solution', status: 'ready',
      options: [
        { id: 'opt-1', title: 'Option A', pros: ['fast'], cons: ['risky'] },
        { title: 'Option B' } // missing id → String(1)
      ]
    }
    const result = parseSolution(raw)
    expect(result.kind).toBe('solution')
    expect(result.options).toHaveLength(2)
    expect(result.options![0].id).toBe('opt-1')
    expect(result.options![1].id).toBe('1') // fallback String(index)
  })

  it('handles missing options gracefully', () => {
    const result = parseSolution({ id: 's1', requestId: 'r', kind: 'diagnosis', status: 'generating' })
    expect(result.options).toBeUndefined()
  })

  it('maps unknown kind/status to "unknown"', () => {
    const result = parseSolution({ id: 'x', requestId: 'r', kind: 'alien', status: 'flying' })
    expect(result.kind).toBe('unknown')
    expect(result.status).toBe('unknown')
  })

  it('keeps raw remainder on SolutionOption', () => {
    const raw = {
      id: 's1', requestId: 'r', kind: 'solution', status: 'ready',
      options: [{ id: 'o1', title: 'T', futureField: 'preserve_me' }]
    }
    const result = parseSolution(raw)
    expect(result.options![0].raw).toMatchObject({ futureField: 'preserve_me' })
  })

  it('handles null/empty without throwing', () => {
    expect(() => parseSolution(null)).not.toThrow()
    expect(() => parseSolution({})).not.toThrow()
  })
})

describe('parseApproval', () => {
  it('parses a valid approval', () => {
    const raw = {
      id: 'a1', requestId: 'r1', subjectType: 'plan', subjectId: 'p1',
      status: 'pending', version: 3, createdAt: '', updatedAt: ''
    }
    const result = parseApproval(raw)
    expect(result.subjectType).toBe('plan')
    expect(result.status).toBe('pending')
    expect(result.version).toBe(3)
  })

  it('maps unknown enum values', () => {
    const result = parseApproval({ id: 'x', requestId: 'r', subjectType: 'alien', subjectId: 's', status: 'flying', createdAt: '', updatedAt: '' })
    expect(result.subjectType).toBe('unknown')
    expect(result.status).toBe('unknown')
  })

  it('handles empty object without throwing', () => {
    expect(() => parseApproval({})).not.toThrow()
  })
})

describe('parseBacklogItem', () => {
  it('parses requests view', () => {
    const raw = {
      request: { id: 'r1', projectId: 'p', number: 5, title: 'My request', type: 'bug', status: 'planning', createdAt: '', updatedAt: '' }
    }
    const result = parseBacklogItem('requests', raw)
    expect(result.kind).toBe('request')
    if (result.kind === 'request') {
      expect(result.request.id).toBe('r1')
    }
  })

  it('parses tasks view', () => {
    const raw = { taskId: 't1', taskTitle: 'Do X', requestId: 'r1', requestNumber: 3, requestTitle: 'Fix Y' }
    const result = parseBacklogItem('tasks', raw)
    expect(result.kind).toBe('task')
  })

  it('parses execute view', () => {
    const raw = { taskId: 't2', taskTitle: 'Run Z', requestId: 'r2', requestNumber: 7, requestTitle: 'Deploy' }
    const result = parseBacklogItem('execute', raw)
    expect(result.kind).toBe('execute')
  })

  it('handles null without throwing', () => {
    expect(() => parseBacklogItem('requests', null)).not.toThrow()
    expect(() => parseBacklogItem('tasks', null)).not.toThrow()
  })
})

describe('parseRequestEvent', () => {
  it('parses a valid event', () => {
    const raw = { requestId: 'r1', eventType: 'request.status_changed', status: 'planning', occurredAt: '2024-01-01T00:00:00Z' }
    const result = parseRequestEvent(raw)
    expect(result.requestId).toBe('r1')
    expect(result.eventType).toBe('request.status_changed')
    expect(result.status).toBe('planning')
  })

  it('preserves full eventType including service prefix', () => {
    const raw = { requestId: 'r', eventType: 'orca.request.request.status_changed', occurredAt: '' }
    const result = parseRequestEvent(raw)
    expect(result.eventType).toBe('orca.request.request.status_changed')
  })

  it('handles null without throwing', () => {
    expect(() => parseRequestEvent(null)).not.toThrow()
  })
})

describe('parseRequestTypeHistoryEntry', () => {
  it('parses a valid entry', () => {
    const raw = {
      id: 'h1', requestId: 'r1', fromType: 'bug', toType: 'task',
      actorKind: 'human', occurredAt: '2024-01-01T00:00:00Z'
    }
    const result = parseRequestTypeHistoryEntry(raw)
    expect(result.fromType).toBe('bug')
    expect(result.toType).toBe('task')
    expect(result.actorKind).toBe('human')
  })

  it('maps unknown types to "unknown"', () => {
    const result = parseRequestTypeHistoryEntry({ id: 'x', requestId: 'r', fromType: 'alien', toType: 'unknown_type', occurredAt: '' })
    expect(result.fromType).toBe('unknown')
    expect(result.toType).toBe('unknown')
  })
})

describe('CONTRACT-request-ui-api shapes (CR-REQ-018 reconciliation)', () => {
  it("maps status 'new' to submitted and reads flat source fields", () => {
    const r = parseRequest({
      id: 'r1', status: 'new', type: null, sourceProvider: 'jira', sourceRef: 'ABC-1',
      sourceUrl: 'https://x.example/ABC-1', returnedBy: 'u1'
    })
    expect(r.status).toBe('submitted')
    expect(r.type).toBe('unknown')
    expect(r.source).toEqual({ provider: 'jira', ref: 'ABC-1', url: 'https://x.example/ABC-1', site: undefined })
    expect(r.returnedById).toBe('u1')
  })

  it('reads typeHistory entries that use `at`', () => {
    const e = parseRequestTypeHistoryEntry({ fromType: null, toType: 'bug', actorKind: 'user', at: '2026-01-01T00:00:00Z' })
    expect(e.occurredAt).toBe('2026-01-01T00:00:00Z')
    expect(e.fromType).toBe('unknown')
    expect(e.id).not.toBe('')
  })

  it('reads approvals with decidedBy / dueAt and aliased subject types', () => {
    const a = parseApproval({ id: 'a', subjectType: 'task_list', status: 'approved', decidedBy: 'u', decidedAt: 'd', dueAt: 'due', createdAt: 'c' })
    expect(a.subjectType).toBe('plan')
    expect(a.approverId).toBe('u')
    expect(a.expiresAt).toBe('due')
    expect(a.updatedAt).toBe('d')
  })
})
