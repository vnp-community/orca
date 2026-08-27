import { describe, it, expect } from 'vitest'
import { translatePlaceholders } from '../pg-adapter'

describe('translatePlaceholders (BUG-BE-RPC-003 follow-up)', () => {
  it('translates a single ? placeholder', () => {
    expect(translatePlaceholders('SELECT * FROM orca_users WHERE id = ?')).toBe(
      'SELECT * FROM orca_users WHERE id = $1'
    )
  })

  it('translates multiple ? placeholders in order', () => {
    expect(
      translatePlaceholders('INSERT INTO orca_users (id, email, name) VALUES (?, ?, ?)')
    ).toBe('INSERT INTO orca_users (id, email, name) VALUES ($1, $2, $3)')
  })

  it('leaves SQL with no placeholders unchanged', () => {
    expect(translatePlaceholders('SELECT COUNT(*) FROM orca_users')).toBe(
      'SELECT COUNT(*) FROM orca_users'
    )
  })

  it('does not translate a literal ? inside a single-quoted string literal', () => {
    expect(translatePlaceholders("SELECT * FROM t WHERE note = 'are you sure?' AND id = ?")).toBe(
      "SELECT * FROM t WHERE note = 'are you sure?' AND id = $1"
    )
  })

  it('correctly resumes placeholder counting after a string containing an escaped quote', () => {
    // SQL '' escape for a literal quote inside a string: 'it''s ok?' — the
    // in-string toggle nets back to the same state across the adjacent ''.
    expect(translatePlaceholders("UPDATE t SET note = 'it''s ok?' WHERE id = ?")).toBe(
      "UPDATE t SET note = 'it''s ok?' WHERE id = $1"
    )
  })

  it('handles a query with no rows/params (exec-style DDL) unchanged', () => {
    const ddl = 'CREATE TABLE IF NOT EXISTS foo (id TEXT PRIMARY KEY)'
    expect(translatePlaceholders(ddl)).toBe(ddl)
  })
})
