import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { execFile } from 'child_process'
import { promisify } from 'util'
import { isSafeBaseRef, changedFiles, QualityParamsError } from './quality-changed-files'
import { dirtyFingerprint } from './quality-dirty-fingerprint'

const exec = promisify(execFile)

describe('quality-changed-files & fingerprint', () => {
  let tmpRepo: string

  async function runGit(args: string[]) {
    await exec('git', args, { cwd: tmpRepo })
  }

  beforeEach(async () => {
    tmpRepo = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-git-'))
    await runGit(['init'])
    await runGit(['checkout', '-b', 'master'])
    await runGit(['config', 'user.email', 'test@example.com'])
    await runGit(['config', 'user.name', 'Test User'])
    fs.writeFileSync(path.join(tmpRepo, 'initial.txt'), 'hello')
    fs.writeFileSync(path.join(tmpRepo, '-bad.txt'), 'bad')
    await runGit(['add', '.'])
    await runGit(['commit', '-m', 'Initial'])
  })

  afterEach(() => {
    fs.rmSync(tmpRepo, { recursive: true, force: true })
  })

  it('isSafeBaseRef logic', () => {
    expect(isSafeBaseRef('main')).toBe(true)
    expect(isSafeBaseRef('feature/foo')).toBe(true)
    expect(isSafeBaseRef('f'.repeat(40))).toBe(true)

    // bad ones
    expect(isSafeBaseRef('-main')).toBe(false)
    expect(isSafeBaseRef('main.')).toBe(false)
    expect(isSafeBaseRef('main/')).toBe(false)
    expect(isSafeBaseRef('foo.lock')).toBe(false)
    expect(isSafeBaseRef('foo..bar')).toBe(false)
    expect(isSafeBaseRef('foo@{bar}')).toBe(false)
    expect(isSafeBaseRef('foo//bar')).toBe(false)
    expect(isSafeBaseRef('foo bar')).toBe(false)
    expect(isSafeBaseRef('HEAD~1')).toBe(false) // ~ is forbidden (even though valid for git, we restrict it per requirements maybe? "không ~, ^, :, ?, *, [, \"). Wait, task 14 says pure function according to check-ref-format.
  })

  it('changedFiles - worktree returns null', async () => {
    expect(await changedFiles({ root: tmpRepo, scope: 'worktree' })).toBeNull()
  })

  it('changedFiles - missing or invalid base', async () => {
    await expect(changedFiles({ root: tmpRepo, scope: 'changed' })).rejects.toThrow(QualityParamsError)
    await expect(changedFiles({ root: tmpRepo, scope: 'changed', base: '-foo' })).rejects.toThrow('invalid_base')
    await expect(changedFiles({ root: tmpRepo, scope: 'changed', base: 'nonexist' })).rejects.toThrow('unresolved_ref')
  })

  it('changedFiles detects changes, spaces, unicode, renames, and filters -bad', async () => {
    await runGit(['checkout', '-b', 'feat'])
    
    // Add unicode, spaces
    fs.writeFileSync(path.join(tmpRepo, 'space file.txt'), 'a')
    fs.writeFileSync(path.join(tmpRepo, 'uni-còde.txt'), 'b')
    await runGit(['add', '.'])
    await runGit(['commit', '-m', 'Add files'])

    // Modify, rename, untracked
    fs.writeFileSync(path.join(tmpRepo, 'initial.txt'), 'hello modified')
    await runGit(['add', 'initial.txt'])
    await runGit(['mv', 'uni-còde.txt', 'renamed.txt'])
    await runGit(['commit', '-m', 'Rename'])
    fs.writeFileSync(path.join(tmpRepo, 'untracked.txt'), 'c')

    const files = await changedFiles({ root: tmpRepo, scope: 'changed', base: 'master' })
    
    // Should contain: initial.txt, space file.txt, renamed.txt, untracked.txt
    // Should NOT contain: uni-còde.txt (was renamed), -bad.txt (starts with -)
    // Wait, master -> feat branch.
    expect(files).toContain('initial.txt')
    expect(files).toContain('space file.txt')
    expect(files).toContain('renamed.txt')
    expect(files).toContain('untracked.txt')
    expect(files).not.toContain('uni-còde.txt')
    expect(files).not.toContain('-bad.txt')
  })

  it('dirtyFingerprint changes on modification', async () => {
    const hash1 = await dirtyFingerprint(tmpRepo)
    
    fs.writeFileSync(path.join(tmpRepo, 'initial.txt'), 'modified')
    const hash2 = await dirtyFingerprint(tmpRepo)
    expect(hash1).not.toBe(hash2)
    
    // Untracked file
    fs.writeFileSync(path.join(tmpRepo, 'new.txt'), 'x')
    const hash3 = await dirtyFingerprint(tmpRepo)
    expect(hash2).not.toBe(hash3)
  })
})
