/**
 * Tests for the ADR-018 migration: ghExecFileAsync/glabExecFileAsync route
 * through a connection-scoped IHostedCliProvider (github.exec/gitlab.exec
 * RPC) instead of spawning gh/glab locally in the backend process — with NO
 * local-exec fallback, even when a repo has no dev-server connection at
 * all (an explicit product decision this session). See
 * specs/agent/api/gaps-and-findings.md.
 *
 * @module main/git/runner-hosted-cli-exec.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { ghExecFileAsync, glabExecFileAsync, redirectPortedHostnameToEnv } from './runner'
import {
  registerRemoteGithubCliProvider,
  unregisterRemoteGithubCliProvider,
  registerRemoteGitlabCliProvider,
  unregisterRemoteGitlabCliProvider
} from '../providers/hosted-cli-dispatch'
import type { IHostedCliProvider } from '../providers/types'

const CONNECTION_ID = 'dev-server-1'

function fakeProvider(exec: IHostedCliProvider['exec']): IHostedCliProvider {
  return { exec }
}

describe('ghExecFileAsync — ADR-018 routing', () => {
  afterEach(() => {
    unregisterRemoteGithubCliProvider(CONNECTION_ID)
  })

  it('throws GH_CLI_NO_DEV_SERVER_CONNECTION when no connectionId is given', async () => {
    await expect(ghExecFileAsync(['pr', 'view'])).rejects.toThrow(
      'GH_CLI_NO_DEV_SERVER_CONNECTION'
    )
  })

  it('throws GH_CLI_NO_DEV_SERVER_CONNECTION when the connectionId has no registered provider', async () => {
    await expect(
      ghExecFileAsync(['pr', 'view'], { connectionId: 'not-registered' })
    ).rejects.toThrow('GH_CLI_NO_DEV_SERVER_CONNECTION')
  })

  it('routes to the registered provider for the given connectionId', async () => {
    const exec = vi.fn().mockResolvedValue({ stdout: 'ok', stderr: '' })
    registerRemoteGithubCliProvider(CONNECTION_ID, fakeProvider(exec))

    const result = await ghExecFileAsync(['pr', 'view'], {
      connectionId: CONNECTION_ID,
      cwd: '/repo',
      userId: 'user-1'
    })

    expect(result).toEqual({ stdout: 'ok', stderr: '' })
    expect(exec).toHaveBeenCalledWith(
      ['pr', 'view'],
      '/repo',
      'user-1',
      expect.objectContaining({ idempotent: undefined })
    )
  })

  it('never spawns gh locally even when connectionId is absent (no fallback path exists)', async () => {
    // Regression guard for the "absolute prohibition" decision: previously
    // (assertLocalGhCliAllowed) this only threw under ORCA_MULTI_USER=1.
    const originalMultiUser = process.env['ORCA_MULTI_USER']
    delete process.env['ORCA_MULTI_USER']
    try {
      await expect(ghExecFileAsync(['auth', 'status'])).rejects.toThrow(
        'GH_CLI_NO_DEV_SERVER_CONNECTION'
      )
    } finally {
      if (originalMultiUser !== undefined) {process.env['ORCA_MULTI_USER'] = originalMultiUser}
    }
  })

  it('retries a transient error once for an idempotent call, then succeeds', async () => {
    const exec = vi
      .fn()
      .mockRejectedValueOnce(new Error('HTTP 502: Bad Gateway'))
      .mockResolvedValueOnce({ stdout: 'ok', stderr: '' })
    registerRemoteGithubCliProvider(CONNECTION_ID, fakeProvider(exec))

    const result = await ghExecFileAsync(['pr', 'view'], {
      connectionId: CONNECTION_ID,
      idempotent: true
    })

    expect(result).toEqual({ stdout: 'ok', stderr: '' })
    expect(exec).toHaveBeenCalledTimes(2)
  })

  it('does not retry a non-idempotent (write) call after a transient error', async () => {
    const exec = vi.fn().mockRejectedValue(new Error('HTTP 502: Bad Gateway'))
    registerRemoteGithubCliProvider(CONNECTION_ID, fakeProvider(exec))

    await expect(
      ghExecFileAsync(['api', '-X', 'POST', 'repos/acme/widgets/issues'], {
        connectionId: CONNECTION_ID
      })
    ).rejects.toThrow('HTTP 502')
    expect(exec).toHaveBeenCalledTimes(1)
  })
})

describe('glabExecFileAsync — ADR-018 routing', () => {
  afterEach(() => {
    unregisterRemoteGitlabCliProvider(CONNECTION_ID)
  })

  it('throws GLAB_CLI_NO_DEV_SERVER_CONNECTION when no connectionId is given', async () => {
    await expect(glabExecFileAsync(['mr', 'view'])).rejects.toThrow(
      'GLAB_CLI_NO_DEV_SERVER_CONNECTION'
    )
  })

  it('routes to the registered provider for the given connectionId', async () => {
    const exec = vi.fn().mockResolvedValue({ stdout: 'ok', stderr: '' })
    registerRemoteGitlabCliProvider(CONNECTION_ID, fakeProvider(exec))

    const result = await glabExecFileAsync(['mr', 'view'], {
      connectionId: CONNECTION_ID,
      cwd: '/repo'
    })

    expect(result).toEqual({ stdout: 'ok', stderr: '' })
    expect(exec).toHaveBeenCalledWith(['mr', 'view'], '/repo', undefined, expect.anything())
  })

  it('forwards a ported --hostname as GITLAB_HOST via the minimal env override', async () => {
    const exec = vi.fn().mockResolvedValue({ stdout: 'ok', stderr: '' })
    registerRemoteGitlabCliProvider(CONNECTION_ID, fakeProvider(exec))

    await glabExecFileAsync(['api', '--hostname', 'gitlab.example.com:8443', 'projects/1'], {
      connectionId: CONNECTION_ID
    })

    expect(exec).toHaveBeenCalledWith(
      ['api', 'projects/1'],
      undefined,
      undefined,
      expect.objectContaining({ env: { GITLAB_HOST: 'gitlab.example.com:8443' } })
    )
  })

  it('leaves a port-less --hostname untouched', async () => {
    const exec = vi.fn().mockResolvedValue({ stdout: 'ok', stderr: '' })
    registerRemoteGitlabCliProvider(CONNECTION_ID, fakeProvider(exec))

    await glabExecFileAsync(['api', '--hostname', 'gitlab.example.com', 'projects/1'], {
      connectionId: CONNECTION_ID
    })

    expect(exec).toHaveBeenCalledWith(
      ['api', '--hostname', 'gitlab.example.com', 'projects/1'],
      undefined,
      undefined,
      expect.anything()
    )
  })
})

describe('redirectPortedHostnameToEnv', () => {
  it('leaves args/options untouched when there is no --hostname flag', () => {
    const result = redirectPortedHostnameToEnv(['mr', 'view'], {})
    expect(result).toEqual({ args: ['mr', 'view'], options: {} })
  })

  it('leaves a port-less hostname untouched', () => {
    const result = redirectPortedHostnameToEnv(['api', '--hostname', 'gitlab.example.com', 'user'], {})
    expect(result.args).toEqual(['api', '--hostname', 'gitlab.example.com', 'user'])
  })

  it('strips a ported hostname and sets GITLAB_HOST, preserving any prior env', () => {
    const result = redirectPortedHostnameToEnv(
      ['api', '--hostname', 'gitlab.example.com:8443', 'user'],
      { env: { EXISTING: '1' } }
    )
    expect(result.args).toEqual(['api', 'user'])
    expect(result.options.env).toEqual({ EXISTING: '1', GITLAB_HOST: 'gitlab.example.com:8443' })
  })
})
