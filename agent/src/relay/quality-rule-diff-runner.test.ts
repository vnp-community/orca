import { describe, it, expect } from 'vitest'
import { runDiffRules } from './quality-rule-diff-runner'
import { ORCA_RULE_PACK } from './quality-rule-pack-orca'
import { ChangedFileDiff } from './quality-diff-changed-lines'

describe('quality-rule-diff-runner', () => {
  it('runs ORCA-012 correctly', async () => {
    const files: ChangedFileDiff[] = [
      {
        path: 'src/utils.ts',
        status: 'A',
        untracked: false,
        binary: false,
        addedLines: []
      },
      {
        path: 'src/helpers.ts',
        status: 'M', // Modified, so ORCA-012 should ignore it because it's not Added/Renamed
        untracked: false,
        binary: false,
        addedLines: []
      }
    ]

    const deps = {
      now: () => Date.now(),
      budgetMsPerLine: 5,
      readFile: () => '',
      cwd: '',
      mergeBaseOf: ''
    }

    const enabledIds = ['ORCA-012']
    const res = await runDiffRules(ORCA_RULE_PACK, files, enabledIds, deps)
    
    expect(res.findings).toHaveLength(1)
    expect(res.findings[0].file).toBe('src/utils.ts')
    expect(res.ruleResults.find(r => r.ruleId === 'ORCA-012')?.status).toBe('ran')
  })

  it('runs ORCA-014 correctly', async () => {
    const files: ChangedFileDiff[] = [
      {
        path: 'desktop/src/renderer/bad.tsx',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'e.metaKey' }]
      },
      {
        path: 'desktop/src/renderer/good.tsx',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'e.metaKey' }]
      }
    ]

    const deps = {
      now: () => Date.now(),
      budgetMsPerLine: 5,
      readFile: (p: string) => p.includes('good') ? 'isMac' : '',
      cwd: '',
      mergeBaseOf: ''
    }

    const enabledIds = ['ORCA-014']
    const res = await runDiffRules(ORCA_RULE_PACK, files, enabledIds, deps)
    
    expect(res.findings).toHaveLength(1)
    expect(res.findings[0].file).toBe('desktop/src/renderer/bad.tsx')
  })
})
