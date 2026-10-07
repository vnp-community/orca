import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {
  parseGitleaksJson,
  createTempDiffFile,
  cleanupTempDiffFile
} from './quality-parser-gitleaks'

describe('quality-parser-gitleaks', () => {
  it('parses redacted gitleaks findings successfully', () => {
    const json = JSON.stringify([
      {
        RuleID: 'aws-access-key',
        Description: 'AWS Access Key ID',
        StartLine: 15,
        StartColumn: 1,
        File: 'config/aws.ts',
        Secret: 'REDACTED',
        Match: 'REDACTED',
        Entropy: 0
      }
    ])

    const result = parseGitleaksJson(json)
    expect(result.formatDrift).toBe(false)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].ruleId).toBe('SEC-SECRET/aws-access-key')
    expect(result.findings[0].line).toBe(15)

    // Ensure raw secret fields are not in finding output
    const str = JSON.stringify(result.findings[0])
    expect(str).not.toContain('Entropy')
    expect(str).not.toContain('Fingerprint')
  })

  it('rejects unredacted secret fields and throws error', () => {
    const json = JSON.stringify([
      {
        RuleID: 'slack-token',
        Description: 'Slack API Token',
        File: 'bot.ts',
        Secret: 'xoxb-real-secret-token'
      }
    ])

    expect(() => parseGitleaksJson(json)).toThrow(/Unredacted secret field detected/)
  })

  it('creates temporary diff file with 0600 mode and cleans up reliably', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'diff-test-'))
    try {
      const diffPath = createTempDiffFile(tmpDir, 'diff added content')
      expect(fs.existsSync(diffPath)).toBe(true)

      const stat = fs.statSync(diffPath)
      // Check file mode ends in 0600 (owner read/write only)
      expect(stat.mode & 0o777).toBe(0o600)

      cleanupTempDiffFile(diffPath)
      expect(fs.existsSync(diffPath)).toBe(false)
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  })
})
