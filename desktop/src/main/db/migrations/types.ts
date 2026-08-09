/**
 * Migration System Types
 *
 * @module db/migrations/types
 */

import type { IDatabase } from '../types'

/** A single database migration */
export type Migration = {
  /** Unique sequential version number (e.g., 1, 2, 3) */
  version: number
  /** Human-readable description (e.g., 'initial_schema') */
  name: string
  /** Forward migration — apply schema changes */
  up(db: IDatabase): Promise<void>
  /** Reverse migration — undo schema changes */
  down(db: IDatabase): Promise<void>
}

/** Record of an applied migration stored in the DB */
export type AppliedMigration = {
  version: number
  name: string
  /** ISO 8601 timestamp when migration was applied */
  appliedAt: string
}

/** Result of a single migration step */
export type MigrationResult = {
  version: number
  name: string
  direction: 'up' | 'down'
  /** Duration of migration execution in milliseconds */
  durationMs: number
}
