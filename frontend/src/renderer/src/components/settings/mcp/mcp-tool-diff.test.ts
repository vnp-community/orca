import { describe, expect, it } from 'vitest'
import { diffTools, hasToolChanges } from './mcp-tool-diff'

const t = (name: string, description = name) => ({ name, description })

describe('diffTools', () => {
  it('detects added, removed, changed and unchanged tools', () => {
    const d = diffTools([t('a'), t('b', 'new'), t('c')], [t('b', 'old'), t('c'), t('gone')])
    expect(d.added.map((x) => x.name)).toEqual(['a'])
    expect(d.removed.map((x) => x.name)).toEqual(['gone'])
    expect(d.changed).toEqual([{ name: 'b', before: 'old', after: 'new' }])
    expect(d.unchanged.map((x) => x.name)).toEqual(['c'])
    expect(hasToolChanges(d)).toBe(true)
  })
  it('reports no changes for identical sets', () => {
    expect(hasToolChanges(diffTools([t('a')], [t('a')]))).toBe(false)
  })
})
