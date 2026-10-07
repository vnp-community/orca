import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { runBench } from './codeintel-bench-runner'

describe('codeintel-bench-runner smoke tests', () => {
  it('generates bench report matching schema with numeric percentiles and limits', async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'bench-runner-smoke-'))
    try {
      const report = await runBench({
        commit: 'test-commit',
        coldRuns: 2,
        warmRuns: 3,
        workspaceRoots: ['/repo1', '/repo2', '/repo3'],
        outDir: tmpDir,
        executor: async (method: string, root: string) => ({
          ms: 45,
          bytes: 2048,
          truncated: false,
          rssKb: 15000
        })
      })

      expect(report.commit).toBe('test-commit')
      expect(typeof report.timestamp).toBe('string')
      expect(report.scenarios.length).toBe(3)

      for (const s of report.scenarios) {
        expect(typeof s.scenario).toBe('string')
        expect(typeof s.p50TotalMs).toBe('number')
        expect(typeof s.p95TotalMs).toBe('number')
        expect(typeof s.p99TotalMs).toBe('number')
        expect(typeof s.totalMs).toBe('number')
        expect(typeof s.payloadBytes).toBe('number')
        expect(typeof s.truncatedRate).toBe('number')
        expect(typeof s.rssPeakKbMax).toBe('number')
        expect(s.concurrency).toBeLessThanOrEqual(3)
      }

      const outFile = path.join(tmpDir, 'codeintel-agent-bench-test-commit.json')
      expect(fs.existsSync(outFile)).toBe(true)
      const parsed = JSON.parse(fs.readFileSync(outFile, 'utf8'))
      expect(parsed.commit).toBe('test-commit')
      expect(parsed.scenarios.length).toBe(3)
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  })
})
