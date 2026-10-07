/**
 * Tests for request store slice (CR-REQ-018-04)
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { createStore } from 'zustand'
import { createRequestSlice } from './request'
import type { RequestSlice } from './request'
import type { OrcaRequest } from '../../../../shared/request-types'

function makeStore() {
  return createStore<RequestSlice>()((set, get, api) =>
    createRequestSlice(set as Parameters<typeof createRequestSlice>[0], get, api)
  )
}

function makeRequest(id: string, overrides: Partial<OrcaRequest> = {}): OrcaRequest {
  return {
    id,
    projectId: 'proj-1',
    number: 1,
    title: `Request ${id}`,
    type: 'bug',
    status: 'submitted',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    ...overrides
  }
}

describe('RequestSlice — upsertRequests', () => {
  it('adds new requests by id', () => {
    const store = makeStore()
    const req = makeRequest('r1')
    store.getState().upsertRequests([req])
    expect(store.getState().requestsById['r1']).toMatchObject({ id: 'r1', title: 'Request r1' })
  })

  it('merges fields without losing existing keys', () => {
    const store = makeStore()
    store.getState().upsertRequests([makeRequest('r1', { title: 'Old' })])
    store.getState().upsertRequests([makeRequest('r1', { title: 'New' })])
    expect(store.getState().requestsById['r1'].title).toBe('New')
  })

  it('inserts multiple requests at once', () => {
    const store = makeStore()
    store.getState().upsertRequests([makeRequest('r1'), makeRequest('r2')])
    expect(Object.keys(store.getState().requestsById)).toHaveLength(2)
  })
})

describe('RequestSlice — removeRequest', () => {
  it('removes by id', () => {
    const store = makeStore()
    store.getState().upsertRequests([makeRequest('r1')])
    store.getState().removeRequest('r1')
    expect(store.getState().requestsById['r1']).toBeUndefined()
  })

  it('is a no-op for unknown id', () => {
    const store = makeStore()
    store.getState().upsertRequests([makeRequest('r1')])
    expect(() => store.getState().removeRequest('r99')).not.toThrow()
    expect(store.getState().requestsById['r1']).toBeDefined()
  })
})

describe('RequestSlice — setPendingApprovalCount', () => {
  it('sets positive count', () => {
    const store = makeStore()
    store.getState().setPendingApprovalCount(5)
    expect(store.getState().pendingApprovalCount).toBe(5)
  })

  it('clamps negative count to 0', () => {
    const store = makeStore()
    store.getState().setPendingApprovalCount(-1)
    expect(store.getState().pendingApprovalCount).toBe(0)
  })
})

describe('RequestSlice — setRequestPageSection', () => {
  it('changes section without resetting requestId', () => {
    const store = makeStore()
    store.getState().setRequestPageRequest('r1')
    store.getState().setRequestPageSection('approvals')
    const page = store.getState().requestPage
    expect(page.section).toBe('approvals')
    expect(page.requestId).toBe('r1')
  })
})

describe('RequestSlice — setRequestPageRequest', () => {
  it('sets requestId', () => {
    const store = makeStore()
    store.getState().setRequestPageRequest('r42')
    expect(store.getState().requestPage.requestId).toBe('r42')
  })

  it('clears requestId to null', () => {
    const store = makeStore()
    store.getState().setRequestPageRequest('r1')
    store.getState().setRequestPageRequest(null)
    expect(store.getState().requestPage.requestId).toBeNull()
  })
})

describe('RequestSlice — defaults', () => {
  it('starts with unknown flow support', () => {
    const store = makeStore()
    expect(store.getState().requestFlowSupport).toBe('unknown')
  })

  it('starts with empty requestsById', () => {
    const store = makeStore()
    expect(store.getState().requestsById).toEqual({})
  })

  it('starts with zero pendingApprovalCount', () => {
    const store = makeStore()
    expect(store.getState().pendingApprovalCount).toBe(0)
  })
})
