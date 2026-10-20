import { describe, expect, it } from 'vitest'
import { buildBranchSections } from './branch-list-sections'

const branches = [
  { name: 'develop', remote: false },
  { name: 'feature/other', remote: false },
  { name: 'main', remote: false },
  { name: 'release/1.x', remote: false },
  { name: 'origin/hotfix/x', remote: true },
  { name: 'origin/release/2.x', remote: true }
]

const refs = (sections: ReturnType<typeof buildBranchSections>) =>
  sections.map((s) => [s.id, s.options.map((o) => o.ref)])

describe('buildBranchSections', () => {
  it('reads an absent `remote` as local, as the gateway omits the false value', () => {
    const wire = [
      { name: 'main' },
      { name: 'release/1.x' },
      { name: 'origin/hotfix/x', remote: true }
    ]
    expect(
      refs(
        buildBranchSections({
          branches: wire as never,
          defaultName: 'main',
          query: '',
          searchMatches: []
        })
      )
    ).toEqual([
      ['default', ['main']],
      ['local', ['release/1.x']],
      ['remote', ['origin/hotfix/x']]
    ])
  })

  it('pins the default first, then local branches, then remote-only ones', () => {
    expect(
      refs(buildBranchSections({ branches, defaultName: 'main', query: '', searchMatches: [] }))
    ).toEqual([
      ['default', ['main']],
      ['local', ['develop', 'feature/other', 'release/1.x']],
      ['remote', ['origin/hotfix/x', 'origin/release/2.x']]
    ])
  })

  it('shows a default that the list hides (origin/main with a local main) once, pinned', () => {
    const sections = buildBranchSections({
      branches,
      defaultName: 'origin/main',
      query: '',
      searchMatches: []
    })
    expect(sections[0].options.map((o) => [o.kind, o.ref])).toEqual([['default', 'origin/main']])
    expect(sections[1].options.map((o) => o.ref)).toContain('main')
  })

  it('offers a nameless default row only while the box is empty', () => {
    const idle = buildBranchSections({ branches, defaultName: null, query: '', searchMatches: [] })
    expect(idle[0]).toMatchObject({ id: 'default', options: [{ kind: 'default', ref: null }] })
    const typed = buildBranchSections({
      branches,
      defaultName: null,
      query: 'rel',
      searchMatches: []
    })
    expect(typed.some((s) => s.id === 'default')).toBe(false)
  })

  it('filters the loaded list in place, ignoring case, and drops empty groups', () => {
    expect(
      refs(buildBranchSections({ branches, defaultName: 'main', query: ' REL', searchMatches: [] }))
    ).toEqual([
      ['local', ['release/1.x']],
      ['remote', ['origin/release/2.x']]
    ])
  })

  it('filters the pinned default too', () => {
    expect(
      refs(buildBranchSections({ branches, defaultName: 'main', query: 'main', searchMatches: [] }))
    ).toEqual([['default', ['main']]])
  })

  it('adds host search hits the list lacks, after the list, and never repeats a branch', () => {
    expect(
      refs(
        buildBranchSections({
          branches,
          defaultName: 'main',
          query: 'rel',
          searchMatches: ['release/1.x', 'release/old', 'v1-rel', 'main']
        })
      )
    ).toEqual([
      ['local', ['release/1.x']],
      ['remote', ['origin/release/2.x']],
      ['match', ['release/old', 'v1-rel']]
    ])
  })

  it('ignores search hits while the box is empty', () => {
    const sections = buildBranchSections({
      branches: [],
      defaultName: 'main',
      query: '',
      searchMatches: ['stale']
    })
    expect(refs(sections)).toEqual([['default', ['main']]])
  })
})
