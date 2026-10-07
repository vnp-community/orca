import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import crypto from 'crypto'

const FIXTURES_DIR = path.resolve(__dirname, '__fixtures__/quality')
const CATALOG_EVIDENCE = path.resolve(__dirname, '__fixtures__/quality-catalog/evidence.json')

describe('Quality Fixture Contract', () => {
  it('each version directory has a valid MANIFEST.json and matching hashes', () => {
    if (!fs.existsSync(FIXTURES_DIR)) return
    
    const tools = fs.readdirSync(FIXTURES_DIR)
    for (const tool of tools) {
      const toolDir = path.join(FIXTURES_DIR, tool)
      if (!fs.statSync(toolDir).isDirectory()) continue
      
      const versions = fs.readdirSync(toolDir)
      for (const ver of versions) {
        const verDir = path.join(toolDir, ver)
        if (!fs.statSync(verDir).isDirectory()) continue
        
        // Exclude empty version strings which signify failed captures
        if (ver.trim() === '') continue
        
        const manifestPath = path.join(verDir, 'MANIFEST.json')
        expect(fs.existsSync(manifestPath)).toBe(true)
        
        const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
        expect(manifest.tool).toBe(tool)
        expect(manifest.toolVersion).toBe(ver)
        expect(Array.isArray(manifest.argv)).toBe(true)
        
        const outPath = path.join(verDir, 'output.txt')
        expect(fs.existsSync(outPath)).toBe(true)
        
        const outContent = fs.readFileSync(outPath, 'utf8')
        const actualHash = crypto.createHash('sha256').update(outContent).digest('hex')
        expect(manifest.sha256).toBe(actualHash)
        
        // Check size constraints
        const stat = fs.statSync(outPath)
        expect(stat.size).toBeLessThanOrEqual(20 * 1024) // <= 20 KiB per file
        
        // No absolute paths
        expect(outContent).not.toMatch(/\/home\//)
        expect(outContent).not.toMatch(/\/opt\/repos/)
        expect(outContent).not.toMatch(/\/tmp\//)
      }
    }
  })
})
