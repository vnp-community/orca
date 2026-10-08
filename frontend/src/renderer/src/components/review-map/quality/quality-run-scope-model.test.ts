import { describe, expect, it } from 'vitest'
import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'
import {
  allowedRunScopes,
  buildQualityRunRequest,
  resolveRunScope,
  toQualityRunTarget
} from './quality-run-scope-model'

const profile = (scopes: string[]): RunnableProfile => ({
  id: 'p',
  title: 'p',
  kind: 'suite',
  ready: true,
  heavy: false,
  scopes,
  missing: []
})

describe('toQualityRunTarget', () => {
  it('maps every ReviewScope kind', () => {
    expect(
      toQualityRunTarget({ kind: 'branch', baseRef: 'main', includeUncommitted: true })
    ).toEqual({ scope: 'changed', base: 'main' })
    expect(
      toQualityRunTarget({ kind: 'branch', baseRef: 'main', includeUncommitted: false })
    ).toEqual({ scope: 'commitRange', base: 'main' })
    expect(toQualityRunTarget({ kind: 'range', baseCommit: 'abc', headCommit: 'def' })).toEqual({
      scope: 'commitRange',
      base: 'abc'
    })
    expect(
      toQualityRunTarget({
        kind: 'hostedReview',
        provider: 'github',
        number: 1,
        headSha: 's',
        baseRefName: 'main'
      })
    ).toEqual({ scope: 'commitRange', base: 'main' })
    expect(
      toQualityRunTarget({
        kind: 'hostedReview',
        provider: 'github',
        number: 1,
        headSha: null,
        baseRefName: null
      })
    ).toEqual({ scope: 'changed' })
  })

  it('defaults to changed files without a scope', () => {
    expect(toQualityRunTarget(null)).toEqual({ scope: 'changed' })
  })
})

describe('allowedRunScopes / resolveRunScope', () => {
  it('an empty scope list means unrestricted', () => {
    expect(allowedRunScopes(profile([]))).toEqual(['changed', 'commitRange', 'worktree'])
    expect(allowedRunScopes(null)).toHaveLength(3)
  })

  it('limits to what the profile can run and falls back to the first allowed scope', () => {
    expect(allowedRunScopes(profile(['worktree']))).toEqual(['worktree'])
    expect(resolveRunScope('changed', profile(['worktree']))).toBe('worktree')
    expect(resolveRunScope('changed', profile(['changed', 'worktree']))).toBe('changed')
    expect(resolveRunScope('changed', profile(['bogus']))).toBeNull()
  })
})

describe('buildQualityRunRequest', () => {
  it('sends base only for commit ranges', () => {
    expect(buildQualityRunRequest('p', { scope: 'changed', base: 'main' }, profile([]))).toEqual({
      profile: 'p',
      scope: 'changed'
    })
    expect(
      buildQualityRunRequest('p', { scope: 'commitRange', base: 'main' }, profile([]))
    ).toEqual({
      profile: 'p',
      scope: 'commitRange',
      base: 'main'
    })
  })

  it('returns null when the profile allows nothing', () => {
    expect(buildQualityRunRequest('p', { scope: 'changed' }, profile(['bogus']))).toBeNull()
  })
})
