import { describe, expect, it, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { handleFsListDirectory, type DirectoryEntry } from './fs-agent-directory-browse'

// Why: fs-agent-directory-browse.ts is Part A's (direct-websocket/relay-websocket)
// implementation of fs.listDirectory, near-verbatim port of Part B's
// fs-handler-directory-browse.ts — see specs/agent/api/gaps-and-findings.md #5.
describe('handleFsListDirectory', () => {
  let tmpDir: string

  beforeEach(() => {
    tmpDir = mkdtempSync(path.join(tmpdir(), 'agent-fs-list-dir-'))
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('lists only subdirectories, skipping files', async () => {
    mkdirSync(path.join(tmpDir, 'repo-a'))
    mkdirSync(path.join(tmpDir, 'repo-b'))
    writeFileSync(path.join(tmpDir, 'readme.txt'), 'hi\n')

    const response = (await handleFsListDirectory(1, { path: tmpDir })) as {
      result?: { entries: DirectoryEntry[]; platform: string }
    }

    const names = response.result?.entries.map((e) => e.name).sort()
    expect(names).toEqual(['repo-a', 'repo-b'])
    expect(response.result?.platform).toBe(process.platform)
    expect(response.result?.entries.every((e) => e.isGitRepo === false)).toBe(true)
  })

  it('flags a subdirectory as isGitRepo when includeGitStatus is true and a .git folder exists', async () => {
    const gitRepoDir = path.join(tmpDir, 'has-git')
    mkdirSync(gitRepoDir)
    mkdirSync(path.join(gitRepoDir, '.git'))
    mkdirSync(path.join(tmpDir, 'no-git'))

    const response = (await handleFsListDirectory(1, {
      path: tmpDir,
      includeGitStatus: true
    })) as { result?: { entries: DirectoryEntry[] } }

    const byName = new Map(response.result?.entries.map((e) => [e.name, e]))
    expect(byName.get('has-git')?.isGitRepo).toBe(true)
    expect(byName.get('no-git')?.isGitRepo).toBe(false)
  })

  it('does not check for .git when includeGitStatus is false/omitted', async () => {
    const gitRepoDir = path.join(tmpDir, 'has-git')
    mkdirSync(gitRepoDir)
    mkdirSync(path.join(gitRepoDir, '.git'))

    const response = (await handleFsListDirectory(1, { path: tmpDir })) as {
      result?: { entries: DirectoryEntry[] }
    }

    expect(response.result?.entries[0]?.isGitRepo).toBe(false)
  })

  it('rejects a missing path param', async () => {
    const response = (await handleFsListDirectory(1, {})) as { error?: { message: string } }
    expect(response.error?.message).toContain('Missing required param: path')
  })

  it('reports an error for a non-existent directory', async () => {
    const response = (await handleFsListDirectory(1, {
      path: path.join(tmpDir, 'does-not-exist')
    })) as { error?: { message: string } }
    expect(response.error).toBeDefined()
  })
})
