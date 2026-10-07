import { describe, it, expect } from 'vitest'
import { validateRulePack } from './quality-rule-pack-schema'
import { ORCA_RULE_PACK } from './quality-rule-pack-orca'

describe('quality-rule-pack-schema', () => {
  it('validates the ORCA pack without throwing', () => {
    expect(() => validateRulePack(ORCA_RULE_PACK)).not.toThrow()
  })

  it('rejects reserved IDs', () => {
    const pack = {
      version: 1,
      pack: 'test',
      rules: [
        {
          id: 'ORCA-008',
          title: 'Test',
          kind: 'diff',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: [], exclude: [], fileStatus: [] },
          match: { type: 'added-file-name', pattern: '.*', maxLineLength: 100 },
          message: '',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }
    expect(() => validateRulePack(pack)).toThrow(/reserved/)
  })

  it('rejects nested repetitions in regex', () => {
    const pack = {
      version: 1,
      pack: 'test',
      rules: [
        {
          id: 'ORCA-999',
          title: 'Test',
          kind: 'diff',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: [], exclude: [], fileStatus: [] },
          match: { type: 'added-file-name', pattern: '(a+)+$', maxLineLength: 100 },
          message: '',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }
    expect(() => validateRulePack(pack)).toThrow(/nested repetitions/)
  })

  it('verifies absence of ORCA-008, 009, 016, 017 in ORCA pack', () => {
    const ids = ORCA_RULE_PACK.rules.map(r => r.id)
    expect(ids).not.toContain('ORCA-008')
    expect(ids).not.toContain('ORCA-009')
    expect(ids).not.toContain('ORCA-016')
    expect(ids).not.toContain('ORCA-017')
  })
})
