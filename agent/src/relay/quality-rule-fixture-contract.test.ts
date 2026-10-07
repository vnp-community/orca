import { describe, it, expect } from 'vitest'
import fs from 'fs/promises'
import path from 'path'

const FIXTURES_DIR = path.join(__dirname, '__fixtures__/quality-rules')

describe('quality-rule-fixture-contract', () => {
  it('fixtures contain MANIFEST.json and are within size limits', async () => {
    let checked = 0
    let dirs = []
    try {
      dirs = await fs.readdir(FIXTURES_DIR)
    } catch {
      return // skip if no fixtures
    }

    for (const ruleDir of dirs) {
      const shas = await fs.readdir(path.join(FIXTURES_DIR, ruleDir))
      for (const sha of shas) {
        const fixtureDir = path.join(FIXTURES_DIR, ruleDir, sha)
        
        const manifestStr = await fs.readFile(path.join(fixtureDir, 'MANIFEST.json'), 'utf8')
        const manifest = JSON.parse(manifestStr)
        expect(manifest.exitCode).toBeDefined()
        expect(manifest.cwd).toBeDefined()
        expect(manifest.durationMs).toBeDefined()

        const stdout = await fs.readFile(path.join(fixtureDir, 'stdout.txt'), 'utf8')
        const stderr = await fs.readFile(path.join(fixtureDir, 'stderr.txt'), 'utf8')
        
        expect(Buffer.from(stdout).length).toBeLessThanOrEqual(20480 + 20) // account for TRUNCATED string
        expect(Buffer.from(stderr).length).toBeLessThanOrEqual(20480 + 20)

        // should not contain absolute paths (assuming /Users or /home or /opt)
        expect(stdout).not.toContain('/Users/')
        expect(stdout).not.toContain('/opt/repos/')

        checked++
      }
    }
    expect(checked).toBeGreaterThan(0)
  })
})
