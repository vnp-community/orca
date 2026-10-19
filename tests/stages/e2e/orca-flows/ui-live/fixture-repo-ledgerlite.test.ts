import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import {
  LEDGERLITE_RELEASE_BRANCH,
  createLedgerliteRepo,
  ledgerliteBugPatch,
  tddAgentAvailable
} from './fixture-repo-ledgerlite'

describe('ledgerlite fixture repo', () => {
  it('builds a green baseline with three branches that have distinct tips', () => {
    const repo = createLedgerliteRepo('t1', {
      releaseBranch: true,
      thirdBranch: 'feature/other',
      verifyBaseline: true
    })
    try {
      expect(repo.currentBranch()).toBe('feature/other')
      const shas = new Set(repo.branches.map((b) => repo.revParse(b)))
      expect(shas.size).toBe(3)
      expect(repo.branches).toEqual(
        expect.arrayContaining(['main', LEDGERLITE_RELEASE_BRANCH, 'feature/other'])
      )
    } finally {
      repo.cleanup()
    }
  }, 120_000)

  it('injects the scenario 1 bug from the scenarios document and leaves a clean tree', () => {
    expect(ledgerliteBugPatch()).toContain('dayStart := now.Add(-24 * time.Hour)')
    const repo = createLedgerliteRepo('t2', { injectBug: true })
    try {
      expect(repo.bugCommit).not.toBe('')
      expect(repo.revParse('HEAD')).toBe(repo.bugCommit)
      expect(readFileSync(join(repo.path, 'internal/ledger/service.go'), 'utf8')).toContain(
        'now.Add(-24 * time.Hour)'
      )
      expect(repo.git('status', '--porcelain').trim()).toBe('')
    } finally {
      repo.cleanup()
    }
  }, 60_000)

  it.runIf(tddAgentAvailable())(
    'commits .mcp.json but not the generated knowledge database',
    () => {
      const repo = createLedgerliteRepo('t3', { knowledge: true })
      try {
        const tracked = repo.git('ls-files').split('\n')
        expect(tracked).toContain('.mcp.json')
        expect(tracked).not.toContain('.tdd-agent/knowledge.db')
      } finally {
        repo.cleanup()
      }
    },
    120_000
  )
})
