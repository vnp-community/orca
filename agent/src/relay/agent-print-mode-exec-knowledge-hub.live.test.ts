import { describe, expect, it } from 'vitest'
import { mkdtemp, readdir, rm } from 'node:fs/promises'
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
