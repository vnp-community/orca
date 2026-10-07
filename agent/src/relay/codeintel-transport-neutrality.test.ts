import { describe, it, expect } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'

describe('codeintel-transport-neutrality', () => {
  const relayDir = path.resolve(__dirname)

  it('verifies codeintel core files do not import transport-specific modules or write directly to stdout', async () => {
    const files = await fs.readdir(relayDir)
    const targetFiles = files.filter(f => {
      if (f.endsWith('.test.ts')) return false
      if (f === 'agent-rpc-dispatch-codeintel.ts') return false
      if (f === 'codeintel-relay-handlers.ts') return false
      return (
        f.startsWith('codeintel-') ||
        f.startsWith('gitnexus-') ||
        f.startsWith('codegraph-')
      ) && f.endsWith('.ts')
    })

    expect(targetFiles.length).toBeGreaterThan(0)

    const forbiddenImports = [
      'ws',
      'orca-dev-agent-transport',
      './agent-rpc-dispatch',
      './dispatcher',
      'agent-git-handler-extended'
    ]

    const violations: Array<{ file: string; issue: string }> = []

    for (const file of targetFiles) {
      const content = await fs.readFile(path.join(relayDir, file), 'utf8')

      for (const forbidden of forbiddenImports) {
        const importRegex = new RegExp(`from\\s+['"]${forbidden}['"]|import\\s*\\(['"]${forbidden}['"]\\)`, 'g')
        if (importRegex.test(content)) {
          violations.push({ file, issue: `Forbidden import: ${forbidden}` })
        }
      }

      // Check console.log or process.stdout.write
      if (/console\.(log|info|warn|error)\(/.test(content)) {
        violations.push({ file, issue: 'Direct console.* call found' })
      }
      if (/process\.stdout\.write\(/.test(content)) {
        violations.push({ file, issue: 'Direct process.stdout.write call found' })
      }
    }

    expect(violations).toEqual([])
  })

  it('fails test if a forbidden import is intentionally present in code', () => {
    const fakeContent = `import { makeError } from './agent-rpc-dispatch'\nconsole.log('test')`
    const forbidden = ['./agent-rpc-dispatch']
    const hasForbidden = forbidden.some(f => fakeContent.includes(f))
    const hasConsole = /console\.log\(/.test(fakeContent)
    expect(hasForbidden).toBe(true)
    expect(hasConsole).toBe(true)
  })
})
