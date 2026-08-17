import { ipcMain } from 'electron'
import { sanitizeOnboardingUpdate, type Store } from '../persistence'
import type { OnboardingState } from '../../shared/types'

// Why: the RPC method registry (runtime/rpc/methods/onboarding.ts) is a static
// array evaluated at module load, before `store` exists — it reads this
// singleton lazily inside each handler instead, the same pattern
// getActiveStarNagService()/getActiveOrcaProfileHandlerContext() use.
let activeOnboardingStore: Store | null = null

export function getActiveOnboardingStore(): Store | null {
  return activeOnboardingStore
}

export function registerOnboardingHandlers(store: Store): void {
  activeOnboardingStore = store

  ipcMain.removeHandler('onboarding:get')
  ipcMain.removeHandler('onboarding:update')

  ipcMain.handle('onboarding:get', (): OnboardingState => store.getOnboarding())
  // Why: never trust renderer input — a compromised/buggy caller could send
  // unknown keys or wrong-typed values that would poison persisted state.
  // Run every update through the shared whitelist sanitizer.
  ipcMain.handle('onboarding:update', (_event, updates: unknown): OnboardingState => {
    return store.updateOnboarding(sanitizeOnboardingUpdate(updates))
  })
}
