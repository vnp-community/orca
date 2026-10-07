import { describe, it, expect } from 'vitest'
import { buildReindexArgv, ReindexCommand } from './codeintel-reindex-commands'

describe('codeintel-reindex-commands', () => {
  it('TestReindexArgvIsIndexOnly', () => {
    const gitnexusCmds: ReindexCommand[] = [
      { tool: 'gitnexus', mode: 'full', repoRoot: '/repo' },
      { tool: 'gitnexus', mode: 'incremental', repoRoot: '/repo' }
    ]

    for (const cmd of gitnexusCmds) {
      const argv = buildReindexArgv(cmd)
      expect(argv[0]).toBe('analyze')
      expect(argv).toContain('--index-only')
      expect(argv).not.toContain('--embeddings')
      expect(argv).not.toContain('--skills')
      expect(argv).not.toContain('--name')
      expect(argv).not.toContain('--branch')
      expect(argv).not.toContain('--drop-embeddings')
    }
  })

  it('builds codegraph reindex argv', () => {
    const argv = buildReindexArgv({ tool: 'codegraph', mode: 'full', projectPath: '/repo' })
    expect(argv).toEqual(['reindex', '-p', '/repo', '--force'])
  })
})
