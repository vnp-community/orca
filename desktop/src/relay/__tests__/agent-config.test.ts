// src/relay/__tests__/agent-config.test.ts
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { loadAgentConfig } from '../agent-config'

describe('loadAgentConfig', () => {
  beforeEach(() => {
    // Vitest sets MODE=test in its environment; default to a valid mode for tests
    // that are NOT testing MODE validation to prevent spurious throws.
    vi.stubEnv('MODE', 'direct-websocket')
  })
  afterEach(() => vi.unstubAllEnvs())

  // ── mode ────────────────────────────────────────────────────────────────────
  it('defaults to "direct-websocket" when MODE is not set', () => {
    vi.stubEnv('MODE', '')
    expect(loadAgentConfig().mode).toBe('direct-websocket')
  })

  it('returns "direct-websocket" when MODE=direct-websocket', () => {
    vi.stubEnv('MODE', 'direct-websocket')
    expect(loadAgentConfig().mode).toBe('direct-websocket')
  })

  it('returns "relay-websocket" when MODE=relay-websocket', () => {
    vi.stubEnv('MODE', 'relay-websocket')
    expect(loadAgentConfig().mode).toBe('relay-websocket')
  })

  it('throws with "Invalid MODE" message for unknown mode', () => {
    vi.stubEnv('MODE', 'ssh-tunnel')
    expect(() => loadAgentConfig()).toThrow('Invalid MODE')
  })

  it('throws for MODE=ftp', () => {
    vi.stubEnv('MODE', 'ftp')
    expect(() => loadAgentConfig()).toThrow()
  })

  // ── agentPort ───────────────────────────────────────────────────────────────
  it('parses AGENT_PORT as integer', () => {
    vi.stubEnv('AGENT_PORT', '7799')
    expect(loadAgentConfig().agentPort).toBe(7799)
  })

  it('defaults to 6799 when AGENT_PORT not set', () => {
    vi.stubEnv('AGENT_PORT', '')
    expect(loadAgentConfig().agentPort).toBe(6799)
  })

  // ── workDir ─────────────────────────────────────────────────────────────────
  it('uses AGENT_WORK_DIR when set', () => {
    vi.stubEnv('AGENT_WORK_DIR', '/custom/workdir')
    expect(loadAgentConfig().workDir).toBe('/custom/workdir')
  })

  it('defaults to process.cwd() when AGENT_WORK_DIR is empty', () => {
    vi.stubEnv('AGENT_WORK_DIR', '')
    expect(loadAgentConfig().workDir).toBe(process.cwd())
  })

  // ── toolPath ────────────────────────────────────────────────────────────────
  it('toolPath contains ~/.local/bin', () => {
    const { homedir } = require('node:os')
    expect(loadAgentConfig().toolPath).toContain(`${homedir()}/.local/bin`)
  })

  it('toolPath is colon-separated', () => {
    expect(loadAgentConfig().toolPath.split(':').length).toBeGreaterThan(1)
  })

  // ── toolEnv ─────────────────────────────────────────────────────────────────
  it('toolEnv.PATH equals toolPath', () => {
    const cfg = loadAgentConfig()
    expect(cfg.toolEnv.PATH).toBe(cfg.toolPath)
  })

  it('toolEnv.ANTHROPIC_API_KEY is set from env', () => {
    vi.stubEnv('ANTHROPIC_API_KEY', 'sk-ant-test')
    expect(loadAgentConfig().toolEnv.ANTHROPIC_API_KEY).toBe('sk-ant-test')
  })

  it('toolEnv.GITHUB_TOKEN is set from env', () => {
    vi.stubEnv('GITHUB_TOKEN', 'ghp_test')
    expect(loadAgentConfig().toolEnv.GITHUB_TOKEN).toBe('ghp_test')
  })

  // ── tlsRejectUnauthorized ────────────────────────────────────────────────────
  it('tlsRejectUnauthorized=true by default', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '')
    expect(loadAgentConfig().tlsRejectUnauthorized).toBe(true)
  })

  it('tlsRejectUnauthorized=false when NODE_TLS_REJECT_UNAUTHORIZED=0', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '0')
    expect(loadAgentConfig().tlsRejectUnauthorized).toBe(false)
  })

  it('tlsRejectUnauthorized=true when NODE_TLS_REJECT_UNAUTHORIZED=1', () => {
    vi.stubEnv('NODE_TLS_REJECT_UNAUTHORIZED', '1')
    expect(loadAgentConfig().tlsRejectUnauthorized).toBe(true)
  })

  // ── credentialDir ────────────────────────────────────────────────────────────
  it('credentialDir ends with .orca/credentials', () => {
    expect(loadAgentConfig().credentialDir).toMatch(/\.orca\/credentials$/)
  })

  it('credentialDir starts with homedir', () => {
    const { homedir } = require('node:os')
    expect(loadAgentConfig().credentialDir.startsWith(homedir())).toBe(true)
  })

  // ── devServerId ──────────────────────────────────────────────────────────────
  it('uses DEV_SERVER_ID from env', () => {
    vi.stubEnv('DEV_SERVER_ID', 'prod-server-01')
    expect(loadAgentConfig().devServerId).toBe('prod-server-01')
  })

  it('defaults to "dev-local" when DEV_SERVER_ID not set', () => {
    vi.stubEnv('DEV_SERVER_ID', '')
    expect(loadAgentConfig().devServerId).toBe('dev-local')
  })

  // ── agentToken ───────────────────────────────────────────────────────────────
  it('reads agentToken from AGENT_TOKEN env', () => {
    vi.stubEnv('AGENT_TOKEN', 'tok-abc123')
    expect(loadAgentConfig().agentToken).toBe('tok-abc123')
  })

  it('agentToken defaults to empty string when not set', () => {
    vi.stubEnv('AGENT_TOKEN', '')
    expect(loadAgentConfig().agentToken).toBe('')
  })
})
