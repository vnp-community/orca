/**
 * Onboarding Checklist Slice — reactive selector hooks (CR-OB-009)
 *
 * Split out from onboarding-checklist.ts: that file is a store SLICE
 * (imported by store/index.ts's own composition), while these hooks need
 * `useAppStore` itself — importing it at that slice file's top level created
 * a real circular dependency (store/index.ts -> onboarding-checklist.ts ->
 * '../index' -> store/index.ts), live-reproduced as
 * "createOnboardingChecklistSlice is not a function" whenever a test (or
 * any other consumer) imported this module before store/index.ts had
 * finished its own evaluation. Same fix shape as this session's
 * `onboarding-project-checklist.ts` cycle: move the `useAppStore`-dependent
 * code somewhere store/index.ts never has to load.
 */
import { useShallow } from 'zustand/react/shallow'
import { useAppStore } from '../index'
import type { PerServerChecklistState } from '../../../../shared/dev-server-types'

/** Reactive selector for a specific server's per-server checklist state */
export function useServerChecklist(devServerId: string | null): PerServerChecklistState {
  return useAppStore(
    useShallow((s) => (devServerId ? (s.checklistState.perServer?.[devServerId] ?? {}) : {}))
  )
}

/** True when all required global items are done */
export function useGlobalChecklistComplete(): boolean {
  return useAppStore((s) => {
    const cl = s.checklistState
    return cl.choseAgent && cl.addedRepo && cl.ranFirstAgent
  })
}
