// One project/repo/worktree the web SPA can open, so Review entry points have a real selector.
// The fake code-intel backend does not check selector values, so these ids are free-form.
export const E2E_PROJECT_ID = 'project-1'
export const E2E_REPO_ID = 'repo-1'
export const E2E_WORKTREE_PATH = '/srv/e2e/demo'
export const E2E_WORKTREE_ID = `${E2E_REPO_ID}::${E2E_WORKTREE_PATH}`
export const E2E_WORKTREE_NAME = 'review-demo'

const worktree = {
  id: E2E_WORKTREE_ID,
  repoId: E2E_REPO_ID,
  projectId: E2E_PROJECT_ID,
  // Why: a worktree without a runtime host resolves environmentId null, and the web bridge's
  // local leg answers method_not_found, so the Review flag would read 'unsupported'.
  hostId: 'runtime:session-auth',
  displayName: E2E_WORKTREE_NAME,
  comment: '',
  linkedIssue: null,
  linkedPR: null,
  linkedLinearIssue: null,
  isArchived: false,
  isUnread: false,
  isPinned: false,
  sortOrder: 0,
  lastActivityAt: 1,
  path: E2E_WORKTREE_PATH,
  head: '0000000000000000000000000000000000000001',
  branch: 'refs/heads/feature/review-demo',
  isBare: false,
  isMainWorktree: false,
  // Why: Review resolves its default scope from the pinned base before git summary loads.
  baseRef: 'main'
}

const detected = { ...worktree, ownership: 'orca-managed', selectedCheckout: false, visible: true }

/** Workspace channels answered on top of the mcp-web boot stubs (which list no project). */
export const WORKSPACE_CHANNEL_STUBS: Record<string, unknown> = {
  'project.list': [{ id: E2E_PROJECT_ID, createdAt: 1 }],
  'repo.list': {
    repos: [
      {
        id: E2E_REPO_ID,
        projectId: E2E_PROJECT_ID,
        url: E2E_WORKTREE_PATH,
        displayName: 'demo',
        position: 0
      }
    ]
  },
  'worktree.list': { worktrees: [worktree] },
  // Source Control and the right-sidebar Review summary resolve the default scope from these.
  'git.status': {
    entries: [],
    conflictOperation: 'unknown',
    branch: 'refs/heads/feature/review-demo'
  },
  'git.branchCompare': {
    summary: {
      baseRef: 'main',
      baseOid: '0000000000000000000000000000000000000000',
      compareRef: 'feature/review-demo',
      headOid: '0000000000000000000000000000000000000001',
      mergeBase: '0000000000000000000000000000000000000000',
      changedFiles: 3,
      commitsAhead: 1,
      status: 'ready'
    },
    entries: []
  },
  'worktree.detectedList': {
    repoId: E2E_REPO_ID,
    authoritative: true,
    source: 'git',
    worktrees: [detected]
  }
}
