import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import { parseGitNexusDetectChangesHeader } from '../gitnexus-detect-changes-header'
import { CodeIntelError } from '../codeintel-errors'

describe('gitnexus-text-commands golden tests', () => {
  const fixtureDir = path.join(__dirname, '__fixtures__', 'gitnexus', '1.6.9')

  it('parses detect-changes-unstaged.txt correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'detect-changes-unstaged.txt'), 'utf8')
    const hint = parseGitNexusDetectChangesHeader(raw)
    expect(hint).not.toBeNull()
    expect(hint?.files).toBe(2)
    expect(hint?.symbols).toBe(4)
    expect(hint?.processes).toBe(3)
    expect(hint?.level).toBe('MEDIUM')
  })

  it('handles CRLF variation in detect-changes output', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'detect-changes-unstaged.txt'), 'utf8')
    const crlf = raw.replace(/\n/g, '\r\n')
    const hint = parseGitNexusDetectChangesHeader(crlf)
    expect(hint).not.toBeNull()
    expect(hint?.files).toBe(2)
    expect(hint?.symbols).toBe(4)
    expect(hint?.level).toBe('MEDIUM')
  })

  it('detects format drift in detect-changes output without silently guessing', () => {
    const driftText = 'Changes: unparseable text\nAffected: ???\nRisk: UNKNOWN'
    const hint = parseGitNexusDetectChangesHeader(driftText)
    expect(hint).toBeNull()
  })

  it('parses meta.json and verifies schemaVersion, lastCommit, indexedAt', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'meta.json'), 'utf8')
    const meta = JSON.parse(raw)
    expect(meta.schemaVersion).toBe(5)
    expect(meta.lastCommit).toBe('abc1234567890abcdef1234567890abcdef1234')
    expect(meta.indexedAt).toBe(1700000000000)
    // Ensures no leak of fileHashes or cacheKeys
    expect(meta.fileHashes).toBeUndefined()
    expect(meta.cacheKeys).toBeUndefined()
  })

  it('parses no-repo-flag.stderr.txt and does not leak external repo or internal paths', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'no-repo-flag.stderr.txt'), 'utf8')
    expect(raw).toContain('Repository not registered')
    expect(raw).not.toMatch(/\/Users\/|\/home\/|\/opt\//)
    expect(raw).not.toContain('.gitnexus')
  })

  it('parses status-stale.txt and list.txt', () => {
    const statusText = fs.readFileSync(path.join(fixtureDir, 'status-stale.txt'), 'utf8')
    expect(statusText).toContain('GitNexus status: stale')

    const listText = fs.readFileSync(path.join(fixtureDir, 'list.txt'), 'utf8')
    expect(listText).toContain('Repositories:')
  })
})
