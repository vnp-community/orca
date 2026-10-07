import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'

describe('quality-coverage-fixture-contract', () => {
  const fixtureDir = path.resolve(__dirname, '__fixtures__/coverage-go')

  it('validates MANIFEST.json and fixture integrity', () => {
    const manifestPath = path.join(fixtureDir, 'MANIFEST.json')
    expect(fs.existsSync(manifestPath)).toBe(true)

    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
    expect(manifest.tool).toBe('go')
    expect(Array.isArray(manifest.fixtures)).toBe(true)

    for (const item of manifest.fixtures) {
      const filePath = path.join(fixtureDir, item.file)
      expect(fs.existsSync(filePath)).toBe(true)

      const content = fs.readFileSync(filePath)
      expect(content.length).toBeLessThanOrEqual(20 * 1024) // <= 20 KiB

      const hash = crypto.createHash('sha256').update(content).digest('hex')
      expect(hash).toBe(item.sha256)

      // Ensure no raw absolute paths leaked
      const text = content.toString('utf8')
      expect(text).not.toMatch(/\/home\//)
      expect(text).not.toMatch(/\/opt\/repos/)
      expect(text).not.toMatch(/\/tmp\//)
    }
  })
})
