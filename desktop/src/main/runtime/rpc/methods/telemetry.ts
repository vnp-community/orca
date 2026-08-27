import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { consumeConsentMutationToken } from '../../../telemetry/burst-cap'
import {
  persistBannerAcknowledgeWithoutEmitting,
  setOptIn,
  track
} from '../../../telemetry/client'
import { getCohortAtEmit } from '../../../telemetry/cohort-classifier'
import { getOnboardingCohortAtEmit } from '../../../telemetry/onboarding-cohort-classifier'
import { resolveConsent, type ConsentState } from '../../../telemetry/consent'
import { isCohortExtendedEvent, isOnboardingEvent } from '../../../../shared/telemetry-events'
import type { EventName, EventProps } from '../../../../shared/telemetry-events'
import { deriveOptInVia, getTelemetryStoreForRpc } from '../../../ipc/telemetry'

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

// Why: mirrors desktop/src/main/ipc/telemetry.ts's four ipcMain handlers
// field-for-field (same cohort injection, same consent-mutation rate limit,
// same `via` derivation reusing `deriveOptInVia`) so an RPC-originated event
// funnels through the identical `track()` enforcement point.
export const TELEMETRY_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'telemetry.track',
    params: Track,
    handler: (params): void => {
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
    handler: (params): Promise<void> | void => {
      const storeRef = getTelemetryStoreForRpc()
      if (!storeRef) {
        return
      }
      if (!consumeConsentMutationToken()) {
        return
      }
      const via = deriveOptInVia(storeRef, params.optedIn)
      return setOptIn(via, params.optedIn)
    }
  }),
  defineMethod({
    name: 'telemetry.getConsentState',
    params: null,
    handler: (): ConsentState => {
      const storeRef = getTelemetryStoreForRpc()
      if (!storeRef) {
        return { effective: 'pending_banner' }
      }
      return resolveConsent(storeRef.getSettings())
    }
  }),
  defineMethod({
    name: 'telemetry.acknowledgeBanner',
    params: null,
    handler: (): Promise<void> | void => {
      const storeRef = getTelemetryStoreForRpc()
      if (!storeRef) {
        return
      }
      const telemetry = storeRef.getSettings().telemetry
      if (telemetry?.existedBeforeTelemetryRelease !== true || telemetry?.optedIn !== null) {
        return
      }
      if (!consumeConsentMutationToken()) {
        return
      }
      return persistBannerAcknowledgeWithoutEmitting()
    }
  })
]
