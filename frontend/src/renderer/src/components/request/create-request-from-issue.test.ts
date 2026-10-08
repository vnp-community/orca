import { describe, expect, it } from 'vitest'
import { githubItemToCreateParams, jiraIssueToCreateParams } from './create-request-from-issue'
import type { JiraIssue } from '../../../../shared/jira-types'

const jira = { key: 'ABC-1', siteId: 'site-1', title: 'T', description: '  body  ', url: 'https://j/ABC-1' } as JiraIssue

describe('create-request-from-issue', () => {
  it('maps a Jira issue with key, site and trimmed description', () => {
    expect(jiraIssueToCreateParams(jira, 'p1')).toEqual({
      projectId: 'p1',
      title: 'T',
      body: 'body',
      source: { provider: 'jira', ref: 'ABC-1', url: 'https://j/ABC-1', site: 'site-1' }
    })
  })

  it('omits body for an empty description and site when absent', () => {
    const p = jiraIssueToCreateParams({ ...jira, description: '', siteId: undefined }, 'p1')
    expect(p.body).toBeUndefined()
    expect(p.source?.site).toBeUndefined()
  })

  it('maps a GitHub issue to owner/repo#number', () => {
    const p = githubItemToCreateParams(
      { type: 'issue', number: 7, title: 'Bug', url: 'https://github.com/o/r/issues/7' },
      { owner: 'o', repo: 'r' },
      'p1'
    )
    expect(p?.source).toEqual({ provider: 'github', ref: 'o/r#7', url: 'https://github.com/o/r/issues/7' })
  })

  it('rejects pull requests', () => {
    expect(
      githubItemToCreateParams({ type: 'pr', number: 1, title: 'x', url: 'u' }, { owner: 'o', repo: 'r' }, 'p')
    ).toBeNull()
  })

  it('never emits a provider the gateway forbids (mcp|webhook|manual)', () => {
    for (const p of [jiraIssueToCreateParams(jira, 'p')]) {
      expect(['mcp', 'webhook', 'manual']).not.toContain(p.source?.provider)
    }
  })

  it('clamps over-long titles to 500 characters', () => {
    expect([...jiraIssueToCreateParams({ ...jira, title: 'x'.repeat(600) }, 'p').title]).toHaveLength(500)
  })
})
