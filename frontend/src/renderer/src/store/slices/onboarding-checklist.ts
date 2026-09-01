/**
 * Onboarding Checklist Slice (CR-OB-009)
 *
 * Stores global + per-server checklist progress for the new remote-relay
 * architecture where agents run on developer machines.
 */
import type { StateCreator } from 'zustand'
import { markRuntimeOnboardingChecklistItem } from '../../runtime/runtime-onboarding-client'
import type { OnboardingChecklistState } from '../../../../shared/types'
import type { PerServerChecklistState } from '../../../../shared/dev-server-types'
import type { AppState } from '../types'
// Why no `useAppStore` import here: this file is a store SLICE (imported by
// store/index.ts's own composition) — importing '../index' at its top level
// is a real circular dependency (store/index.ts -> this file -> '../index'
// -> store/index.ts), live-reproduced as "createOnboardingChecklistSlice is
// not a function". The two one-time settings reads below use this slice
// creator's own `get` instead; the reactive selector hooks that genuinely
// need `useAppStore` moved to onboarding-checklist-selectors.ts, which
// store/index.ts never has to load.

// ─── Extended state types ─────────────────────────────────────────────────────

export type OnboardingExtendedChecklistState = OnboardingChecklistState & {
  perServer: Record<string, PerServerChecklistState>
}

// ─── Slice type ───────────────────────────────────────────────────────────────

export type OnboardingChecklistSlice = {
  checklistState: OnboardingExtendedChecklistState

  markGlobalChecklistItem: (item: keyof OnboardingChecklistState, value?: boolean) => void

  markServerChecklistItem: (
    devServerId: string,
    item: keyof PerServerChecklistState,
    value?: boolean
  ) => void

  setChecklistState: (state: Partial<OnboardingExtendedChecklistState>) => void
}

// ─── Default state ────────────────────────────────────────────────────────────

const DEFAULT_GLOBAL_CHECKLIST: OnboardingChecklistState = {
  addedRepo: false,
  choseAgent: false,
  ranFirstAgent: false,
  ranSecondAgentOnSameTask: false,
  triedCmdJ: false,
  shapedSidebar: false,
  reviewedDiff: false,
  openedPr: false,
  addedFolder: false,
  openedFile: false,
  ranAgentOnFile: false,
  dismissed: false
}

export const DEFAULT_CHECKLIST_STATE: OnboardingExtendedChecklistState = {
  ...DEFAULT_GLOBAL_CHECKLIST,
  perServer: {}
}

// ─── Slice creator ────────────────────────────────────────────────────────────

export const createOnboardingChecklistSlice: StateCreator<
  AppState,
  [],
  [],
  OnboardingChecklistSlice
> = (set, get) => ({
  checklistState: DEFAULT_CHECKLIST_STATE,

  markGlobalChecklistItem: (item, value = true) => {
    set((state) => ({
      checklistState: { ...state.checklistState, [item]: value }
    }))
    void markRuntimeOnboardingChecklistItem(get().settings, {
      item: item as string,
      value
    })
  },

  markServerChecklistItem: (devServerId, item, value = true) => {
    set((state) => ({
      checklistState: {
        ...state.checklistState,
        perServer: {
          ...state.checklistState.perServer,
          [devServerId]: {
            ...state.checklistState.perServer?.[devServerId],
            [item]: value
          }
        }
      }
    }))
    void markRuntimeOnboardingChecklistItem(get().settings, {
      item: item as string,
      devServerId,
      value
    })
  },

  setChecklistState: (partial) => {
    set((state) => ({
      checklistState: { ...state.checklistState, ...partial }
    }))
  }
})

// ─── Selectors ────────────────────────────────────────────────────────────────
//
// useServerChecklist/useGlobalChecklistComplete moved to
// onboarding-checklist-selectors.ts (see the doc comment at this file's top
// for why).
