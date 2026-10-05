import { describe, expect, it } from 'vitest'
import { getJiraIssueKeyPrefix, matchProjectForJiraIssue } from './jira-project-matching'

const p = (id: string, jiraProjectKey: string, jiraSiteId = '') => ({
  id,
  jiraProjectKey,
  jiraSiteId
})

describe('getJiraIssueKeyPrefix', () => {
  it('extracts the prefix', () => {
    expect(getJiraIssueKeyPrefix('abc-12')).toBe('ABC')
    expect(getJiraIssueKeyPrefix('A1_B-7')).toBe('A1_B')
  })
  it('rejects non-keys', () => {
    expect(getJiraIssueKeyPrefix('nope')).toBeNull()
    expect(getJiraIssueKeyPrefix('')).toBeNull()
    expect(getJiraIssueKeyPrefix(undefined)).toBeNull()
  })
})

describe('matchProjectForJiraIssue', () => {
  it('matches a single project by key', () => {
    expect(matchProjectForJiraIssue([p('a', 'ABC'), p('b', 'XYZ')], 'ABC-1')?.id).toBe('a')
  })
  it('returns null when unmapped or no key', () => {
    expect(matchProjectForJiraIssue([p('a', '')], 'ABC-1')).toBeNull()
    expect(matchProjectForJiraIssue([p('a', 'ABC')], 'bad')).toBeNull()
  })
  it('prefers the site match among several', () => {
    const list = [p('a', 'ABC', 's1'), p('b', 'ABC', 's2')]
    expect(matchProjectForJiraIssue(list, 'ABC-1', 's2')?.id).toBe('b')
  })
  it('does not guess when several match and site is unknown or ties', () => {
    expect(matchProjectForJiraIssue([p('a', 'ABC', 's1'), p('b', 'ABC', 's2')], 'ABC-1')).toBeNull()
    expect(
      matchProjectForJiraIssue([p('a', 'ABC', 's1'), p('b', 'ABC', 's1')], 'ABC-1', 's1')
    ).toBeNull()
  })
  it('ignores projects mapped to another site, accepts site-less ones', () => {
    expect(matchProjectForJiraIssue([p('a', 'ABC', 's1')], 'ABC-1', 's2')).toBeNull()
    expect(matchProjectForJiraIssue([p('a', 'ABC')], 'ABC-1', 's2')?.id).toBe('a')
  })
})
