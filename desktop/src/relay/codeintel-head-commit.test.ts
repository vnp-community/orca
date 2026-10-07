import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { execSync } from 'child_process'
import { getHeadCommit, invalidateHeadCommit } from './codeintel-head-commit'

describe('codeintel-head-commit', () => {
  const tmpDir = os.tmpdir()
  let repoDir: string

  beforeEach(() => {
    repoDir = fs.mkdtempSync(path.join(tmpDir, 'orca-repo-'))
    execSync('git init', { cwd: repoDir })
    invalidateHeadCommit()
  })

  afterEach(() => {
    try { fs.rmSync(repoDir, { recursive: true, force: true }) } catch {}
  })

  it('returns null for unborn repo', async () => {
    const commit = await getHeadCommit(repoDir)
    expect(commit).toBeNull()
  })

  it('returns 40 hex string for committed repo', async () => {
    // create a commit
    fs.writeFileSync(path.join(repoDir, 'test.txt'), 'hello')
    execSync('git add test.txt', { cwd: repoDir })
    execSync('git config user.name "Test"', { cwd: repoDir })
    execSync('git config user.email "test@example.com"', { cwd: repoDir })
    execSync('git commit -m "Init"', { cwd: repoDir })

    const commit = await getHeadCommit(repoDir)
    expect(commit).toMatch(/^[0-9a-f]{40}$/)
  })

  it('returns null for non-repo directory', async () => {
    const nonRepo = fs.mkdtempSync(path.join(tmpDir, 'non-repo-'))
    try {
      const commit = await getHeadCommit(nonRepo)
      expect(commit).toBeNull()
    } finally {
      fs.rmSync(nonRepo, { recursive: true, force: true })
    }
  })

  it('caches the result for 5 seconds', async () => {
    fs.writeFileSync(path.join(repoDir, 'test.txt'), 'hello')
    execSync('git add test.txt', { cwd: repoDir })
    execSync('git config user.name "Test"', { cwd: repoDir })
    execSync('git config user.email "test@example.com"', { cwd: repoDir })
    execSync('git commit -m "Init"', { cwd: repoDir })

    const commit1 = await getHeadCommit(repoDir)
    
    // delete the .git to simulate it becoming a non-repo, 
    // but the cache should still return the commit
    fs.rmSync(path.join(repoDir, '.git'), { recursive: true, force: true })
    
    const commit2 = await getHeadCommit(repoDir)
    expect(commit2).toBe(commit1)
  })
})
