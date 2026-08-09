/**
 * Tests for the onboarding-checklist Zustand slice.
 * TASK-FE-026 — CR-OB-009
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createStore } from 'zustand'
import type { OnboardingChecklistSlice } from './onboarding-checklist'
import {
  createOnboardingChecklistSlice,
  DEFAULT_CHECKLIST_STATE,
  useServerChecklist,
} from './onboarding-checklist'

// ─── Mock window.api ────────────────────────────────────────────────────────────

const markChecklistItem = vi.fn().mockResolvedValue(undefined)

vi.stubGlobal('window', {
  api: {
    onboarding: { markChecklistItem },
  },
})

// ─── Store factory ────────────────────────────────────────────────────────────

type TestStore = OnboardingChecklistSlice

function makeStore() {
  return createStore<TestStore>((set, get, store) =>
    createOnboardingChecklistSlice(set, get, store)
  )
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('createOnboardingChecklistSlice', () => {
  beforeEach(() => {
    markChecklistItem.mockClear()
  })

  it('initialises with all-false global checklist', () => {
    const store = makeStore()
    expect(store.getState().checklistState.addedRepo).toBe(false)
    expect(store.getState().checklistState.choseAgent).toBe(false)
    expect(store.getState().checklistState.perServer).toEqual({})
  })

  it('markGlobalChecklistItem sets the field and calls IPC', () => {
    const store = makeStore()
    store.getState().markGlobalChecklistItem('choseAgent', true)

    expect(store.getState().checklistState.choseAgent).toBe(true)
    expect(markChecklistItem).toHaveBeenCalledWith({ item: 'choseAgent', value: true })
  })

  it('markGlobalChecklistItem defaults value to true', () => {
    const store = makeStore()
    store.getState().markGlobalChecklistItem('addedRepo')

    expect(store.getState().checklistState.addedRepo).toBe(true)
    expect(markChecklistItem).toHaveBeenCalledWith({ item: 'addedRepo', value: true })
  })

  it('markServerChecklistItem sets per-server field and calls IPC', () => {
    const store = makeStore()
    store.getState().markServerChecklistItem('ds-123', 'ranFirstAgent')

    const perServer = store.getState().checklistState.perServer
    expect(perServer['ds-123']?.ranFirstAgent).toBe(true)
    expect(markChecklistItem).toHaveBeenCalledWith({
      item: 'ranFirstAgent',
      devServerId: 'ds-123',
      value: true,
    })
  })

  it('markServerChecklistItem is isolated per server', () => {
    const store = makeStore()
    store.getState().markServerChecklistItem('ds-AAA', 'addedRepo', true)
    store.getState().markServerChecklistItem('ds-BBB', 'ranFirstAgent', true)

    const perServer = store.getState().checklistState.perServer
    expect(perServer['ds-AAA']?.addedRepo).toBe(true)
    expect(perServer['ds-AAA']?.ranFirstAgent).toBeUndefined()
    expect(perServer['ds-BBB']?.ranFirstAgent).toBe(true)
    expect(perServer['ds-BBB']?.addedRepo).toBeUndefined()
  })

  it('setChecklistState merges partial state', () => {
    const store = makeStore()
    store.getState().setChecklistState({ choseAgent: true, addedRepo: true })

    expect(store.getState().checklistState.choseAgent).toBe(true)
    expect(store.getState().checklistState.addedRepo).toBe(true)
    expect(store.getState().checklistState.ranFirstAgent).toBe(false) // unchanged
  })

  it('markServerChecklistItem preserves other fields on same server', () => {
    const store = makeStore()
    store.getState().markServerChecklistItem('ds-X', 'addedRepo', true)
    store.getState().markServerChecklistItem('ds-X', 'ranFirstAgent', true)

    const cl = store.getState().checklistState.perServer['ds-X']!
    expect(cl.addedRepo).toBe(true)
    expect(cl.ranFirstAgent).toBe(true)
  })
})
