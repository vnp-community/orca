import { describe, expect, it } from 'vitest'
import {
  isPathInsideOrEqual,
  isRuntimePathAbsolute,
  isWindowsAbsolutePathLike,
  relativePathInsideRoot,
  resolveRuntimePath
} from './cross-platform-path'

describe('isWindowsAbsolutePathLike', () => {
  it('recognizes drive-letter and UNC paths', () => {
    expect(isWindowsAbsolutePathLike('C:\\Users\\jin\\repo')).toBe(true)
    expect(isWindowsAbsolutePathLike('\\\\host\\share')).toBe(true)
  })

  it('rejects POSIX paths', () => {
    expect(isWindowsAbsolutePathLike('/home/jin/repo')).toBe(false)
  })

  // Regression test: a ProjectHostSetup derived from a bare project-service
  // Repo (no `path` field on the wire) can reach this with `path` genuinely
  // undefined at runtime despite the `string` type — found live crashing
  // the whole sidebar list (worktree-list-groups.ts's
  // getProjectSetupSurfaceKey).
  it('returns false instead of throwing for a missing path', () => {
    expect(isWindowsAbsolutePathLike(undefined as unknown as string)).toBe(false)
    expect(isWindowsAbsolutePathLike('')).toBe(false)
  })
})

describe('cross-platform path containment', () => {
  it('keeps POSIX sibling prefixes outside the root', () => {
    expect(isPathInsideOrEqual('/repo/app', '/repo/app')).toBe(true)
    expect(isPathInsideOrEqual('/repo/app', '/repo/app/src/index.ts')).toBe(true)
    expect(isPathInsideOrEqual('/repo/app', '/repo/application/src/index.ts')).toBe(false)
    expect(relativePathInsideRoot('/repo/app/', '/repo/app/src/index.ts')).toBe('src/index.ts')
  })

  it('handles Windows drive roots and sibling drives case-insensitively', () => {
    expect(isPathInsideOrEqual('C:\\Repo', 'c:\\repo\\src\\index.ts')).toBe(true)
    expect(relativePathInsideRoot('C:\\Repo', 'c:\\repo\\src\\index.ts')).toBe('src/index.ts')
    expect(isPathInsideOrEqual('C:\\Repo', 'D:\\Repo\\src\\index.ts')).toBe(false)
    expect(relativePathInsideRoot('C:\\', 'c:\\repo\\src\\index.ts')).toBe('repo/src/index.ts')
  })

  it('handles UNC roots, trailing slashes, mixed separators, and case', () => {
    expect(isPathInsideOrEqual('\\\\Server\\Share\\Repo\\', '//server/share/repo/src')).toBe(true)
    expect(relativePathInsideRoot('\\\\Server\\Share\\Repo\\', '//server/share/repo/src')).toBe(
      'src'
    )
    expect(isPathInsideOrEqual('\\\\Server\\Share\\Repo', '\\\\server\\share\\repo2')).toBe(false)
  })

  it('resolves POSIX relative paths without using the process cwd', () => {
    expect(resolveRuntimePath('/repos/app/repo', '../worktrees/feature')).toBe(
      '/repos/app/worktrees/feature'
    )
    expect(resolveRuntimePath('/repos/app/repo', '/custom/worktrees')).toBe('/custom/worktrees')
    expect(isRuntimePathAbsolute('../worktrees')).toBe(false)
  })

  it('resolves Windows relative paths with Windows semantics', () => {
    expect(resolveRuntimePath('C:\\Repos\\app\\repo', '..\\worktrees\\feature')).toBe(
      'C:/Repos/app/worktrees/feature'
    )
    expect(resolveRuntimePath('C:\\Repos\\app\\repo', 'D:\\worktrees')).toBe('D:/worktrees')
    expect(isRuntimePathAbsolute('/remote/worktrees', 'windows')).toBe(true)
  })
})
