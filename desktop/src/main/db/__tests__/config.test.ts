import { describe, it, expect } from 'vitest'
import { DatabaseConfigSchema } from '../config'

describe('DatabaseConfigSchema', () => {
  describe('sqlite', () => {
    it('validates minimal SQLite config', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite', path: '/data/db.sqlite' })
      expect(result.success).toBe(true)
    })

    it('defaults readonly to false', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite', path: ':memory:' })
      expect(result.success && result.data.readonly).toBe(false)
    })

    it('rejects sqlite without path', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'sqlite' })
      expect(result.success).toBe(false)
    })
  })

  describe('mysql', () => {
    const base = { dialect: 'mysql', host: 'localhost', database: 'orca', username: 'root' }

    it('validates minimal MySQL config', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success).toBe(true)
    })

    it('defaults port to 3306', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(
        result.success && result.data.dialect !== 'sqlite' && result.data.port
      ).toBe(3306)
    })

    it('defaults password to empty string', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(
        result.success && result.data.dialect !== 'sqlite' && result.data.password
      ).toBe('')
    })

    it('validates tidb dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, dialect: 'tidb', port: 4000 })
      expect(result.success).toBe(true)
    })

    it('validates mariadb dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, dialect: 'mariadb' })
      expect(result.success).toBe(true)
    })

    it('rejects mysql without host', () => {
      const result = DatabaseConfigSchema.safeParse({
        dialect: 'mysql',
        database: 'orca',
        username: 'root'
      })
      expect(result.success).toBe(false)
    })

    it('validates pool config with defaults', () => {
      const result = DatabaseConfigSchema.safeParse({ ...base, pool: {} })
      expect(result.success).toBe(true)
      if (result.success && result.data.dialect === 'mysql') {
        expect(result.data.pool?.min).toBe(2)
        expect(result.data.pool?.max).toBe(10)
        expect(result.data.pool?.acquireTimeoutMs).toBe(5000)
      }
    })
  })

  describe('postgresql', () => {
    const base = {
      dialect: 'postgresql',
      host: 'pg.host',
      database: 'orca',
      username: 'orca_user'
    }

    it('validates minimal PostgreSQL config', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(result.success).toBe(true)
    })

    it('defaults port to 5432', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(
        result.success && result.data.dialect === 'postgresql' && result.data.port
      ).toBe(5432)
    })

    it('defaults schema to public', () => {
      const result = DatabaseConfigSchema.safeParse(base)
      expect(
        result.success && result.data.dialect === 'postgresql' && result.data.schema
      ).toBe('public')
    })
  })

  describe('invalid', () => {
    it('rejects unknown dialect', () => {
      const result = DatabaseConfigSchema.safeParse({ dialect: 'redis', host: 'localhost' })
      expect(result.success).toBe(false)
    })

    it('rejects port out of range', () => {
      const result = DatabaseConfigSchema.safeParse({
        dialect: 'mysql',
        host: 'h',
        database: 'd',
        username: 'u',
        port: 99999
      })
      expect(result.success).toBe(false)
    })
  })
})
