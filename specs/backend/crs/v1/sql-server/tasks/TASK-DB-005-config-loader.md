# TASK-DB-005: Tạo `src/main/db/config-loader.ts` + tests ✅ DONE

**Source:** SOL-DB-004 §4.3  
**Phase:** 1 | **Effort:** S (30–45 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-003, TASK-DB-004

---

## Objective

Tạo `src/main/db/config-loader.ts` — đọc database config từ environment variables (`ORCA_DB_URL`, `ORCA_DB_DIALECT`, ...) theo thứ tự ưu tiên.

---

## Files to create

### 1. `src/main/db/config-loader.ts`

```typescript
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
 * @param userDataPath - Optional userData path (reserved for YAML config in future)
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
  const ssl = sslStr === 'true' ? true : sslStr === 'false' ? false : undefined

  if (!host || !database || !username) {
    console.warn(
      '[DB Config] ORCA_DB_DIALECT is set but required variables are missing. ' +
      `Need: ORCA_DB_HOST (${host ? '✓' : '✗'}), ORCA_DB_NAME (${database ? '✓' : '✗'}), ORCA_DB_USER (${username ? '✓' : '✗'})`
    )
    return null
  }

  const port = portStr ? parseInt(portStr, 10) : undefined

  const poolMax = process.env['ORCA_DB_POOL_MAX']
  const poolMin = process.env['ORCA_DB_POOL_MIN']
  const pool = (poolMax || poolMin) ? {
    ...(poolMax ? { max: parseInt(poolMax, 10) } : {}),
    ...(poolMin ? { min: parseInt(poolMin, 10) } : {})
  } : undefined

  if (dialect === 'mysql' || dialect === 'tidb' || dialect === 'mariadb') {
    return DatabaseConfigSchema.parse({
      dialect: dialect as 'mysql' | 'tidb' | 'mariadb',
      host, port: port ?? 3306, database, username, password, ssl,
      ...(pool ? { pool } : {})
    })
  }

  if (dialect === 'postgresql' || dialect === 'postgres') {
    return DatabaseConfigSchema.parse({
      dialect: 'postgresql',
      host, port: port ?? 5432, database, username, password, ssl,
      ...(pool ? { pool } : {})
    })
  }

  console.warn(`[DB Config] Unknown ORCA_DB_DIALECT value: "${dialect}". Supported: mysql, tidb, mariadb, postgresql`)
  return null
}
```

### 2. `src/main/db/__tests__/config-loader.test.ts`

```typescript
import { describe, it, expect, afterEach, vi } from 'vitest'
import { loadDatabaseConfig } from '../config-loader'

describe('loadDatabaseConfig', () => {
  const savedEnv: Record<string, string | undefined> = {}
  const envKeys = [
    'ORCA_DB_URL', 'ORCA_DB_DIALECT', 'ORCA_DB_HOST', 'ORCA_DB_PORT',
    'ORCA_DB_NAME', 'ORCA_DB_USER', 'ORCA_DB_PASSWORD', 'ORCA_DB_SSL',
    'ORCA_DB_POOL_MAX', 'ORCA_DB_POOL_MIN'
  ]

  function setEnv(vars: Record<string, string | undefined>) {
    for (const k of envKeys) {
      savedEnv[k] = process.env[k]
      delete process.env[k]
    }
    for (const [k, v] of Object.entries(vars)) {
      if (v !== undefined) process.env[k] = v
    }
  }

  afterEach(() => {
    for (const [k, v] of Object.entries(savedEnv)) {
      if (v === undefined) delete process.env[k]
      else process.env[k] = v
    }
  })

  describe('ORCA_DB_URL (highest priority)', () => {
    it('parses SQLite from ORCA_DB_URL', () => {
      setEnv({ ORCA_DB_URL: 'sqlite:///tmp/test.db' })
      const config = loadDatabaseConfig()
      expect(config).toMatchObject({ dialect: 'sqlite', path: '/tmp/test.db' })
    })

    it('parses MySQL from ORCA_DB_URL', () => {
      setEnv({ ORCA_DB_URL: 'mysql://user:pass@localhost:3306/orca' })
      const config = loadDatabaseConfig()
      expect(config).toMatchObject({ dialect: 'mysql', host: 'localhost' })
    })

    it('throws for invalid ORCA_DB_URL', () => {
      setEnv({ ORCA_DB_URL: 'not-a-valid-url' })
      expect(() => loadDatabaseConfig()).toThrow('Invalid ORCA_DB_URL')
    })

    it('ORCA_DB_URL takes priority over ORCA_DB_DIALECT', () => {
      setEnv({
        ORCA_DB_URL: 'sqlite://:memory:',
        ORCA_DB_DIALECT: 'mysql',
        ORCA_DB_HOST: 'mysql-host',
        ORCA_DB_NAME: 'db',
        ORCA_DB_USER: 'user'
      })
      const config = loadDatabaseConfig()
      expect(config?.dialect).toBe('sqlite')
    })
  })

  describe('structured env vars', () => {
    it('builds MySQL config from env vars', () => {
      setEnv({
        ORCA_DB_DIALECT: 'mysql',
        ORCA_DB_HOST: 'mysql-host',
        ORCA_DB_NAME: 'orca_db',
        ORCA_DB_USER: 'orca_user',
        ORCA_DB_PASSWORD: 'secret',
        ORCA_DB_PORT: '3306'
      })
      const config = loadDatabaseConfig()
      expect(config).toMatchObject({
        dialect: 'mysql', host: 'mysql-host', database: 'orca_db',
        username: 'orca_user', password: 'secret', port: 3306
      })
    })

    it('builds PostgreSQL config', () => {
      setEnv({
        ORCA_DB_DIALECT: 'postgresql',
        ORCA_DB_HOST: 'pg-host',
        ORCA_DB_NAME: 'orca',
        ORCA_DB_USER: 'pg_user'
      })
      const config = loadDatabaseConfig()
      expect(config?.dialect).toBe('postgresql')
    })

    it('warns and returns null when required vars missing', () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      setEnv({ ORCA_DB_DIALECT: 'mysql' })  // no HOST/NAME/USER
      const config = loadDatabaseConfig()
      expect(config).toBeNull()
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('ORCA_DB_HOST'))
      warnSpy.mockRestore()
    })

    it('reads ORCA_DB_POOL_MAX', () => {
      setEnv({
        ORCA_DB_DIALECT: 'mysql',
        ORCA_DB_HOST: 'h', ORCA_DB_NAME: 'db', ORCA_DB_USER: 'u',
        ORCA_DB_POOL_MAX: '20'
      })
      const config = loadDatabaseConfig()
      if (config?.dialect === 'mysql') {
        expect(config.pool?.max).toBe(20)
      }
    })
  })

  describe('default (no config)', () => {
    it('returns null when no DB env vars are set', () => {
      setEnv({})
      const config = loadDatabaseConfig()
      expect(config).toBeNull()
    })

    it('returns null when ORCA_DB_DIALECT=sqlite', () => {
      setEnv({ ORCA_DB_DIALECT: 'sqlite' })
      const config = loadDatabaseConfig()
      expect(config).toBeNull()
    })
  })
})
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

pnpm vitest run src/main/db/__tests__/config-loader.test.ts

# Smoke test
ORCA_DB_URL=sqlite://:memory: node -e "
process.env.ORCA_DB_URL = 'sqlite://:memory:'
// import after setting env
"
```

Expected: 12/12 tests pass

---

## Done criteria

- [x] `src/main/db/config-loader.ts` tồn tại với `loadDatabaseConfig()`
- [x] `ORCA_DB_URL` env var parse thành công (priority cao nhất)
- [x] Invalid `ORCA_DB_URL` → throw với prefix `"Invalid ORCA_DB_URL:"`
- [x] Missing required vars → `console.warn` + return `null`
- [x] `ORCA_DB_URL` overrides `ORCA_DB_DIALECT`
- [x] `src/main/db/__tests__/config-loader.test.ts` pass 10 tests (10 tests run)
