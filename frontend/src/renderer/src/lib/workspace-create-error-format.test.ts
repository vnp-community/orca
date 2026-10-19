import { describe, expect, it } from 'vitest'
import {
  formatWorkspaceCreateError,
  getWorkspaceCreateErrorToastMessage
} from './workspace-create-error-format'

describe('formatWorkspaceCreateError', () => {
  it('detects the missing-base-ref signature', () => {
    const result = formatWorkspaceCreateError(new Error('could not resolve a default base ref'))
    expect(result.title).toBe('No base branch found')
    expect(result.help).toBeTruthy()
    expect(result.kind).toBeUndefined()
  })

  it('detects a raw "not a git repository" error and sets kind', () => {
    const result = formatWorkspaceCreateError(
      new Error(
        'WORKTREE_CREATE_FAILED: fatal: not a git repository (or any of the parent directories): .git'
      )
    )
    expect(result.kind).toBe('not-a-git-repo')
  })

  it('detects "not a valid git repository" (the legacy desktop message shape)', () => {
    const result = formatWorkspaceCreateError(
      new Error('Not a valid git repository: /opt/aiops-v3')
    )
    expect(result.kind).toBe('not-a-git-repo')
  })

  it('detects WORKTREE_DETECT_FAILED', () => {
    const result = formatWorkspaceCreateError(
      new Error(
        'rpc error: code = Internal desc = WORKTREE_DETECT_FAILED: git worktree list failed'
      )
    )
    expect(result.kind).toBe('not-a-git-repo')
  })

  it('is case-insensitive', () => {
    const result = formatWorkspaceCreateError(new Error('NOT A GIT REPOSITORY'))
    expect(result.kind).toBe('not-a-git-repo')
  })

  it('falls back to the raw message for an unrecognized error', () => {
    const result = formatWorkspaceCreateError(new Error('some other failure'))
    expect(result.title).toBe('some other failure')
    expect(result.message).toBe('some other failure')
    expect(result.kind).toBeUndefined()
  })

  it('falls back to a generic message for a non-Error value', () => {
    const result = formatWorkspaceCreateError('a plain string')
    expect(result.title).toBe('Failed to create worktree.')
  })
})

describe('getWorkspaceCreateErrorToastMessage', () => {
  it('prefers title when help is present', () => {
    expect(
      getWorkspaceCreateErrorToastMessage({ title: 'Title', message: 'Message', help: 'Help' })
    ).toBe('Title')
  })

  it('uses message when there is no help', () => {
    expect(getWorkspaceCreateErrorToastMessage({ title: 'Title', message: 'Message' })).toBe(
      'Message'
    )
  })

  // SOL-FE-E2E-004: backend host-binding codes get their own kind and a useful explanation
  // instead of "Couldn't create worktree: rpc error ...".
  describe('host-binding codes', () => {
    it.each([
      ['GITGATEWAY_REPO_HOST_UNBOUND: repo has no dev server'],
      ['rpc error: code = FailedPrecondition desc = PROJECT_REPO_HOST_REQUIRED: choose a host'],
      ['GITGATEWAY_MISSING_DEV_SERVER_ID: dev_server_id is required']
    ])('treats %s as an unassigned host', (raw) => {
      const result = formatWorkspaceCreateError(new Error(raw))
      expect(result.kind).toBe('host-unbound')
      expect(result.title).toBe('This project isn’t assigned to a dev server')
      expect(result.message).toContain('Choose the dev server')
    })

    it.each([
      ['GITGATEWAY_DEV_SERVER_UNREACHABLE: dev server is not online'],
      ['INFRA_DEV_SERVER_NOT_CONNECTED: dev server ds-1 is not connected']
    ])('treats %s as an offline host with a retry hint', (raw) => {
      const result = formatWorkspaceCreateError(new Error(raw))
      expect(result.kind).toBe('host-unreachable')
      expect(result.title).toBe('The dev server is offline')
      expect(result.message).toContain('retry')
    })

    it('treats GITGATEWAY_HOST_CHECK_FAILED as a check failure, not a missing repo', () => {
      const result = formatWorkspaceCreateError(
        new Error('GITGATEWAY_HOST_CHECK_FAILED: could not verify host')
      )
      expect(result.kind).toBe('host-check-failed')
      expect(getWorkspaceCreateErrorToastMessage(result)).toContain('confirm the dev server')
    })

    it('wins over WORKTREE_DETECT_FAILED, which may wrap a host code', () => {
      const result = formatWorkspaceCreateError(
        new Error('WORKTREE_DETECT_FAILED: GITGATEWAY_REPO_HOST_UNBOUND')
      )
      expect(result.kind).toBe('host-unbound')
    })

    it('still reports a real "not a git repository" as before', () => {
      expect(formatWorkspaceCreateError(new Error('fatal: not a git repository')).kind).toBe(
        'not-a-git-repo'
      )
    })
  })
})
