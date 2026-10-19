// task-enrichment slice unit tests
import { describe, expect, it } from 'vitest'
import { createStore } from 'zustand'
import { createTaskEnrichmentSlice } from '../../slices/task-enrichment'
import type { TaskEnrichmentSlice } from '../../slices/task-enrichment'

// Minimal store only with TaskEnrichmentSlice for unit testing
// (AppState has many slices — here we test slice in isolation)
function makeStore() {
  return createStore<TaskEnrichmentSlice>()((...args: any[]) =>
    (createTaskEnrichmentSlice as any)(...args)
  )
}

describe('TaskEnrichmentSlice', () => {
  it('setTaskEnrichmentLoading sets loading=true', () => {
    const store = makeStore()
    store.getState().setTaskEnrichmentLoading('T-1')
    expect(store.getState().taskEnrichments['T-1']?.loading).toBe(true)
  })

  it('setTaskEnrichment saves crsContext and loading=false', () => {
    const store = makeStore()
    const crs = { id: 'CRS-045', status: 'pending' }
    store.getState().setTaskEnrichment('T-1', crs, undefined)
    const entry = store.getState().taskEnrichments['T-1']
    expect(entry?.crsContext).toEqual(crs)
    expect(entry?.loading).toBe(false)
    expect(entry?.fetchedAt).toBeGreaterThan(0)
  })

  it('setTaskEnrichmentError sets error and loading=false', () => {
    const store = makeStore()
    store.getState().setTaskEnrichmentError('T-1', 'fetch failed')
    const entry = store.getState().taskEnrichments['T-1']
    expect(entry?.error).toBe('fetch failed')
    expect(entry?.loading).toBe(false)
  })

  it('updateEnrichmentCRSStatus patches status in crsContext', () => {
    const store = makeStore()
    store.getState().setTaskEnrichment('T-1', { id: 'CRS-045', status: 'pending' }, undefined)
    store.getState().updateEnrichmentCRSStatus('T-1', 'done')
    const crs = store.getState().taskEnrichments['T-1']?.crsContext as Record<string, unknown>
    expect(crs?.status).toBe('done')
  })

  it('updateEnrichmentCRSStatus is no-op when task has no crsContext', () => {
    const store = makeStore()
    store.getState().updateEnrichmentCRSStatus('T-nonexistent', 'done')
    expect(store.getState().taskEnrichments['T-nonexistent']).toBeUndefined()
  })

  it('updateEnrichmentCRSStatus does not mutate other tasks', () => {
    const store = makeStore()
    store.getState().setTaskEnrichment('T-1', { status: 'pending' }, undefined)
    store.getState().setTaskEnrichment('T-2', { status: 'pending' }, undefined)
    store.getState().updateEnrichmentCRSStatus('T-1', 'done')
    const t2 = store.getState().taskEnrichments['T-2']?.crsContext as Record<string, unknown>
    expect(t2?.status).toBe('pending')
  })
})
