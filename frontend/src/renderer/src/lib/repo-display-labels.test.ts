import { describe, expect, it } from 'vitest'
import { getRepoDisplayLabelsByPath } from './repo-display-labels'

describe('getRepoDisplayLabelsByPath', () => {
  it('keeps non-colliding repository names basename-only', () => {
    const labels = getRepoDisplayLabelsByPath([
      { path: '/workspace/platform/web', displayName: 'web' },
      { path: '/workspace/platform/worker', displayName: 'worker' }
    ])

    expect(labels.get('/workspace/platform/web')).toBe('web')
    expect(labels.get('/workspace/platform/worker')).toBe('worker')
  })

  it('adds the minimal real parent suffix only for colliding basenames', () => {
    const labels = getRepoDisplayLabelsByPath([
      { path: '/workspace/platform/web', displayName: 'web' },
      { path: '/workspace/platform/payments/api', displayName: 'api' },
      { path: '/workspace/platform/billing/api', displayName: 'api' }
    ])

    expect(labels.get('/workspace/platform/web')).toBe('web')
    expect(labels.get('/workspace/platform/payments/api')).toBe('payments/api')
    expect(labels.get('/workspace/platform/billing/api')).toBe('billing/api')
  })

  it('expands colliding labels in lockstep without skipping shared segments', () => {
    const labels = getRepoDisplayLabelsByPath([
      { path: '/workspace/team1/shared/api', displayName: 'api' },
      { path: '/workspace/team2/shared/api', displayName: 'api' }
    ])

    expect(labels.get('/workspace/team1/shared/api')).toBe('team1/shared/api')
    expect(labels.get('/workspace/team2/shared/api')).toBe('team2/shared/api')
  })

  it('normalizes Windows separators to slash display labels', () => {
    const labels = getRepoDisplayLabelsByPath([
      { path: 'C:\\workspace\\payments\\api', displayName: 'api' },
      { path: 'C:\\workspace\\billing\\api', displayName: 'api' }
    ])

    expect(labels.get('C:\\workspace\\payments\\api')).toBe('payments/api')
    expect(labels.get('C:\\workspace\\billing\\api')).toBe('billing/api')
  })

  // Regression guard: a repo created through project-service's Repo model
  // ({id, projectId, url, displayName, position} — no `path` field) reaching
  // the legacy sidebar via its tenant-wide repo.list fetch used to crash the
  // whole worktree list here — "Cannot read properties of undefined
  // (reading 'replace')" — the moment 2+ such repos collided on displayName
  // (including two both missing displayName, which both fall back to the
  // same `undefined` path here). Live-reproduced right after
  // CreateProjectDialog's repo.add fix started actually populating
  // project.repos for the first time.
  it('does not throw when a colliding repo has no path, and falls back to its displayName', () => {
    const labels = getRepoDisplayLabelsByPath([
      // @ts-expect-error — exercising the real runtime shape a project-service Repo has (no path)
      { path: undefined, displayName: 'api' },
      { path: '/workspace/billing/api', displayName: 'api' }
    ])

    expect(labels.get(undefined as unknown as string)).toBe('api')
    expect(labels.get('/workspace/billing/api')).toBe('billing/api')
  })
})
