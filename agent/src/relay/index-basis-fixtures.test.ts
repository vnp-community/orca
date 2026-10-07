import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import crypto from 'crypto'

describe('index-basis fixtures', () => {
  const fixturesDir = path.join(__dirname, '__fixtures__', 'index-basis')

  it('verifies all files are listed in MANIFEST.json and match hashes', () => {
    if (!fs.existsSync(fixturesDir) || !fs.existsSync(path.join(fixturesDir, 'MANIFEST.json'))) {
      console.warn('Skipping index-basis tests, no MANIFEST.json')
      return
    }

    const manifest = JSON.parse(fs.readFileSync(path.join(fixturesDir, 'MANIFEST.json'), 'utf8'))
    const files = fs.readdirSync(fixturesDir).filter(f => f !== 'MANIFEST.json')

    for (const file of files) {
      const filePath = path.join(fixturesDir, file)
      const stat = fs.statSync(filePath)
      
      expect(manifest.files[file]).toBeDefined()
      expect(stat.size).toBeLessThanOrEqual(20 * 1024)

      const content = fs.readFileSync(filePath)
      const text = content.toString('utf8')
      const hash = crypto.createHash('sha256').update(content).digest('hex')

      expect(hash).toBe(manifest.files[file].sha256)
      expect(manifest.files[file].bytes).toBe(stat.size)

      expect(text).not.toContain('/home/')
      expect(text).not.toContain('/opt/repos')
      expect(text).not.toContain('/tmp/')
    }
  })
})
