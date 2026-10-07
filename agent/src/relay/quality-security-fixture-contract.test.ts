import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'

describe('quality-security-fixture-contract', () => {
  const fixturesDir = path.resolve(__dirname, '__fixtures__/quality-security')

  it('validates MANIFEST.json and fixture integrity', () => {
    const manifestPath = path.join(fixturesDir, 'MANIFEST.json')
    expect(fs.existsSync(manifestPath)).toBe(true)

    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
    expect(manifest.tools).toBeDefined()
    expect(manifest.tools.govulncheck.status).toBe('BLOCKED')
    expect(manifest.tools['osv-scanner'].status).toBe('BLOCKED')
    expect(manifest.tools.gitleaks.status).toBe('BLOCKED')

    for (const [filename, meta] of Object.entries<any>(manifest.fixtures)) {
      const filePath = path.join(fixturesDir, filename)
      expect(fs.existsSync(filePath)).toBe(true)

      const content = fs.readFileSync(filePath, 'utf8')
      const hash = crypto.createHash('sha256').update(content).digest('hex')
      expect(hash).toBe(meta.sha256)
      expect(Buffer.byteLength(content)).toBeLessThanOrEqual(20 * 1024)

      // Ensure no absolute paths from local machine
      expect(content).not.toContain('/Users/')
      expect(content).not.toContain('/home/')
    }
  })
})
