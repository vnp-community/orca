import { describe, expect, it } from 'vitest'
import type { McpToolView } from '../../../../../shared/mcp-types'
import { buildUpsertPayload, expandMatch, validatePolicyDraft } from './mcp-policy-form'
import { reasonLabel } from './mcp-policy-reasons'
import { diffSettings, validateSettings } from './mcp-settings-form'

const tool = (name: string, over: Partial<McpToolView> = {}): McpToolView =>
  ({
    name,
    namespace: 'terminal',
    risk: 'read',
    hardDenied: false,
    ...over
  }) as McpToolView

const tools = [
  tool('a'),
  tool('run', { risk: 'exec' }),
  tool('wipe', { risk: 'destructive', namespace: 'files' }),
  tool('creds', { hardDenied: true, risk: 'admin', namespace: 'credentials' })
]

describe('expandMatch', () => {
  it('narrows by tool, namespace and risk only', () => {
    expect(expandMatch({ namespace: 'terminal' }, tools).map((t) => t.name)).toEqual(['a', 'run'])
    expect(expandMatch({ risk: 'exec', clientId: 'x' }, tools).map((t) => t.name)).toEqual(['run'])
    expect(expandMatch({ tool: 'a' }, tools)).toHaveLength(1)
  })
})

describe('validatePolicyDraft', () => {
  it('rejects a match without any dimension', () => {
    expect(validatePolicyDraft({ match: {}, decision: 'deny' }, tools).errors).toEqual([
      { kind: 'no_dimension' }
    ])
  })
  it('blocks allow/require_approval on hard-denied tools but permits deny', () => {
    const allow = validatePolicyDraft({ match: { tool: 'creds' }, decision: 'allow' }, tools)
    expect(allow.errors).toContainEqual({ kind: 'hard_deny', tools: ['creds'] })
    expect(
      validatePolicyDraft({ match: { tool: 'creds' }, decision: 'require_approval' }, tools).errors
    ).toContainEqual({ kind: 'hard_deny', tools: ['creds'] })
    expect(
      validatePolicyDraft({ match: { tool: 'creds' }, decision: 'deny' }, tools).errors
    ).toEqual([])
  })
  it('requires an exact tool to allow exec/destructive', () => {
    const broad = validatePolicyDraft({ match: { namespace: 'files' }, decision: 'allow' }, tools)
    expect(broad.errors).toContainEqual({ kind: 'exact_tool_required', risks: ['destructive'] })
    expect(
      validatePolicyDraft({ match: { tool: 'run' }, decision: 'allow' }, tools).errors
    ).toEqual([])
  })
})

describe('buildUpsertPayload', () => {
  it('drops empty dimensions and keeps id+version only when editing', () => {
    const p = buildUpsertPayload({
      match: { tool: ' ', namespace: '' as never, roles: [], clientId: 'c' },
      decision: 'deny',
      note: '  '
    })
    expect(p).toEqual({ match: { clientId: 'c' }, decision: 'deny' })
    const e = buildUpsertPayload({ id: 'p1', version: 3, match: { tool: 't' }, decision: 'allow' })
    expect(e).toMatchObject({ id: 'p1', version: 3 })
  })
})

describe('reasonLabel', () => {
  it('maps known codes and keeps unknown ones raw', () => {
    expect(reasonLabel('hard_deny').raw).toBe('hard_deny')
    expect(reasonLabel('policy:abc:deny').text).toContain('abc')
    expect(reasonLabel('weird_code')).toEqual({ text: 'weird_code', raw: 'weird_code' })
  })
})

describe('settings form', () => {
  const base = { enabled: true, dcrEnabled: false, maxTokenDays: 30, approvalTtlSeconds: 120 }
  it('diffs only changed fields', () => {
    expect(diffSettings(base, { ...base, dcrEnabled: true })).toEqual({ dcrEnabled: true })
  })
  it('enforces 1-90 days and 30-900 seconds', () => {
    expect(validateSettings(base)).toEqual({})
    expect(validateSettings({ ...base, maxTokenDays: 91 }).maxTokenDays).toBeTruthy()
    expect(validateSettings({ ...base, maxTokenDays: 0 }).maxTokenDays).toBeTruthy()
    expect(validateSettings({ ...base, approvalTtlSeconds: 29 }).approvalTtlSeconds).toBeTruthy()
    expect(validateSettings({ ...base, approvalTtlSeconds: 901 }).approvalTtlSeconds).toBeTruthy()
    expect(validateSettings({ ...base, approvalTtlSeconds: 1.5 }).approvalTtlSeconds).toBeTruthy()
  })
})
