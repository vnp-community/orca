/**
 * telemetry.* — server-mode port of desktop/src/main/runtime/rpc/methods/telemetry.ts.
 *
 * Every dependency below already exists server-side (backend/src/main/telemetry/*)
 * except `deriveOptInVia` (small, ported inline here — see its doc comment)
 * and the Store reference, reused from `getActiveOnboardingStore()`
 * (onboarding-ipc.ts's lazy singleton, set from the same `store` instance
 * server-bootstrap.ts constructs — not a second Store reference).
 *
 * Note: `initTelemetry(store)`/`initCohortClassifier(store)` are not called
 * anywhere in server-bootstrap.ts yet (server mode has never sent telemetry —
 * a separate, deliberate product decision: what identity/project should
 * server-mode events report under). `track()` already no-ops safely when
 * uninitialized (`!posthog || !commonProps || !storeRef` guard in client.ts),
 * so wiring this RPC surface now is safe — it just means events are silently
 * dropped until that initialization decision is made, instead of throwing
 * "Unknown method" on every frontend telemetry call.
 *
 * @module main/runtime/rpc/methods/telemetry
 */

import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { getActiveOnboardingStore } from '../../../ipc/onboarding-ipc'
import { consumeConsentMutationToken } from '../../../telemetry/burst-cap'
import { persistBannerAcknowledgeWithoutEmitting, setOptIn, track } from '../../../telemetry/client'
import { getCohortAtEmit } from '../../../telemetry/cohort-classifier'
import { getOnboardingCohortAtEmit } from '../../../telemetry/onboarding-cohort-classifier'
import { resolveConsent, type ConsentState } from '../../../telemetry/consent'
import { isCohortExtendedEvent, isOnboardingEvent } from '../../../../shared/telemetry-events'
import type { EventName, EventProps, OptInVia } from '../../../../shared/telemetry-events'
import type { Store } from '../../../persistence'

const MAIN_OWNED_TELEMETRY_EVENTS = new Set<EventName>([
  'app_starred_orca',
  'star_nag_outcome',
  'feature_interaction_usage_bucket_reached'
])

const Track = z.object({
  name: z.string().min(1),
  props: z.record(z.string(), z.unknown()).optional()
})

const SetOptIn = z.object({ optedIn: z.boolean() })

/**
 * Ported from desktop/src/main/ipc/telemetry.ts's `deriveOptInVia` (small,
 * pure — see that file for the full two-case rationale: existing-user
 * first-launch-banner "Turn off" click vs. any other Settings/Privacy flip).
 */
function deriveOptInVia(store: Store, incomingOptedIn: boolean): OptInVia {
  const telemetry = store.getSettings().telemetry
  const existedBefore = telemetry?.existedBeforeTelemetryRelease === true
  const currentOptedIn = telemetry?.optedIn
  if (existedBefore && currentOptedIn === null && incomingOptedIn === false) {
    return 'first_launch_banner'
  }
  return 'settings'
}

export const TELEMETRY_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'telemetry.track',
    params: Track,
    handler: async (params): Promise<void> => {
      const eventName = params.name as EventName
      if (MAIN_OWNED_TELEMETRY_EVENTS.has(eventName)) {
        return
      }
      const baseProps = (params.props ?? {}) as Record<string, unknown>
      const withRepoCohort = isCohortExtendedEvent(eventName)
        ? { ...baseProps, ...getCohortAtEmit() }
        : baseProps
      const finalProps = isOnboardingEvent(eventName)
        ? { ...withRepoCohort, ...getOnboardingCohortAtEmit() }
        : withRepoCohort
      track(eventName, finalProps as EventProps<EventName>)
    }
  }),
  defineMethod({
    name: 'telemetry.setOptIn',
    params: SetOptIn,
    handler: async (params): Promise<void> => {
      const store = getActiveOnboardingStore()
      if (!store) {return}
      if (!consumeConsentMutationToken()) {return}
      const via = deriveOptInVia(store, params.optedIn)
      await setOptIn(via, params.optedIn)
    }
  }),
  defineMethod({
    name: 'telemetry.getConsentState',
    params: null,
    handler: async (): Promise<ConsentState> => {
      const store = getActiveOnboardingStore()
      if (!store) {return { effective: 'pending_banner' }}
      return resolveConsent(store.getSettings())
    }
  }),
  defineMethod({
    name: 'telemetry.acknowledgeBanner',
    params: null,
    handler: async (): Promise<void> => {
      const store = getActiveOnboardingStore()
      if (!store) {return}
      const telemetry = store.getSettings().telemetry
      if (telemetry?.existedBeforeTelemetryRelease !== true || telemetry?.optedIn !== null) {return}
      if (!consumeConsentMutationToken()) {return}
      await persistBannerAcknowledgeWithoutEmitting()
    }
  })
]
