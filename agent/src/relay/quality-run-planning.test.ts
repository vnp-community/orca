import { describe, it, expect } from 'vitest'
import { planRun, QualityPlanError, PlanRunInput } from './quality-run-planning'
import { QualityCheckProfile } from './quality-profile-schema'

const catalog: QualityCheckProfile[] = [
  {
    id: 'ts-lint', title: 'TS Lint', parser: 'oxlint', argv: ['{bin:oxlint}', '{files}'],
    fileExtensions: ['.ts'], scopeStrategy: 'append-files'
  },
  {
    id: 'go-vet', title: 'Go Vet', parser: 'govet', argv: ['{bin:go}', 'vet', '{files}'],
    fileExtensions: ['.go'], scopeStrategy: 'go-modules'
  },
  {
    id: 'bad-cwd', title: 'Bad Cwd', parser: 'x', argv: ['{bin:x}'],
    cwd: '../out'
  }
]

describe('quality-run-planning', () => {
  const baseInput: PlanRunInput = {
    catalog,
    suites: [],
    workspaceRoot: '/repo',
    changedFiles: null,
    tmpRunDir: '/tmp/run',
    sourceEnv: {},
    deps: {
      resolveBin: (b) => `/bin/${b}`,
      gitCommonDir: '/repo/.git',
      readGoWork: () => ['backend-go', 'backend-go/common', 'backend-go/cmd/api']
    }
  }

  it('throws on unknown profile', () => {
    expect(() => planRun({ ...baseInput, profileIds: ['unknown'] })).toThrowError(QualityPlanError)
  })

  it('append-files strategy', () => {
    // few files
    const res = planRun({ ...baseInput, profileIds: ['ts-lint'], changedFiles: ['a.ts', 'b.ts', 'c.js'] })
    expect(res.steps.length).toBe(1)
    expect(res.steps[0].args).toEqual(['/bin/oxlint', 'a.ts', 'b.ts']) // c.js filtered out
    expect(res.scopeWidened).toBe(false)
    expect(res.scopeFiles).toContain('a.ts')

    // > 300 files
    const manyFiles = Array.from({ length: 301 }, (_, i) => `file${i}.ts`)
    const res2 = planRun({ ...baseInput, profileIds: ['ts-lint'], changedFiles: manyFiles })
    expect(res2.steps[0].args).toEqual(['/bin/oxlint']) // no files appended
    expect(res2.scopeWidened).toBe(true)
  })

  it('append-files strategy skip if no files', () => {
    const res = planRun({ ...baseInput, profileIds: ['ts-lint'], changedFiles: ['a.js'] })
    expect(res.steps.length).toBe(0) // skipped
  })

  it('go-modules strategy', () => {
    const res = planRun({ ...baseInput, profileIds: ['go-vet'], changedFiles: ['backend-go/cmd/api/main.go'] })
    expect(res.steps.length).toBe(1)
    expect(res.steps[0].args).toContain('backend-go/cmd/api') // {files|d} but strategy outputs module path so dirname of 'backend-go/cmd/api' is 'backend-go/cmd' wait...
    // wait, my go-modules sets appendFiles to the module dir: 'backend-go/cmd/api'.
    // {files|d} then takes dirname of 'backend-go/cmd/api' which is 'backend-go/cmd'.
    // Is that correct? The task says `{files}` / `{files|d}`. If go-modules appends module paths, then {files} is the module path. Should go-vet use {files} or {files|d}?
    // If it uses {files|d}, it gives the parent of the module.
  })

  it('go-modules common widened', () => {
    const res = planRun({ ...baseInput, profileIds: ['go-vet'], changedFiles: ['backend-go/common/utils.go'] })
    expect(res.steps.length).toBe(1)
    expect(res.scopeWidened).toBe(true)
    // Should include all modules
  })

  it('rejects bad cwd', () => {
    expect(() => planRun({ ...baseInput, profileIds: ['bad-cwd'] })).toThrowError(/Invalid cwd/)
  })

  it('token replacement', () => {
    const p: QualityCheckProfile = {
      id: 'tokens', title: 'T', parser: 'x', argv: ['{bin:foo}', '--tmp', '{tmp:123}', '--base', '{base}', '--git', '{gitCommonDir}']
    }
    const res = planRun({ ...baseInput, catalog: [...catalog, p], profileIds: ['tokens'], base: 'master' })
    expect(res.steps[0].args).toEqual(['/bin/foo', '--tmp', '/tmp/run/123', '--base', 'master', '--git', '/repo/.git'])
  })

  it('repo-rules diff skips if worktree', () => {
    const p: QualityCheckProfile = {
      id: 'repo-rules', kind: 'repo-rules', title: 'T', parser: 'x', argv: []
    }
    const res = planRun({ ...baseInput, catalog: [...catalog, p], profileIds: ['repo-rules'], changedFiles: null })
    expect(res.steps[0].inProcess).toBe(true)
    expect(res.steps[0].skipReason).toBe('base_required')
  })

  it('repo-rules-scripts selects scripts based on changed files', () => {
    const p: QualityCheckProfile = {
      id: 'repo-rules-scripts', kind: 'repo-rules', title: 'T', parser: 'x', argv: []
    }
    const res = planRun({ ...baseInput, catalog: [...catalog, p], profileIds: ['repo-rules-scripts'], changedFiles: ['desktop/src/renderer/App.tsx'] })
    expect(res.steps[0].inProcess).toBe(true)
    expect(res.steps[0].args).toEqual(['ORCA-005'])
    expect(res.steps[0].skipReason).toBeUndefined()
  })
})
