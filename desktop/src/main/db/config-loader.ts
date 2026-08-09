/**
 * Database Configuration Loader
 *
 * Loads DatabaseConfig from environment variables with priority:
 *   1. ORCA_DB_URL (DSN format — highest priority)
 *   2. ORCA_DB_DIALECT + structured env vars (ORCA_DB_HOST, etc.)
 *   3. null → caller uses its own default (SQLite)
 *
 * @module db/config-loader
 */

import { DatabaseConfigSchema, type DatabaseConfig } from './config'
import { parseDsn, formatDsn } from './dsn-parser'

/**
 * Load database configuration from environment.
 * Returns null when no DB env vars are set — caller should use default SQLite.
 *
 * @param _userDataPath - Reserved for YAML config file support in future
 */
export function loadDatabaseConfig(_userDataPath?: string): DatabaseConfig | null {
  // 1. ORCA_DB_URL — single DSN string (highest priority)
  const dbUrl = process.env['ORCA_DB_URL']
  if (dbUrl) {
    try {
      const config = parseDsn(dbUrl)
      console.log(`[DB Config] Using database from ORCA_DB_URL: ${formatDsn(config)}`)
      return config
    } catch (err) {
      throw new Error(`Invalid ORCA_DB_URL: ${(err as Error).message}`)
    }
  }

  // 2. Structured env vars — ORCA_DB_DIALECT + ORCA_DB_HOST + ...
  const dialect = process.env['ORCA_DB_DIALECT']
  if (dialect && dialect !== 'sqlite') {
    const config = buildFromEnv(dialect)
    if (config) {
      console.log(`[DB Config] Using database from env vars: ${formatDsn(config)}`)
      return config
    }
    // buildFromEnv returns null when required vars are missing (already warned)
    return null
  }

  // 3. No config → return null
  return null
}

function buildFromEnv(dialect: string): DatabaseConfig | null {
  const host = process.env['ORCA_DB_HOST']
  const portStr = process.env['ORCA_DB_PORT']
  const database = process.env['ORCA_DB_NAME']
  const username = process.env['ORCA_DB_USER']
  const password = process.env['ORCA_DB_PASSWORD'] ?? ''
  const sslStr = process.env['ORCA_DB_SSL']
  const ssl =
    sslStr === 'true' ? true : sslStr === 'false' ? false : undefined

  if (!host || !database || !username) {
    console.warn(
      '[DB Config] ORCA_DB_DIALECT is set but required variables are missing. ' +
        `Need: ORCA_DB_HOST (${host ? '✓' : '✗'}), ORCA_DB_NAME (${database ? '✓' : '✗'}), ORCA_DB_USER (${username ? '✓' : '✗'})`
    )
    return null
  }

  const port = portStr ? Number.parseInt(portStr, 10) : undefined

  const poolMax = process.env['ORCA_DB_POOL_MAX']
  const poolMin = process.env['ORCA_DB_POOL_MIN']
  const pool =
    poolMax || poolMin
      ? {
          ...(poolMax ? { max: Number.parseInt(poolMax, 10) } : {}),
          ...(poolMin ? { min: Number.parseInt(poolMin, 10) } : {})
        }
      : undefined

  if (dialect === 'mysql' || dialect === 'tidb' || dialect === 'mariadb') {
    return DatabaseConfigSchema.parse({
      dialect: dialect as 'mysql' | 'tidb' | 'mariadb',
      host,
      port: port ?? 3306,
      database,
      username,
      password,
      ssl,
      ...(pool ? { pool } : {})
    })
  }

  if (dialect === 'postgresql' || dialect === 'postgres') {
    return DatabaseConfigSchema.parse({
      dialect: 'postgresql',
      host,
      port: port ?? 5432,
      database,
      username,
      password,
      ssl,
      ...(pool ? { pool } : {})
    })
  }

  console.warn(
    `[DB Config] Unknown ORCA_DB_DIALECT value: "${dialect}". Supported: mysql, tidb, mariadb, postgresql`
  )
  return null
}
