import { track } from '@/lib/telemetry'
import {
  getRuntimeOnboardingState,
  updateRuntimeOnboardingState,
  type RuntimeOnboardingSettings
} from '@/runtime/runtime-onboarding-client'
import type { OnboardingState } from '../../../shared/types'

export type OnboardingProjectChecklistItem = 'addedRepo' | 'addedFolder'

// Why settings is a parameter, not `useAppStore.getState().settings` read
// internally: this module used to import `useAppStore` from '@/store' at
// module scope — repos.ts (a real slice of that same store) imports this
// module, and `@/store`'s own composition imports every slice including
// repos.ts, so store/index.ts -> repos.ts -> this module -> store/index.ts
// was a genuine import cycle. Under ESM's circular-import semantics,
// whichever file loads first sees an unfinished `createRepoSlice` export
// from the other and crashes at composition time
// ("createRepoSlice is not a function") — reproduced live in this exact
// shape across all 19 repos*.test.ts files. The caller (repos.ts) already
// has `get().settings` on hand, so threading it through as a parameter
// removes this module's only edge back into the store entirely.
export async function markOnboardingProjectAdded(
  item: OnboardingProjectChecklistItem,
  settings: RuntimeOnboardingSettings | null | undefined
): Promise<void> {
  if (typeof window === 'undefined' || !window.api?.onboarding) {
    return
  }
  const onboarding = await getRuntimeOnboardingState(settings).catch(() => null)
  if (!onboarding || onboarding.checklist[item]) {
    return
  }

  const checklist: Partial<OnboardingState['checklist']> = {}
  checklist[item] = true
  try {
    await updateRuntimeOnboardingState(settings, { checklist })
  } catch (err) {
    console.warn('[onboarding] Failed to update project checklist item:', err)
    return
  }

  track('activation_checklist_item_completed', {
    item,
    time_since_completed_ms: 0
  })
}
