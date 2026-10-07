import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createResultsStore } from './quality-results-store'
import fs from 'fs'
import path from 'path'
import os from 'os'

describe('quality-results-store', () => {
  let deps: any
  let tmpRoot: string

  beforeEach(() => {
    vi.useFakeTimers()
    deps = {
      now: vi.fn().mockReturnValue(1000),
      ttlMs: 3600000,
      maxRunsPerWorktree: 20,
      redact: (s: string) => s.replace(/secret/g, '***')
    }
    tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-results-test-'))
  })

  afterEach(() => {
    vi.useRealTimers()
    fs.rmSync(tmpRoot, { recursive: true, force: true })
  })

  it('paginates 1200 findings 500+500+200', () => {
    const store = createResultsStore(deps)
    const findings = Array.from({ length: 1200 }).map((_, i) => ({
      ruleId: `r${i}`, level: 'info', message: 'm'
    }))

    store.put('/repo', 'r1', { steps: [], findings, logPaths: {} })

    const p1 = store.getFindings('/repo', 'r1', { offset: 0, limit: 1000 })
    expect(p1.items.length).toBe(500)
    expect(p1.nextOffset).toBe(500)

    const p2 = store.getFindings('/repo', 'r1', { offset: p1.nextOffset!, limit: 1000 })
    expect(p2.items.length).toBe(500)
    expect(p2.nextOffset).toBe(1000)

    const p3 = store.getFindings('/repo', 'r1', { offset: p2.nextOffset!, limit: 1000 })
    expect(p3.items.length).toBe(200)
    expect(p3.nextOffset).toBeNull()
  })

  it('cuts page at 1 MiB', () => {
    const store = createResultsStore(deps)
    
    // Each is ~100 KB
    const giantMsg = 'X'.repeat(100 * 1024)
    const findings = Array.from({ length: 20 }).map((_, i) => ({
      ruleId: `r${i}`, level: 'info', message: giantMsg
    }))

    store.put('/repo', 'r1', { steps: [], findings, logPaths: {} })

    const p = store.getFindings('/repo', 'r1', { offset: 0, limit: 500 })
    expect(p.items.length).toBeLessThan(20) // should stop before 1MB
    // 1MB / 100KB is roughly 10 items
    expect(p.items.length).toBe(10)
    expect(p.nextOffset).toBe(10)
  })

  it('returns empty if TTL expired', () => {
    const store = createResultsStore(deps)
    store.put('/repo', 'r1', { steps: [], findings: [{ ruleId: 'x', level: 'w', message: 'x' }], logPaths: {} })
    
    deps.now.mockReturnValue(1000 + 3600000 + 1)
    
    const p = store.getFindings('/repo', 'r1', { offset: 0, limit: 10 })
    expect(p.items).toEqual([]) // empty valid page
    expect(p.totalCount).toBe(0)
  })

  it('evicts oldest when exceeding maxRunsPerWorktree', () => {
    const store = createResultsStore(deps)
    
    for (let i = 1; i <= 21; i++) {
      deps.now.mockReturnValue(1000 + i)
      store.put('/repo', `r${i}`, { steps: [], findings: [], logPaths: {} })
    }

    const p1 = store.getFindings('/repo', 'r1', { offset: 0, limit: 10 })
    expect(p1.items).toEqual([]) // r1 evicted
    
    const p21 = store.getFindings('/repo', 'r21', { offset: 0, limit: 10 })
    expect(p21.view).toBe('findings') // r21 exists
  })

  it('throws RUN_NOT_FOUND for wrong worktree', () => {
    const store = createResultsStore(deps)
    store.put('/repo1', 'r1', { steps: [], findings: [], logPaths: {} })
    
    expect(() => {
      store.getFindings('/repo2', 'r1', { offset: 0, limit: 10 })
    }).toThrow(/Run not found/)
  })

  it('paginates logs across stdout and stderr, redacts and cuts at newline', () => {
    const store = createResultsStore(deps)
    
    const stdoutP = path.join(tmpRoot, 'out')
    const stderrP = path.join(tmpRoot, 'err')
    
    // Total size < 64KB so it can fit in one read if offset=0, but we test newline cut
    fs.writeFileSync(stdoutP, 'line1\\nsecret log\\nline3 part')
    fs.writeFileSync(stderrP, '2\\nerr line\\n')
    
    store.put('/repo', 'r1', { steps: [], findings: [], logPaths: { s1: { stdout: stdoutP, stderr: stderrP } } })

    const p = store.getLog('/repo', 'r1', 's1', 0)
    // The cut should happen at the last newline (after "err line")
    expect(p.text).toContain('line1\\n*** log\\nline3 part2\\nerr line\\n')
    expect(p.nextOffset).toBeNull() // because we read to EOF (total size < 64KB + 512)
    // Wait, the test string is "line1\nsecret log\nline3 part2\nerr line\n" (length ~40)
  })

  it('preserves ruleResults in steps', () => {
    const store = createResultsStore(deps)
    store.put('/repo', 'r1', {
      steps: [{ id: 's1', tool: 't', state: 'failed', durationMs: 10, ruleResults: [{ ruleId: 'R-1', status: 'ran' }] }],
      findings: [],
      logPaths: {}
    })
    const p = store.getSteps('/repo', 'r1', { offset: 0, limit: 10 })
    expect(p.items[0].ruleResults).toBeDefined()
    expect(p.items[0].ruleResults![0].ruleId).toBe('R-1')
  })
})
