/**
 * MobileCompanionService — Web Push notification service (TASK-MB-001)
 *
 * Implements RFC 8030 Web Push protocol to send notifications to paired mobile browsers.
 * Requires VAPID keys in environment variables:
 *   VAPID_PUBLIC_KEY  — base64url-encoded VAPID public key
 *   VAPID_PRIVATE_KEY — base64url-encoded VAPID private key
 *   ORCA_ADMIN_EMAIL  — contact email for VAPID subject
 *
 * Generate VAPID keys:
 *   npx web-push generate-vapid-keys
 *
 * Database table: orca_push_subscriptions (see migration)
 * Migration SQL:
 *   CREATE TABLE IF NOT EXISTS orca_push_subscriptions (
 *     user_id    TEXT NOT NULL,
 *     endpoint   TEXT NOT NULL PRIMARY KEY,
 *     auth       TEXT NOT NULL,   -- base64url auth key
 *     p256dh     TEXT NOT NULL,   -- base64url p256dh key
 *     updated_at INTEGER NOT NULL
 *   )
 *
 * @module main/mobile/MobileCompanionService
 */

import webpush from 'web-push'
import type { IConnectionPool } from '../db/pool'

// ── Types ─────────────────────────────────────────────────────────────────────

export interface PushSubscriptionKeys {
  auth:   string
  p256dh: string
}

export interface PushSubscription {
  endpoint: string
  keys:     PushSubscriptionKeys
}

export interface NotificationPayload {
  title: string
  body:  string
  url?:  string
  icon?: string
  tag?:  string
}

// ── MobileCompanionService ────────────────────────────────────────────────────

export class MobileCompanionService {
  private readonly vapidConfigured: boolean

  constructor(private readonly pool: IConnectionPool) {
    // FIX TASK-MB-001: Initialize VAPID keys from environment variables.
    // VAPID allows the push service to verify the origin of push messages.
    const publicKey  = process.env['VAPID_PUBLIC_KEY']
    const privateKey = process.env['VAPID_PRIVATE_KEY']
    const adminEmail = process.env['ORCA_ADMIN_EMAIL'] ?? 'admin@example.com'

    if (publicKey && privateKey) {
      try {
        webpush.setVapidDetails(`mailto:${adminEmail}`, publicKey, privateKey)
        this.vapidConfigured = true
      } catch (err) {
        console.error('[MobileCompanionService] Invalid VAPID keys:', err)
        this.vapidConfigured = false
      }
    } else {
      console.warn('[MobileCompanionService] VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY not set — push disabled')
      this.vapidConfigured = false
    }
  }

  // ── Subscription management ─────────────────────────────────────────────────

  /**
   * Register or update a device push subscription for a user.
   * Uses UPSERT by endpoint (idempotent for re-subscriptions).
   */
  async registerDevice(userId: string, subscription: PushSubscription): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT OR REPLACE INTO orca_push_subscriptions
           (user_id, endpoint, auth, p256dh, updated_at)
         VALUES (?, ?, ?, ?, ?)`,
        [
          userId,
          subscription.endpoint,
          subscription.keys.auth,
          subscription.keys.p256dh,
          Date.now(),
        ]
      )
    )
  }

  /**
   * Remove a specific device subscription.
   */
  async unregisterDevice(endpoint: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `DELETE FROM orca_push_subscriptions WHERE endpoint = ?`,
        [endpoint]
      )
    )
  }

  /**
   * List all subscriptions for a user.
   */
  async listDevices(userId: string): Promise<PushSubscription[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ endpoint: string; auth: string; p256dh: string }>(
        `SELECT endpoint, auth, p256dh
         FROM orca_push_subscriptions
         WHERE user_id = ?
         ORDER BY updated_at DESC`,
        [userId]
      )
    )
    return rows.map((r) => ({
      endpoint: r.endpoint,
      keys: { auth: r.auth, p256dh: r.p256dh },
    }))
  }

  // ── Push notification ───────────────────────────────────────────────────────

  /**
   * Send a push notification to all registered devices for a user.
   * Non-throwing — individual device failures are logged but don't block others.
   * Automatically removes expired/gone subscriptions (HTTP 410).
   */
  async notify(userId: string, payload: NotificationPayload): Promise<void> {
    if (!this.vapidConfigured) {
      console.warn('[MobileCompanionService] Push skipped — VAPID not configured')
      return
    }

    const subscriptions = await this.listDevices(userId)
    if (subscriptions.length === 0) return

    const body = JSON.stringify(payload)

    await Promise.allSettled(
      subscriptions.map(async (sub) => {
        try {
          await webpush.sendNotification(
            { endpoint: sub.endpoint, keys: sub.keys },
            body
          )
        } catch (err: unknown) {
          const httpErr = err as { statusCode?: number }
          if (httpErr?.statusCode === 410) {
            // 410 Gone — subscription expired, clean it up
            console.log(`[MobileCompanionService] Removing expired subscription: ${sub.endpoint.slice(0, 40)}...`)
            await this.unregisterDevice(sub.endpoint).catch(() => {})
          } else {
            console.warn('[MobileCompanionService] Push failed:', err)
          }
        }
      })
    )
  }

  /**
   * Broadcast a notification to all users (admin use).
   * Fetches all unique users with active subscriptions.
   */
  async broadcast(payload: NotificationPayload): Promise<void> {
    if (!this.vapidConfigured) return

    const rows = await this.pool.withConnection((db) =>
      db.query<{ user_id: string }>(
        `SELECT DISTINCT user_id FROM orca_push_subscriptions`
      )
    )

    await Promise.allSettled(
      rows.map((r) => this.notify(r.user_id, payload))
    )
  }
}
