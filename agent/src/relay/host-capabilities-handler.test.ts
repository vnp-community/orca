// Why: locks in TASK-070's host.capabilities handler — the JSON-RPC method
// infra-fleet-service's GetHostCapabilities usecase relays via
// DevServerAgentClient.Exec(ctx, devServer, "host.capabilities", nil). Before
// this handler existed, every call permanently failed with
// INFRA_HOST_CAPABILITIES_UNSUPPORTED (see get_host_capabilities.go).
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { execFileAsyncMock } = vi.hoisted(() => ({
  execFileAsyncMock: vi.fn()
}))

const { isPwshAvailableMock, isWslAvailableMock, listWslDistrosMock, isGitBashAvailableMock } =
  vi.hoisted(() => ({
    isPwshAvailableMock: vi.fn(),
    isWslAvailableMock: vi.fn(),
    listWslDistrosMock: vi.fn(),
    isGitBashAvailableMock: vi.fn()
  }))

vi.mock('child_process', () => {
  const execFileWithPromisify = Object.assign(vi.fn(), {
    [Symbol.for('nodejs.util.promisify.custom')]: execFileAsyncMock
  })
  return { execFile: execFileWithPromisify }
})

vi.mock('../main/pwsh', () => ({ isPwshAvailable: isPwshAvailableMock }))
vi.mock('../main/wsl', () => ({
  isWslAvailable: isWslAvailableMock,
  listWslDistros: listWslDistrosMock
}))
vi.mock('../main/git-bash', () => ({ isGitBashAvailable: isGitBashAvailableMock }))

import { handleHostCapabilities } from './agent-preflight-handler'

beforeEach(() => {
  execFileAsyncMock.mockReset()
  isPwshAvailableMock.mockReset()
  isWslAvailableMock.mockReset()
  listWslDistrosMock.mockReset()
  isGitBashAvailableMock.mockReset()
})

describe('host.capabilities', () => {
  it('reports all capabilities available', async () => {
    isWslAvailableMock.mockReturnValue(true)
    listWslDistrosMock.mockReturnValue(['Ubuntu', 'Debian'])
    isPwshAvailableMock.mockReturnValue(true)
    execFileAsyncMock.mockResolvedValue({ stdout: '7.4.0\n' })
    isGitBashAvailableMock.mockReturnValue(true)

    const resp = (await handleHostCapabilities(1)) as {
      jsonrpc: string
      id: number
      result: {
        wslAvailable: boolean
        wslDistros: string[]
        pwshAvailable: boolean
        gitBashAvailable: boolean
      }
    }

    expect(resp).toEqual({
      jsonrpc: '2.0',
      id: 1,
      result: {
        wslAvailable: true,
        wslDistros: ['Ubuntu', 'Debian'],
        pwshAvailable: true,
        gitBashAvailable: true
      }
    })
  })

  it('reports all capabilities unavailable', async () => {
    isWslAvailableMock.mockReturnValue(false)
    isPwshAvailableMock.mockReturnValue(false)
    isGitBashAvailableMock.mockReturnValue(false)

    const resp = (await handleHostCapabilities(2)) as {
      result: {
        wslAvailable: boolean
        wslDistros: string[]
        pwshAvailable: boolean
        gitBashAvailable: boolean
      }
    }

    expect(resp.result).toEqual({
      wslAvailable: false,
      wslDistros: [],
      pwshAvailable: false,
      gitBashAvailable: false
    })
    // Why: wslDistros must not be probed when WSL itself is unavailable.
    expect(listWslDistrosMock).not.toHaveBeenCalled()
  })

  it('reports a partial mix of available capabilities', async () => {
    isWslAvailableMock.mockReturnValue(true)
    listWslDistrosMock.mockReturnValue(['Ubuntu'])
    isPwshAvailableMock.mockReturnValue(false)
    isGitBashAvailableMock.mockReturnValue(true)

    const resp = (await handleHostCapabilities(3)) as {
      result: {
        wslAvailable: boolean
        wslDistros: string[]
        pwshAvailable: boolean
        gitBashAvailable: boolean
      }
    }

    expect(resp.result).toEqual({
      wslAvailable: true,
      wslDistros: ['Ubuntu'],
      pwshAvailable: false,
      gitBashAvailable: true
    })
  })

  it('never returns extra fields beyond the 4-field contract', async () => {
    isWslAvailableMock.mockReturnValue(true)
    listWslDistrosMock.mockReturnValue([])
    isPwshAvailableMock.mockReturnValue(true)
    execFileAsyncMock.mockResolvedValue({ stdout: '7.4.0\n' })
    isGitBashAvailableMock.mockReturnValue(true)

    const resp = (await handleHostCapabilities(4)) as { result: Record<string, unknown> }

    expect(Object.keys(resp.result).sort()).toEqual([
      'gitBashAvailable',
      'pwshAvailable',
      'wslAvailable',
      'wslDistros'
    ])
  })
})
