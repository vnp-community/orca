import { describe, expect, it } from 'vitest'
import { selectOpenFindingsBySymbolKey } from './open-findings-by-symbol'
import { makeFinding } from '../../../test-support/contract-findings-fixtures'

const ev = (key: string) => [{ path: 'a', symbol: { key, kind: 'function', name: 'f', filePath: 'a' } as never }]

describe('selectOpenFindingsBySymbolKey', () => {
  it('counts open findings per symbol with the top severity and ignores dismissed ones', () => {
    const out = selectOpenFindingsBySymbolKey([
      makeFinding({ findingKey: '1', severity: 'warning', evidence: ev('s1') }),
      makeFinding({ findingKey: '2', severity: 'error', evidence: ev('s1') }),
      makeFinding({
        findingKey: '3',
        evidence: ev('s2'),
        dismissed: { by: 'u', at: 'x', reason: 'later', disposition: 'ignored' }
      }),
      makeFinding({ findingKey: '4', evidence: [{ path: 'no-symbol' }] })
    ])
    expect(out).toEqual({ s1: { count: 2, topSeverity: 'error' } })
  })
})
