// src/relay/__tests__/agent-git-hooks-issue-handler.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { tmpdir } from 'node:os'
import { mkdtempSync, rmSync, mkdirSync, writeFileSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import {
  handleGitCheckHooks,
  handleGitReadIssueCommand,
  handleGitWriteIssueCommand,
  handleGitScanSetupScriptImports
} from '../agent-git-hooks-issue-handler'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

type GitHandlerTestResponse = {
  jsonrpc?: string
  id?: string | number | null
  result?: Record<string, unknown>
  error?: { code: number; message: string }
}

const mockConfig: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: '',
  agentToken: '',
  agentPort: 6799,
  devServerId: 'test',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/bin',
  toolEnv: { PATH: '/usr/bin:/usr/local/bin' },
  credentialDir: '/tmp/.creds',
  tlsRejectUnauthorized: true
}

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

let repoPath: string

beforeEach(() => {
  repoPath = mkdtempSync(join(tmpdir(), 'agent-git-hooks-issue-'))
})

afterEach(() => {
  rmSync(repoPath, { recursive: true, force: true })
  vi.clearAllMocks()
})

describe('handleGitCheckHooks', () => {
  it('reports installed hooks and orcaHooksCurrent=true when both orca hooks exist', async () => {
    const hooksDir = join(repoPath, '.git', 'hooks')
    mkdirSync(hooksDir, { recursive: true })
    writeFileSync(join(hooksDir, 'pre-commit'), '#!/bin/sh\n')
    writeFileSync(join(hooksDir, 'post-checkout'), '#!/bin/sh\n')
    writeFileSync(join(hooksDir, 'pre-commit.sample'), '#!/bin/sh\n')

    const resp = (await handleGitCheckHooks(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.orcaHooksCurrent).toBe(true)
    expect(resp.result!.installedHooks).toEqual(
      expect.arrayContaining(['pre-commit', 'post-checkout'])
    )
    expect(resp.result!.installedHooks).not.toEqual(expect.arrayContaining(['pre-commit.sample']))
  })

  it('reports orcaHooksCurrent=false when only one orca hook is installed', async () => {
    const hooksDir = join(repoPath, '.git', 'hooks')
    mkdirSync(hooksDir, { recursive: true })
    writeFileSync(join(hooksDir, 'pre-commit'), '#!/bin/sh\n')

    const resp = (await handleGitCheckHooks(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.orcaHooksCurrent).toBe(false)
  })

  it('errors when .git/hooks does not exist', async () => {
    const resp = (await handleGitCheckHooks(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.error!.message).toContain('git.checkHooks failed')
  })
})

describe('handleGitReadIssueCommand / handleGitWriteIssueCommand', () => {
  it('reports exists=false when the file has never been written', async () => {
    const resp = (await handleGitReadIssueCommand(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.exists).toBe(false)
    expect(resp.result!.content).toBe('')
  })

  it('write then read round-trips the content, creating .orca/ if absent', async () => {
    const writeResp = (await handleGitWriteIssueCommand(
      1,
      { repoPath, content: '{"issue":"ORCA-1"}' },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse
    expect(writeResp.error).toBeUndefined()
    expect(readFileSync(join(repoPath, '.orca', 'issue-command.json'), 'utf8')).toBe(
      '{"issue":"ORCA-1"}'
    )

    const readResp = (await handleGitReadIssueCommand(
      2,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse
    expect(readResp.result!.exists).toBe(true)
    expect(readResp.result!.content).toBe('{"issue":"ORCA-1"}')
  })
})

describe('handleGitScanSetupScriptImports', () => {
  it('returns an empty array when no setup script exists', async () => {
    const resp = (await handleGitScanSetupScriptImports(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.importedPaths).toEqual([])
  })

  it('prefers setup.sh over setup.ts/setup.js and collects source/import/require lines', async () => {
    const orcaDir = join(repoPath, '.orca')
    mkdirSync(orcaDir, { recursive: true })
    writeFileSync(
      join(orcaDir, 'setup.sh'),
      ['#!/bin/sh', 'source ./lib/env.sh', 'echo hello', 'import "./ignored-in-sh"'].join('\n')
    )
    writeFileSync(join(orcaDir, 'setup.ts'), 'import { x } from "./should-not-be-read"\n')

    const resp = (await handleGitScanSetupScriptImports(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.importedPaths).toEqual(['source ./lib/env.sh', 'import "./ignored-in-sh"'])
  })

  it('falls back to setup.js when neither setup.sh nor setup.ts exist', async () => {
    const orcaDir = join(repoPath, '.orca')
    mkdirSync(orcaDir, { recursive: true })
    writeFileSync(join(orcaDir, 'setup.js'), 'require("./config")\n')

    const resp = (await handleGitScanSetupScriptImports(
      1,
      { repoPath },
      mockConfig,
      mockLog
    )) as GitHandlerTestResponse

    expect(resp.result!.importedPaths).toEqual(['require("./config")'])
  })
})
