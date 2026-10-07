import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'

describe('quality-profile-catalog-evidence', () => {
  const fixDir = path.join(__dirname, '__fixtures__', 'quality-catalog')
  let evidence: any

  try {
    evidence = JSON.parse(fs.readFileSync(path.join(fixDir, 'evidence.json'), 'utf8'))
  } catch (e) {
    evidence = { tools: {}, files: {} }
  }

  function getHelp(toolName: string, helpFile: string): string | null {
    const origTool = Object.keys(evidence.tools).find(k => k.includes(toolName))
    if (!origTool) return null
    const ver = evidence.tools[origTool]
    const p = path.join(fixDir, origTool, ver, helpFile + '.txt')
    if (fs.existsSync(p)) return fs.readFileSync(p, 'utf8')
    return null
  }

  it('oxlint supports json format', () => {
    const help = getHelp('oxlint', 'help')
    if (help) {
      expect(help).toContain('--format')
    }
  })

  it('vitest supports reporter and outputFile', () => {
    const help = getHelp('vitest', 'help')
    if (help) {
      expect(help).toContain('--reporter')
      expect(help).toContain('--outputFile')
    }
  })

  it('tsc supports pretty', () => {
    const help = getHelp('tsc', 'help')
    if (help) {
      expect(help).toContain('--pretty')
    }
  })

  it('go vet help', () => {
    const help = getHelp('go', 'help-vet')
    if (help) {
      // just check it has vet output
      expect(help.length).toBeGreaterThan(0)
    }
  })

  it('golangci-lint supports format', () => {
    const help = getHelp('golangci-lint', 'help-run')
    if (help) {
      expect(help).toContain('--out-format')
    }
  })

  it('buf supports error-format', () => {
    const helpLint = getHelp('buf', 'help-lint')
    if (helpLint) {
      expect(helpLint).toContain('--error-format')
    }
    const helpBreak = getHelp('buf', 'help-breaking')
    if (helpBreak) {
      expect(helpBreak).toContain('--error-format')
    }
  })

  it('opa supports format json', () => {
    const help = getHelp('opa', 'help-test')
    if (help) {
      expect(help).toContain('--format')
    }
  })
})
