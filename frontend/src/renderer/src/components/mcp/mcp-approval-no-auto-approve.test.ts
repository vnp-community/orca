import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

const read = (p: string): string => readFileSync(p, 'utf8')

describe('approval safety (static)', () => {
  const dir = __dirname
  const sources = readdirSync(dir)
    .filter((f) => /\.tsx?$/.test(f) && !/\.test\./.test(f))
    .map((f) => [f, read(join(dir, f))] as const)

  it("has exactly one decide('approve') call-site, in the Approve button's onClick", () => {
    const hits = sources.filter(([, s]) => /decide\(\s*'approve'\s*\)/.test(s))
    expect(hits.map(([f]) => f)).toEqual(['McpApprovalPrompt.tsx'])
    const prompt = read(join(dir, 'McpApprovalPrompt.tsx'))
    expect(prompt.match(/decide\(\s*'approve'\s*\)/g)).toHaveLength(1)
    expect(prompt).toMatch(/onClick=\{\(e\) => \{[\s\S]*?decide\('approve'\)/)
  })

  it('only the decision hook talks to mcp.approval.decide, and never renders HTML', () => {
    expect(sources.filter(([, s]) => s.includes('mcp.approval.decide')).map(([f]) => f)).toEqual([
      'use-mcp-approval-decision.ts'
    ])
    for (const [, s] of sources) {
      expect(s).not.toContain('dangerouslySetInnerHTML')
      expect(s).not.toMatch(/console\./)
    }
  })
})
