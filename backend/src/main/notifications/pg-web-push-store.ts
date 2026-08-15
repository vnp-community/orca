/**
 * PgWebPushStore — Postgres-backed `WebPushStoreDependency` (ADR-021 Phase 1)
 *
 * Writes/reads `notification.push_subscriptions` /
 * `notification.vapid_key_metadata` (migration 0022_notification_usage_schema.ts).
 * Server mode only — see ADR-021 §"Không áp dụng cho Electron Desktop mode".
 *
 * Per ADR-021 §4 (and this file's own migration's column comment): the VAPID
 * *private* key never touches Postgres. `getVapidKeys()`/`setVapidKeys()`
 * split across two stores — `notification.vapid_key_metadata` (public key +
 * status only) and `VapidKeySecretStore` (encrypted file, the private key) —
 * and stitch the two back into the single `{publicKey, privateKey}` shape
 * `WebPushStoreDependency` (and `WebPushManager`) expects, so this class is
 * still a drop-in implementation of that interface.
 *
 * @module main/notifications/pg-web-push-store
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import { serviceQualifiedTable } from '../db/migrations/sql-dialect'
import type { WebPushSubscription } from '../../shared/types'
import type { WebPushStoreDependency } from './web-push-manager'
import type { VapidKeySecretStore } from './vapid-key-secret-store'

type SubscriptionRow = {
  id: string
  endpoint: string
  auth: string
  p256dh: string
  userAgent: string | null
  addedAt: number
}

function rowToSubscription(row: SubscriptionRow): WebPushSubscription {
  return {
    id: row.id,
    endpoint: row.endpoint,
    keys: { auth: row.auth, p256dh: row.p256dh },
    addedAt: row.addedAt,
    userAgent: row.userAgent ?? undefined
  }
}

export class PgWebPushStore implements WebPushStoreDependency {
  /**
   * @param tenantId / @param userId Resolved once per user-process (ADR-021
   * §3 — same invariant as PgAutomationStore). `userId` defaults subscriptions
   * to a shared 'system' row-owner when absent, matching `Store`'s
   * single-install (not-really-per-user) subscription list — server-mode
   * multi-user push scoping needs `saveSubscription()`'s caller to plumb a
   * real per-request userId through, which `WebPushStoreDependency`'s
   * signature doesn't carry today (see also ADR-021 §5's Phase 1 TODO list).
   */
  constructor(
    private readonly pool: IConnectionPool,
    private readonly vapidSecrets: VapidKeySecretStore,
    private readonly tenantId: string | undefined,
    private readonly userId: string = 'system'
  ) {}

  private subscriptionsTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'notification', 'push_subscriptions')
  }

  private vapidMetadataTable(dialect: Parameters<typeof serviceQualifiedTable>[0]): string {
    return serviceQualifiedTable(dialect, 'notification', 'vapid_key_metadata')
  }

  async getWebPushSubscriptions(): Promise<WebPushSubscription[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<SubscriptionRow>(
        `SELECT id, endpoint, auth, p256dh, user_agent as userAgent, added_at as addedAt
         FROM ${this.subscriptionsTable(db.capabilities.dialect)}` +
          (this.tenantId ? ' WHERE tenant_id = ?' : ''),
        this.tenantId ? [this.tenantId] : []
      )
    )
    return rows.map(rowToSubscription)
  }

  async setWebPushSubscriptions(subscriptions: WebPushSubscription[]): Promise<void> {
    // Why replace-all, not diff/upsert: mirrors Store.setWebPushSubscriptions'
    // own contract exactly — WebPushManager always calls it with the full
    // desired list (add: existing+new, remove: filtered existing), never a
    // delta. withTransaction so a crash mid-write can't leave the table
    // half-cleared.
    await this.pool.withTransaction(async (db) => {
      const table = this.subscriptionsTable(db.capabilities.dialect)
      if (this.tenantId) {
        await db.query(`DELETE FROM ${table} WHERE tenant_id = ?`, [this.tenantId])
      } else {
        await db.query(`DELETE FROM ${table}`)
      }
      for (const sub of subscriptions) {
        await db.query(
          `INSERT INTO ${table} (id, tenant_id, user_id, endpoint, auth, p256dh, user_agent, added_at)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
          [
            sub.id || randomUUID(), this.tenantId ?? null, this.userId, sub.endpoint,
            sub.keys.auth, sub.keys.p256dh, sub.userAgent ?? null, sub.addedAt
          ]
        )
      }
    })
  }

  async getVapidKeys(): Promise<{ publicKey: string; privateKey: string } | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ publicKey: string }>(
        `SELECT public_key as publicKey FROM ${this.vapidMetadataTable(db.capabilities.dialect)}
         WHERE status = 'active'` + (this.tenantId ? ' AND tenant_id = ?' : ' AND tenant_id IS NULL'),
        this.tenantId ? [this.tenantId] : []
      )
    )
    const publicKey = rows[0]?.publicKey
    if (!publicKey) {return null}
    const privateKey = await this.vapidSecrets.getPrivateKey()
    if (!privateKey) {
      // Metadata exists but the secret file doesn't (host migrated, file
      // deleted, wrong ORCA_SERVER_SECRET...) — treat as "no usable keypair"
      // so WebPushManager regenerates, rather than sending with a public key
      // whose private half we can no longer prove possession of.
      return null
    }
    return { publicKey, privateKey }
  }

  async setVapidKeys(keys: { publicKey: string; privateKey: string }): Promise<void> {
    await this.vapidSecrets.setPrivateKey(keys.privateKey)
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO ${this.vapidMetadataTable(db.capabilities.dialect)}
           (key_id, tenant_id, public_key, status, created_at)
         VALUES (?, ?, ?, 'active', ?)`,
        [randomUUID(), this.tenantId ?? null, keys.publicKey, Date.now()]
      )
    )
  }
}
