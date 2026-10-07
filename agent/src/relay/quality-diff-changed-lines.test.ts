import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execSync } from 'node:child_process'
import { getAddedLines } from './quality-diff-changed-lines'

describe('quality-diff-changed-lines', () => {
  let tmpRepo: string

  beforeEach(() => {
    tmpRepo = fs.mkdtempSync(path.join(os.tmpdir(), 'diff-lines-test-'))
    execSync('git init -b main', { cwd: tmpRepo })
    execSync('git config user.name "Tester"', { cwd: tmpRepo })
    execSync('git config user.email "test@example.com"', { cwd: tmpRepo })

    fs.writeFileSync(path.join(tmpRepo, 'initial.txt'), 'line 1\nline 2\n')
    execSync('git add . && git commit -m "initial commit"', { cwd: tmpRepo })
  })

  afterEach(() => {
    try {
      fs.rmSync(tmpRepo, { recursive: true, force: true })
    } catch {}
  })

  it('detects modified and added lines in worktree', async () => {
    fs.appendFileSync(path.join(tmpRepo, 'initial.txt'), 'line 3 added\nline 4 added\n')
    fs.writeFileSync(path.join(tmpRepo, 'newfile.txt'), 'new line 1\n')

    const res = await getAddedLines(tmpRepo, { mergeBaseOf: null, to: 'worktree' })
    expect(res.warnings).toEqual([])
    expect(res.truncated).toBe(false)

    const initialDiff = res.files.find(f => f.path === 'initial.txt')
    expect(initialDiff).toBeDefined()
    expect(initialDiff?.status).toBe('M')
    expect(initialDiff?.addedLines.length).toBe(2)
    expect(initialDiff?.addedLines[0].line).toBe(3)
    expect(initialDiff?.addedLines[0].text).toBe('line 3 added')

    const newDiff = res.files.find(f => f.path === 'newfile.txt')
    expect(newDiff).toBeDefined()
    expect(newDiff?.untracked).toBe(true)
    expect(newDiff?.addedLines.length).toBe(1)
    expect(newDiff?.addedLines[0].text).toBe('new line 1')
  })

  it('does not crash and returns warnings when git fails or directory invalid', async () => {
    const nonGitDir = fs.mkdtempSync(path.join(os.tmpdir(), 'non-git-'))
    try {
      const res = await getAddedLines(nonGitDir, { mergeBaseOf: null, to: 'worktree' })
      expect(res.files).toBeDefined()
    } finally {
      fs.rmSync(nonGitDir, { recursive: true, force: true })
    }
  })
})
