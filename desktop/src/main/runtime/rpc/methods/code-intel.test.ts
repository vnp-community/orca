import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { CODE_INTEL_METHODS } from './code-intel'
import {
  CodeIntelPortError,
  setCodeIntelSummaryPort,
  type CodeIntelSummaryPort
} from './code-intel-summary-port'

type ReviewSummaryResult = {
  available: boolean
  counts: Record<string, number>
  risk: { level: string }
  index: { state: string }
  findings: {
    totalOpen: number
    truncated: boolean
    items: { title: string; filePath?: string }[]
  }
}

const method = CODE_INTEL_METHODS.find((m) => m.name === 'codeIntel.reviewSummary')!
const ctx = { runtime: {} as never }
const params = { worktree: 'id:wt-1' }

function overlay(): Awaited<ReturnType<CodeIntelSummaryPort['getOverlay']>> {
  return {
    changedFiles: [{ path: 'a/b.go' }],
    risk: {
      level: 'MEDIUM',
      reasons: [{ messageKey: 'risk.flows', params: { n: '3' } }]
    },
    indexFreshness: {
      state: 'fresh',
      indexedCommit: 'abc',
      headOid: 'def',
      generatedAt: 't'
    },
    limits: {
      totalCounts: {
        files: 12,
        symbols: 38,
        flows: 3,
        tables: 2,
        contracts: 1,
        uncovered: 5
      }
    }
  }
}

function fakePort(over: Partial<CodeIntelSummaryPort> = {}): CodeIntelSummaryPort {
  return {
    getOverlay: async () => overlay(),
    getFindings: async () => ({
      totalCount: 120,
      findings: Array.from({ length: 60 }, (_, i) => ({
        findingKey: `k${i}`,
        rule: 'layer.usecase-adapter',
        kind: 'layer_violation',
        severity: i === 0 ? 'error' : 'weird',
        titleKey: 'finding.layer_violation.title',
        params: { from: 'usecase', to: 'adapter' },
        origin: i === 0 ? 'introduced' : 'nope',
        evidence: [
          {
            path: i === 0 ? 'a/b.go' : '/home/me/secret/x.go',
            line: 88,
            symbol: 'leak'
          }
        ] as never
      }))
    }),
    getStatus: async () => ({ overall: 'READY' }),
    ...over
  }
}

afterEach(() => setCodeIntelSummaryPort(null))

describe('codeIntel.reviewSummary', () => {
  it('maps overlay and findings, caps at 50, hides evidence and absolute paths', async () => {
    setCodeIntelSummaryPort(fakePort())
    const result = (await method.handler(params, ctx)) as ReviewSummaryResult
    expect(result.available).toBe(true)
    expect(result.counts).toEqual({
      files: 12,
      symbols: 38,
      flows: 3,
      tables: 2,
      contracts: 1,
      uncovered: 5
    })
    expect(result.risk.level).toBe('MEDIUM')
    expect(result.findings.items).toHaveLength(50)
    expect(result.findings.truncated).toBe(true)
    expect(result.findings.totalOpen).toBe(120)
    expect(result.findings.items[0]).toMatchObject({
      title: 'usecase depends on adapter',
      severity: 'error',
      origin: 'introduced',
      filePath: 'a/b.go',
      startLine: 88,
      inChangedFiles: true
    })
    expect(result.findings.items[1]).toMatchObject({
      severity: 'info',
      origin: 'unknown',
      inChangedFiles: false
    })
    expect(result.findings.items[1].filePath).toBeUndefined()
    expect(JSON.stringify(result)).not.toContain('evidence')
    expect(JSON.stringify(result)).not.toContain('/home/me')
    expect(result.index.state).toBe('ready')
  })

  it('falls back to the rule id for unknown title keys', async () => {
    setCodeIntelSummaryPort(
      fakePort({
        getFindings: async () => ({
          findings: [
            {
              findingKey: 'k',
              rule: 'r.x',
              kind: 'k',
              severity: 'info',
              titleKey: 'nope',
              origin: 'unknown'
            }
          ]
        })
      })
    )
    const result = (await method.handler(params, ctx)) as ReviewSummaryResult
    expect(result.findings.items[0].title).toBe('r.x')
  })

  it.each([
    ['CODEINTEL_DISABLED', { available: false, reason: 'flag_off' }],
    ['CODEINTEL_NO_BINDING', { available: false, reason: 'no_binding' }],
    ['CODEINTEL_REPO_NOT_REGISTERED', { available: false, reason: 'no_binding' }],
    ['CODEINTEL_INDEX_MISSING', { available: false, reason: 'index_missing' }],
    ['CODEINTEL_TOOL_UNAVAILABLE', { available: false, reason: 'tool_unavailable' }],
    ['CODEINTEL_UNAVAILABLE', { available: false }]
  ])('maps %s to an unavailable payload', async (code, expected) => {
    setCodeIntelSummaryPort(
      fakePort({
        getOverlay: async () => {
          throw new CodeIntelPortError(code)
        }
      })
    )
    expect(await method.handler(params, ctx)).toEqual(expected)
  })

  it('rethrows other errors', async () => {
    setCodeIntelSummaryPort(
      fakePort({
        getStatus: async () => {
          throw new CodeIntelPortError('CODEINTEL_TIMEOUT')
        }
      })
    )
    await expect(method.handler(params, ctx)).rejects.toThrow('CODEINTEL_TIMEOUT')
  })

  it('default port yields available:false', async () => {
    expect(await method.handler(params, ctx)).toEqual({ available: false })
  })

  it('rejects invalid params', () => {
    const schema = method.params!
    expect(schema.safeParse({ worktree: '' }).success).toBe(false)
    expect(schema.safeParse({ worktree: 'x'.repeat(513) }).success).toBe(false)
    expect(schema.safeParse({ worktree: 'id:a', scope: 'all' }).success).toBe(false)
    expect(schema.safeParse(params).success).toBe(true)
  })
})

describe('mobile allowlist for codeIntel', () => {
  it('allows only codeIntel.reviewSummary', () => {
    const source = readFileSync(join(process.cwd(), 'src/main/runtime/runtime-rpc.ts'), 'utf8')
    const block = source.match(/const MOBILE_RPC_METHOD_ALLOWLIST = new Set\(\[([\s\S]*?)\]\)/)![1]!
    const allowed = [...block.matchAll(/'([^']+)'/g)].map((m) => m[1]!)
    expect(allowed.filter((name) => name.startsWith('codeIntel.'))).toEqual([
      'codeIntel.reviewSummary'
    ])
  })
})
