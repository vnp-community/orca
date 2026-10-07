import { describe, it, expect } from 'vitest'
import { BUILTIN_PROFILES, BUILTIN_SUITES, getCatalog, registerBuiltinProfiles } from './quality-profile-catalog'
import { validateProfile } from './quality-profile-schema'
import fs from 'fs'
import path from 'path'

describe('quality-profile-catalog', () => {
  it('all built-in profiles are valid schemas', () => {
    for (const p of BUILTIN_PROFILES) {
      const res = validateProfile(p)
      expect(res.ok, `Profile ${p.id} invalid: ${(res as any).errors?.join(', ')}`).toBe(true)
    }
  })

  it('all built-in profile IDs are unique', () => {
    const ids = BUILTIN_PROFILES.map(p => p.id)
    const set = new Set(ids)
    expect(ids.length).toBe(set.size)
  })

  it('prohibits forbidden tokens in argv and scopeArgv', () => {
    const forbidden = ['install', 'rebuild', 'mod download', '--runtime=', '--init', '--prune', 'analyze', 'clean']
    for (const p of BUILTIN_PROFILES) {
      const allArgv = [...p.argv]
      if (p.scopeArgv) {
        for (const k of Object.keys(p.scopeArgv)) {
          allArgv.push(...(p.scopeArgv as any)[k])
        }
      }
      const fullStr = allArgv.join(' ')
      for (const bad of forbidden) {
        expect(fullStr).not.toContain(bad)
      }
    }
  })

  it('requires.file points to real files in the repo', () => {
    const root = path.resolve(__dirname, '../../../') // root of orca (agent/src/relay is 3 levels deep)
    for (const p of BUILTIN_PROFILES) {
      if (!p.requires) continue
      for (const req of p.requires) {
        if (req.type === 'file') {
          const fp = path.join(root, req.path)
          expect(fs.existsSync(fp), `File required by ${p.id} not found: ${req.path} (resolved to ${fp})`).toBe(true)
        }
      }
    }
  })

  it('built-in suites reference real profiles', () => {
    const validIds = new Set(BUILTIN_PROFILES.map(p => p.id))
    for (const s of BUILTIN_SUITES) {
      for (const id of s.profiles) {
        // either valid or we assume they will be added. But builtins should only reference builtins
        // actually the solution says "thêm coverage-go... do các solution khác điền".
        // so maybe they reference external ones?
        // Let's check if the current profiles are in validIds or are the external ones.
        const external = ['coverage-go', 'coverage-ts', 'repo-rules', 'repo-rules-scripts', 'security-go-vuln', 'security-deps-osv', 'security-secrets-diff', 'dependency-diff']
        if (!validIds.has(id)) {
          expect(external).toContain(id)
        }
      }
    }
  })

  it('getCatalog filters out enabled:false and invalid', () => {
    const { profiles } = getCatalog()
    expect(profiles.find(p => p.id === 'ts-typecheck-agent')).toBeUndefined()
  })
})
