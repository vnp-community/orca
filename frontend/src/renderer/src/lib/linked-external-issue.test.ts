import { describe, expect, it } from 'vitest'
import { getLinkedExternalIssue } from './linked-external-issue'

describe('getLinkedExternalIssue', () => {
  it('returns the Jira key for a Jira work item', () => {
    expect(
      getLinkedExternalIssue({
        type: 'issue',
        provider: 'jira',
        number: 0,
        title: 'ENG-1 fix',
        url: 'https://x.atlassian.net/browse/ENG-1',
        jiraIdentifier: 'ENG-1'
      })
    ).toEqual({
      provider: 'jira',
      ref: 'ENG-1',
      title: 'ENG-1 fix',
      url: 'https://x.atlassian.net/browse/ENG-1'
    })
  })

  it('ignores a Jira item without an identifier', () => {
    expect(
      getLinkedExternalIssue({
        type: 'issue',
        provider: 'jira',
        number: 0,
        title: 't',
        url: 'https://x.atlassian.net/browse/ENG-1'
      })
    ).toBeUndefined()
  })

  it('does not forward non-Jira providers or a missing item', () => {
    expect(
      getLinkedExternalIssue({
        type: 'issue',
        provider: 'github',
        number: 4,
        title: 't',
        url: 'https://github.com/o/r/issues/4'
      })
    ).toBeUndefined()
    expect(getLinkedExternalIssue(null)).toBeUndefined()
  })
})
