/**
 * create-request-from-issue.ts — CR-REQ-019-06
 *
 * Maps a Jira / GitHub issue to `request.create` params. Only providers the
 * gateway accepts as external sources are emitted (jira|github); anything
 * else is a manual request without `source` (REQUEST_SOURCE_FORBIDDEN).
 *
 * @module components/request/create-request-from-issue
 */

import type { JiraIssue } from '../../../../shared/jira-types'
import type { GitHubWorkItem } from '../../../../shared/types'
import type { RequestSourceProvider } from '../../../../shared/request-types'

export const REQUEST_TITLE_MAX = 500
export const REQUEST_BODY_MAX = 100_000

export type CreateRequestSource = {
  provider: Extract<RequestSourceProvider, 'jira' | 'github' | 'gitlab' | 'linear'>
  ref: string
  url: string
  site?: string
}

export type CreateRequestParams = {
  projectId: string
  title: string
  body?: string
  source?: CreateRequestSource
  hints?: { priority?: string }
  clientRequestId?: string
}

function clamp(text: string, max: number): string {
  const chars = [...text]
  return chars.length <= max ? text : chars.slice(0, max).join('')
}

export function jiraIssueToCreateParams(issue: JiraIssue, projectId: string): CreateRequestParams {
  const description = issue.description?.trim()
  return {
    projectId,
    title: clamp(issue.title, REQUEST_TITLE_MAX),
    ...(description ? { body: clamp(description, REQUEST_BODY_MAX) } : {}),
    source: { provider: 'jira', ref: issue.key, url: issue.url, ...(issue.siteId ? { site: issue.siteId } : {}) }
  }
}

export type GitHubRepoIdentity = { owner: string; repo: string }

/** Returns null for pull requests: reviews do not enter the request flow. */
export function githubItemToCreateParams(
  item: Pick<GitHubWorkItem, 'type' | 'number' | 'title' | 'url'> & { body?: string },
  repoIdentity: GitHubRepoIdentity,
  projectId: string
): CreateRequestParams | null {
  if (item.type === 'pr') {return null}
  const body = item.body?.trim()
  return {
    projectId,
    title: clamp(item.title, REQUEST_TITLE_MAX),
    ...(body ? { body: clamp(body, REQUEST_BODY_MAX) } : {}),
    source: { provider: 'github', ref: `${repoIdentity.owner}/${repoIdentity.repo}#${item.number}`, url: item.url }
  }
}
