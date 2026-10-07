import { describe, it, expect } from 'vitest'
import { validateProfile, parseArgvTemplate, definitionHashOf, displayOf } from './quality-profile-schema'

describe('quality-profile-schema', () => {
  it('validates a correct profile', () => {
    const res = validateProfile({
      id: 'eslint-main',
      title: 'ESLint',
      parser: 'eslint-json',
      argv: ['{bin:eslint}', '-f', 'json', '{files}'],
      cwd: 'frontend',
      timeoutMs: 60000
    })
    expect(res.ok).toBe(true)
  })

  it('rejects invalid id', () => {
    const res = validateProfile({
      id: '-bad',
      title: 'A',
      parser: 'B',
      argv: ['{bin:foo}']
    })
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.errors[0]).toMatch(/id must match/)
  })

  it('rejects argv[0] not node or bin', () => {
    const res = validateProfile({
      id: 'a', title: 'b', parser: 'c',
      argv: ['sh', '-c', 'echo']
    })
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.errors[0]).toMatch(/argv\[0\] must be node or/)
  })

  it('rejects {files} not being the whole token', () => {
    const res = validateProfile({
      id: 'a', title: 'b', parser: 'c',
      argv: ['{bin:x}', '--files={files}']
    })
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.errors[0]).toMatch(/{files} must be the entire argument/)
  })

  it('rejects absolute or escaping cwd', () => {
    const r1 = validateProfile({ id: 'a', title: 'b', parser: 'c', argv: ['node'], cwd: '/foo' })
    expect(r1.ok).toBe(false)
    const r2 = validateProfile({ id: 'a', title: 'b', parser: 'c', argv: ['node'], cwd: '../foo' })
    expect(r2.ok).toBe(false)
    const r3 = validateProfile({ id: 'a', title: 'b', parser: 'c', argv: ['node'], cwd: 'foo\\bar' })
    expect(r3.ok).toBe(false)
  })

  it('rejects secret env', () => {
    const res = validateProfile({
      id: 'a', title: 'b', parser: 'c', argv: ['node'],
      env: { set: { 'AWS_SECRET_ACCESS_KEY': '123' } }
    })
    console.log(res); expect(res.ok).toBe(false)
    if (!res.ok) expect(res.errors[0]).toMatch(/cannot contain secret-matching key/)
  })

  it('hash is deterministic and ignores title', () => {
    const p1 = {
      id: 'a', title: 'T1', parser: 'p', argv: ['node'], cwd: 'a', env: { copy: ['X'] }
    }
    const p2 = {
      title: 'T2', env: { copy: ['X'] }, cwd: 'a', argv: ['node'], parser: 'p', id: 'a'
    }
    
    expect(definitionHashOf(p1)).toBe(definitionHashOf(p2 as any))
    
    const p3 = { ...p1, argv: ['node', 'foo'] }
    expect(definitionHashOf(p3)).not.toBe(definitionHashOf(p1))
  })

  it('displayOf redacts tmp correctly', () => {
    const s = displayOf({
      id: 'my-linter',
      title: 'x',
      parser: 'y',
      argv: ['{bin:my-linter}', '--config', '{tmp:cfg.json}', '{files}'],
      cwd: 'src'
    })
    expect(s).toBe('my-linter: {bin:my-linter} --config <tmp>cfg.json {files} (cwd: src)')
  })
})
