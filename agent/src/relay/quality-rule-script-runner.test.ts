import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { runScriptRules } from './quality-rule-script-runner'
import { RulePack } from './quality-rule-pack-schema'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

describe('quality-rule-script-runner', () => {
  let tmpDir: string
  let runDir: string

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-test-'))
    runDir = path.join(tmpDir, 'run')
    fs.mkdirSync(path.join(tmpDir, 'config/scripts'), { recursive: true })
    fs.mkdirSync(path.join(tmpDir, 'desktop/config/scripts'), { recursive: true })
  })

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true })
  })

  it('handles exit code 0', async () => {
    const scriptPath = path.join(tmpDir, 'config/scripts/good.mjs')
    fs.writeFileSync(scriptPath, 'console.log("ok"); process.exit(0);')

    const pack: RulePack = {
      rules: [
        {
          id: 'ORCA-004',
          title: 'Test script',
          kind: 'script',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
          script: { name: 'good', cwd: 'repo', args: [] },
          message: 'Failed',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }

    const res = await runScriptRules(pack, [{ path: 'test.ts', status: 'A', untracked: false, binary: false, addedLines: [] }], ['ORCA-004'], {
      repoRoot: tmpDir,
      runDir,
      signal: new AbortController().signal
    })

    expect(res.ruleResults).toHaveLength(1)
    expect(res.ruleResults[0].status).toBe('ran')
    expect(res.findings).toHaveLength(0)
  })

  it('handles exit code 1 and redacts secret', async () => {
    const scriptPath = path.join(tmpDir, 'desktop/config/scripts/bad.mjs')
    fs.writeFileSync(scriptPath, 'console.log("has secret ghp_123456789012345678901234567890123456"); process.exit(1);')

    const pack: RulePack = {
      rules: [
        {
          id: 'ORCA-005',
          title: 'Test script',
          kind: 'script',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
          script: { name: 'bad', cwd: 'desktop', args: ['--check'] },
          message: 'Failed bad script',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }

    const res = await runScriptRules(pack, [{ path: 'test.ts', status: 'A', untracked: false, binary: false, addedLines: [] }], ['ORCA-005'], {
      repoRoot: tmpDir,
      runDir,
      signal: new AbortController().signal
    })

    expect(res.ruleResults).toHaveLength(1)
    expect(res.ruleResults[0].status).toBe('ran')
    expect(res.findings).toHaveLength(1)
    expect(res.findings[0].message).toContain('Failed bad script')
    expect(res.findings[0].message).toContain('***')
    expect(res.findings[0].message).not.toContain('ghp_123456789012345678901234567890123456')
    expect(res.findings[0].anchorOverride).toBe('bad')
  })

  it('handles missing script', async () => {
    const pack: RulePack = {
      rules: [
        {
          id: 'ORCA-006',
          title: 'Missing',
          kind: 'script',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
          script: { name: 'missing', cwd: 'repo', args: [] },
          message: 'Failed',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }

    const res = await runScriptRules(pack, [{ path: 'test.ts', status: 'A', untracked: false, binary: false, addedLines: [] }], ['ORCA-006'], {
      repoRoot: tmpDir,
      runDir,
      signal: new AbortController().signal
    })

    expect(res.ruleResults[0].status).toBe('script_not_found')
    expect(res.findings).toHaveLength(0)
  })

  it('handles Cannot find module (env_not_ready)', async () => {
    const scriptPath = path.join(tmpDir, 'config/scripts/module.mjs')
    fs.writeFileSync(scriptPath, 'import "does-not-exist"; process.exit(1);')

    const pack: RulePack = {
      rules: [
        {
          id: 'ORCA-004',
          title: 'Module',
          kind: 'script',
          category: 'convention',
          severity: 'error',
          enabled: true,
          scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
          script: { name: 'module', cwd: 'repo', args: [] },
          message: 'Failed',
          fixHint: '',
          source: { doc: '' }
        }
      ]
    }

    const res = await runScriptRules(pack, [{ path: 'test.ts', status: 'A', untracked: false, binary: false, addedLines: [] }], ['ORCA-004'], {
      repoRoot: tmpDir,
      runDir,
      signal: new AbortController().signal
    })

    expect(res.ruleResults[0].status).toBe('env_not_ready')
    expect(res.findings).toHaveLength(0)
  })
})
