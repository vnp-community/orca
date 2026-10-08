import { describe, expect, it } from 'vitest'
import { create } from 'zustand'
import { createRequestSlice, type RequestSlice } from './request'

function makeStore() {
  return create<RequestSlice>()((...a) =>
    createRequestSlice(...(a as Parameters<typeof createRequestSlice>))
  )
}

describe('setTaskExecutionGates', () => {
  it('replaces one request and keeps the others', () => {
    const store = makeStore()
    store
      .getState()
      .setTaskExecutionGates('r1', { a: 'phase_not_approved', b: 'plan_not_approved' })
    store.getState().setTaskExecutionGates('r2', { c: 'phase_not_approved' })
    store.getState().setTaskExecutionGates('r1', { a: 'phase_not_approved' })
    expect(Object.keys(store.getState().executionGateByTaskId).sort()).toEqual(['a', 'c'])
    store.getState().setTaskExecutionGates('r1', {})
    expect(Object.keys(store.getState().executionGateByTaskId)).toEqual(['c'])
  })
})
