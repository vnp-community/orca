import { describe, it, expect } from 'vitest'
import { nowTextDefaultSql, nowEpochMsDefaultSql, autoIncrementPrimaryKeySql } from '../sql-dialect'

describe('sql-dialect helpers', () => {
  describe('nowTextDefaultSql', () => {
    it('sqlite uses datetime(\'now\')', () => {
      expect(nowTextDefaultSql('sqlite')).toBe("(datetime('now'))")
    })
    it('postgresql formats CURRENT_TIMESTAMP to match SQLite\'s text format', () => {
      expect(nowTextDefaultSql('postgresql')).toContain('CURRENT_TIMESTAMP')
      expect(nowTextDefaultSql('postgresql')).toContain('YYYY-MM-DD HH24:MI:SS')
    })
    it('mysql/tidb/mariadb use UTC_TIMESTAMP()', () => {
      expect(nowTextDefaultSql('mysql')).toBe('(UTC_TIMESTAMP())')
      expect(nowTextDefaultSql('tidb')).toBe('(UTC_TIMESTAMP())')
      expect(nowTextDefaultSql('mariadb')).toBe('(UTC_TIMESTAMP())')
    })
  })

  describe('nowEpochMsDefaultSql', () => {
    it('sqlite uses strftime epoch * 1000', () => {
      expect(nowEpochMsDefaultSql('sqlite')).toBe("(strftime('%s', 'now') * 1000)")
    })
    it('postgresql uses EXTRACT(EPOCH ...) * 1000', () => {
      expect(nowEpochMsDefaultSql('postgresql')).toContain('EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)')
    })
    it('mysql/tidb/mariadb use UNIX_TIMESTAMP() * 1000', () => {
      expect(nowEpochMsDefaultSql('mysql')).toBe('(UNIX_TIMESTAMP() * 1000)')
    })
  })

  describe('autoIncrementPrimaryKeySql', () => {
    it('sqlite uses AUTOINCREMENT', () => {
      expect(autoIncrementPrimaryKeySql('sqlite')).toBe('INTEGER PRIMARY KEY AUTOINCREMENT')
    })
    it('postgresql uses GENERATED ALWAYS AS IDENTITY (no AUTOINCREMENT keyword)', () => {
      const sql = autoIncrementPrimaryKeySql('postgresql')
      expect(sql).toContain('GENERATED ALWAYS AS IDENTITY')
      expect(sql).not.toContain('AUTOINCREMENT')
    })
    it('mysql/tidb/mariadb use AUTO_INCREMENT', () => {
      expect(autoIncrementPrimaryKeySql('mysql')).toBe('INTEGER PRIMARY KEY AUTO_INCREMENT')
    })
  })
})
