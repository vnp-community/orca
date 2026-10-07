import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import {
  assertNoSecretFields,
  makeCanary,
  scanTreeForCanary
} from './quality-secret-redaction'
import { scanAddedLines } from './quality-secret-scanner-diff'
import { createTempDiffFile, cleanupTempDiffFile } from './quality-parser-gitleaks'
import type { ChangedFileDiff } from './quality-diff-changed-lines'

describe('quality-secret-redaction and canary harness', () => {
  it('assertNoSecretFields passes when secret fields are redacted or absent', () => {
    expect(() => {
      assertNoSecretFields({
        RuleID: 'rule-1',
        Secret: 'REDACTED',
        Match: '***'
      })
    }).not.toThrow()

    expect(() => {
      assertNoSecretFields({
        RuleID: 'rule-2',
        Secret: ''
      })
    }).not.toThrow()
  })

  it('assertNoSecretFields throws when unredacted secret values exist', () => {
    expect(() => {
      assertNoSecretFields({
        RuleID: 'rule-1',
        Secret: 'my-sensitive-token'
      })
    }).toThrow(/Unredacted secret field detected: Secret/)

    expect(() => {
      assertNoSecretFields({
        nested: {
          Match: 'secret-key-123'
        }
      })
    }).toThrow(/Unredacted secret field detected: Match/)
  })

  it('makeCanary generates dynamic canary at runtime', () => {
    const c1 = makeCanary()
    const c2 = makeCanary()
    expect(c1).not.toBe(c2)
    expect(c1.startsWith('AKIA')).toBe(true)
    expect(c1.length).toBe(20)
  })

  it('scanTreeForCanary detects canary leak and verifies cleanup', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'canary-test-'))
    try {
      const canary = makeCanary()
      const leakedFile = path.join(tmpDir, 'leak.log')
      fs.writeFileSync(leakedFile, `Log output containing ${canary} in stream`, 'utf8')

      const detected = scanTreeForCanary([tmpDir], canary)
      expect(detected).toContain(leakedFile)

      // Overwrite with redacted content
      fs.writeFileSync(leakedFile, 'Log output containing REDACTED in stream', 'utf8')
      const afterRedact = scanTreeForCanary([tmpDir], canary)
      expect(afterRedact).toHaveLength(0)
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  })

  it('end-to-end canary scan on diff and verifies canary is absent from all findings and logs', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'canary-e2e-'))
    try {
      const canary = makeCanary()
      const diffFiles: ChangedFileDiff[] = [
        {
          path: 'src/secrets.ts',
          status: 'M',
          untracked: false,
          binary: false,
          addedLines: [
            { line: 5, text: `const accessKey = "${canary}";` }
          ]
        }
      ]

      // Create temporary diff file
      const tempDiffPath = createTempDiffFile(tmpDir, `+ const accessKey = "${canary}";`)
      expect(fs.existsSync(tempDiffPath)).toBe(true)

      // Scan added lines
      const scanResult = scanAddedLines(diffFiles)
      expect(scanResult.findings).toHaveLength(1)
      expect(scanResult.findings[0].ruleId).toBe('SEC-SECRET/aws-access-key')

      // Assert canary never appears in findings JSON
      const serializedFindings = JSON.stringify(scanResult.findings)
      expect(serializedFindings).not.toContain(canary)

      // Simulate log recording
      const logs: string[] = []
      logs.push(`[INFO] Run started with 1 file`)
      logs.push(`[INFO] Scan completed, found rule ${scanResult.findings[0].ruleId}`)
      const serializedLogs = JSON.stringify(logs)
      expect(serializedLogs).not.toContain(canary)

      // Cleanup temp diff file on completion / error
      cleanupTempDiffFile(tempDiffPath)
      expect(fs.existsSync(tempDiffPath)).toBe(false)

      // Verify no remaining leaks in directory
      const remainingLeaks = scanTreeForCanary([tmpDir], canary)
      expect(remainingLeaks).toHaveLength(0)
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    }
  })
})
