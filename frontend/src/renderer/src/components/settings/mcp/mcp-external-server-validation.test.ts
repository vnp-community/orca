import { describe, expect, it } from 'vitest'
import {
  buildUpsertBody,
  editNeedsReReview,
  emptyServerDraft,
  externalServerErrorText,
  looksLikeSecretArg,
  validateServerDraft,
  validateServerUrl
} from './mcp-external-server-validation'

describe('validateServerUrl', () => {
  it('accepts https host names and rejects http, userinfo and IP literals', () => {
    expect(validateServerUrl('https://mcp.example.com/mcp')).toBeNull()
    expect(validateServerUrl('http://mcp.example.com')).not.toBeNull()
    expect(validateServerUrl('https://u:p@mcp.example.com')).not.toBeNull()
    expect(validateServerUrl('https://169.254.169.254/')).not.toBeNull()
    expect(validateServerUrl('nope')).not.toBeNull()
  })
})

describe('validateServerDraft', () => {
  it('checks name, team scope, command and ref names', () => {
    const d = { ...emptyServerDraft(true), name: 'Bad Name', scope: 'team' as const }
    expect(validateServerDraft(d).name).toBeTruthy()
    expect(validateServerDraft(d).team).toBeTruthy()
    const s = {
      ...emptyServerDraft(false),
      name: 'ok',
      transport: 'stdio' as const,
      command: 'a b'
    }
    expect(validateServerDraft(s).command).toBeTruthy()
    expect(
      validateServerDraft({ ...s, command: 'npx', envNames: ['A', 'a'] }).envNames
    ).toBeTruthy()
    expect(validateServerDraft({ ...s, command: 'npx', envNames: ['API_KEY'] })).toEqual({})
  })
})

describe('buildUpsertBody', () => {
  it('sends names only, never hasSecret/status/digest, and scopeId only for team', () => {
    const body = buildUpsertBody({
      ...emptyServerDraft(false),
      name: 'srv',
      url: ' https://x.example.com/mcp ',
      headerNames: ['Authorization'],
      scopeId: 'ignored'
    })
    expect(body).toEqual({
      name: 'srv',
      scope: 'user',
      transport: 'http',
      url: 'https://x.example.com/mcp',
      headerRefs: [{ name: 'Authorization' }],
      envRefs: []
    })
    const team = buildUpsertBody({
      ...emptyServerDraft(true),
      name: 't',
      scope: 'team',
      scopeId: 'T1',
      url: 'https://a.b'
    })
    expect(team.scopeId).toBe('T1')
  })
})

describe('helpers', () => {
  it('flags secret-looking arguments', () => {
    expect(looksLikeSecretArg('sk-abcdefghijklmnopqrstuv')).toBe(true)
    expect(looksLikeSecretArg('--verbose')).toBe(false)
  })
  it('detects edits that need re-review', () => {
    const a = { ...emptyServerDraft(false), url: 'https://a.b' }
    expect(editNeedsReReview(a, { ...a })).toBe(false)
    expect(editNeedsReReview(a, { ...a, url: 'https://c.d' })).toBe(true)
  })
  it('maps known codes to fixed copy and unknown to the server detail', () => {
    expect(externalServerErrorText('MCP_SERVER_SSRF_BLOCKED', 'x')).toContain("isn't allowed")
    expect(externalServerErrorText('MCP_SERVER_NAME_CONFLICT', 'x')).toContain('already exists')
    expect(externalServerErrorText('MCP_SERVER_STDIO_NOT_ALLOWED', 'x')).toContain('stdio')
    expect(externalServerErrorText('MCP_SERVER_NOT_APPROVED', 'x')).toContain('not approved')
    expect(externalServerErrorText('MCP_NOT_ADMIN', 'x')).toContain('admin')
    expect(externalServerErrorText('MCP_SERVER_INVALID', 'bad url')).toContain('bad url')
    expect(externalServerErrorText(null, 'raw detail')).toBe('raw detail')
  })
})
