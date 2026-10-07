import { describe, it, expect } from 'vitest'
import { toRepoRelative } from './quality-repo-path-mapping'

describe('quality-repo-path-mapping', () => {
  it('posix: relative inside repo', () => {
    const res = toRepoRelative('internal/x.go', '/opt/repos/backend/services/a', '/opt/repos/backend', 'linux')
    expect(res).toEqual({ file: 'services/a/internal/x.go', outside: false })
  })

  it('posix: absolute inside repo', () => {
    const res = toRepoRelative('/opt/repos/backend/services/a/internal/x.go', '/tmp', '/opt/repos/backend', 'linux')
    expect(res).toEqual({ file: 'services/a/internal/x.go', outside: false })
  })

  it('posix: absolute outside repo (go module cache)', () => {
    const res = toRepoRelative('/home/user/go/pkg/mod/github.com/foo/bar@v1.0.0/baz.go', '/opt/repos/backend/services/a', '/opt/repos/backend', 'linux')
    expect(res).toEqual({ file: '', outside: true })
  })

  it('posix: relative that escapes repo', () => {
    const res = toRepoRelative('../../../../../tmp/foo.txt', '/opt/repos/backend/services/a', '/opt/repos/backend', 'linux')
    expect(res).toEqual({ file: '', outside: true })
  })

  it('win32: relative inside repo', () => {
    const res = toRepoRelative('internal\\x.go', 'C:\\repos\\backend\\services\\a', 'C:\\repos\\backend', 'win32')
    expect(res).toEqual({ file: 'services/a/internal/x.go', outside: false })
  })

  it('win32: absolute inside repo', () => {
    const res = toRepoRelative('C:\\repos\\backend\\services\\a\\internal\\x.go', 'C:\\tmp', 'C:\\repos\\backend', 'win32')
    expect(res).toEqual({ file: 'services/a/internal/x.go', outside: false })
  })

  it('win32: different drive escapes repo', () => {
    const res = toRepoRelative('D:\\foo.go', 'C:\\repos\\backend', 'C:\\repos\\backend', 'win32')
    expect(res).toEqual({ file: '', outside: true })
  })

  it('posix: handles spaces and unicode', () => {
    const res = toRepoRelative('thư mục/tệp tin.ts', '/opt/repos/backend/tên repo', '/opt/repos/backend', 'darwin')
    expect(res).toEqual({ file: 'tên repo/thư mục/tệp tin.ts', outside: false })
  })

  it('posix: returns empty string (not outside) for repo root itself', () => {
    const res = toRepoRelative('.', '/opt/repos/backend', '/opt/repos/backend', 'linux')
    expect(res).toEqual({ file: '', outside: false })
  })
})
