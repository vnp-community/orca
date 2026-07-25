# TASK-DB-004: Tạo `src/main/db/dsn-parser.ts` + tests ✅ DONE

**Source:** SOL-DB-004 §4.2  
**Phase:** 1 | **Effort:** S (45–60 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-003

---

## Objective

Tạo `src/main/db/dsn-parser.ts` — parse DSN/connection URL thành `DatabaseConfig`, và `formatDsn()` để log (mask password).

---

## Context cần đọc

- `src/main/db/config.ts` (TASK-DB-003)
- SOL-DB-004 §4.2

---

## Files to create

### 1. `src/main/db/dsn-parser.ts`

```typescript
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
  let url: URL
  try {
    url = new URL(dsn)
  } catch {
    throw new Error(`Invalid DSN — not a valid URL: "${dsn}"`)
  }

  const protocol = url.protocol.replace(':', '')

  // ── SQLite special case ──────────────────────────────────────────────────
  if (protocol === 'sqlite') {
    // sqlite:///absolute/path  →  path starts after 'sqlite://'
    // sqlite://:memory:        →  ':memory:'
    const path = dsn.slice('sqlite://'.length)
    return DatabaseConfigSchema.parse({ dialect: 'sqlite', path })
  }

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
  const database = url.pathname.slice(1)  // strip leading '/'
  const username = url.username ? decodeURIComponent(url.username) : ''
  const password = url.password ? decodeURIComponent(url.password) : ''

  // Parse ssl query param
  const sslParam = url.searchParams.get('ssl')
  const ssl = sslParam === 'true' ? true : sslParam === 'false' ? false : undefined

  const port = portStr ? parseInt(portStr, 10) : undefined

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

  const { host, database, username } = config as { host: string; port: number; database: string; username: string; password: string }
  const password = maskPassword ? '***' : ((config as any).password || '')
  const port = (config as any).port as number

  const dialectStr = config.dialect === 'postgresql' ? 'postgresql' : config.dialect

  const defaultPort = config.dialect === 'postgresql' ? 5432 : 3306
  const portStr = port !== defaultPort ? `:${port}` : ''
  const authStr = username
    ? `${encodeURIComponent(username)}:${encodeURIComponent(password)}@`
    : ''

  return `${dialectStr}://${authStr}${host}${portStr}/${database}`
}
```

### 2. `src/main/db/__tests__/dsn-parser.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { parseDsn, formatDsn } from '../dsn-parser'

describe('parseDsn', () => {
  describe('sqlite', () => {
    it('parses sqlite:///absolute/path', () => {
      const config = parseDsn('sqlite:///data/orca/db.sqlite')
      expect(config).toMatchObject({ dialect: 'sqlite', path: '/data/orca/db.sqlite' })
    })

    it('parses sqlite://:memory:', () => {
      const config = parseDsn('sqlite://:memory:')
      expect(config).toMatchObject({ dialect: 'sqlite', path: ':memory:' })
    })

    it('parses sqlite:// with relative path', () => {
      const config = parseDsn('sqlite://./orca.db')
      expect(config).toMatchObject({ dialect: 'sqlite', path: './orca.db' })
    })
  })

  describe('mysql', () => {
    it('parses full mysql DSN', () => {
      const config = parseDsn('mysql://myuser:mypass@db.example.com:3306/orca_prod')
      expect(config).toMatchObject({
        dialect: 'mysql', host: 'db.example.com', port: 3306,
        database: 'orca_prod', username: 'myuser', password: 'mypass'
      })
    })

    it('defaults port to 3306 when omitted', () => {
      const config = parseDsn('mysql://user:pass@host/dbname')
      expect(config.dialect !== 'sqlite' && config.port).toBe(3306)
    })

    it('handles empty password', () => {
      const config = parseDsn('mysql://user@host:3306/db')
      expect(config.dialect !== 'sqlite' && config.password).toBe('')
    })

    it('parses ?ssl=true', () => {
      const config = parseDsn('mysql://user:pass@host/db?ssl=true')
      expect(config.dialect !== 'sqlite' && config.ssl).toBe(true)
    })

    it('parses ?ssl=false', () => {
      const config = parseDsn('mysql://user:pass@host/db?ssl=false')
      expect(config.dialect !== 'sqlite' && config.ssl).toBe(false)
    })

    it('URL-decodes credentials with special characters', () => {
      const config = parseDsn('mysql://my%40user:p%40ss@host/db')
      expect(config.dialect !== 'sqlite' && config.username).toBe('my@user')
      expect(config.dialect !== 'sqlite' && config.password).toBe('p@ss')
    })
  })

  describe('tidb', () => {
    it('parses tidb DSN', () => {
      const config = parseDsn('tidb://root:pass@tidb-host:4000/orca')
      expect(config).toMatchObject({ dialect: 'tidb', host: 'tidb-host', port: 4000 })
    })
  })

  describe('mariadb', () => {
    it('parses mariadb DSN', () => {
      const config = parseDsn('mariadb://user:pass@host:3306/db')
      expect(config).toMatchObject({ dialect: 'mariadb', host: 'host' })
    })
  })

  describe('postgresql', () => {
    it('parses postgresql DSN', () => {
      const config = parseDsn('postgresql://pguser:pgpass@pg.example.com:5432/orca_db')
      expect(config).toMatchObject({
        dialect: 'postgresql', host: 'pg.example.com', port: 5432,
        database: 'orca_db', username: 'pguser', password: 'pgpass'
      })
    })

    it('parses postgres:// alias', () => {
      const config = parseDsn('postgres://user:pass@host/db')
      expect(config).toMatchObject({ dialect: 'postgresql', port: 5432 })
    })
  })

  describe('error cases', () => {
    it('throws for unsupported protocol', () => {
      expect(() => parseDsn('redis://host:6379')).toThrow(/unsupported.*protocol/i)
    })

    it('throws for non-URL string', () => {
      expect(() => parseDsn('not-a-url')).toThrow()
    })
  })
})

describe('formatDsn', () => {
  it('masks password by default', () => {
    const config = parseDsn('mysql://user:secret@host:3306/db')
    const dsn = formatDsn(config)
    expect(dsn).not.toContain('secret')
    expect(dsn).toContain('***')
  })

  it('shows password when maskPassword=false', () => {
    const config = parseDsn('mysql://user:secret@host:3306/db')
    const dsn = formatDsn(config, false)
    expect(dsn).toContain('secret')
  })

  it('formats sqlite as sqlite://path', () => {
    const dsn = formatDsn({ dialect: 'sqlite', path: '/data/db.sqlite', readonly: false })
    expect(dsn).toBe('sqlite:///data/db.sqlite')
  })

  it('omits default port for mysql (3306)', () => {
    const config = parseDsn('mysql://u:p@host:3306/db')
    expect(formatDsn(config)).not.toContain(':3306')
  })

  it('includes non-default port', () => {
    const config = parseDsn('tidb://u:p@host:4000/db')
    expect(formatDsn(config)).toContain(':4000')
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

pnpm vitest run src/main/db/__tests__/dsn-parser.test.ts

# Spot test
node -e "
const { parseDsn, formatDsn } = require('./out/server/dsn-parser.js')
const c = parseDsn('mysql://user:secret@localhost:3306/orca')
console.log(formatDsn(c))  // should NOT show 'secret'
"
```

Expected: 18/18 tests pass

---

## Done criteria

- [x] `src/main/db/dsn-parser.ts` tồn tại với `parseDsn()` và `formatDsn()`
- [x] `src/main/db/__tests__/dsn-parser.test.ts` pass 20 tests (20 tests run)
- [x] `formatDsn()` mặc định mask password (không log credentials)
- [x] `postgres://` alias → `dialect: 'postgresql'`
- [x] `tidb://` → `dialect: 'tidb'` với default port 3306
- [x] URL-encoded credentials được decode đúng
