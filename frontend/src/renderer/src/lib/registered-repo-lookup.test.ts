import { describe, expect, it } from 'vitest'
import {
  findRegisteredRepo,
  normalizeRepoPathKey,
  parseRepoAlreadyRegistered,
  RepoAlreadyRegisteredError
} from './registered-repo-lookup'

describe('normalizeRepoPathKey', () => {
  it('ignores trailing slashes and repeated separators', () => {
    expect(normalizeRepoPathKey('/home/me/app/')).toBe('/home/me/app')
    expect(normalizeRepoPathKey(' /home//me/app ')).toBe('/home/me/app')
  })

  it('treats Windows paths case-insensitively with either separator', () => {
    expect(normalizeRepoPathKey('C:\\Work\\App\\')).toBe(normalizeRepoPathKey('c:/work/app'))
  })

  it('keeps POSIX paths case-sensitive', () => {
    expect(normalizeRepoPathKey('/Home/App')).not.toBe(normalizeRepoPathKey('/home/app'))
  })
})

describe('findRegisteredRepo', () => {
  const repos = [
    { id: 'r1', projectId: 'p1', devServerId: 'dev-a', path: '/x/foo' },
    { id: 'r2', projectId: 'p2', devServerId: 'dev-b', url: '/x/bar/' }
  ]

  it('matches on host and normalised path (path or url)', () => {
    expect(findRegisteredRepo(repos, { devServerId: 'dev-a', path: '/x/foo/' })?.id).toBe('r1')
    expect(findRegisteredRepo(repos, { devServerId: 'dev-b', path: '/x/bar' })?.id).toBe('r2')
  })

  it('does not match the same path on another host', () => {
    expect(findRegisteredRepo(repos, { devServerId: 'dev-b', path: '/x/foo' })).toBeUndefined()
  })

  it('does not match without a host or path', () => {
    expect(findRegisteredRepo(repos, { devServerId: '', path: '/x/foo' })).toBeUndefined()
    expect(findRegisteredRepo(repos, { devServerId: 'dev-a', path: '  ' })).toBeUndefined()
  })
})

describe('parseRepoAlreadyRegistered', () => {
  it('extracts the project from the backend message', () => {
    const parsed = parseRepoAlreadyRegistered(
      new Error(
        'PROJECT_REPO_ALREADY_REGISTERED: repo /x/foo on dev-a is already in project My App (p-123)'
      )
    )
    expect(parsed).toBeInstanceOf(RepoAlreadyRegisteredError)
    expect(parsed?.projectName).toBe('My App')
    expect(parsed?.projectId).toBe('p-123')
  })

  it('reads the exact backend wording with a quoted project name', () => {
    const parsed = parseRepoAlreadyRegistered(
      new Error(
        'PROJECT_REPO_ALREADY_REGISTERED: repo /x/foo on dev server dev-a is already registered in project "My App" (p-123)'
      )
    )
    expect(parsed?.projectName).toBe('My App')
    expect(parsed?.projectId).toBe('p-123')
  })

  it('has no project to point at when the backend hides a private project', () => {
    const parsed = parseRepoAlreadyRegistered(
      new Error(
        'PROJECT_REPO_ALREADY_REGISTERED: repo /x/foo on dev server dev-a is already registered in another project'
      )
    )
    expect(parsed).toBeInstanceOf(RepoAlreadyRegisteredError)
    expect(parsed?.projectId).toBe('')
    expect(parsed?.projectName).toBe('')
  })

  it('handles project names that contain parentheses', () => {
    const parsed = parseRepoAlreadyRegistered(
      new Error(
        'PROJECT_REPO_ALREADY_REGISTERED: already registered in project "Demo (this Mac)" (p-9)'
      )
    )
    expect(parsed?.projectName).toBe('Demo (this Mac)')
    expect(parsed?.projectId).toBe('p-9')
  })

  it('still reports the code when the message has no project details', () => {
    const parsed = parseRepoAlreadyRegistered('PROJECT_REPO_ALREADY_REGISTERED')
    expect(parsed?.projectId).toBe('')
  })

  it('returns null for unrelated errors', () => {
    expect(parseRepoAlreadyRegistered(new Error('boom'))).toBeNull()
    expect(parseRepoAlreadyRegistered(undefined)).toBeNull()
  })
})
