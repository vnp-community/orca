/**
 * Migration 0012 — Port Forwards Persistence + Push Subscriptions
 *
 * FIX BUG-BE-SSH-002: Add orca_port_forwards table for SSH port forward persistence.
 *   Previously in-memory only — data lost on restart.
 *
 * FIX TASK-MB-001: Add orca_push_subscriptions table for Web Push notification.
 *   Required by MobileCompanionService.
 *
 * @module db/migrations/0012_port_forwards_push
 */

import type { Migration } from './types'

export const migration0012PortForwardsPush: Migration = {
  version: 12,
  name:    'port_forwards_push',

  async up(db) {
    // ── orca_port_forwards ────────────────────────────────────────────────────
    // FIX BUG-BE-SSH-002: Persist SSH port forward mappings across server restarts.
    // localPort: ephemeral port on Orca Server (127.0.0.1:localPort → remote:remotePort)
    // hostId:    SSH target ID (from ssh-connection-store)
    // active:    1 = currently forwarding, 0 = closed/stale
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_port_forwards (
        id            TEXT    NOT NULL PRIMARY KEY,
        host_id       TEXT    NOT NULL,
        local_port    INTEGER NOT NULL,
        remote_host   TEXT    NOT NULL DEFAULT 'localhost',
        remote_port   INTEGER NOT NULL,
        label         TEXT    NOT NULL DEFAULT '',
        active        INTEGER NOT NULL DEFAULT 1,
        created_at    BIGINT NOT NULL,
        closed_at     BIGINT
      )
    `)

    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_port_forwards_host
        ON orca_port_forwards(host_id, active)
    `)

    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_port_forwards_active
        ON orca_port_forwards(active, created_at DESC)
    `)

    // ── orca_push_subscriptions ───────────────────────────────────────────────
    // TASK-MB-001: Web Push (RFC 8030) device subscriptions for mobile companion.
    // endpoint:  Push service URL (unique per device)
    // auth/p256dh: ECDH keys for encrypted push messages
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_push_subscriptions (
        user_id    TEXT    NOT NULL,
        endpoint   TEXT    NOT NULL PRIMARY KEY,
        auth       TEXT    NOT NULL,
        p256dh     TEXT    NOT NULL,
        updated_at BIGINT NOT NULL
      )
    `)

    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_push_user
        ON orca_push_subscriptions(user_id, updated_at DESC)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_push_user')
    await db.exec('DROP TABLE IF EXISTS orca_push_subscriptions')
    await db.exec('DROP INDEX IF EXISTS idx_orca_port_forwards_active')
    await db.exec('DROP INDEX IF EXISTS idx_orca_port_forwards_host')
    await db.exec('DROP TABLE IF EXISTS orca_port_forwards')
  },
}
