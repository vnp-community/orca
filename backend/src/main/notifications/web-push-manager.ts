// ─── web-push-manager.ts ────────────────────────────────────────────────────
// VAPID key lifecycle, subscription CRUD, and send-to-all for Web Push.
// Phase 3 — TASK-032.
//
// Why: sends push notifications to subscribed browsers so Orca can alert on
// long-running events (clone complete, preflight failure) without the
// renderer being open. VAPID keys are generated once and reused across
// restarts via the Store persistence layer (top-level PersistedState — TASK-033).

import webPush from 'web-push'
import { randomUUID } from 'node:crypto'
import type { Store } from '../persistence'
import type { WebPushSubscription } from '../../shared/types'

export type { WebPushSubscription }

/**
 * ADR-021 Phase 1: narrow interface seam for WebPushManager's `Store`
 * dependency — the 4 methods below (verified by grep) are its complete
 * persistence surface. A future Postgres-backed implementation (migration
 * 0022's `notification` schema —
 * specs/backend/models/08-postgres-microservices-target-architecture.md §4)
 * only needs these 4 methods to be a drop-in replacement in server mode. See
 * automations/automation-store-dependency.ts's module doc comment for the
 * same pattern applied to AutomationService — including why this is ASYNC
 * (not a direct `Pick<Store, ...>`, unlike this file's first version): `Store`
 * no longer satisfies this for free, `notifications/web-push-store-adapter.ts`
 * bridges the two.
 *
 * ⚠️ `getVapidKeys()`/`setVapidKeys()` carry the VAPID *private* key — per
 * ADR-021 §4, a Postgres-backed implementation of this interface must NOT
 * store that value in the `notification.vapid_key_metadata` table (public
 * key/status only); it needs its own credential-store-backed path for the
 * private key, same as every other secret in specs/backend/models/05.
 */
export type WebPushStoreDependency = {
  getWebPushSubscriptions(): Promise<WebPushSubscription[]>
  setWebPushSubscriptions(subscriptions: WebPushSubscription[]): Promise<void>
  getVapidKeys(): Promise<{ publicKey: string; privateKey: string } | null | undefined>
  setVapidKeys(keys: { publicKey: string; privateKey: string }): Promise<void>
}

export type WebPushPayload = {
  /** Short notification title (required). */
  title: string
  /** Notification body text. */
  body: string
  /** Icon URL (optional). */
  icon?: string
  /** Notification tag — same-tag notifications replace each other. */
  tag?: string
  /** URL to open when the notification is clicked. */
  url?: string
}

// TTL: 24 hours — notifications are delivered within a day or dropped.
const DEFAULT_TTL_SECONDS = 86_400

export class WebPushManager {
  private vapidKeys: { publicKey: string; privateKey: string } | null = null
  // Why memoized as a promise, not just lazily awaited each call: two
  // concurrent first-callers (e.g. a subscribe RPC racing sendToAll on
  // startup) must not each independently generateVAPIDKeys() and clobber
  // each other's write — the old constructor-eager-load never had this race
  // (Node's single-threaded constructor ran once, synchronously); this is the
  // async equivalent of that same "exactly once" guarantee.
  private vapidKeysPromise: Promise<{ publicKey: string; privateKey: string }> | null = null

  // Why no eager load here (unlike the sync-Store version this replaces):
  // WebPushStoreDependency is async now (ADR-021 §"Deliberately ASYNC" in
  // automation-store-dependency.ts applies the same here), and constructors
  // cannot be async — key loading moves to ensureVapidKeys(), called lazily
  // by every method that needs it.
  constructor(private store: WebPushStoreDependency) {}

  private async ensureVapidKeys(): Promise<{ publicKey: string; privateKey: string }> {
    if (this.vapidKeys) {return this.vapidKeys}
    if (!this.vapidKeysPromise) {
      this.vapidKeysPromise = this.loadOrCreateVapidKeys().then((keys) => {
        this.vapidKeys = keys
        webPush.setVapidDetails('mailto:admin@orca.local', keys.publicKey, keys.privateKey)
        return keys
      })
    }
    return this.vapidKeysPromise
  }

  /** Returns the VAPID public key so the renderer can subscribe. */
  async getPublicKey(): Promise<string> {
    const keys = await this.ensureVapidKeys()
    return keys.publicKey
  }

  /**
   * Save (or upsert) a push subscription.
   * Deduplication is by endpoint — a new subscription from the same browser
   * replaces the old one (keys may rotate).
   */
  async saveSubscription(
    subscription: PushSubscriptionJSON,
    meta?: { userAgent?: string }
  ): Promise<WebPushSubscription> {
    const record: WebPushSubscription = {
      id: randomUUID(),
      endpoint: subscription.endpoint!,
      keys: {
        auth: subscription.keys!.auth,
        p256dh: subscription.keys!.p256dh
      },
      addedAt: Date.now(),
      userAgent: meta?.userAgent
    }

    const existing = await this.store.getWebPushSubscriptions()
    await this.store.setWebPushSubscriptions([
      ...existing.filter((s) => s.endpoint !== record.endpoint),
      record
    ])
    return record
  }

  /** Remove a subscription by endpoint URL. */
  async removeSubscription(endpoint: string): Promise<void> {
    const existing = await this.store.getWebPushSubscriptions()
    await this.store.setWebPushSubscriptions(existing.filter((s) => s.endpoint !== endpoint))
  }

  /**
   * Send a notification to all saved subscriptions.
   * Uses Promise.allSettled so one failed delivery doesn't block the others.
   */
  async sendToAll(payload: WebPushPayload): Promise<void> {
    await this.ensureVapidKeys()
    const subscriptions = await this.store.getWebPushSubscriptions()
    await Promise.allSettled(
      subscriptions.map((sub) => this.sendToSubscription(sub, payload))
    )
  }

  private async sendToSubscription(
    sub: WebPushSubscription,
    payload: WebPushPayload
  ): Promise<void> {
    try {
      await webPush.sendNotification(
        { endpoint: sub.endpoint, keys: sub.keys },
        JSON.stringify(payload),
        { TTL: DEFAULT_TTL_SECONDS }
      )
    } catch (err: unknown) {
      // 410 Gone: the browser revoked the subscription — auto-remove to keep
      // the subscription list lean and avoid repeated failed sends.
      if ((err as { statusCode?: number }).statusCode === 410) {
        await this.removeSubscription(sub.endpoint)
      }
      // All other errors: log but do not rethrow (other subs must still deliver).
      // In production, plug in a structured logger here.
    }
  }

  /**
   * Load existing VAPID keys from store or generate and persist new ones.
   * Why: VAPID keys must be stable across restarts — browsers associate
   * subscriptions with the public key and reject notifications from a
   * different key even for the same endpoint.
   */
  private async loadOrCreateVapidKeys(): Promise<{ publicKey: string; privateKey: string }> {
    const stored = await this.store.getVapidKeys()
    if (stored) {return stored}
    const keys = webPush.generateVAPIDKeys()
    await this.store.setVapidKeys(keys)
    return keys
  }
}

/** Re-exported so callers constructing WebPushManager don't need a separate `Store` import. */
export type { Store }
