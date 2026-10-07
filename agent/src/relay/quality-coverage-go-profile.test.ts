import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import {
  parseGoCoverProfile,
  parseGoCoverFunc,
  rollupFileCoverage,
  readGoWorkModules,
  COVERAGE_EXCLUDE_DEFAULTS
} from './quality-coverage-go-profile'

describe('quality-coverage-go-profile', () => {
  const fixtureDir = path.resolve(__dirname, '__fixtures__/coverage-go')

  it('parses go coverprofile fixture and maps paths correctly', () => {
    const content = fs.readFileSync(path.join(fixtureDir, 'mod1.out'), 'utf8')
    const ctx = {
      modules: [
        { dir: 'backend-go/mod1', modulePath: 'github.com/example/mod1' }
      ]
    }

    const res = parseGoCoverProfile(content, ctx)
    expect(res.mode).toBe('set')
    expect(res.unmappedBlocks).toBe(0)
    expect(res.blocks.length).toBe(4)

    expect(res.blocks[0].file).toBe('backend-go/mod1/pkg/calc.go')
    expect(res.blocks[0].startLine).toBe(5)
    expect(res.blocks[0].endLine).toBe(7)
    expect(res.blocks[0].count).toBe(1)

    // Check rollups
    const rollups = rollupFileCoverage(res.blocks)
    expect(rollups['backend-go/mod1/pkg/calc.go']).toBeDefined()
    expect(rollups['backend-go/mod1/pkg/calc.go'].stmts).toBe(2)
    expect(rollups['backend-go/mod1/pkg/calc.go'].coveredStmts).toBe(1)
    expect(rollups['backend-go/mod1/pkg/calc.go'].pct).toBe(0.5)
  })

  it('handles unmapped blocks and excludes generated/test files', () => {
    const raw = `mode: set
github.com/unknown/pkg/x.go:1.1,2.2 1 1
github.com/example/mod1/pkg/test.pb.go:1.1,2.2 1 1
github.com/example/mod1/pkg/calc_test.go:1.1,2.2 1 1
github.com/example/mod1/pkg/real.go:1.1,2.2 1 1
`
    const ctx = {
      modules: [
        { dir: 'backend-go/mod1', modulePath: 'github.com/example/mod1' }
      ]
    }

    const res = parseGoCoverProfile(raw, ctx)
    expect(res.unmappedBlocks).toBe(1) // unknown module
    expect(res.blocks.length).toBe(1) // only real.go kept; .pb.go and _test.go excluded
    expect(res.blocks[0].file).toBe('backend-go/mod1/pkg/real.go')
  })

  it('parses go tool cover -func output', () => {
    const funcContent = fs.readFileSync(path.join(fixtureDir, 'mod1-func.txt'), 'utf8')
    const funcs = parseGoCoverFunc(funcContent)

    expect(funcs.length).toBe(4)
    expect(funcs[0].name).toBe('Add')
    expect(funcs[0].pct).toBe(1.0)
    expect(funcs[1].name).toBe('Sub')
    expect(funcs[1].pct).toBe(0.0)
  })

  it('reads go.work modules if exists', () => {
    const repoRoot = path.resolve(__dirname, '../../..')
    const mods = readGoWorkModules(repoRoot)
    expect(Array.isArray(mods)).toBe(true)
    if (mods.length > 0) {
      expect(mods[0].dir.startsWith('backend-go')).toBe(true)
    }
  })
})
