import { describe, expect, it, vi, beforeEach } from 'vitest'
import { tmpdir } from 'node:os'
import type { AgentConfig } from './agent-config'
import { handleGithubExec } from './agent-github-cli-handler'

const execGhCapturedMock = vi.hoisted(() =>
  vi.fn(async () => ({ stdout: 'ok', stderr: '', exitCode: 0 }))
)
const buildGhEnvMock = vi.hoisted(() =>
  vi.fn((userId: string, base: NodeJS.ProcessEnv) => ({ ...base, GH_CONFIG_DIR: `/home/x/.config/gh/${userId}/` }))
)
vi.mock('./external-api-connector', () => ({
  execGhCaptured: execGhCapturedMock,
  buildGhEnv: buildGhEnvMock
}))

const MOCK_CONFIG: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: '',
  agentToken: '',
  agentPort: 6799,
  devServerId: 'test-server',
  logLevel: 'info',
  workDir: tmpdir(),
  toolPath: '/usr/local/bin:/usr/bin:/bin',
  toolEnv: { PATH: '/usr/local/bin:/usr/bin:/bin' },
  credentialDir: tmpdir(),
  tlsRejectUnauthorized: true
}

// Why: agent-github-cli-handler.ts is Part A's implementation of `github.exec`
// — the ADR-018 migration moving gh CLI execution out of backend/ into
// agent/. See specs/agent/api/gaps-and-findings.md.
describe('handleGithubExec', () => {
  beforeEach(() => {
    execGhCapturedMock.mockClear()
    buildGhEnvMock.mockClear()
  })

  it('rejects a disallowed subcommand before calling execGhCaptured', async () => {
    const result = (await handleGithubExec(
      1,
      { args: ['admin', 'foo'] },
      MOCK_CONFIG
    )) as { error?: { code: number; message: string } }

    expect(result.error?.message).toContain('is not allowed')
    expect(execGhCapturedMock).not.toHaveBeenCalled()
  })

  it('rejects an out-of-allowlist api endpoint', async () => {
    const result = (await handleGithubExec(
      1,
      { args: ['api', 'orgs/acme/members'] },
      MOCK_CONFIG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('not in the allowlist')
    expect(execGhCapturedMock).not.toHaveBeenCalled()
  })

  it('executes an allowed call and isolates per-user credentials via buildGhEnv', async () => {
    const result = (await handleGithubExec(
      1,
      { args: ['pr', 'create', '--title', 'x'], cwd: '/repo', userId: 'user-42' },
      MOCK_CONFIG
    )) as { result?: { stdout: string; exitCode: number } }

    expect(result.result).toEqual({ stdout: 'ok', stderr: '', exitCode: 0 })
    expect(buildGhEnvMock).toHaveBeenCalledWith('user-42', process.env)
    expect(execGhCapturedMock).toHaveBeenCalledWith(
      ['pr', 'create', '--title', 'x'],
      expect.objectContaining({ cwd: '/repo' })
    )
  })

  it('falls back to config.workDir when cwd is omitted', async () => {
    await handleGithubExec(1, { args: ['api', 'rate_limit'] }, MOCK_CONFIG)

    expect(execGhCapturedMock).toHaveBeenCalledWith(
      ['api', 'rate_limit'],
      expect.objectContaining({ cwd: MOCK_CONFIG.workDir })
    )
  })

  it('surfaces a non-zero exitCode from execGhCaptured without throwing', async () => {
    execGhCapturedMock.mockResolvedValueOnce({ stdout: '', stderr: 'boom', exitCode: 1 })

    const result = (await handleGithubExec(
      1,
      { args: ['issue', 'list'] },
      MOCK_CONFIG
    )) as { result?: { exitCode: number; stderr: string } }

    expect(result.result).toEqual({ stdout: '', stderr: 'boom', exitCode: 1 })
  })
})
