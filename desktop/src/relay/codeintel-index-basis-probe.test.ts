import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { execSync } from 'child_process'
import { probeIndexBasis } from './codeintel-index-basis-probe'

describe('probeIndexBasis', () => {
  let tmpBase: string
  let repoRoot: string

  beforeAll(() => {
    tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'probe-test-'))
    repoRoot = path.join(tmpBase, 'repo')
    fs.mkdirSync(repoRoot)
    execSync('git init', { cwd: repoRoot })
  })

  afterAll(() => {
    fs.rmSync(tmpBase, { recursive: true, force: true })
  })

  it('unborn HEAD', async () => {
    const res = await probeIndexBasis(repoRoot, { indexedCommit: 'abc', indexedAtMs: 1000, baseRef: 'origin/HEAD' })
    expect(res.headCommit).toBeNull()
  })

  it('missing baseRef -> mergeBase: null', async () => {
    fs.writeFileSync(path.join(repoRoot, 'a.txt'), '1')
    execSync('git add a.txt', { cwd: repoRoot })
    execSync('git commit -m "commit 1"', { cwd: repoRoot })

    const res = await probeIndexBasis(repoRoot, { indexedCommit: 'abc', indexedAtMs: 1000, baseRef: 'origin/HEAD' })
    expect(res.headCommit).toBeDefined()
    expect(res.mergeBase).toBeNull()
  })

  it('deadbeef indexedCommit -> changedFilesNotInIndex: null, dirtySinceIndex: true', async () => {
    const res = await probeIndexBasis(repoRoot, { indexedCommit: 'deadbeefdeadbeefdeadbeefdeadbeefdeadbeef', indexedAtMs: 1000, baseRef: 'HEAD' })
    expect(res.changedFilesNotInIndex).toBeNull()
    expect(res.dirtySinceIndex).toBe(true)
  })

  it('clean tree, indexedCommit is HEAD -> dirtySinceIndex: false', async () => {
    const head = execSync('git rev-parse HEAD', { cwd: repoRoot }).toString().trim()
    const res = await probeIndexBasis(repoRoot, { indexedCommit: head, indexedAtMs: Date.now() + 10000, baseRef: 'HEAD' })
    expect(res.changedFilesNotInIndex).toBe(0)
    expect(res.dirtySinceIndex).toBe(false)
  })

  it('dirty tree (uncommitted edit) -> dirtySinceIndex: true', async () => {
    const head = execSync('git rev-parse HEAD', { cwd: repoRoot }).toString().trim()
    fs.writeFileSync(path.join(repoRoot, 'a.txt'), '2')
    const res = await probeIndexBasis(repoRoot, { indexedCommit: head, indexedAtMs: Date.now() - 10000, baseRef: 'HEAD' })
    expect(res.changedFilesNotInIndex).toBe(1)
    expect(res.dirtySinceIndex).toBe(true)
  })

  it('new file -> dirtySinceIndex: true', async () => {
    const head = execSync('git rev-parse HEAD', { cwd: repoRoot }).toString().trim()
    fs.writeFileSync(path.join(repoRoot, 'b space unicode ✨.txt'), 'new')
    const res = await probeIndexBasis(repoRoot, { indexedCommit: head, indexedAtMs: Date.now() - 10000, baseRef: 'HEAD' })
    expect(res.changedFilesNotInIndex).toBe(2) // a.txt and b.txt
    expect(res.dirtySinceIndex).toBe(true)
  })

  it('git commands do not use forbidden flags', async () => {
    const head = execSync('git rev-parse HEAD', { cwd: repoRoot }).toString().trim()
    const argsSpy: string[][] = []
    
    await probeIndexBasis(repoRoot, { indexedCommit: head, indexedAtMs: 0, baseRef: 'HEAD' }, {
      git: async (args, cwd) => {
        argsSpy.push(args)
        return { stdout: '', stderr: '' }
      }
    })

    for (const args of argsSpy) {
      expect(args.includes('--path-format')).toBe(false)
      expect(args.includes('merge-tree')).toBe(false)
      expect(args.includes('--merge-base')).toBe(false)
    }
  })
})
