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
})
