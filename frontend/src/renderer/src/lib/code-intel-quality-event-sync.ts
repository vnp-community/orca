/**
 * code-intel-quality-event-sync.ts — FE-CV-TASK-087-02
 *
 * Feeds code-intel push events (event bus) into the quality slice. Ref-counted: the single bus
 * subscription lives while at least one quality hook is mounted, so no quality surface means no
 * listener and no state growth.
 *
 * @module lib/code-intel-quality-event-sync
 */

import { useAppStore } from '@/store'
import { subscribeCodeIntelEvents } from './code-intel-event-bus'

let refCount = 0
let unsubscribe: (() => void) | null = null

export function retainQualityEventSync(): () => void {
  refCount += 1
  if (refCount === 1) {
    unsubscribe = subscribeCodeIntelEvents((event) => {
      if (
        event.event === 'qualityProgress' ||
        event.event === 'qualityFinished' ||
        event.event === 'gateChanged' ||
        event.event === 'changed'
      ) {
        useAppStore.getState().applyQualityPushEvent(event)
      }
    })
  }
  let released = false
  return () => {
    if (released) {
      return
    }
    released = true
    refCount -= 1
    if (refCount === 0) {
      unsubscribe?.()
      unsubscribe = null
    }
  }
}
