/**
 * Migration 0015 — AI Provider Key Rotation (BUG-BE-HLD-014)
 *
 * Adds rotation_grace_until so a 'rotating' account can be recovered by
 * ProviderHealthChecker's cron sweep if Orca Server restarts mid-rotation
 * (see AIProviderService.rotateKey()/completeRotation()).
 *
 * Numbered 0015, not 0014 as the source solution doc suggested — 0014 was
 * already taken by 0014_workflow_pause_state.ts (TASK-HLD-015, same session).
 *
 * @module db/migrations/0015_ai_provider_rotation
 */

import type { Migration } from './types'

export const migration0015AiProviderRotation: Migration = {
  version: 15,
  name: 'ai_provider_rotation',

  async up(db) {
    // Why: NULL means "not rotating". Set by rotateKey(), cleared by
    // completeRotation() — see AIProviderService.ts §rotateKey.
    await db.exec(`ALTER TABLE orca_ai_provider_accounts ADD COLUMN rotation_grace_until BIGINT`)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_ai_providers_rotating
        ON orca_ai_provider_accounts(status, rotation_grace_until)
    `)
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern migration 0013).
  },
}
