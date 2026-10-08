/**
 * Tests for code-intel-worktree-selector.ts (FE-CV-TASK-050-09)
 */

import { describe, it, expect, vi } from 'vitest'
import { resolveCodeIntelSelector } from './code-intel-worktree-selector'

// Mock getRuntimeEnvironmentIdForWorktree
vi.mock('./worktree-runtime-owner', () => ({
  getRuntimeEnvironmentIdForWorktree: vi.fn().mockReturnValue('env-1')
}))

type FakeState = Parameters<typeof resolveCodeIntelSelector>[0]

function makeState(
  overrides: {
    worktrees?: { id: string; repoId: string; projectId?: string }[]
    repos?: { id: string; projectId?: string }[]
  } = {}
): FakeState {
  const worktreesByRepo: Record<string, unknown[]> = {}
  for (const wt of overrides.worktrees ?? []) {
    ;(worktreesByRepo[wt.repoId] ??= []).push(wt)
  }
  return {
    worktreesByRepo,
    repos: overrides.repos ?? []
  } as unknown as FakeState
}

describe('resolveCodeIntelSelector — ready cases', () => {
  it('returns ready when worktree has projectId', () => {
    const state = makeState({
      worktrees: [{ id: 'repo-1::main', repoId: 'repo-1', projectId: 'proj-1' }]
    })
    const result = resolveCodeIntelSelector(state, 'repo-1::main')
    expect(result.state).toBe('ready')
    if (result.state === 'ready') {
      expect(result.projectId).toBe('proj-1')
      expect(result.worktreeId).toBe('repo-1::main')
    }
  })

  it('falls back to repo.projectId when worktree lacks it', () => {
    const state = makeState({
      worktrees: [{ id: 'repo-1::main', repoId: 'repo-1' }],
      repos: [{ id: 'repo-1', projectId: 'proj-from-repo' }]
    })
    const result = resolveCodeIntelSelector(state, 'repo-1::main')
    expect(result.state).toBe('ready')
    if (result.state === 'ready') {
      expect(result.projectId).toBe('proj-from-repo')
    }
  })

  it('strips id: prefix', () => {
    const state = makeState({
      worktrees: [{ id: 'repo-1::main', repoId: 'repo-1', projectId: 'proj-1' }]
    })
    const result = resolveCodeIntelSelector(state, 'id:repo-1::main')
    expect(result.state).toBe('ready')
  })
})

describe('resolveCodeIntelSelector — unsupported cases', () => {
  it('workspace id → workspace-scope', () => {
    const result = resolveCodeIntelSelector(makeState(), 'workspace')
    expect(result.state).toBe('unsupported')
    if (result.state === 'unsupported') {expect(result.reason).toBe('workspace-scope')}
  })

  it('::workspace: prefix → workspace-scope', () => {
    const result = resolveCodeIntelSelector(makeState(), '::workspace:root')
    expect(result.state).toBe('unsupported')
    if (result.state === 'unsupported') {expect(result.reason).toBe('workspace-scope')}
  })

  it('folder: prefix → workspace-scope', () => {
    const result = resolveCodeIntelSelector(makeState(), 'folder:/tmp')
    expect(result.state).toBe('unsupported')
    if (result.state === 'unsupported') {expect(result.reason).toBe('workspace-scope')}
  })

  it('unknown worktree id → unknown-worktree', () => {
    const result = resolveCodeIntelSelector(makeState(), 'nonexistent::main')
    expect(result.state).toBe('unsupported')
    if (result.state === 'unsupported') {expect(result.reason).toBe('unknown-worktree')}
  })

  it('no projectId on worktree or repo → no-project', () => {
    const state = makeState({
      worktrees: [{ id: 'repo-1::main', repoId: 'repo-1' }],
      repos: [{ id: 'repo-1' }]
    })
    const result = resolveCodeIntelSelector(state, 'repo-1::main')
    expect(result.state).toBe('unsupported')
    if (result.state === 'unsupported') {expect(result.reason).toBe('no-project')}
  })
})
