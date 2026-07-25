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
  private vapidKeys: { publicKey: string; privateKey: string }

  constructor(private store: Store) {
    this.vapidKeys = this.loadOrCreateVapidKeys()
    webPush.setVapidDetails(
      'mailto:admin@orca.local',
      this.vapidKeys.publicKey,
      this.vapidKeys.privateKey
    )
  }

  /** Returns the VAPID public key so the renderer can subscribe. */
  getPublicKey(): string {
    return this.vapidKeys.publicKey
  }

  /**
   * Save (or upsert) a push subscription.
   * Deduplication is by endpoint — a new subscription from the same browser
   * replaces the old one (keys may rotate).
   */
  saveSubscription(
    subscription: PushSubscriptionJSON,
    meta?: { userAgent?: string }
  ): WebPushSubscription {
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

    const existing = this.store.getWebPushSubscriptions()
    this.store.setWebPushSubscriptions([
      ...existing.filter((s) => s.endpoint !== record.endpoint),
      record
    ])
    return record
  }

  /** Remove a subscription by endpoint URL. */
  removeSubscription(endpoint: string): void {
    const existing = this.store.getWebPushSubscriptions()
    this.store.setWebPushSubscriptions(existing.filter((s) => s.endpoint !== endpoint))
  }

  /**
   * Send a notification to all saved subscriptions.
   * Uses Promise.allSettled so one failed delivery doesn't block the others.
   */
  async sendToAll(payload: WebPushPayload): Promise<void> {
    const subscriptions = this.store.getWebPushSubscriptions()
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
        this.removeSubscription(sub.endpoint)
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
  private loadOrCreateVapidKeys(): { publicKey: string; privateKey: string } {
    const stored = this.store.getVapidKeys()
    if (stored) return stored
    const keys = webPush.generateVAPIDKeys()
    this.store.setVapidKeys(keys)
    return keys
  }
}
