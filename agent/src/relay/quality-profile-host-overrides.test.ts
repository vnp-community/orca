import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { loadHostOverrides, applyOverrides } from './quality-profile-host-overrides'
import { QualityCheckProfile } from './quality-profile-schema'

describe('quality-profile-host-overrides', () => {
  let tmpHome: string
  let warnings: string[] = []
  const deps = { warn: (m: string) => warnings.push(m) }

  beforeEach(() => {
    tmpHome = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-qa-'))
    warnings = []
  })

  afterEach(() => {
    fs.rmSync(tmpHome, { recursive: true, force: true })
  })

  it('returns empty when no file exists', () => {
    const res = loadHostOverrides(tmpHome, deps)
    expect(res).toEqual({ add: [], disable: [], replace: [] })
    expect(warnings.length).toBe(0)
  })

  it('skips file if permissions are too broad (POSIX)', () => {
    if (process.platform === 'win32') return
    const dir = path.join(tmpHome, '.orca', 'quality')
    fs.mkdirSync(dir, { recursive: true, mode: 0o755 }) // Too broad
    fs.writeFileSync(path.join(dir, 'profiles.json'), '{}', { mode: 0o644 }) // Too broad

    const res = loadHostOverrides(tmpHome, deps)
    expect(res).toEqual({ add: [], disable: [], replace: [] })
    expect(warnings.some(w => w.includes('too broad permissions'))).toBe(true)
  })

  it('loads valid file', () => {
    const dir = path.join(tmpHome, '.orca', 'quality')
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 })
    
    const validProfile = {
      id: 'my-custom', title: 'X', parser: 'p', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }
    }
    
    const data = {
      version: 1,
      add: [validProfile],
      disable: ['go-vet'],
      replace: [{ id: 'ts-lint', reason: 'Because we need custom flags for ts-lint', profile: { ...validProfile, id: 'ts-lint' } }]
    }
    fs.writeFileSync(path.join(dir, 'profiles.json'), JSON.stringify(data), { mode: 0o600 })

    const res = loadHostOverrides(tmpHome, deps)
    expect(res.add[0].id).toBe('my-custom')
    expect(res.disable).toContain('go-vet')
    expect(res.replace[0].id).toBe('ts-lint')
  })

  it('skips replace if reason < 10 chars', () => {
    const dir = path.join(tmpHome, '.orca', 'quality')
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 })
    
    const validProfile = {
      id: 'ts-lint', title: 'X', parser: 'p', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }
    }
    const data = {
      version: 1,
      replace: [{ id: 'ts-lint', reason: 'short', profile: validProfile }]
    }
    fs.writeFileSync(path.join(dir, 'profiles.json'), JSON.stringify(data), { mode: 0o600 })

    const res = loadHostOverrides(tmpHome, deps)
    expect(res.replace.length).toBe(0)
    expect(warnings.some(w => w.includes('reason < 10 chars'))).toBe(true)
  })

  it('applyOverrides works correctly', () => {
    const base: QualityCheckProfile[] = [
      { id: 'ts-lint', title: 'A', parser: 'A', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }, scopes: ['worktree'], scopeStrategy: 'none', timeoutMs: 1, maxOutputBytes: 1, heavy: false, requires: [] },
      { id: 'go-vet', title: 'B', parser: 'B', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }, scopes: ['worktree'], scopeStrategy: 'none', timeoutMs: 1, maxOutputBytes: 1, heavy: false, requires: [] }
    ]

    const ov = {
      add: [{ id: 'new', title: 'C', parser: 'C', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }, scopes: ['worktree'], scopeStrategy: 'none', timeoutMs: 1, maxOutputBytes: 1, heavy: false, requires: [] }],
      disable: ['go-vet', 'unknown-id'],
      replace: [{ id: 'ts-lint', reason: 'Because test', profile: { id: 'ts-lint', title: 'A2', parser: 'A2', argv: ['node'], cwd: '.', env: { set: {}, allowExtra: [] }, exit: { ok: [0], findings: [1] }, scopes: ['worktree'], scopeStrategy: 'none', timeoutMs: 1, maxOutputBytes: 1, heavy: false, requires: [] } }]
    }

    const { catalog, source } = applyOverrides(base, ov, deps)
    
    expect(catalog.find(p => p.id === 'go-vet')).toBeUndefined()
    expect(catalog.find(p => p.id === 'unknown-id')).toBeUndefined()
    expect(catalog.find(p => p.id === 'new')).toBeDefined()
    expect(source['new']).toBe('host')
    
    const tslint = catalog.find(p => p.id === 'ts-lint')
    expect(tslint!.title).toBe('A2')
    expect(source['ts-lint']).toBe('host')

    expect(warnings.some(w => w.includes('unknown profile: unknown-id'))).toBe(true)
    expect(warnings.some(w => w.includes('Replacing profile ts-lint'))).toBe(true)
  })
})
