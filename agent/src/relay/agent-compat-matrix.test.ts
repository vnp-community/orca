import { describe, expect, it, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'
import { spawn } from 'node:child_process'
import type * as ChildProcess from 'node:child_process'
import { tmpdir } from 'node:os'
import { createWireState, decodeFrame } from 'orca-dev-agent-transport'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleAgentExecPrompt } from './agent-print-mode-exec'
import { sendHandshake } from './agent-session-handshake'
import { dispatchAiRpc } from './agent-rpc-dispatch-ai'
import { AGENT_PROTOCOL_VERSION, AGENT_FEATURES } from './agent-protocol-features'
import { AGENT_BUILD_VERSION } from './agent-build-version'

// ── Mocks ────────────────────────────────────────────────────────────────────

vi.mock('./agent-credential-store', () => ({
  readDecryptedKey: vi.fn(async () => null)
}))

vi.mock('./agent-readonly-tool-policy', () => ({
  detectClaudeFlags: vi.fn(async () => ({
    tools: true,
    permissionMode: true,
    disallowedTools: true
  })),
  readonlyUnsupportedReason: vi.fn(() => null),
  readonlyUnsupportedError: vi.fn(() => ({ code: -32602, message: 'unsupported' })),
  buildReadonlyArgs: vi.fn(() => ['--permission-mode', 'dont-ask', '--tools', 'Read,Glob,Grep'])
}))

vi.mock('node:child_process', async (importOriginal) => {
  const actual = await importOriginal<typeof ChildProcess>()
  return { ...actual, spawn: vi.fn() }
})
const spawnMock = vi.mocked(spawn)

type FakeChild = EventEmitter & {
  stdout: EventEmitter
  stderr: EventEmitter
  kill: ReturnType<typeof vi.fn>
}
function createFakeChild(): FakeChild {
  return Object.assign(new EventEmitter(), {
    stdout: new EventEmitter(),
    stderr: new EventEmitter(),
    kill: vi.fn()
  })
}

async function waitForSpawn(): Promise<void> {
  for (let i = 0; i < 50; i++) {
    if (spawnMock.mock.calls.length > 0) return
    await Promise.resolve()
  }
  throw new Error('spawn() was never called')
}

const MOCK_CONFIG: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: '',
  agentToken: 'test-token',
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: tmpdir(),
  toolPath: '/usr/local/bin:/usr/bin:/bin',
  toolEnv: { PATH: '/usr/local/bin:/usr/bin:/bin' },
  credentialDir: tmpdir(),
  tlsRejectUnauthorized: true
}

const MOCK_LOGGER: AgentLogger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
  debug: vi.fn()
}

/**
 * Simulates a legacy Orca Dev Server Agent (< 2.2.0) that only understood
 * standard execPrompt parameters (prompt, model, workDir, env, timeoutMs)
 * and returned basic execution results without `applied` or other CR-033 metadata.
 */
export function legacyExecPromptEcho(params: Record<string, unknown>): Record<string, unknown> {
  return {
    stdout: `legacy output for: ${params.prompt ?? ''}`,
    stderr: '',
    exitCode: 0,
    timedOut: false,
    stepId: params.stepId
    // Note: `applied` is intentionally absent in legacy agents
  }
}

describe('agent compatibility matrix (CR-REQ-033 Task 13 / Solution 3.5)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    spawnMock.mockReset()
  })

  describe('1. New agent + legacy caller parameters', () => {
    it('returns standard execution frame without applied field when no new options provided', async () => {
      const fakeChild = createFakeChild()
      spawnMock.mockReturnValue(fakeChild as any)

      const promise = handleAgentExecPrompt(
        1,
        { prompt: 'legacy call', worktreePath: '/repo' },
        MOCK_CONFIG,
        MOCK_LOGGER
      )

      await waitForSpawn()
      fakeChild.stdout.emit('data', Buffer.from('output generated'))
      fakeChild.emit('close', 0)

      const res = (await promise) as any
      expect(res.jsonrpc).toBe('2.0')
      expect(res.id).toBe(1)
      expect(res.result).toBeDefined()
      expect(res.result.stdout).toBe('output generated')
      expect(res.result.exitCode).toBe(0)
      expect(res.result.timedOut).toBe(false)
      // When caller sends legacy parameters, applied is undefined to avoid unexpected keys
      expect(res.result.applied).toBeUndefined()
    })
  })

  describe('2. New agent + readonly accessMode (degradation detection signal)', () => {
    it('returns applied.accessMode === "readonly" when readonly requested', async () => {
      const fakeChild = createFakeChild()
      spawnMock.mockReturnValue(fakeChild as any)

      const promise = handleAgentExecPrompt(
        2,
        { prompt: 'readonly call', worktreePath: '/repo', accessMode: 'readonly' },
        MOCK_CONFIG,
        MOCK_LOGGER
      )

      await waitForSpawn()
      fakeChild.stdout.emit('data', Buffer.from('readonly output'))
      fakeChild.emit('close', 0)

      const res = (await promise) as any
      expect(res.jsonrpc).toBe('2.0')
      expect(res.id).toBe(2)
      expect(res.result.applied).toBeDefined()
      expect(res.result.applied.accessMode).toBe('readonly')
      expect(res.result.applied.workspaceKind).toBe('worktree')
    })
  })

  describe('3. Handshake protocol compatibility', () => {
    it('sends protocolVersion, buildVersion, features, and preserves agentVersion 5.0.0', async () => {
      const sentFrames: Buffer[] = []
      const fakeWs = {
        send: (data: Buffer) => sentFrames.push(Buffer.from(data))
      } as any

      const wireState = createWireState()
      const tools = [{ name: 'claude_code', description: 'Claude Code' }]

      await sendHandshake(
        fakeWs,
        wireState,
        MOCK_CONFIG,
        tools as any,
        MOCK_LOGGER,
        ['fs', 'git', 'preflight']
      )

      expect(sentFrames.length).toBe(1)
      const decoded = decodeFrame(wireState, sentFrames[0])
      expect(decoded).not.toBeNull()
      expect(decoded!.type).toBe(1) // Regular data frame

      const rpc = JSON.parse(decoded!.payload.toString('utf-8'))
      expect(rpc.jsonrpc).toBe('2.0')
      expect(rpc.method).toBe('agent.handshake')

      const params = rpc.params
      expect(params.agentVersion).toBe('5.0.0') // Design decision 2: preserved for Go compatibility
      expect(params.protocolVersion).toBe(AGENT_PROTOCOL_VERSION)
      expect(params.protocolVersion).toBe(2)
      expect(params.buildVersion).toBe(AGENT_BUILD_VERSION)
      expect(params.features).toEqual(expect.arrayContaining([...AGENT_FEATURES]))
      expect(params.features).toContain('agent.capabilities')
      expect(params.features).toContain('agent.execPrompt.readonly')
      expect(params.features).toContain('ai.complete.usage')
    })
  })

  describe('4. ai.complete backwards compatibility', () => {
    it('returns content and model even when maxTokens is omitted', async () => {
      vi.doMock('./ai-complete-handler', () => ({
        handleAIComplete: async (params: any) => ({
          content: 'generated text without explicit maxTokens',
          model: params.model ?? 'claude-3-5-sonnet',
          provider: 'anthropic',
          latencyMs: 95
        })
      }))

      const res = (await dispatchAiRpc(
        {
          jsonrpc: '2.0',
          id: 42,
          method: 'ai.complete',
          params: { prompt: 'generate commit message' }
        },
        MOCK_CONFIG,
        MOCK_LOGGER
      )) as any

      expect(res.jsonrpc).toBe('2.0')
      expect(res.id).toBe(42)
      expect(res.result.content).toBe('generated text without explicit maxTokens')
      expect(res.result.model).toBe('claude-3-5-sonnet')
      expect(res.result.provider).toBe('anthropic')
      expect(res.result.latencyMs).toBe(95)

      vi.doUnmock('./ai-complete-handler')
    })
  })

  describe('5. Legacy agent contract simulation (reference for backend)', () => {
    it('demonstrates how backend distinguishes new agent from legacy agent', async () => {
      const requestParams = {
        prompt: 'analyze security vulnerabilities',
        worktreePath: '/repo',
        accessMode: 'readonly' as const
      }

      // Legacy agent response
      const legacyResult = legacyExecPromptEcho(requestParams)
      // Backend test contract assertion:
      const legacyHonoredReadonly = (legacyResult.applied as any)?.accessMode === 'readonly'
      expect(legacyHonoredReadonly).toBe(false)
      expect(legacyResult.applied).toBeUndefined()

      // New agent response
      const fakeChild = createFakeChild()
      spawnMock.mockReturnValue(fakeChild as any)

      const promise = handleAgentExecPrompt(
        5,
        requestParams,
        MOCK_CONFIG,
        MOCK_LOGGER
      )
      await waitForSpawn()
      fakeChild.stdout.emit('data', Buffer.from('analysis complete'))
      fakeChild.emit('close', 0)

      const newAgentRes = (await promise) as any
      const newHonoredReadonly = newAgentRes.result.applied?.accessMode === 'readonly'
      expect(newHonoredReadonly).toBe(true)
    })
  })
})
