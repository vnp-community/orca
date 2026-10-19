import { describe, expect, it } from 'vitest'
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleAgentExecPrompt } from './agent-print-mode-exec'

// Runs the genuine claude and tdd-agent against a real project; opt in with ORCA_LIVE_CLAUDE=1 and
// ORCA_LIVE_KNOWLEDGE_ROOT=<absolute path of a checkout that has .tdd-agent/>. Never point it at a
// checkout whose knowledge graph you care about: the server is read-only, but keep a copy anyway.
const root = process.env['ORCA_LIVE_KNOWLEDGE_ROOT'] ?? ''
const LOG: AgentLogger = { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} }

describe.skipIf(process.env['ORCA_LIVE_CLAUDE'] !== '1' || root === '')(
  'agent.execPrompt with a knowledge root (live claude + tdd-agent)',
  () => {
    it('lets claude call kg_search through the config the agent built, then removes the config', async () => {
      const workDir = await mkdtemp(path.join(tmpdir(), 'orca-live-kh-'))
      const config = {
        mode: 'direct-websocket',
        orcaUrl: '',
        orcaHttpUrl: '',
        apiSecret: '',
        agentToken: '',
        agentPort: 0,
        devServerId: 'live',
        logLevel: 'info',
        workDir,
        toolPath: process.env['PATH'] ?? '',
        toolEnv: { PATH: process.env['PATH'] ?? '' },
        credentialDir: workDir,
        tlsRejectUnauthorized: true
      } as unknown as AgentConfig
      try {
        const res = (await handleAgentExecPrompt(
          1,
          {
            prompt:
              'Call the MCP tool kg_search of the tdd-agent server with the query "FR-SYS-07", then reply with exactly the id of the first result and nothing else.',
            worktreePath: workDir,
            workspaceKind: 'scratch',
            accessMode: 'readonly',
            knowledgeRoot: root,
            timeoutMs: 240_000
          },
          config,
          LOG
        )) as { result?: { stdout: string; exitCode: number | null }; error?: unknown }
        expect(res.error).toBeUndefined()
        expect(res.result?.exitCode).toBe(0)
        expect(res.result?.stdout).toContain('FR-SYS-07')
        expect(await readdir(path.join(workDir, 'tmp'))).toEqual([])
      } finally {
        await rm(workDir, { recursive: true, force: true })
      }
    }, 300_000)
  }
)

describe.skipIf(process.env['ORCA_LIVE_CLAUDE'] !== '1')(
  'agent.execPrompt with the openspec-author profile (live claude)',
  () => {
    it('can write a file and run openspec, but cannot write outside the workspace or reach the network', async () => {
      const workDir = await mkdtemp(path.join(tmpdir(), 'orca-live-prof-'))
      const config = {
        mode: 'direct-websocket',
        orcaUrl: '',
        orcaHttpUrl: '',
        apiSecret: '',
        agentToken: '',
        agentPort: 0,
        devServerId: 'live',
        logLevel: 'info',
        workDir,
        toolPath: process.env['PATH'] ?? '',
        toolEnv: { PATH: process.env['PATH'] ?? '' },
        credentialDir: workDir,
        tlsRejectUnauthorized: true
      } as unknown as AgentConfig
      try {
        const outside = path.join(tmpdir(), `orca-live-evil-${path.basename(workDir)}.txt`)
        const res = (await handleAgentExecPrompt(
          1,
          {
            prompt: `Do these in order and report each result in one line: (1) create the file note.txt containing the word hi with the Write tool; (2) run 'openspec --version' with Bash; (3) run 'touch ${outside}' with Bash; (4) run 'curl --version' with Bash. If a tool call is denied, say DENIED for that step.`,
            worktreePath: workDir,
            workspaceKind: 'scratch',
            accessMode: 'write',
            permissionProfile: 'openspec-author',
            timeoutMs: 240_000
          },
          config,
          LOG
        )) as { result?: { stdout: string; exitCode: number | null }; error?: unknown }
        expect(res.error).toBeUndefined()
        expect(res.result?.exitCode).toBe(0)
        expect(await readFile(path.join(workDir, 'note.txt'), 'utf8')).toContain('hi')
        const out = res.result?.stdout ?? ''
        expect(out).toMatch(/\d+\.\d+\.\d+/)
        expect(out.toUpperCase()).toContain('DENIED')
        // acceptEdits lets claude touch files inside the working directory (the proposal scope check polices which); outside it, and the network, stay refused.
        await expect(readFile(outside, 'utf8')).rejects.toThrow()
      } finally {
        await rm(workDir, { recursive: true, force: true })
      }
    }, 300_000)
  }
)
