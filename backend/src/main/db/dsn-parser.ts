/**
 * DSN / Connection URL Parser
 *
 * Parses connection strings into typed DatabaseConfig objects.
 *
 * Supported formats:
 *   sqlite:///path/to/db.sqlite
 *   sqlite://:memory:
 *   mysql://user:pass@host:3306/dbname?ssl=true
 *   mysql://user:pass@host/dbname
 *   postgresql://user:pass@host:5432/dbname
 *   postgres://user:pass@host/dbname
 *   tidb://user:pass@host:4000/dbname
 *   mariadb://user:pass@host:3306/dbname
 *
 * @module db/dsn-parser
 */

import { DatabaseConfigSchema, type DatabaseConfig } from './config'

/** Map of URL protocols to dialect names */
const PROTOCOL_TO_DIALECT: Record<string, string> = {
  sqlite: 'sqlite',
  mysql: 'mysql',
  tidb: 'tidb',
  mariadb: 'mariadb',
  postgresql: 'postgresql',
  postgres: 'postgresql'
}

/**
 * Parse a DSN URL string into a validated DatabaseConfig.
 * @throws Error for unsupported protocols or malformed URLs.
 */
export function parseDsn(dsn: string): DatabaseConfig {
  const trimmed = dsn.trim()

  // ── SQLite special case — handle BEFORE new URL() ────────────────────────
  // sqlite://:memory: is not a valid URL (colon in host), so we handle it first.
  if (trimmed.startsWith('sqlite://')) {
    const path = trimmed.slice('sqlite://'.length)
    return DatabaseConfigSchema.parse({ dialect: 'sqlite', path })
  }

  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    throw new Error(`Invalid DSN — not a valid URL: "${dsn}"`)
  }

  const protocol = url.protocol.replace(':', '')

  // ── Network databases ────────────────────────────────────────────────────
  const dialect = PROTOCOL_TO_DIALECT[protocol]
  if (!dialect) {
    const supported = Object.keys(PROTOCOL_TO_DIALECT).join(', ')
    throw new Error(
      `Unsupported database protocol: "${protocol}". Supported: ${supported}`
    )
  }

  const host = url.hostname
  const portStr = url.port
  const database = url.pathname.slice(1) // strip leading '/'
  const username = url.username ? decodeURIComponent(url.username) : ''
  const password = url.password ? decodeURIComponent(url.password) : ''

  // Parse ssl query param
  const sslParam = url.searchParams.get('ssl')
  const ssl =
    sslParam === 'true' ? true : sslParam === 'false' ? false : undefined

  const port = portStr ? Number.parseInt(portStr, 10) : undefined

  return DatabaseConfigSchema.parse({ dialect, host, port, database, username, password, ssl })
}

/**
 * Format a DatabaseConfig back to a DSN string.
 * By default, masks the password with '***' to prevent logging secrets.
 */
export function formatDsn(config: DatabaseConfig, maskPassword = true): string {
  if (config.dialect === 'sqlite') {
    return `sqlite://${config.path}`
  }

  const networkConfig = config as {
    host: string
    port: number
    database: string
    username: string
    password: string
  }
  const { host, database, username } = networkConfig
  const password = maskPassword ? '***' : (networkConfig.password || '')
  const port = networkConfig.port

  const dialectStr = config.dialect === 'postgresql' ? 'postgresql' : config.dialect

  const defaultPort = config.dialect === 'postgresql' ? 5432 : 3306
  const portStr = port !== defaultPort ? `:${port}` : ''
  const authStr = username
    ? `${encodeURIComponent(username)}:${encodeURIComponent(password)}@`
    : ''

  return `${dialectStr}://${authStr}${host}${portStr}/${database}`
}
