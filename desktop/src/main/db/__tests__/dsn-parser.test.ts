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
        dialect: 'mysql',
        host: 'db.example.com',
        port: 3306,
        database: 'orca_prod',
        username: 'myuser',
        password: 'mypass'
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
        dialect: 'postgresql',
        host: 'pg.example.com',
        port: 5432,
        database: 'orca_db',
        username: 'pguser',
        password: 'pgpass'
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
