/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
pre-existing GitHub/GitLab issue-tracking method block (~1,040 lines
verbatim, already covered by orca-runtime.ts's own grandfathered max-lines
disable before this move). Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-issue-tracking.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-042): GitHub/GitLab issue-tracking +
// hosted-review command wrappers extracted from OrcaRuntimeService via the
// composition pattern. Jira wrapper methods turned out to live elsewhere in
// the class (not in this scoped block, despite Jira imports sitting
// textually adjacent at the top of orca-runtime.ts) — left for a follow-on
// task rather than widening this move's scope mid-extraction. Repo
// hooks/setup-script inspection (textually adjacent too) is also out of
// scope, same reasoning.
import type { GitHubCreateIssueFields, GitHubOwnerRepo, Repo } from '../../shared/types'
import type { StatsCollector } from '../stats/collector'
import type { RuntimeStore } from './orca-runtime'
import {
  getPRForBranch,
  getWorkItem,
  listIssues as listGitHubIssues,
  countWorkItems,
  getPRChecks,
  getPRCheckDetails,
  rerunPRChecks,
  getPRComments,
  getIssue,
  resolveReviewThread,
  setPRFileViewed,
  getWorkItemByOwnerRepo,
  updatePRTitle,
  updatePRDetails,
  mergePR,
  setPRAutoMerge,
  updatePRState,
  requestPRReviewers,
  removePRReviewers,
  createIssue,
  updateIssue,
  addIssueComment,
  addPRReviewComment,
  addPRReviewCommentReply,
  listLabels,
  listAssignableUsers
} from '../github/client'
import type { GitHubPRBranchLookupOptions } from '../github/client'
import { getWorkItemDetails, getPRFileContents } from '../github/work-item-details'
import { getRateLimit } from '../github/rate-limit'
import {
  closeMR as closeGitLabMR,
  createIssue as createGitLabIssue,
  diagnoseAuth as diagnoseGitLabAuthClient,
  getJobTrace as getGitLabJobTrace,
  getRateLimit as getGitLabRateLimit,
  getWorkItemByProjectRef as getGitLabWorkItemByProjectRef,
  addIssueComment as addGitLabIssueComment,
  addMRInlineComment as addGitLabMRInlineComment,
  addMRComment as addGitLabMRComment,
  listTodos as listGitLabTodos,
  listIssues as listGitLabIssues,
  listLabels as listGitLabLabels,
  listMergeRequests as listGitLabMergeRequests,
  listWorkItems as listGitLabWorkItems,
  mergeMR as mergeGitLabMR,
  reopenMR as reopenGitLabMR,
  resolveMRDiscussion as resolveGitLabMRDiscussion,
  retryJob as retryGitLabJob,
  updateMR as updateGitLabMR,
  updateMRReviewers as updateGitLabMRReviewers,
  updateIssue as updateGitLabIssue
} from '../gitlab/client'
import { getWorkItemDetails as getGitLabWorkItemDetails } from '../gitlab/work-item-details'
import {
  normalizeGitLabIssueListArgs,
  normalizeGitLabMRListState,
  normalizeGitLabPositiveInteger,
  type GitLabIssueListState
} from '../gitlab/gitlab-preload-args'
import { recordGitLabProjectRecent } from '../gitlab/gitlab-project-recents'
import type {
  GitHubIssueUpdate,
  GitHubPullRequestStateUpdate,
  GitHubPRFile,
  GitHubPRReviewCommentInput,
  GitLabIssueUpdate,
  GitLabMRInlineCommentInput,
  GitLabProjectRef,
  GitLabWorkItem,
  MRListState
} from '../../shared/types'
import type {
  CreateHostedReviewInput,
  CreateHostedReviewResult,
  HostedReviewCreationEligibility,
  HostedReviewCreationEligibilityArgs,
  HostedReviewInfo
} from '../../shared/hosted-review'
import { getHostedReviewForBranch as getHostedReviewForBranchFromRepo } from '../source-control/hosted-review'
import {
  createHostedReview as createHostedReviewFromRepo,
  getHostedReviewCreationEligibility as getHostedReviewCreationEligibilityFromRepo
} from '../source-control/hosted-review-creation'
import {
  clearProjectItemFieldValue,
  getProjectViewTable,
  getWorkItemDetailsBySlug,
  listAccessibleProjects,
  listProjectViews,
  resolveProjectRef,
  addIssueCommentBySlug,
  deleteIssueCommentBySlug,
  listAssignableUsersBySlug,
  listIssueTypesBySlug,
  listLabelsBySlug,
  updateIssueCommentBySlug,
  updateIssueBySlug,
  updateIssueTypeBySlug,
  updateProjectItemFieldValue,
  updatePullRequestBySlug
} from '../github/project-view'
import type {
  ClearProjectItemFieldArgs,
  GetProjectViewTableArgs,
  ListAssignableUsersBySlugArgs,
  ListIssueTypesBySlugArgs,
  ListLabelsBySlugArgs,
  ListProjectViewsArgs,
  ProjectWorkItemDetailsBySlugArgs,
  ResolveProjectRefArgs,
  AddIssueCommentBySlugArgs,
  DeleteIssueCommentBySlugArgs,
  UpdateIssueBySlugArgs,
  UpdateIssueCommentBySlugArgs,
  UpdateIssueTypeBySlugArgs,
  UpdateProjectItemFieldArgs,
  UpdatePullRequestBySlugArgs
} from '../../shared/github-project-types'

export type RuntimeIssueTrackingCommandHost = {
  getStore(): RuntimeStore | null
  getStats(): StatsCollector | null
  resolveRepoSelector(selector: string): Promise<Repo>
  resolveWorktreeSelector(selector: string): Promise<{ id: string; repoId: string; path: string }>
  getLocalGitExecutionOptionArgs(repo: Repo): [] | [{ wslDistro?: string }]
  getHostedReviewExecutionOptions(
    repo: Repo
  ): { localGitExecOptions: { wslDistro?: string } } | undefined
}

export class RuntimeIssueTrackingCommands {
  constructor(private readonly host: RuntimeIssueTrackingCommandHost) {}

  private async resolveHostedReviewTarget(args: {
    repoSelector: string
    worktreeSelector?: string
  }): Promise<{ repo: Repo; repoPath: string }> {
    const repo = await this.host.resolveRepoSelector(args.repoSelector)
    if (!args.worktreeSelector) {
      return { repo, repoPath: repo.path }
    }

    const worktree = await this.host.resolveWorktreeSelector(args.worktreeSelector)
    if (worktree.repoId !== repo.id) {
      throw new Error('Access denied: worktree does not belong to repository')
    }
    return { repo, repoPath: worktree.path }
  }

  async listRepoIssues(
    repoSelector: string,
    limit?: number
  ): Promise<Awaited<ReturnType<typeof listGitHubIssues>>['items']> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const result = await listGitHubIssues(
      repo.path,
      limit,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
    return result.items
  }

  async getRepoWorkItem(
    repoSelector: string,
    number: number,
    type?: 'issue' | 'pr'
  ): Promise<Awaited<ReturnType<typeof getWorkItem>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getWorkItem(
      repo.path,
      number,
      type,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoWorkItemByOwnerRepo(
    repoSelector: string,
    ownerRepo: { owner: string; repo: string },
    number: number,
    type: 'issue' | 'pr'
  ): Promise<Awaited<ReturnType<typeof getWorkItemByOwnerRepo>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getWorkItemByOwnerRepo(
      repo.path,
      ownerRepo,
      number,
      type,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoWorkItemDetails(
    repoSelector: string,
    number: number,
    type?: 'issue' | 'pr'
  ): Promise<Awaited<ReturnType<typeof getWorkItemDetails>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getWorkItemDetails(
      repo.path,
      number,
      type,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async countRepoWorkItems(repoSelector: string, query?: string): Promise<number> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return countWorkItems(
      repo.path,
      query,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async listRepoLabels(repoSelector: string): Promise<Awaited<ReturnType<typeof listLabels>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listLabels(
      repo.path,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async listRepoAssignableUsers(
    repoSelector: string
  ): Promise<Awaited<ReturnType<typeof listAssignableUsers>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listAssignableUsers(
      repo.path,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  getGitHubRateLimit(options?: {
    force?: boolean
  }): Promise<Awaited<ReturnType<typeof getRateLimit>>> {
    return getRateLimit(options)
  }

  async getRepoPRForBranch(
    repoSelector: string,
    branch: string,
    linkedPRNumber?: number | null,
    fallbackPRNumber?: number | null,
    acceptMergedFallbackPR?: boolean,
    currentHeadOid?: string | null
  ): Promise<Awaited<ReturnType<typeof getPRForBranch>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const options: GitHubPRBranchLookupOptions =
      this.host.getHostedReviewExecutionOptions(repo) ?? {}
    const lookupOptions = { ...options }
    if (acceptMergedFallbackPR === true) {
      lookupOptions.acceptMergedFallbackPR = true
    }
    if (typeof currentHeadOid === 'string' && currentHeadOid.trim().length > 0) {
      lookupOptions.currentHeadOid = currentHeadOid.trim()
    }
    const lookupOptionArgs: [] | [GitHubPRBranchLookupOptions] =
      Object.keys(lookupOptions).length > 0 ? [lookupOptions] : []
    return getPRForBranch(
      repo.path,
      branch,
      linkedPRNumber ?? null,
      repo.connectionId ?? null,
      linkedPRNumber == null ? (fallbackPRNumber ?? null) : null,
      ...lookupOptionArgs
    )
  }

  async getHostedReviewForBranch(args: {
    repoSelector: string
    branch: string
    currentHeadOid?: string | null
    linkedGitHubPR?: number | null
    fallbackGitHubPR?: number | null
    linkedGitLabMR?: number | null
    linkedBitbucketPR?: number | null
    linkedAzureDevOpsPR?: number | null
    linkedGiteaPR?: number | null
  }): Promise<HostedReviewInfo | null> {
    const repo = await this.host.resolveRepoSelector(args.repoSelector)
    const executionOptions = this.host.getHostedReviewExecutionOptions(repo)
    const review = await getHostedReviewForBranchFromRepo({
      repoPath: repo.path,
      connectionId: repo.connectionId ?? null,
      branch: args.branch,
      currentHeadOid: args.currentHeadOid ?? null,
      linkedGitHubPR: args.linkedGitHubPR ?? null,
      fallbackGitHubPR: args.linkedGitHubPR == null ? (args.fallbackGitHubPR ?? null) : null,
      linkedGitLabMR: args.linkedGitLabMR ?? null,
      linkedBitbucketPR: args.linkedBitbucketPR ?? null,
      linkedAzureDevOpsPR: args.linkedAzureDevOpsPR ?? null,
      linkedGiteaPR: args.linkedGiteaPR ?? null,
      ...executionOptions
    })
    const statsA = this.host.getStats()
    if (review?.provider === 'github' && statsA && !statsA.hasCountedPR(review.url)) {
      statsA.record({
        type: 'pr_created',
        at: Date.now(),
        repoId: repo.id,
        meta: { prNumber: review.number, prUrl: review.url }
      })
    }
    return review
  }

  async getHostedReviewCreationEligibility(
    args: Omit<HostedReviewCreationEligibilityArgs, 'repoPath'> & {
      repoSelector: string
      worktreeSelector?: string
    }
  ): Promise<HostedReviewCreationEligibility> {
    const { repo, repoPath } = await this.resolveHostedReviewTarget(args)
    const executionOptions = this.host.getHostedReviewExecutionOptions(repo)
    return getHostedReviewCreationEligibilityFromRepo({
      repoPath,
      connectionId: repo.connectionId ?? null,
      branch: args.branch,
      base: args.base ?? null,
      hasUncommittedChanges: args.hasUncommittedChanges,
      hasUpstream: args.hasUpstream,
      ahead: args.ahead,
      behind: args.behind,
      linkedGitHubPR: args.linkedGitHubPR ?? null,
      fallbackGitHubPR: args.linkedGitHubPR == null ? (args.fallbackGitHubPR ?? null) : null,
      linkedGitLabMR: args.linkedGitLabMR ?? null,
      linkedBitbucketPR: args.linkedBitbucketPR ?? null,
      linkedAzureDevOpsPR: args.linkedAzureDevOpsPR ?? null,
      linkedGiteaPR: args.linkedGiteaPR ?? null,
      ...executionOptions
    })
  }

  async createHostedReview(
    args: CreateHostedReviewInput & { repoSelector: string; worktreeSelector?: string }
  ): Promise<CreateHostedReviewResult> {
    const { repo, repoPath } = await this.resolveHostedReviewTarget(args)
    const executionOptions = this.host.getHostedReviewExecutionOptions(repo)
    const input = {
      provider: args.provider,
      base: args.base,
      head: args.head,
      title: args.title,
      body: args.body,
      draft: args.draft,
      ...(args.useTemplate !== undefined ? { useTemplate: args.useTemplate } : {})
    }
    const result = executionOptions
      ? await createHostedReviewFromRepo(
          repoPath,
          input,
          repo.connectionId ?? null,
          executionOptions
        )
      : await createHostedReviewFromRepo(repoPath, input, repo.connectionId ?? null)
    const statsB = this.host.getStats()
    if (result.ok && statsB && !statsB.hasCountedPR(result.url)) {
      statsB.record({
        type: 'pr_created',
        at: Date.now(),
        repoId: repo.id,
        meta: { prNumber: result.number, prUrl: result.url }
      })
    }
    return result
  }

  async listGitLabRepoWorkItems(
    repoSelector: string,
    state?: MRListState,
    page?: number,
    perPage?: number,
    query?: string
  ): Promise<Awaited<ReturnType<typeof listGitLabWorkItems>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listGitLabWorkItems(
      repo.path,
      state ?? 'opened',
      page ?? 1,
      perPage ?? 20,
      repo.issueSourcePreference,
      query,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async listGitLabRepoMRs(
    repoSelector: string,
    state?: MRListState,
    page?: number,
    perPage?: number,
    query?: string
  ): Promise<Awaited<ReturnType<typeof listGitLabMergeRequests>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listGitLabMergeRequests(
      repo.path,
      normalizeGitLabMRListState(state),
      normalizeGitLabPositiveInteger(page, 1, 10_000),
      normalizeGitLabPositiveInteger(perPage, 20, 100),
      repo.issueSourcePreference,
      query,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async listGitLabRepoIssues(
    repoSelector: string,
    state?: GitLabIssueListState,
    assignee?: string,
    limit?: number
  ): Promise<{
    items: GitLabWorkItem[]
    error?: Awaited<ReturnType<typeof listGitLabIssues>>['error']
  }> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const normalized = normalizeGitLabIssueListArgs({ state, assignee, limit })
    const result = await listGitLabIssues(
      repo.path,
      normalized.limit,
      repo.issueSourcePreference,
      normalized.state,
      normalized.assignee,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
    // Why: web runtime mirrors the desktop preload contract, where GitLab
    // issue rows share the GitLabWorkItem shape with MRs on TaskPage.
    const items: GitLabWorkItem[] = result.items.map((issue) => ({
      id: `gitlab-issue-${repo.id}-${issue.number}`,
      type: 'issue' as const,
      number: issue.number,
      title: issue.title,
      state: issue.state,
      url: issue.url,
      labels: issue.labels,
      updatedAt: issue.updatedAt ?? '',
      author: issue.author ?? null,
      repoId: repo.id
    }))
    return { items, ...(result.error ? { error: result.error } : {}) }
  }

  async listGitLabRepoTodos(
    repoSelector: string
  ): Promise<Awaited<ReturnType<typeof listGitLabTodos>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listGitLabTodos(
      repo.path,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async diagnoseGitLabAuth(): Promise<Awaited<ReturnType<typeof diagnoseGitLabAuthClient>>> {
    return diagnoseGitLabAuthClient()
  }

  async getGitLabRateLimit(options?: {
    force?: boolean
    host?: string | null
  }): Promise<Awaited<ReturnType<typeof getGitLabRateLimit>>> {
    return getGitLabRateLimit(options)
  }

  async listGitLabRepoLabels(
    repoSelector: string
  ): Promise<Awaited<ReturnType<typeof listGitLabLabels>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return listGitLabLabels(
      repo.path,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async createGitLabRepoIssue(
    repoSelector: string,
    title: string,
    body: string
  ): Promise<Awaited<ReturnType<typeof createGitLabIssue>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return createGitLabIssue(
      repo.path,
      title,
      body,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateGitLabRepoIssue(
    repoSelector: string,
    number: number,
    updates: GitLabIssueUpdate,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof updateGitLabIssue>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updateGitLabIssue(
      repo.path,
      number,
      updates,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async addGitLabRepoIssueComment(
    repoSelector: string,
    number: number,
    body: string,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof addGitLabIssueComment>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addGitLabIssueComment(
      repo.path,
      number,
      body,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async addGitLabRepoMRComment(
    repoSelector: string,
    iid: number,
    body: string,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof addGitLabMRComment>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addGitLabMRComment(
      repo.path,
      iid,
      body,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async addGitLabRepoMRInlineComment(
    repoSelector: string,
    iid: number,
    input: GitLabMRInlineCommentInput,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof addGitLabMRInlineComment>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addGitLabMRInlineComment(
      repo.path,
      iid,
      input,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async resolveGitLabRepoMRDiscussion(
    repoSelector: string,
    iid: number,
    discussionId: string,
    resolved: boolean,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof resolveGitLabMRDiscussion>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return resolveGitLabMRDiscussion(
      repo.path,
      iid,
      discussionId,
      resolved,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getGitLabRepoJobTrace(
    repoSelector: string,
    jobId: number,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof getGitLabJobTrace>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getGitLabJobTrace(
      repo.path,
      jobId,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async retryGitLabRepoJob(
    repoSelector: string,
    jobId: number,
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof retryGitLabJob>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return retryGitLabJob(
      repo.path,
      jobId,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async mergeGitLabRepoMR(
    repoSelector: string,
    iid: number,
    method?: 'merge' | 'squash' | 'rebase',
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof mergeGitLabMR>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return mergeGitLabMR(
      repo.path,
      iid,
      method ?? 'merge',
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateGitLabRepoMRState(
    repoSelector: string,
    iid: number,
    state: 'opened' | 'closed',
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof closeGitLabMR>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return state === 'closed'
      ? closeGitLabMR(
          repo.path,
          iid,
          repo.issueSourcePreference,
          repo.connectionId ?? null,
          projectRef,
          ...this.host.getLocalGitExecutionOptionArgs(repo)
        )
      : reopenGitLabMR(
          repo.path,
          iid,
          repo.issueSourcePreference,
          repo.connectionId ?? null,
          projectRef,
          ...this.host.getLocalGitExecutionOptionArgs(repo)
        )
  }

  async updateGitLabRepoMR(
    repoSelector: string,
    iid: number,
    updates: { title?: string; body?: string; addLabels?: string[]; removeLabels?: string[] },
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof updateGitLabMR>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updateGitLabMR(
      repo.path,
      iid,
      updates,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateGitLabRepoMRReviewers(
    repoSelector: string,
    iid: number,
    reviewerIds: number[],
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof updateGitLabMRReviewers>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updateGitLabMRReviewers(
      repo.path,
      iid,
      reviewerIds,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getGitLabRepoWorkItemDetails(
    repoSelector: string,
    iid: number,
    type: 'issue' | 'mr',
    projectRef?: GitLabProjectRef | null
  ): Promise<Awaited<ReturnType<typeof getGitLabWorkItemDetails>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getGitLabWorkItemDetails(
      repo.path,
      iid,
      type,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      projectRef,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getGitLabRepoWorkItemByPath(
    repoSelector: string,
    projectRef: GitLabProjectRef,
    iid: number,
    type: 'issue' | 'mr'
  ): Promise<Awaited<ReturnType<typeof getGitLabWorkItemByProjectRef>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const result = await getGitLabWorkItemByProjectRef(
      repo.path,
      projectRef,
      iid,
      type,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
    // Why: remote pasted-URL lookups should update GitLab recents exactly
    // like the desktop IPC path, but only after a successful lookup.
    const store = this.host.getStore()
    if (result && store?.updateSettings) {
      recordGitLabProjectRecent(
        {
          getSettings: () => store.getSettings(),
          updateSettings: (updates) => store.updateSettings?.(updates)
        },
        projectRef.host,
        projectRef.path
      )
    }
    return result
  }

  async getRepoIssue(
    repoSelector: string,
    number: number
  ): Promise<Awaited<ReturnType<typeof getIssue>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getIssue(
      repo.path,
      number,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoPRChecks(
    repoSelector: string,
    prNumber: number,
    headSha?: string,
    prRepo?: GitHubOwnerRepo | null,
    options?: { noCache?: boolean }
  ): Promise<Awaited<ReturnType<typeof getPRChecks>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getPRChecks(
      repo.path,
      prNumber,
      headSha,
      prRepo ?? null,
      options,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async rerunRepoPRChecks(
    repoSelector: string,
    prNumber: number,
    options?: { headSha?: string; failedOnly?: boolean }
  ): Promise<Awaited<ReturnType<typeof rerunPRChecks>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return rerunPRChecks(
      repo.path,
      prNumber,
      options,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoPRCheckDetails(
    repoSelector: string,
    args: {
      checkRunId?: number
      workflowRunId?: number
      checkName?: string
      url?: string | null
      prRepo?: GitHubOwnerRepo | null
    }
  ): Promise<Awaited<ReturnType<typeof getPRCheckDetails>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getPRCheckDetails(
      repo.path,
      { ...args, prRepo: args.prRepo ?? null },
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoPRComments(
    repoSelector: string,
    prNumber: number,
    prRepo?: GitHubOwnerRepo | null,
    options?: { noCache?: boolean }
  ): Promise<Awaited<ReturnType<typeof getPRComments>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getPRComments(
      repo.path,
      prNumber,
      { ...options, prRepo: prRepo ?? null },
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async getRepoPRFileContents(
    repoSelector: string,
    args: {
      prNumber: number
      path: string
      oldPath?: string
      status: GitHubPRFile['status']
      headSha: string
      baseSha: string
    }
  ): Promise<Awaited<ReturnType<typeof getPRFileContents>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return getPRFileContents({
      repoPath: repo.path,
      connectionId: repo.connectionId ?? null,
      localGitOptions: this.host.getLocalGitExecutionOptionArgs(repo)[0],
      ...args
    })
  }

  async resolveRepoReviewThread(
    repoSelector: string,
    threadId: string,
    resolve: boolean
  ): Promise<Awaited<ReturnType<typeof resolveReviewThread>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return resolveReviewThread(
      repo.path,
      threadId,
      resolve,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async setRepoPRFileViewed(
    repoSelector: string,
    args: {
      pullRequestId: string
      path: string
      viewed: boolean
    }
  ): Promise<Awaited<ReturnType<typeof setPRFileViewed>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return setPRFileViewed({
      repoPath: repo.path,
      connectionId: repo.connectionId ?? null,
      localGitOptions: this.host.getLocalGitExecutionOptionArgs(repo)[0],
      ...args
    })
  }

  async updateRepoPRTitle(
    repoSelector: string,
    prNumber: number,
    title: string,
    prRepo?: GitHubOwnerRepo | null
  ): Promise<Awaited<ReturnType<typeof updatePRTitle>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updatePRTitle(
      repo.path,
      prNumber,
      title,
      repo.connectionId ?? null,
      prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateRepoPRDetails(
    repoSelector: string,
    prNumber: number,
    updates: { title?: string; body?: string },
    prRepo?: GitHubOwnerRepo | null
  ): Promise<Awaited<ReturnType<typeof updatePRDetails>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updatePRDetails(
      repo.path,
      prNumber,
      updates,
      repo.connectionId ?? null,
      prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async mergeRepoPR(
    repoSelector: string,
    prNumber: number,
    method?: 'merge' | 'squash' | 'rebase',
    prRepo?: GitHubOwnerRepo | null
  ): Promise<Awaited<ReturnType<typeof mergePR>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return mergePR(
      repo.path,
      prNumber,
      method,
      repo.connectionId ?? null,
      prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async setRepoPRAutoMerge(
    repoSelector: string,
    prNumber: number,
    enabled: boolean,
    method?: 'merge' | 'squash' | 'rebase',
    prRepo?: GitHubOwnerRepo | null
  ): Promise<Awaited<ReturnType<typeof setPRAutoMerge>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return setPRAutoMerge(
      repo.path,
      prNumber,
      enabled,
      method,
      repo.connectionId ?? null,
      prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateRepoPRState(
    repoSelector: string,
    prNumber: number,
    updates: GitHubPullRequestStateUpdate
  ): Promise<Awaited<ReturnType<typeof updatePRState>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updatePRState(
      repo.path,
      prNumber,
      updates,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async requestRepoPRReviewers(
    repoSelector: string,
    prNumber: number,
    reviewers: string[]
  ): Promise<Awaited<ReturnType<typeof requestPRReviewers>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return requestPRReviewers(
      repo.path,
      prNumber,
      reviewers,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async removeRepoPRReviewers(
    repoSelector: string,
    prNumber: number,
    reviewers: string[]
  ): Promise<Awaited<ReturnType<typeof removePRReviewers>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return removePRReviewers(
      repo.path,
      prNumber,
      reviewers,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async createRepoIssue(
    repoSelector: string,
    title: string,
    body: string,
    fields?: GitHubCreateIssueFields
  ): Promise<Awaited<ReturnType<typeof createIssue>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return createIssue(
      repo.path,
      title,
      body,
      repo.issueSourcePreference,
      repo.connectionId ?? null,
      fields,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async updateRepoIssue(
    repoSelector: string,
    number: number,
    updates: GitHubIssueUpdate
  ): Promise<Awaited<ReturnType<typeof updateIssue>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return updateIssue(
      repo.path,
      number,
      updates,
      repo.connectionId ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async addRepoIssueComment(
    repoSelector: string,
    number: number,
    body: string,
    prRepo?: GitHubOwnerRepo | null
  ): Promise<Awaited<ReturnType<typeof addIssueComment>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addIssueComment(
      repo.path,
      number,
      body,
      repo.connectionId ?? null,
      prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async addRepoPRReviewComment(
    repoSelector: string,
    args: Omit<GitHubPRReviewCommentInput, 'repoPath'>
  ): Promise<Awaited<ReturnType<typeof addPRReviewComment>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addPRReviewComment({
      repoPath: repo.path,
      connectionId: repo.connectionId ?? null,
      localGitOptions: this.host.getLocalGitExecutionOptionArgs(repo)[0],
      ...args
    })
  }

  async addRepoPRReviewCommentReply(
    repoSelector: string,
    args: {
      prNumber: number
      commentId: number
      body: string
      threadId?: string
      path?: string
      line?: number
      prRepo?: GitHubOwnerRepo | null
    }
  ): Promise<Awaited<ReturnType<typeof addPRReviewCommentReply>>> {
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return addPRReviewCommentReply(
      repo.path,
      args.prNumber,
      args.commentId,
      args.body,
      args.threadId,
      args.path,
      args.line,
      repo.connectionId ?? null,
      args.prRepo ?? null,
      ...this.host.getLocalGitExecutionOptionArgs(repo)
    )
  }

  async listGitHubProjects(): Promise<Awaited<ReturnType<typeof listAccessibleProjects>>> {
    return listAccessibleProjects()
  }

  async listGitHubLabelsBySlug(
    args: ListLabelsBySlugArgs
  ): Promise<Awaited<ReturnType<typeof listLabelsBySlug>>> {
    return listLabelsBySlug(args)
  }

  async listGitHubAssignableUsersBySlug(
    args: ListAssignableUsersBySlugArgs
  ): Promise<Awaited<ReturnType<typeof listAssignableUsersBySlug>>> {
    return listAssignableUsersBySlug(args)
  }

  async listGitHubIssueTypesBySlug(
    args: ListIssueTypesBySlugArgs
  ): Promise<Awaited<ReturnType<typeof listIssueTypesBySlug>>> {
    return listIssueTypesBySlug(args)
  }

  async resolveGitHubProjectRef(
    args: ResolveProjectRefArgs
  ): Promise<Awaited<ReturnType<typeof resolveProjectRef>>> {
    return resolveProjectRef(args)
  }

  async listGitHubProjectViews(
    args: ListProjectViewsArgs
  ): Promise<Awaited<ReturnType<typeof listProjectViews>>> {
    return listProjectViews(args)
  }

  async getGitHubProjectViewTable(
    args: GetProjectViewTableArgs
  ): Promise<Awaited<ReturnType<typeof getProjectViewTable>>> {
    return getProjectViewTable(args)
  }

  async getGitHubProjectWorkItemDetailsBySlug(
    args: ProjectWorkItemDetailsBySlugArgs
  ): Promise<Awaited<ReturnType<typeof getWorkItemDetailsBySlug>>> {
    return getWorkItemDetailsBySlug(args)
  }

  async updateGitHubProjectItemField(
    args: UpdateProjectItemFieldArgs
  ): Promise<Awaited<ReturnType<typeof updateProjectItemFieldValue>>> {
    return updateProjectItemFieldValue(args)
  }

  async clearGitHubProjectItemField(
    args: ClearProjectItemFieldArgs
  ): Promise<Awaited<ReturnType<typeof clearProjectItemFieldValue>>> {
    return clearProjectItemFieldValue(args)
  }

  async updateGitHubIssueBySlug(
    args: UpdateIssueBySlugArgs
  ): Promise<Awaited<ReturnType<typeof updateIssueBySlug>>> {
    return updateIssueBySlug(args)
  }

  async updateGitHubPullRequestBySlug(
    args: UpdatePullRequestBySlugArgs
  ): Promise<Awaited<ReturnType<typeof updatePullRequestBySlug>>> {
    return updatePullRequestBySlug(args)
  }

  async updateGitHubIssueTypeBySlug(
    args: UpdateIssueTypeBySlugArgs
  ): Promise<Awaited<ReturnType<typeof updateIssueTypeBySlug>>> {
    return updateIssueTypeBySlug(args)
  }

  async addGitHubIssueCommentBySlug(
    args: AddIssueCommentBySlugArgs
  ): Promise<Awaited<ReturnType<typeof addIssueCommentBySlug>>> {
    return addIssueCommentBySlug(args)
  }

  async updateGitHubIssueCommentBySlug(
    args: UpdateIssueCommentBySlugArgs
  ): Promise<Awaited<ReturnType<typeof updateIssueCommentBySlug>>> {
    return updateIssueCommentBySlug(args)
  }

  async deleteGitHubIssueCommentBySlug(
    args: DeleteIssueCommentBySlugArgs
  ): Promise<Awaited<ReturnType<typeof deleteIssueCommentBySlug>>> {
    return deleteIssueCommentBySlug(args)
  }
}
