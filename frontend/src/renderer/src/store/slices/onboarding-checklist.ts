/**
 * Onboarding Checklist Slice (CR-OB-009)
 *
 * Stores global + per-server checklist progress for the new remote-relay
 * architecture where agents run on developer machines.
 */
import type { StateCreator } from 'zustand'
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '../index'
import { markRuntimeOnboardingChecklistItem } from '../../runtime/runtime-onboarding-client'
import type { OnboardingChecklistState } from '../../../../shared/types'
import type { PerServerChecklistState } from '../../../../shared/dev-server-types'
import type { AppState } from '../types'
// Why: useAppStore is imported lazily inside each hook to avoid circular
// dependency at module evaluation time (this file is imported by store/index.ts).

// ─── Extended state types ─────────────────────────────────────────────────────

export type OnboardingExtendedChecklistState = OnboardingChecklistState & {
  perServer: Record<string, PerServerChecklistState>
}

// ─── Slice type ───────────────────────────────────────────────────────────────

export type OnboardingChecklistSlice = {
  checklistState: OnboardingExtendedChecklistState

  markGlobalChecklistItem: (
    item: keyof OnboardingChecklistState,
    value?: boolean
  ) => void

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
  dismissed: false,
}

export const DEFAULT_CHECKLIST_STATE: OnboardingExtendedChecklistState = {
  ...DEFAULT_GLOBAL_CHECKLIST,
  perServer: {},
}

// ─── Slice creator ────────────────────────────────────────────────────────────

export const createOnboardingChecklistSlice: StateCreator<
  AppState,
  [],
  [],
  OnboardingChecklistSlice
> = (set) => ({
  checklistState: DEFAULT_CHECKLIST_STATE,

  markGlobalChecklistItem: (item, value = true) => {
    set((state) => ({
      checklistState: { ...state.checklistState, [item]: value },
    }))
    void markRuntimeOnboardingChecklistItem(useAppStore.getState().settings, {
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
            [item]: value,
          },
        },
      },
    }))
    void markRuntimeOnboardingChecklistItem(useAppStore.getState().settings, {
      item: item as string,
      devServerId,
      value,
    })
  },

  setChecklistState: (partial) => {
    set((state) => ({
      checklistState: { ...state.checklistState, ...partial },
    }))
  },
})

// ─── Selectors ────────────────────────────────────────────────────────────────

/** Reactive selector for a specific server's per-server checklist state */
export function useServerChecklist(devServerId: string | null): PerServerChecklistState {
  return useAppStore(
    useShallow((s) =>
      devServerId ? (s.checklistState.perServer?.[devServerId] ?? {}) : {}
    )
  )
}

/** True when all required global items are done */
export function useGlobalChecklistComplete(): boolean {
  return useAppStore((s) => {
    const cl = s.checklistState
    return cl.choseAgent && cl.addedRepo && cl.ranFirstAgent
  })
}
