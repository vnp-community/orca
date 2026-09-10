import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getRuntimeCliInstallStatus,
  installRuntimeCli,
  removeRuntimeCli,
  getRuntimeWslCliInstallStatus,
  installRuntimeWslCli,
  removeRuntimeWslCli
} from './runtime-cli-client'
import type { CliInstallStatus } from '../../../shared/cli-install-types'

const cliGetInstallStatus = vi.fn()
const cliInstall = vi.fn()
const cliRemove = vi.fn()
const cliGetWslInstallStatus = vi.fn()
const cliInstallWsl = vi.fn()
const cliRemoveWsl = vi.fn()
const runtimeCall = vi.fn()

const fakeStatus = { supported: true } as unknown as CliInstallStatus

beforeEach(() => {
  cliGetInstallStatus.mockReset().mockResolvedValue(fakeStatus)
  cliInstall.mockReset().mockResolvedValue(fakeStatus)
  cliRemove.mockReset().mockResolvedValue(fakeStatus)
  cliGetWslInstallStatus.mockReset().mockResolvedValue(fakeStatus)
  cliInstallWsl.mockReset().mockResolvedValue(fakeStatus)
  cliRemoveWsl.mockReset().mockResolvedValue(fakeStatus)
  runtimeCall.mockReset()
  vi.stubGlobal('window', {
    api: {
      cli: {
        getInstallStatus: cliGetInstallStatus,
        install: cliInstall,
        remove: cliRemove,
        getWslInstallStatus: cliGetWslInstallStatus,
        installWsl: cliInstallWsl,
        removeWsl: cliRemoveWsl
      },
      // Why: regression guard — these functions used to route through
      // window.api.runtime.call (a real network call to backend-go on the
      // web build, which has no cli.* channels and always threw "not yet
      // implemented"). Asserting this is never called catches a regression
      // back to that indirection.
      runtime: { call: runtimeCall }
    }
  })
})

describe('runtime-cli-client', () => {
  it('getRuntimeCliInstallStatus calls window.api.cli.getInstallStatus directly, not runtime.call', async () => {
    const result = await getRuntimeCliInstallStatus()
    expect(cliGetInstallStatus).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
    expect(result).toBe(fakeStatus)
  })

  it('installRuntimeCli calls window.api.cli.install directly', async () => {
    await installRuntimeCli()
    expect(cliInstall).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
  })

  it('removeRuntimeCli calls window.api.cli.remove directly', async () => {
    await removeRuntimeCli()
    expect(cliRemove).toHaveBeenCalledWith()
    expect(runtimeCall).not.toHaveBeenCalled()
  })

  it('getRuntimeWslCliInstallStatus calls window.api.cli.getWslInstallStatus directly with args', async () => {
    await getRuntimeWslCliInstallStatus({ distro: 'Ubuntu' })
    expect(cliGetWslInstallStatus).toHaveBeenCalledWith({ distro: 'Ubuntu' })
    expect(runtimeCall).not.toHaveBeenCalled()
  })

  it('installRuntimeWslCli calls window.api.cli.installWsl directly with args', async () => {
    await installRuntimeWslCli({ distro: 'Ubuntu' })
    expect(cliInstallWsl).toHaveBeenCalledWith({ distro: 'Ubuntu' })
    expect(runtimeCall).not.toHaveBeenCalled()
  })

  it('removeRuntimeWslCli calls window.api.cli.removeWsl directly with args', async () => {
    await removeRuntimeWslCli({ distro: 'Ubuntu' })
    expect(cliRemoveWsl).toHaveBeenCalledWith({ distro: 'Ubuntu' })
    expect(runtimeCall).not.toHaveBeenCalled()
  })
})
