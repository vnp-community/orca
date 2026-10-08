import { describe, expect, it } from 'vitest'
import {
  contractKindGroup,
  countByCompatibility,
  filterContractChanges,
  groupContractChanges,
  listContractServices,
  normalizeCompatibility
} from './contract-grouping'
import { CONTRACT_CHANGES_MIXED, makeContractChange } from '../../../test-support/contract-findings-fixtures'

describe('contract-grouping', () => {
  it('maps unrecognised compatibility to unknown and never infers breaking', () => {
    expect(normalizeCompatibility('weird')).toBe('unknown')
    expect(normalizeCompatibility(undefined)).toBe('unknown')
    expect(normalizeCompatibility('risky')).toBe('risky')
  })

  it('classifies kind groups, unknown kinds fall back to unknown', () => {
    expect(contractKindGroup('proto-field')).toBe('proto')
    expect(contractKindGroup('ws-channel-arg')).toBe('ws-channel')
    expect(contractKindGroup('route-field')).toBe('route')
    expect(contractKindGroup('sql-column')).toBe('migration')
    expect(contractKindGroup('graphql-thing')).toBe('unknown')
  })

  it('counts by compatibility', () => {
    expect(countByCompatibility(CONTRACT_CHANGES_MIXED)).toEqual({
      breaking: 2,
      risky: 1,
      compatible: 1,
      unknown: 1
    })
  })

  it('groups service -> kind with breaking first and the no-service group last', () => {
    const groups = groupContractChanges(CONTRACT_CHANGES_MIXED)
    expect(groups.map((g) => g.service)).toEqual(['infra-fleet-service', 'api-gateway', ''])
    const fleet = groups[0]
    expect(fleet.total).toBe(2)
    expect(fleet.kinds[0].kindGroup).toBe('proto')
    expect(fleet.kinds[0].changes.map((c) => c.id)).toEqual(['c1', 'c2'])
    expect(groups[1].kinds.map((k) => k.kindGroup)).toEqual(['route', 'ws-channel'])
  })

  it('filters by breaking, kinds, service, query and chip', () => {
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { onlyBreaking: true }).map((c) => c.id)).toEqual([
      'c1',
      'c5'
    ])
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { kinds: ['route', 'ws-channel'] })).toHaveLength(2)
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { service: 'api-gateway' })).toHaveLength(2)
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { query: 'FINDINGS' }).map((c) => c.id)).toEqual(['c3'])
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { compatibility: 'unknown' })).toHaveLength(1)
  })

  it('onlyChangedByAgent keeps changes whose files intersect the changed set', () => {
    const changed = new Set(['gateway/ws.go'])
    expect(
      filterContractChanges(CONTRACT_CHANGES_MIXED, { onlyChangedByAgent: true, changedFiles: changed }).map(
        (c) => c.id
      )
    ).toEqual(['c3'])
    expect(filterContractChanges(CONTRACT_CHANGES_MIXED, { onlyChangedByAgent: true })).toEqual([])
  })

  it('lists distinct services', () => {
    expect(listContractServices([...CONTRACT_CHANGES_MIXED, makeContractChange({ id: 'x' })])).toEqual([
      'api-gateway',
      'infra-fleet-service'
    ])
  })
})
