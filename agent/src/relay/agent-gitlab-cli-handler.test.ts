import { describe, expect, it, vi, beforeEach } from 'vitest'
import { tmpdir } from 'node:os'
import type { AgentConfig } from './agent-config'
import { handleGitlabExec } from './agent-gitlab-cli-handler'

const execGlabCapturedMock = vi.hoisted(() =>
  vi.fn(async () => ({ stdout: 'ok', stderr: '', exitCode: 0 }))
)
const buildGlabEnvMock = vi.hoisted(() =>
  vi.fn((userId: string, base: NodeJS.ProcessEnv) => ({
    ...base,
    GLAB_CONFIG_DIR: `/home/x/.config/glab-cli/${userId}/`
  }))
)
vi.mock('./external-api-connector', () => ({
  execGlabCaptured: execGlabCapturedMock,
  buildGlabEnv: buildGlabEnvMock
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

// Why: agent-gitlab-cli-handler.ts is Part A's implementation of `gitlab.exec`
// — mirrors agent-github-cli-handler.ts for the ADR-018 migration. See
// specs/agent/api/gaps-and-findings.md.
describe('handleGitlabExec', () => {
  beforeEach(() => {
    execGlabCapturedMock.mockClear()
    buildGlabEnvMock.mockClear()
  })

  it('rejects a disallowed subcommand before calling execGlabCaptured', async () => {
    const result = (await handleGitlabExec(
      1,
      { args: ['admin', 'foo'] },
      MOCK_CONFIG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('is not allowed')
    expect(execGlabCapturedMock).not.toHaveBeenCalled()
  })

  it('rejects an out-of-allowlist api endpoint', async () => {
    const result = (await handleGitlabExec(
      1,
      { args: ['api', 'groups/acme/members'] },
      MOCK_CONFIG
    )) as { error?: { message: string } }

    expect(result.error?.message).toContain('not in the allowlist')
    expect(execGlabCapturedMock).not.toHaveBeenCalled()
  })

  it('executes an allowed call and isolates per-user credentials via buildGlabEnv', async () => {
    const result = (await handleGitlabExec(
      1,
      { args: ['mr', 'create', '--title', 'x'], cwd: '/repo', userId: 'user-7' },
      MOCK_CONFIG
    )) as { result?: { stdout: string; exitCode: number } }

    expect(result.result).toEqual({ stdout: 'ok', stderr: '', exitCode: 0 })
    expect(buildGlabEnvMock).toHaveBeenCalledWith('user-7', process.env)
    expect(execGlabCapturedMock).toHaveBeenCalledWith(
      ['mr', 'create', '--title', 'x'],
      expect.objectContaining({ cwd: '/repo' })
    )
  })

  it('falls back to config.workDir when cwd is omitted', async () => {
    await handleGitlabExec(1, { args: ['api', 'user'] }, MOCK_CONFIG)

    expect(execGlabCapturedMock).toHaveBeenCalledWith(
      ['api', 'user'],
      expect.objectContaining({ cwd: MOCK_CONFIG.workDir })
    )
  })

  it('surfaces a non-zero exitCode from execGlabCaptured without throwing', async () => {
    execGlabCapturedMock.mockResolvedValueOnce({ stdout: '', stderr: 'boom', exitCode: 1 })

    const result = (await handleGitlabExec(
      1,
      { args: ['issue', 'list'] },
      MOCK_CONFIG
    )) as { result?: { exitCode: number; stderr: string } }

    expect(result.result).toEqual({ stdout: '', stderr: 'boom', exitCode: 1 })
  })
})
