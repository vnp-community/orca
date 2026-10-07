import { describe, it, expect, vi, beforeEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { readGitNexusRegistry, findRegistryEntry } from './gitnexus-registry-reader'
import { CodeIntelError } from './codeintel-errors'
import { AgentConfig } from './agent-config'

describe('gitnexus-registry-reader', () => {
  const tmpDir = os.tmpdir()
  let mockConfig: AgentConfig
  let homeDir: string

  beforeEach(() => {
    homeDir = fs.mkdtempSync(path.join(tmpDir, 'orca-test-home-'))
    mockConfig = {
      toolEnv: { HOME: homeDir }
    } as any
  })

  it('returns empty array if file missing', () => {
    const res = readGitNexusRegistry(mockConfig)
    expect(res).toEqual([])
  })

  it('throws registry_unreadable if file is invalid json', () => {
    fs.mkdirSync(path.join(homeDir, '.gitnexus'), { recursive: true })
    fs.writeFileSync(path.join(homeDir, '.gitnexus', 'registry.json'), 'invalid json')
    
    try {
      readGitNexusRegistry(mockConfig)
      expect.fail()
    } catch (e: any) {
      expect(e).toBeInstanceOf(CodeIntelError)
      expect(e.data.reason).toBe('registry_unreadable')
    }
  })

  it('throws registry_unreadable if not array', () => {
    fs.mkdirSync(path.join(homeDir, '.gitnexus'), { recursive: true })
    fs.writeFileSync(path.join(homeDir, '.gitnexus', 'registry.json'), '{"path": "/test"}')
    
    expect(() => readGitNexusRegistry(mockConfig)).toThrowError(CodeIntelError)
  })

  it('returns parsed array and caches it', () => {
    fs.mkdirSync(path.join(homeDir, '.gitnexus'), { recursive: true })
    const data = [{ path: '/test1', indexedAt: 100 }, { path: '/test2', indexedAt: 200 }]
    fs.writeFileSync(path.join(homeDir, '.gitnexus', 'registry.json'), JSON.stringify(data))
    
    const res1 = readGitNexusRegistry(mockConfig)
    expect(res1).toHaveLength(2)

    const res2 = readGitNexusRegistry(mockConfig)
    expect(res2).toBe(res1)
  })

  it('findRegistryEntry resolves realpath and handles duplicates', () => {
    const repoDir = fs.mkdtempSync(path.join(tmpDir, 'repo-'))
    const symlinkDir = path.join(tmpDir, 'sym-repo-' + Date.now())
    fs.symlinkSync(repoDir, symlinkDir)

    const entries = [
      { path: '/nonexistent', indexedAt: 100 },
      { path: symlinkDir, indexedAt: 200 },
      { path: repoDir, indexedAt: 100 }
    ]

    const warnings: string[] = []
    const log = { warn: (msg: string) => warnings.push(msg) }

    const realToplevel = fs.realpathSync.native(repoDir)
    const match = findRegistryEntry(entries, realToplevel, log)

    expect(match?.path).toBe(symlinkDir)
    expect(warnings).toHaveLength(1)
    expect(warnings[0]).toContain('registry_duplicate_path')
  })
})
