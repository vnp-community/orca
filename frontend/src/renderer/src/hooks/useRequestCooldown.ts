/**
 * useRequestCooldown — TASK-FE-E2E-006
 *
 * `coolingDown` is true while the gateway's limiter for this class told us to wait.
 * Buttons that start a call of that class disable themselves on it.
 *
 * @module hooks/useRequestCooldown
 */

import { useEffect, useState, useSyncExternalStore } from 'react'
import {
  cooldownRemainingMs,
  requestCooldownVersion,
  subscribeRequestCooldown,
  type RateLimitClass
} from '../runtime/request-rate-limit'

export type RequestCooldown = { coolingDown: boolean; remainingMs: number }

export function useRequestCooldown(cls: RateLimitClass): RequestCooldown {
  useSyncExternalStore(subscribeRequestCooldown, requestCooldownVersion)
  const remainingMs = cooldownRemainingMs(cls)
  const [, setTick] = useState(0)
  // Why: nothing else changes when the wait ends, so schedule one re-render at the end.
  useEffect(() => {
    if (remainingMs <= 0) {
      return undefined
    }
    const t = setTimeout(() => setTick((n) => n + 1), remainingMs + 5)
    return () => clearTimeout(t)
  }, [remainingMs])
  return { coolingDown: remainingMs > 0, remainingMs }
}
