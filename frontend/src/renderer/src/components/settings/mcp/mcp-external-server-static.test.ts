import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const dir = __dirname
const sources = readdirSync(dir)
  .filter((f) =>
    /^(McpExternalServer|UntrustedToolText|mcp-external-server-validation\.ts|mcp-tool-diff\.ts|use-external-servers\.ts)/.test(
      f
    )
  )
  .filter((f) => !/\.test\./.test(f))

describe('external server sources', () => {
  it('finds the source files', () => {
    expect(sources.length).toBeGreaterThanOrEqual(12)
  })
  it.each(sources)('%s has no HTML injection, logging, browser storage or e2e claims', (f) => {
    const text = readFileSync(join(dir, f), 'utf8')
    expect(text).not.toMatch(/dangerouslySetInnerHTML|innerHTML|marked|react-markdown/)
    expect(text).not.toMatch(
      /console\.|localStorage|sessionStorage|encryptCredential|encryptedBlob/
    )
    expect(text.toLowerCase()).not.toContain('end-to-end')
  })
  it('keeps secret handling out of React state', () => {
    const field = readFileSync(join(dir, 'McpExternalServerSecretField.tsx'), 'utf8')
    expect(field).not.toMatch(/useState/)
  })
})
