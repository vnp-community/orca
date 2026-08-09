import { describe, it, expect, afterEach, vi } from 'vitest'
import { loadDatabaseConfig } from '../config-loader'

describe('loadDatabaseConfig', () => {
  const savedEnv: Record<string, string | undefined> = {}
  const envKeys = [
    'ORCA_DB_URL',
    'ORCA_DB_DIALECT',
    'ORCA_DB_HOST',
    'ORCA_DB_PORT',
    'ORCA_DB_NAME',
    'ORCA_DB_USER',
    'ORCA_DB_PASSWORD',
    'ORCA_DB_SSL',
    'ORCA_DB_POOL_MAX',
    'ORCA_DB_POOL_MIN'
  ]

  function setEnv(vars: Record<string, string | undefined>) {
    for (const k of envKeys) {
      savedEnv[k] = process.env[k]
      delete process.env[k]
    }
    for (const [k, v] of Object.entries(vars)) {
      if (v !== undefined) {process.env[k] = v}
    }
  }

  afterEach(() => {
    for (const [k, v] of Object.entries(savedEnv)) {
      if (v === undefined) {delete process.env[k]}
      else {process.env[k] = v}
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
        dialect: 'mysql',
        host: 'mysql-host',
        database: 'orca_db',
        username: 'orca_user',
        password: 'secret',
        port: 3306
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
      setEnv({ ORCA_DB_DIALECT: 'mysql' }) // no HOST/NAME/USER
      const config = loadDatabaseConfig()
      expect(config).toBeNull()
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('ORCA_DB_HOST'))
      warnSpy.mockRestore()
    })

    it('reads ORCA_DB_POOL_MAX', () => {
      setEnv({
        ORCA_DB_DIALECT: 'mysql',
        ORCA_DB_HOST: 'h',
        ORCA_DB_NAME: 'db',
        ORCA_DB_USER: 'u',
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
