import { describe, expect, it, vi } from 'vitest'
import {
  resolveWorktreeStartPoint,
  WorktreeBaseRefError,
  type WorktreeStartPointProbe
} from './worktree-base-ref'

const SHA = 'a'.repeat(40)

// Pure precedence logic; the real-git behaviour is covered in
// relay/__tests__/agent-git-worktree-start-point.test.ts.
function probeWith(refs: string[], remotes: string[] = ['origin']): WorktreeStartPointProbe {
  return {
    resolveCommit: vi.fn(async (revision: string) => (refs.includes(revision) ? SHA : null)),
    listRemotes: vi.fn(async () => remotes)
  }
}

describe('resolveWorktreeStartPoint', () => {
  it('keeps fully qualified refs without touching git', async () => {
    const probe = probeWith([])
    await expect(resolveWorktreeStartPoint('refs/pull/123/head', probe)).resolves.toBe(
      'refs/pull/123/head'
    )
    expect(probe.resolveCommit).not.toHaveBeenCalled()
    expect(probe.listRemotes).not.toHaveBeenCalled()
  })

  it('rejects empty and dash-prefixed names before any git call', async () => {
    const probe = probeWith([])
    for (const bad of ['', '-b', '--detach']) {
      await expect(resolveWorktreeStartPoint(bad, probe)).rejects.toMatchObject({ code: 'INVALID' })
    }
    expect(probe.resolveCommit).not.toHaveBeenCalled()
    expect(probe.listRemotes).not.toHaveBeenCalled()
  })

  it('prefers a local branch over remote-tracking, tag and commit', async () => {
    const probe = probeWith(['refs/heads/main', 'refs/remotes/origin/main', 'refs/tags/main'])
    await expect(resolveWorktreeStartPoint('main', probe)).resolves.toBe('refs/heads/main')
  })

  it('qualifies remote-display names as remote-tracking refs', async () => {
    const probe = probeWith(['refs/remotes/origin/main'])
    await expect(resolveWorktreeStartPoint('origin/main', probe)).resolves.toBe(
      'refs/remotes/origin/main'
    )
  })

  it('qualifies a slash-containing name that only exists on origin (the DWIM bug)', async () => {
    const probe = probeWith(['refs/remotes/origin/release/y'])
    await expect(resolveWorktreeStartPoint('release/y', probe)).resolves.toBe(
      'refs/remotes/origin/release/y'
    )
  })

  it('falls back to the single other remote carrying the name', async () => {
    const probe = probeWith(['refs/remotes/fork/feature/z'], ['origin', 'upstream', 'fork'])
    await expect(resolveWorktreeStartPoint('feature/z', probe)).resolves.toBe(
      'refs/remotes/fork/feature/z'
    )
  })

  it('treats several matching non-origin remotes as an explicit error', async () => {
    const probe = probeWith(
      ['refs/remotes/upstream/feature/z', 'refs/remotes/fork/feature/z'],
      ['origin', 'upstream', 'fork']
    )
    const error = await resolveWorktreeStartPoint('feature/z', probe).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(WorktreeBaseRefError)
    expect(error).toMatchObject({ code: 'AMBIGUOUS' })
    expect((error as Error).message).toContain('upstream, fork')
  })

  it('prefers origin over other remotes without calling it ambiguous', async () => {
    const probe = probeWith(
      ['refs/remotes/origin/feature/z', 'refs/remotes/fork/feature/z'],
      ['origin', 'fork']
    )
    await expect(resolveWorktreeStartPoint('feature/z', probe)).resolves.toBe(
      'refs/remotes/origin/feature/z'
    )
  })

  it('resolves tags, then commit-ish to a verified sha', async () => {
    await expect(resolveWorktreeStartPoint('v1', probeWith(['refs/tags/v1']))).resolves.toBe(
      'refs/tags/v1'
    )
    await expect(resolveWorktreeStartPoint('abc1234', probeWith(['abc1234']))).resolves.toBe(SHA)
  })

  it('reports an unknown name with git-gateway-service compatible wording', async () => {
    const error = await resolveWorktreeStartPoint('nope', probeWith([])).catch((e: unknown) => e)
    expect(error).toMatchObject({ code: 'NOT_FOUND' })
    expect((error as Error).message).toContain('invalid reference')
  })
})
