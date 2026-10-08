import { describe, expect, it } from 'vitest'
import { CONTRACT_DETAIL_VALUE_MAX, contractDetailRows } from './contract-detail-rows'

describe('contractDetailRows', () => {
  it('extracts before/after into signature and keeps the rest as sorted rows', () => {
    const r = contractDetailRows({ details: { before: 'a', after: 'b', z: '1', m: '2' } })
    expect(r.signature).toEqual({ before: 'a', after: 'b' })
    expect(r.rows).toEqual([
      { key: 'm', value: '2' },
      { key: 'z', value: '1' }
    ])
  })

  it('has no signature when neither key exists', () => {
    const r = contractDetailRows({ details: { note: 'x' } })
    expect(r.signature).toBeUndefined()
    expect(r.rows).toEqual([{ key: 'note', value: 'x' }])
  })

  it('supports a one-sided signature', () => {
    expect(contractDetailRows({ details: { before: 'only' } }).signature).toEqual({ before: 'only' })
  })

  it('masks DSN-like values and truncates long ones', () => {
    const r = contractDetailRows({
      details: { dsn: 'postgres://user:hunter2@db/prod', long: 'x'.repeat(CONTRACT_DETAIL_VALUE_MAX + 50) }
    })
    expect(r.rows.find((x) => x.key === 'dsn')?.value).not.toContain('hunter2')
    const long = r.rows.find((x) => x.key === 'long')?.value ?? ''
    expect(long.endsWith('…')).toBe(true)
    expect(long.length).toBe(CONTRACT_DETAIL_VALUE_MAX + 1)
  })
})
