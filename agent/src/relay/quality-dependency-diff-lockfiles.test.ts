import { describe, it, expect } from 'vitest'
import {
  diffPnpmLock,
  diffGoMod,
  checkPackageJsonDrift,
  generateDependencyFindings
} from './quality-dependency-diff-lockfiles'

describe('quality-dependency-diff-lockfiles', () => {
  it('diffs pnpm-lock yaml with added, removed, bumped, downgraded, newSource', () => {
    const baseYaml = `
packages:
  /react@18.2.0:
    resolution: { integrity: sha512-xxx }
  /lodash@4.17.20:
    resolution: { integrity: sha512-yyy }
  /old-lib@1.0.0:
    resolution: { integrity: sha512-zzz }
  /down-lib@2.0.0:
    resolution: { integrity: sha512-www }
`

    const headYaml = `
packages:
  /react@19.0.0:
    resolution: { integrity: sha512-xxx2 }
  /lodash@4.17.21:
    resolution: { integrity: sha512-yyy2 }
  /new-lib@1.0.0:
    resolution: { integrity: sha512-aaa }
  /git-lib@git+https://github.com/foo/bar.git#commit:
    resolution: { integrity: sha512-bbb }
  /down-lib@1.5.0:
    resolution: { integrity: sha512-ccc }
`

    const diff = diffPnpmLock(baseYaml, headYaml)

    // added
    expect(diff.added.map(x => x.name)).toContain('new-lib')
    expect(diff.added.map(x => x.name)).toContain('git-lib')

    // removed
    expect(diff.removed.map(x => x.name)).toContain('old-lib')

    // major bump (react 18 -> 19)
    const reactBump = diff.bumped.find(x => x.name === 'react')
    expect(reactBump).toBeDefined()
    expect(reactBump?.major).toBe(true)

    // minor bump (lodash 4.17.20 -> 4.17.21)
    const lodashBump = diff.bumped.find(x => x.name === 'lodash')
    expect(lodashBump).toBeDefined()
    expect(lodashBump?.major).toBe(false)

    // downgrade (down-lib 2.0.0 -> 1.5.0)
    const down = diff.downgraded.find(x => x.name === 'down-lib')
    expect(down).toBeDefined()
    expect(down?.from).toBe('2.0.0')
    expect(down?.to).toBe('1.5.0')

    // new source
    expect(diff.newSource.map(x => x.name)).toContain('git-lib')
  })

  it('diffs go.mod require and replace directives', () => {
    const baseMod = `
module example.com/repo

go 1.22

require (
    github.com/stretchr/testify v1.8.0
    golang.org/x/sync v0.3.0
)
`

    const headMod = `
module example.com/repo

go 1.22

require (
    github.com/stretchr/testify v1.9.0
    golang.org/x/crypto v0.20.0
)

replace github.com/stretchr/testify => ./fork/testify
`

    const diff = diffGoMod(baseMod, headMod)
    expect(diff.requiresAdded.map(x => x.module)).toContain('golang.org/x/crypto')
    expect(diff.requiresRemoved.map(x => x.module)).toContain('golang.org/x/sync')
    expect(diff.replaced).toHaveLength(1)
    expect(diff.replaced[0].local).toBe(true)
  })

  it('detects package.json and lockfile drift', () => {
    const pkg1 = JSON.stringify({ dependencies: { foo: '^1.0.0' } })
    const pkg2 = JSON.stringify({ dependencies: { foo: '^2.0.0' } })

    // pkg changed but lock did not
    expect(checkPackageJsonDrift(pkg1, pkg2, false)).toBe(true)

    // both changed -> no drift
    expect(checkPackageJsonDrift(pkg1, pkg2, true)).toBe(false)

    // neither changed -> no drift
    expect(checkPackageJsonDrift(pkg1, pkg1, false)).toBe(false)
  })

  it('generates findings with correct ruleId and severity', () => {
    const pnpmDiff = {
      added: [{ name: 'pkg-a', version: '1.0.0' }],
      removed: [{ name: 'pkg-b', version: '1.0.0' }],
      bumped: [{ name: 'pkg-c', from: '1.0.0', to: '2.0.0', major: true }],
      downgraded: [{ name: 'pkg-d', from: '2.0.0', to: '1.0.0' }],
      newSource: [{ name: 'pkg-e', version: 'git', source: 'git+https://...' }]
    }

    const { findings } = generateDependencyFindings({
      pnpm: pnpmDiff,
      drift: true
    })

    const ruleIds = findings.map(f => f.ruleId)
    expect(ruleIds).toContain('DEP-ADDED')
    expect(ruleIds).toContain('DEP-REMOVED')
    expect(ruleIds).toContain('DEP-MAJOR-BUMP')
    expect(ruleIds).toContain('DEP-DOWNGRADE')
    expect(ruleIds).toContain('DEP-NEW-SOURCE')
    expect(ruleIds).toContain('DEP-LOCKFILE-DRIFT')

    const majorBump = findings.find(f => f.ruleId === 'DEP-MAJOR-BUMP')
    expect(majorBump?.severity).toBe('warning')
  })
})
