import { describe, expect, it } from 'vitest'
import {
  projectHostSetupProjectionFromRepos,
  getProjectHostSetupsForProject,
  getProjectHostSetupWorktreeMeta,
  isGitHubBackedRepo
} from './project-host-setup-projection'
import type { Repo } from './types'

function repo(overrides: Partial<Repo> & Pick<Repo, 'id' | 'path' | 'displayName'>): Repo {
  return {
    badgeColor: '#737373',
    addedAt: 100,
    kind: 'git',
    ...overrides
  }
}

describe('project host setup projection', () => {
  it('projects a legacy local repo into one project and one ready local setup', () => {
    const projection = projectHostSetupProjectionFromRepos(
      [repo({ id: 'repo-1', path: '/Users/alice/orca', displayName: 'orca' })],
      500
    )

    expect(projection.projects).toEqual([
      {
        id: 'repo:repo-1',
        displayName: 'orca',
        badgeColor: '#737373',
        kind: 'git',
        sourceRepoIds: ['repo-1'],
        createdAt: 100,
        updatedAt: 100
      }
    ])
    expect(projection.setups).toEqual([
      {
        id: 'repo-1',
        projectId: 'repo:repo-1',
        hostId: 'local',
        repoId: 'repo-1',
        path: '/Users/alice/orca',
        displayName: 'orca',
        kind: 'git',
        setupState: 'ready',
        setupMethod: 'legacy-repo',
        createdAt: 100,
        updatedAt: 100
      }
    ])
  })

  it('preserves host-local setup fields on SSH repos', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'remote-repo',
        path: '/home/alice/orca',
        displayName: 'orca',
        connectionId: 'openclaw 2',
        worktreeBasePath: '../worktrees',
        gitUsername: 'alice'
      })
    ])

    expect(projection.setups[0]).toMatchObject({
      id: 'remote-repo',
      hostId: 'ssh:openclaw%202',
      connectionId: 'openclaw 2',
      worktreeBasePath: '../worktrees',
      gitUsername: 'alice'
    })
  })

  it('preserves repo-backed setup method metadata', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'repo-1',
        path: '/Users/alice/orca',
        displayName: 'orca',
        projectHostSetupMethod: 'cloned'
      })
    ])

    expect(projection.setups[0]?.setupMethod).toBe('cloned')
  })

  it('keeps repo checkouts with the same provider identity as separate projects (Phase 10: one project per repo, no cross-host merging)', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'local-repo',
        path: '/Users/alice/orca',
        displayName: 'Orca',
        upstream: { owner: 'StablyAI', repo: 'Orca' }
      }),
      repo({
        id: 'remote-repo',
        path: '/home/alice/orca',
        displayName: 'orca',
        connectionId: 'gpu-vm',
        upstream: { owner: 'stablyai', repo: 'orca' }
      })
    ])

    // Matching GitHub identity no longer merges repos into one Project (Phase 10):
    // each repo always gets its own Project, one per host setup.
    expect(projection.projects).toHaveLength(2)
    expect(projection.projects[0]).toMatchObject({
      id: 'repo:local-repo',
      sourceRepoIds: ['local-repo'],
      providerIdentity: { provider: 'github', owner: 'StablyAI', repo: 'Orca' }
    })
    expect(projection.projects[1]).toMatchObject({
      id: 'repo:remote-repo',
      sourceRepoIds: ['remote-repo'],
      providerIdentity: { provider: 'github', owner: 'stablyai', repo: 'orca' }
    })
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:local-repo')).toHaveLength(1)
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:remote-repo')).toHaveLength(1)
  })

  it('keeps repo-icon-derived provider identity from merging repos across hosts (Phase 10: one project per repo)', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'local-repo',
        path: '/Users/alice/orca',
        displayName: 'Orca',
        repoIcon: {
          type: 'image',
          src: 'https://github.com/stablyai.png?size=64',
          source: 'github',
          label: 'stablyai/orca'
        }
      }),
      repo({
        id: 'remote-repo',
        path: '/home/alice/orca',
        displayName: 'orca',
        connectionId: 'gpu-vm',
        repoIcon: {
          type: 'image',
          src: 'https://github.com/stablyai.png?size=64',
          source: 'github',
          label: 'StablyAI/Orca'
        }
      })
    ])

    // Same GitHub-icon-derived identity still no longer merges (Phase 10):
    // each repo keeps its own Project even though the provider identity matches.
    expect(projection.projects).toHaveLength(2)
    expect(projection.projects[0]).toMatchObject({
      id: 'repo:local-repo',
      sourceRepoIds: ['local-repo'],
      providerIdentity: { provider: 'github', owner: 'stablyai', repo: 'orca' }
    })
    expect(projection.projects[1]).toMatchObject({
      id: 'repo:remote-repo',
      sourceRepoIds: ['remote-repo'],
      providerIdentity: { provider: 'github', owner: 'StablyAI', repo: 'Orca' }
    })
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:local-repo')).toHaveLength(1)
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:remote-repo')).toHaveLength(1)
  })

  it('keeps git-remote-identity-matching repos as separate projects (Phase 10: one project per repo)', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'canonical-local-repo',
        path: '/Users/alice/stably/orca',
        displayName: 'orca',
        gitRemoteIdentity: {
          canonicalKey: 'github.com/stablyai/orca',
          remoteName: 'origin',
          remoteUrl: 'git@github.com:stablyai/orca.git'
        }
      }),
      repo({
        id: 'old-branch-checkout',
        path: '/Users/alice/orca/workspaces/orca/re-enable-webgl-for-remote-runtime-terminals',
        displayName: 're-enable-webgl-for-remote-runtime-terminals',
        repoIcon: {
          type: 'image',
          src: 'https://github.com/stablyai.png?size=64',
          source: 'github',
          label: 'stablyai/orca'
        }
      })
    ])

    // A shared git remote identity (or GitHub provider identity) no longer
    // merges checkouts into one Project (Phase 10): a stale branch checkout
    // of the same repo stays its own card, even though it resolves to the
    // same GitHub owner/repo.
    expect(projection.projects).toHaveLength(2)
    expect(projection.projects[0]).toMatchObject({
      id: 'repo:canonical-local-repo',
      displayName: 'orca',
      sourceRepoIds: ['canonical-local-repo'],
      providerIdentity: { provider: 'github', owner: 'stablyai', repo: 'orca' }
    })
    expect(projection.projects[1]).toMatchObject({
      id: 'repo:old-branch-checkout',
      displayName: 're-enable-webgl-for-remote-runtime-terminals',
      sourceRepoIds: ['old-branch-checkout'],
      providerIdentity: { provider: 'github', owner: 'stablyai', repo: 'orca' }
    })
  })

  it('does not guess that same-named folders are the same project without identity', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({ id: 'local-repo', path: '/Users/alice/app', displayName: 'app' }),
      repo({
        id: 'remote-repo',
        path: '/srv/app',
        displayName: 'app',
        connectionId: 'work-server'
      })
    ])

    expect(projection.projects.map((project) => project.id)).toEqual([
      'repo:local-repo',
      'repo:remote-repo'
    ])
  })

  it('keeps same-git-remote-identity records across local, SSH, and runtime hosts as separate projects (Phase 10: one project per repo)', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'local-sample-app',
        path: '/Users/alice/work/sample-app',
        displayName: 'sample-app',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/team/sample-app',
          remoteName: 'origin',
          remoteUrl: 'git@git.company.test:team/sample-app.git'
        }
      }),
      repo({
        id: 'ssh-sample-app',
        path: '/home/alice/src/sample-app',
        displayName: 'sample-app',
        connectionId: 'build server',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/team/sample-app',
          remoteName: 'origin',
          remoteUrl: 'https://git.company.test/team/sample-app.git'
        }
      }),
      repo({
        id: 'runtime-sample-app',
        path: '/workspace/sample-app',
        displayName: 'sample-app',
        executionHostId: 'runtime:dev-container',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/team/sample-app',
          remoteName: 'origin',
          remoteUrl: 'ssh://git@git.company.test/team/sample-app.git'
        }
      })
    ])

    // Previously a shared git remote identity grouped local/SSH/runtime
    // checkouts of the same repo under one Project. Phase 10 removes that:
    // each host's checkout is its own Project, one per repo, no cross-host
    // merging regardless of how many hosts share the same git remote.
    expect(projection.projects).toHaveLength(3)
    expect(projection.projects.map((project) => project.id)).toEqual([
      'repo:local-sample-app',
      'repo:ssh-sample-app',
      'repo:runtime-sample-app'
    ])
    for (const project of projection.projects) {
      expect(project).toMatchObject({
        displayName: 'sample-app',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/team/sample-app',
          remoteName: 'origin'
        }
      })
    }
    expect(projection.projects[0]?.sourceRepoIds).toEqual(['local-sample-app'])
    expect(projection.projects[1]?.sourceRepoIds).toEqual(['ssh-sample-app'])
    expect(projection.projects[2]?.sourceRepoIds).toEqual(['runtime-sample-app'])
    expect(projection.setups.map((setup) => setup.hostId)).toEqual([
      'local',
      'ssh:build%20server',
      'runtime:dev-container'
    ])
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:local-sample-app')).toHaveLength(
      1
    )
    expect(getProjectHostSetupsForProject(projection.setups, 'repo:ssh-sample-app')).toHaveLength(1)
    expect(
      getProjectHostSetupsForProject(projection.setups, 'repo:runtime-sample-app')
    ).toHaveLength(1)
  })

  it('keeps same-named cross-host records separate when there is no shared repo identity', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'local-sample-app',
        path: '/Users/alice/work/sample-app',
        displayName: 'sample-app'
      }),
      repo({
        id: 'ssh-sample-app',
        path: '/srv/unrelated/sample-app',
        displayName: 'sample-app',
        connectionId: 'staging server'
      }),
      repo({
        id: 'runtime-sample-app',
        path: '/workspace/sample-app',
        displayName: 'sample-app',
        executionHostId: 'runtime:preview'
      })
    ])

    // Display names are labels, not identity, so these were already kept
    // separate before Phase 10. Phase 10 makes this the *only* outcome now:
    // even a shared git remote identity no longer merges records (see the
    // git-remote-identity tests above), so this case needs no identity match
    // to stay correct.
    expect(projection.projects).toHaveLength(3)
  })

  it('keeps case-distinct git-remote-identity repos as separate projects (Phase 10: project id is always repo-keyed)', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'uppercase-repo',
        path: '/Users/alice/work/sample-app',
        displayName: 'sample-app',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/Team/Sample-App',
          remoteName: 'origin',
          remoteUrl: 'git@git.company.test:Team/Sample-App.git'
        }
      }),
      repo({
        id: 'lowercase-repo',
        path: '/home/alice/src/sample-app',
        displayName: 'sample-app',
        connectionId: 'build server',
        gitRemoteIdentity: {
          canonicalKey: 'git.company.test/team/sample-app',
          remoteName: 'origin',
          remoteUrl: 'git@git.company.test:team/sample-app.git'
        }
      })
    ])

    // These never shared identity (different-case canonicalKey) so they were
    // already separate before Phase 10. Phase 10 also changes project ids to
    // always be `repo:<id>` rather than an identity-derived `git:<key>`, since
    // ids no longer need to disambiguate merge groups.
    expect(projection.projects.map((project) => project.id)).toEqual([
      'repo:uppercase-repo',
      'repo:lowercase-repo'
    ])
  })

  it('ignores malformed provider identity values', () => {
    const projection = projectHostSetupProjectionFromRepos([
      repo({
        id: 'repo-1',
        path: '/Users/alice/orca',
        displayName: 'orca',
        upstream: { owner: 'stablyai', repo: 42 } as never
      })
    ])

    expect(projection.projects[0]?.id).toBe('repo:repo-1')
    expect(projection.projects[0]?.providerIdentity).toBeUndefined()
  })

  it('derives workspace ownership metadata from the repo setup', () => {
    const targetRepo = repo({
      id: 'remote-repo',
      path: '/home/alice/orca',
      displayName: 'orca',
      connectionId: 'openclaw 2',
      upstream: { owner: 'stablyai', repo: 'orca' }
    })
    const projection = projectHostSetupProjectionFromRepos([targetRepo])

    // Phase 10: projectId is always repo-keyed now, not a GitHub-identity id,
    // since a repo's Project is never shared with another repo's.
    expect(getProjectHostSetupWorktreeMeta(projection.setups, targetRepo)).toEqual({
      projectId: 'repo:remote-repo',
      hostId: 'ssh:openclaw%202',
      projectHostSetupId: 'remote-repo'
    })
  })
})

describe('isGitHubBackedRepo', () => {
  it('is true when an explicit upstream owner/repo is present', () => {
    const target = repo({
      id: 'r',
      path: '/r',
      displayName: 'r',
      upstream: { owner: 'stablyai', repo: 'orca' }
    })
    expect(isGitHubBackedRepo(target)).toBe(true)
  })

  it('is true when a GitHub-sourced avatar icon encodes the slug', () => {
    const target = repo({
      id: 'r',
      path: '/r',
      displayName: 'r',
      repoIcon: {
        type: 'image',
        src: 'https://github.com/stablyai.png?size=64',
        source: 'github',
        label: 'stablyai/orca'
      }
    })
    expect(isGitHubBackedRepo(target)).toBe(true)
  })

  it('is false for a non-GitHub icon and no upstream (GitLab/folder)', () => {
    const target = repo({
      id: 'r',
      path: '/r',
      displayName: 'r',
      repoIcon: { type: 'lucide', name: 'gitlab' }
    })
    expect(isGitHubBackedRepo(target)).toBe(false)
  })

  it('is false for a plain local repo with no provider signal', () => {
    expect(isGitHubBackedRepo(repo({ id: 'r', path: '/r', displayName: 'r' }))).toBe(false)
  })
})
