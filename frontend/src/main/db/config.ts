/**
 * Database Configuration Schemas
 *
 * Zod schemas for type-safe database configuration.
 * Used by: DSN parser, config loader, server bootstrap.
 *
 * @module db/config
 */

import { z } from 'zod'

// ── Pool Config ─────────────────────────────────────────────────────────────

const PoolConfigSchema = z.object({
  /** Minimum connections to keep alive in pool */
  min: z.number().int().min(0).default(2),
  /** Maximum connections allowed in pool */
  max: z.number().int().min(1).default(10),
  /** Milliseconds to wait for connection before timeout */
  acquireTimeoutMs: z.number().int().positive().default(5_000),
  /** Milliseconds before an idle connection is destroyed */
  idleTimeoutMs: z.number().int().positive().default(30_000),
  /** Number of retries when creating a connection fails */
  connectionRetries: z.number().int().min(0).default(3),
  /** Milliseconds between connection retry attempts */
  retryDelayMs: z.number().int().positive().default(500)
})

// ── Dialect-specific schemas ─────────────────────────────────────────────────

const SqliteConfigSchema = z.object({
  dialect: z.literal('sqlite'),
  /** File path or ':memory:' for in-memory database */
  path: z.string().min(1),
  /** Open database in read-only mode */
  readonly: z.boolean().default(false)
})

const MysqlConfigSchema = z.object({
  dialect: z.union([
    z.literal('mysql'),
    z.literal('tidb'),
    z.literal('mariadb')
  ]),
  host: z.string().min(1),
  port: z.number().int().min(1).max(65535).default(3306),
  database: z.string().min(1),
  username: z.string().min(1),
  password: z.string().default(''),
  ssl: z.boolean().optional(),
  pool: PoolConfigSchema.optional()
})

const PostgresConfigSchema = z.object({
  dialect: z.literal('postgresql'),
  host: z.string().min(1),
  port: z.number().int().min(1).max(65535).default(5432),
  database: z.string().min(1),
  username: z.string().min(1),
  password: z.string().default(''),
  ssl: z.boolean().optional(),
  /** PostgreSQL schema name (default: 'public') */
  schema: z.string().default('public'),
  pool: PoolConfigSchema.optional()
})

// ── Main discriminated union ─────────────────────────────────────────────────

export const DatabaseConfigSchema = z.discriminatedUnion('dialect', [
  SqliteConfigSchema,
  MysqlConfigSchema,
  PostgresConfigSchema
])

// ── Exported types ───────────────────────────────────────────────────────────

export type DatabaseConfig = z.infer<typeof DatabaseConfigSchema>
export type PoolConfig = z.infer<typeof PoolConfigSchema>
export type SqliteConfig = z.infer<typeof SqliteConfigSchema>
export type MysqlConfig = z.infer<typeof MysqlConfigSchema>
export type PostgresConfig = z.infer<typeof PostgresConfigSchema>

export { PoolConfigSchema }
