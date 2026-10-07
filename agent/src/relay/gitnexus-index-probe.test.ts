import { describe, it, expect, vi, beforeEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import { gitnexusIndexProbe } from './gitnexus-index-probe'
import * as RepoRes from './codeintel-repo-resolution'
import * as RegistryReader from './gitnexus-registry-reader'
import * as HeadCommit from './codeintel-head-commit'

vi.mock('fs', () => ({
  default: {
    statSync: vi.fn(),
    readdirSync: vi.fn(),
    readFileSync: vi.fn()
  }
}))

vi.mock('./codeintel-repo-resolution', () => ({
  resolveCodeIntelRepo: vi.fn()
}))

vi.mock('./gitnexus-registry-reader', () => ({
  readGitNexusRegistry: vi.fn(),
  findRegistryEntry: vi.fn()
}))

vi.mock('./codeintel-head-commit', () => ({
  getHeadCommit: vi.fn()
}))

describe('gitnexus-index-probe', () => {
  const mockCtx: any = { config: {}, log: {} }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(RepoRes.resolveCodeIntelRepo).mockResolvedValue({
      toplevel: '/repo',
      gitNexusRegistryPath: '/repo/.gitnexus',
      linkedWorktree: false,
      worktreeMismatch: false,
      stale: false
    })
    vi.mocked(HeadCommit.getHeadCommit).mockResolvedValue('commit1')
  })

  it('returns missing if no entry', async () => {
    vi.mocked(RegistryReader.findRegistryEntry).mockReturnValue(undefined)
    const res = await gitnexusIndexProbe('/repo', mockCtx)
    expect(res.state).toBe('missing')
  })

  it('returns missing if no lbug', async () => {
    vi.mocked(RegistryReader.findRegistryEntry).mockReturnValue({ path: '/repo' })
    vi.mocked(fs.statSync).mockImplementation((p: any) => {
      if (p.endsWith('lbug')) throw new Error('ENOENT')
      return { mtimeMs: 1, isFile: () => true } as any
    })
    const res = await gitnexusIndexProbe('/repo', mockCtx)
    expect(res.state).toBe('missing')
  })

  it('returns ready and reads meta.json and indicators', async () => {
    vi.mocked(RegistryReader.findRegistryEntry).mockReturnValue({
      path: '/repo',
      lastCommit: 'commit1',
      indexedAt: 12345,
      stats: { files: 10 }
    })
    vi.mocked(fs.statSync).mockImplementation((p: any) => {
      return { mtimeMs: 100, isFile: () => true } as any
    })
    vi.mocked(fs.readFileSync).mockImplementation((p: any) => {
      if (p.endsWith('meta.json')) return JSON.stringify({ schemaVersion: 5 })
      return ''
    })
    vi.mocked(fs.readdirSync).mockImplementation((p: any) => {
      return ['lbug', 'lbug.wal.missing-shadow.1', 'lbug.wal.missing-shadow.2'] as any
    })

    const res = await gitnexusIndexProbe('/repo', mockCtx)
    expect(res.state).toBe('ready')
    expect(res.indexedCommit).toBe('commit1')
    expect(res.schemaVersion).toBe(5)
    expect(res.indicators).toContain('wal_missing_shadow_files:2')
    expect(res.stats).toEqual({ files: 10 })
  })

  it('returns stale if commit mismatch', async () => {
    vi.mocked(RegistryReader.findRegistryEntry).mockReturnValue({
      path: '/repo',
      lastCommit: 'commit2' // different from commit1
    })
    vi.mocked(fs.statSync).mockImplementation((p: any) => {
      return { mtimeMs: 100, isFile: () => true } as any
    })
    vi.mocked(fs.readdirSync).mockReturnValue([] as any)

    const res = await gitnexusIndexProbe('/repo', mockCtx)
    expect(res.state).toBe('stale')
  })
})
