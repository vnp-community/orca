/**
 * wrapStoreAsWebPushStoreDependency — bridges `Store` (sync, in-memory) to
 * `WebPushStoreDependency` (async, ADR-021 Phase 1)
 *
 * Same pattern as automations/automation-store-store-adapter.ts — see that
 * file's module doc comment for why this bridge exists (a sync-only `Store`
 * can never structurally satisfy an interface a real async backend also has
 * to implement) and why `Promise.resolve()`-wrapping a sync call is always
 * safe in this direction.
 *
 * @module main/notifications/web-push-store-adapter
 */

import type { Store } from '../persistence'
import type { WebPushStoreDependency } from './web-push-manager'

export function wrapStoreAsWebPushStoreDependency(store: Store): WebPushStoreDependency {
  return {
    getWebPushSubscriptions: async () => store.getWebPushSubscriptions(),
    setWebPushSubscriptions: async (subscriptions) => store.setWebPushSubscriptions(subscriptions),
    getVapidKeys: async () => store.getVapidKeys(),
    setVapidKeys: async (keys) => store.setVapidKeys(keys)
  }
}
