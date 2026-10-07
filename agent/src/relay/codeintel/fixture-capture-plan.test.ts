import { describe, it, expect } from 'vitest'
import { buildCapturePlan } from './fixture-capture-plan'
import { GITNEXUS_VERBS, CODEGRAPH_VERBS } from '../codeintel-command-whitelist'

describe('fixture-capture-plan', () => {
  it('gitnexus: plan generates verbs that are in the whitelist', () => {
    const plan = buildCapturePlan('gitnexus', '1.6.9', '/tmp/registry', '/tmp/repo')
    for (const step of plan) {
      if (step.name === 'check-cycles') {
        expect(step.argv[0]).toBe('check')
      } else {
        expect(GITNEXUS_VERBS).toContain(step.argv[0] as any)
      }
      // No analyze
      expect(step.argv).not.toContain('analyze')
    }
  })

  it('codegraph: plan generates verbs that are in the whitelist', () => {
    const plan = buildCapturePlan('codegraph', '1.4.1', '/tmp/registry', '/tmp/repo')
    for (const step of plan) {
      expect(CODEGRAPH_VERBS).toContain(step.argv[0] as any)
      expect(step.argv).not.toContain('analyze')
    }
  })

  it('rejects unsupported versions', () => {
    expect(() => buildCapturePlan('gitnexus', '1.6.8', '/r', '/repo')).toThrow(/Unsupported gitnexus version/)
    expect(() => buildCapturePlan('codegraph', '1.4.0', '/r', '/repo')).toThrow(/Unsupported codegraph version/)
    expect(() => buildCapturePlan('unknown', '1.0.0', '/r', '/repo')).toThrow(/Unknown tool/)
  })

  it('does not leak real absolute paths in sample argv', () => {
    const plan = buildCapturePlan('gitnexus', '1.6.9', '/fake/registry', '/fake/repo')
    const str = JSON.stringify(plan)
    expect(str).not.toMatch(/\/(home|Users|opt)/)
  })
})
