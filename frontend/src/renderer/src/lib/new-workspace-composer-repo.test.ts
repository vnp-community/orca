import { describe, expect, it } from 'vitest'
import type { Repo } from '../../../shared/types'
import {
  getComposerEligibleRepos,
  resolveComposerActiveRepoId,
  resolveComposerGitRepoId,
  resolveComposerRepoId
} from './new-workspace-composer-repo'

function makeRepo(id: string, overrides: Partial<Repo> = {}): Repo {
  return {
    id,
    path: `/repos/${id}`,
    displayName: id,
    badgeColor: '#000000',
    addedAt: 0,
    ...overrides
  }
}

describe('new-workspace-composer-repo', () => {
  it('matches the composer repo priority order', () => {
    const eligibleRepos = [
      makeRepo('first'),
      makeRepo('active'),
      makeRepo('initial'),
      makeRepo('draft')
    ]

    expect(
      resolveComposerRepoId({
        eligibleRepos,
        draftRepoId: 'draft',
        initialRepoId: 'initial',
        activeRepoId: 'active'
      })
    ).toBe('draft')
  })

  it('falls back through initial, active, then first eligible repo', () => {
    const eligibleRepos = [makeRepo('first'), makeRepo('active')]

    expect(resolveComposerRepoId({ eligibleRepos, initialRepoId: 'missing' })).toBe('first')
    expect(resolveComposerRepoId({ eligibleRepos, activeRepoId: 'active' })).toBe('active')
  })

  it('returns null for create-base prefetch when the composer default is a folder repo', () => {
    const eligibleRepos = [makeRepo('folder', { kind: 'folder' }), makeRepo('git')]

    expect(resolveComposerGitRepoId({ eligibleRepos })).toBeNull()
  })

  it('excludes repos without paths from composer defaults', () => {
    expect(
      getComposerEligibleRepos([makeRepo('missing-path', { path: '' }), makeRepo('repo')])
    ).toEqual([expect.objectContaining({ id: 'repo' })])
  })

  it('defaults to a repo on the focused host when no explicit repo is chosen', () => {
    const eligibleRepos = [
      makeRepo('local-repo'),
      makeRepo('ssh-repo', { connectionId: 'win-vm' }),
      makeRepo('runtime-repo', { executionHostId: 'runtime:env-1' })
    ]

    expect(resolveComposerRepoId({ eligibleRepos, focusedHostScope: 'ssh:win-vm' })).toBe(
      'ssh-repo'
    )
    expect(resolveComposerRepoId({ eligibleRepos, focusedHostScope: 'runtime:env-1' })).toBe(
      'runtime-repo'
    )
    expect(resolveComposerRepoId({ eligibleRepos, focusedHostScope: 'local' })).toBe('local-repo')
  })

  it('lets explicit draft/initial/active choices win over the focused host', () => {
    const eligibleRepos = [makeRepo('local-repo'), makeRepo('ssh-repo', { connectionId: 'win-vm' })]

    expect(
      resolveComposerRepoId({
        eligibleRepos,
        activeRepoId: 'local-repo',
        focusedHostScope: 'ssh:win-vm'
      })
    ).toBe('local-repo')
  })

  it('ignores host scope "all" and falls back to the first eligible repo', () => {
    const eligibleRepos = [makeRepo('local-repo'), makeRepo('ssh-repo', { connectionId: 'win-vm' })]

    expect(resolveComposerRepoId({ eligibleRepos, focusedHostScope: 'all' })).toBe('local-repo')
  })

  it('falls back to the first eligible repo when the focused host has no repos', () => {
    const eligibleRepos = [makeRepo('local-repo')]

    expect(resolveComposerRepoId({ eligibleRepos, focusedHostScope: 'ssh:gone' })).toBe(
      'local-repo'
    )
  })

  describe('resolveComposerActiveRepoId', () => {
    const localOrca = makeRepo('local-orca', { upstream: { owner: 'stablyai', repo: 'orca' } })
    const runtimeOrca = makeRepo('runtime-orca', {
      connectionId: 'runtime-ssh-orca-1',
      upstream: { owner: 'stablyai', repo: 'orca' }
    })
    const otherProject = makeRepo('noqa', { upstream: { owner: 'stablyai', repo: 'noqa' } })
    const repos = [otherProject, localOrca, runtimeOrca]
    const eligibleRepos = getComposerEligibleRepos(repos)

    // Phase 10: a legacy Project is always exactly one repo now, so sharing a
    // GitHub identity (both repos point at stablyai/orca here) no longer makes
    // two repos "the same project" — getProjectIdentityKey returns a distinct
    // `repo:<id>` key per repo, so no sibling is ever found this way anymore.
    // The runtime-owned repo id passes through unchanged, same as when no
    // sibling exists at all.
    it('keeps the runtime repo id even when another repo shares its GitHub identity (Phase 10: no cross-repo project merging)', () => {
      expect(resolveComposerActiveRepoId(repos, eligibleRepos, 'runtime-orca')).toBe('runtime-orca')
    })

    it('leaves a normal active repo unchanged', () => {
      expect(resolveComposerActiveRepoId(repos, eligibleRepos, 'local-orca')).toBe('local-orca')
    })

    it('keeps the runtime repo id when no same-project sibling is eligible', () => {
      const onlyRuntime = [runtimeOrca]
      expect(
        resolveComposerActiveRepoId(
          onlyRuntime,
          getComposerEligibleRepos(onlyRuntime),
          'runtime-orca'
        )
      ).toBe('runtime-orca')
    })

    it('passes through null/undefined active repo', () => {
      expect(resolveComposerActiveRepoId(repos, eligibleRepos, null)).toBeNull()
    })
  })
})
