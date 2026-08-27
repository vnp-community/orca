/**
 * Migration 0018 — Code Review Inline Annotations
 *
 * Adds orca_annotations for BL-CR-02 (inline code-review comments):
 * annotation-panel.tsx already calls annotation.list / annotation.create —
 * the RPC methods existed nowhere and had no table to back them
 * (specs/frontend/tdd/api/gaps-and-mismatches.md §"Category 2").
 *
 * FKs orca_v5_projects(id) (migration 0007). author_id is a plain TEXT
 * actor-id column (no FK) — same choice as orca_task_grants.granted_by
 * (migration 0010): RpcContext.userId isn't populated by every transport
 * (see core.ts's RpcContext.userId doc comment), so a hard FK to
 * orca_users(id) would reject inserts whenever the caller's identity
 * isn't a clean users-table row.
 *
 * @module db/migrations/0018_annotations
 */

import type { Migration } from './types'

export const migration0018Annotations: Migration = {
  version: 18,
  name: 'annotations',

  async up(db) {
    // Why: id is TEXT (client-generated UUID, see ProjectService/orca_tasks
    // pattern) rather than an autoincrement int — annotation-panel.tsx's
    // Annotation.id is typed string, and this is an entity table (read back
    // by id), not an append-only log like orca_task_comments.
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_annotations (
        id           TEXT    PRIMARY KEY,
        project_id   TEXT    NOT NULL REFERENCES orca_v5_projects(id) ON DELETE CASCADE,
        review_id    TEXT,
        file_path    TEXT    NOT NULL,
        line_number  INTEGER NOT NULL,
        body         TEXT    NOT NULL,
        author_id    TEXT    NOT NULL,
        created_at   BIGINT  NOT NULL
      )
    `)
    // Why: annotation-panel.tsx loads by exactly this tuple on line-click —
    // keeps the panel's fetch a single indexed lookup instead of a table scan.
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_annotations_lookup
        ON orca_annotations(project_id, file_path, line_number)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_annotations_review
        ON orca_annotations(review_id)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_annotations_review')
    await db.exec('DROP INDEX IF EXISTS idx_orca_annotations_lookup')
    await db.exec('DROP TABLE IF EXISTS orca_annotations')
  }
}
