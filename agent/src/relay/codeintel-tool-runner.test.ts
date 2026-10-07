import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { runCodeIntelTool } from './codeintel-tool-runner'
import { createConcurrencyGate } from './codeintel-concurrency-gate'
import { AgentConfig } from './agent-config'
import { CodeIntelError } from './codeintel-errors'

describe('codeintel-tool-runner', () => {
  const tmpDir = os.tmpdir()
  let mockConfig: AgentConfig

  beforeEach(() => {
    mockConfig = {
      mode: 'direct-websocket',
      orcaUrl: 'wss://test',
      agentToken: 'tok',
      agentPort: 6799,
      devServerId: 'test-server',
      logLevel: 'info',
      workDir: '/tmp',
      toolPath: process.env.PATH || '',
      toolEnv: { PATH: process.env.PATH || '', HOME: '/home/test' },
      credentialDir: '/home/test/.orca/credentials',
      tlsRejectUnauthorized: true,
    } as any
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('runs tool and returns result', async () => {
    const gate = createConcurrencyGate({ maxTotal: 3, perTool: {}, queueMax: 10, queueWaitMs: 10000 })
    const res = await runCodeIntelTool(
      ['-e', 'console.log("hello")'],
      '/tmp',
      { ...mockConfig, toolPath: process.execPath }, // point to node
      gate,
      { tool: 'codegraph', deadline: Date.now() + 5000 }
    )

    expect(res.exitCode).toBe(0)
    expect(res.stdout).toBe('hello\n')
    expect(res.durationMs).toBeGreaterThanOrEqual(0)
  })

  it('throws CODEINTEL_TIMEOUT if deadline exceeded before starting', async () => {
    const gate = createConcurrencyGate({ maxTotal: 3, perTool: {}, queueMax: 10, queueWaitMs: 10000 })
    await expect(runCodeIntelTool(
      [],
      '/tmp',
      mockConfig,
      gate,
      { tool: 'gitnexus', deadline: Date.now() - 1000 }
    )).rejects.toThrowError(CodeIntelError)
  })

  it('uses temp file for gitnexus', async () => {
    const gate = createConcurrencyGate({ maxTotal: 3, perTool: {}, queueMax: 10, queueWaitMs: 10000 })
    
    const readdirSpy = vi.spyOn(fs, 'readdirSync')
    
    const res = await runCodeIntelTool(
      ['-e', 'console.log("temp file test")'],
      '/tmp',
      { ...mockConfig, toolPath: process.execPath },
      gate,
      { tool: 'gitnexus', deadline: Date.now() + 5000 }
    )

    expect(res.exitCode).toBe(0)
    expect(res.stdout).toBe('temp file test\n')
  })

  it('throws CODEINTEL_TOOL_FAILED if tool exits with error', async () => {
    const gate = createConcurrencyGate({ maxTotal: 3, perTool: {}, queueMax: 10, queueWaitMs: 10000 })
    await expect(runCodeIntelTool(
      ['-e', 'process.exit(1)'],
      '/tmp',
      { ...mockConfig, toolPath: process.execPath },
      gate,
      { tool: 'codegraph', deadline: Date.now() + 5000 }
    )).rejects.toThrowError(/Tool exited with non-zero code/)
  })
})
