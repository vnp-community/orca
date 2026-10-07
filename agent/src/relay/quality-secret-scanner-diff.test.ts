import { describe, it, expect } from 'vitest'
import { scanAddedLines } from './quality-secret-scanner-diff'
import { makeCanary } from './quality-secret-redaction'
import type { ChangedFileDiff } from './quality-diff-changed-lines'

describe('quality-secret-scanner-diff', () => {
  it('detects AWS access key dynamically generated at runtime', () => {
    const canary = makeCanary()
    const diffs: ChangedFileDiff[] = [
      {
        path: 'src/config.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 10, text: `const awsKey = "${canary}";` }]
      }
    ]

    const result = scanAddedLines(diffs)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].ruleId).toBe('SEC-SECRET/aws-access-key')
    expect(result.findings[0].file).toBe('src/config.ts')
    expect(result.findings[0].line).toBe(10)
    expect(result.findings[0].severity).toBe('error')

    // Ensure canary value is completely absent from finding fields
    const findingStr = JSON.stringify(result.findings[0])
    expect(findingStr).not.toContain(canary)
  })

  it('detects private key block', () => {
    const diffs: ChangedFileDiff[] = [
      {
        path: 'keys/server.key',
        status: 'A',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: '-----BEGIN RSA PRIVATE KEY-----' }]
      }
    ]

    const result = scanAddedLines(diffs)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].ruleId).toBe('SEC-SECRET/private-key')
  })

  it('skips excluded lockfiles and test directories', () => {
    const canary = makeCanary()
    const diffs: ChangedFileDiff[] = [
      {
        path: 'pnpm-lock.yaml',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 100, text: `spec: "${canary}"` }]
      },
      {
        path: 'src/__fixtures__/sample.ts',
        status: 'A',
        untracked: false,
        binary: false,
        addedLines: [{ line: 5, text: `const token = "${canary}";` }]
      }
    ]

    const result = scanAddedLines(diffs)
    expect(result.findings).toHaveLength(0)
  })

  it('does not trigger on normal code lines (negative test)', () => {
    const diffs: ChangedFileDiff[] = [
      {
        path: 'src/utils.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [
          { line: 1, text: 'export function add(a: number, b: number): number {' },
          { line: 2, text: '  return a + b;' },
          { line: 3, text: '}' }
        ]
      }
    ]

    const result = scanAddedLines(diffs)
    expect(result.findings).toHaveLength(0)
  })

  it('handles truncation when limit is reached', () => {
    const diffs: ChangedFileDiff[] = [
      {
        path: 'file1.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'some content' }]
      },
      {
        path: 'file2.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'some other content' }]
      }
    ]

    const result = scanAddedLines(diffs, { maxFiles: 1 })
    expect(result.truncated).toBe(true)
  })
})
