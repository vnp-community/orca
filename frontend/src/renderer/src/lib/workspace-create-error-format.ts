import { translate } from '@/i18n/i18n'
export type WorkspaceCreateErrorDisplay = {
  title: string
  message: string
  help?: string
  /** 'not-a-git-repo' drives WorktreeCreationPanel's "Initialize as Git repo"
   *  action — kept a discriminant here (not string-matched again downstream)
   *  so the detection signature lives in exactly one place. The host-* kinds
   *  come from backend host-binding codes (SOL-FE-E2E-004): 'host-unbound'
   *  offers "Choose host"; the other two are retryable infrastructure states. */
  kind?: WorkspaceCreateErrorKind
}

export type WorkspaceCreateErrorKind =
  | 'not-a-git-repo'
  | 'host-unbound'
  | 'host-unreachable'
  | 'host-check-failed'

const MISSING_BASE_REF_ANCHOR = 'could not resolve a default base ref'

// Why these three substrings: WORKTREE_DETECT_FAILED (git-gateway-service's
// `git worktree list` failure, detect_worktrees.go) and the raw git/agent
// error text both surface for a repo whose folder exists but was never
// `git init`'d — seen live as WORKTREE_CREATE_FAILED wrapping
// INFRA_AGENT_EXEC_FAILED wrapping a bare "not a git repository" from the
// agent's own spawned git process. Matching all three keeps this robust to
// which layer's wrapping happens to be on top for a given failure path.
// Host-binding codes checked before NOT_A_GIT_REPO_ANCHORS: a repo with no usable host fails with
// these (never with "not a git repository"), and WORKTREE_DETECT_FAILED may wrap them.
const HOST_UNBOUND_ANCHORS = [
  'gitgateway_repo_host_unbound',
  'repo_host_unbound',
  'project_repo_host_required',
  'gitgateway_missing_dev_server_id'
]
const HOST_UNREACHABLE_ANCHORS = [
  'gitgateway_dev_server_unreachable',
  'dev_server_unreachable',
  'infra_dev_server_not_connected'
]
const HOST_CHECK_FAILED_ANCHORS = ['gitgateway_host_check_failed']

const NOT_A_GIT_REPO_ANCHORS = [
  'not a git repository',
  'not a valid git repository',
  'worktree_detect_failed'
]

export function formatWorkspaceCreateError(error: unknown): WorkspaceCreateErrorDisplay {
  const message = error instanceof Error ? error.message : 'Failed to create worktree.'
  const lower = message.toLowerCase()

  if (lower.includes(MISSING_BASE_REF_ANCHOR)) {
    return {
      title: translate('auto.lib.workspace.create.error.format.64555d0014', 'No base branch found'),
      message: translate(
        'auto.lib.workspace.create.error.format.37cf0bc991',
        'Orca could not resolve a usable base ref for this workspace.'
      ),
      help: 'Create an initial commit (for example on main), or select an existing branch in Create From, then try again.'
    }
  }

  if (HOST_UNBOUND_ANCHORS.some((anchor) => lower.includes(anchor))) {
    return {
      title: translate(
        'auto.lib.workspace.create.error.format.hostUnbound.title',
        'This project isn’t assigned to a dev server'
      ),
      message: translate(
        'auto.lib.workspace.create.error.format.hostUnbound.message',
        'Choose the dev server this project lives on, then try again.'
      ),
      kind: 'host-unbound'
    }
  }

  if (HOST_UNREACHABLE_ANCHORS.some((anchor) => lower.includes(anchor))) {
    return {
      title: translate(
        'auto.lib.workspace.create.error.format.hostUnreachable.title',
        'The dev server is offline'
      ),
      message: translate(
        'auto.lib.workspace.create.error.format.hostUnreachable.message',
        'Orca can’t reach the dev server this project lives on. Reconnect it, then retry.'
      ),
      kind: 'host-unreachable'
    }
  }

  if (HOST_CHECK_FAILED_ANCHORS.some((anchor) => lower.includes(anchor))) {
    return {
      title: translate(
        'auto.lib.workspace.create.error.format.hostCheckFailed.title',
        'Couldn’t check the dev server'
      ),
      message: translate(
        'auto.lib.workspace.create.error.format.hostCheckFailed.message',
        'Orca couldn’t confirm the dev server is online. Retry in a moment.'
      ),
      kind: 'host-check-failed'
    }
  }

  if (NOT_A_GIT_REPO_ANCHORS.some((anchor) => lower.includes(anchor))) {
    return {
      title: translate(
        'auto.lib.workspace.create.error.format.notAGitRepo.title',
        'This folder isn’t a Git repository yet'
      ),
      message: translate(
        'auto.lib.workspace.create.error.format.notAGitRepo.message',
        'Orca can run git init here for you, optionally attaching a remote.'
      ),
      kind: 'not-a-git-repo'
    }
  }

  return {
    title: message,
    message
  }
}

export function getWorkspaceCreateErrorToastMessage(error: WorkspaceCreateErrorDisplay): string {
  return error.help ? error.title : error.message
}
