import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  checkRuntimeHooks,
  inspectRuntimeSetupScriptImports,
  readRuntimeIssueCommand,
  writeRuntimeIssueCommand
} from './runtime-hooks-client'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from './runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from './runtime-rpc-client'

const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()
const runtimeLocalCall = vi.fn()
const hooksCheck = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()
  runtimeLocalCall.mockReset()
  hooksCheck.mockReset()
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })
  vi.stubGlobal('window', {
    api: {
      runtime: { call: runtimeLocalCall },
      runtimeEnvironments: { call: runtimeEnvironmentTransportCall },
      hooks: {
        check: hooksCheck
      }
    }
  })
})

describe('runtime hooks client', () => {
  it('routes setupScriptImports/issueCommand hook operations through the local RPC registry when no runtime environment is active', async () => {
    runtimeLocalCall.mockResolvedValue({
      id: 'rpc-1',
      ok: true,
      result: { ok: true },
      _meta: { runtimeId: 'runtime-1' }
    })
    hooksCheck.mockResolvedValue({ hasHooks: false, hooks: null, mayNeedUpdate: false })

    await checkRuntimeHooks({ activeRuntimeEnvironmentId: null }, 'repo-1')
    await inspectRuntimeSetupScriptImports({ activeRuntimeEnvironmentId: null }, 'repo-1')
    await readRuntimeIssueCommand({ activeRuntimeEnvironmentId: null }, 'repo-1')
    await writeRuntimeIssueCommand({ activeRuntimeEnvironmentId: null }, 'repo-1', 'Fix it')

    // checkRuntimeHooks with no hostId unifies onto the local RPC registry too.
    expect(runtimeLocalCall).toHaveBeenNthCalledWith(1, {
      method: 'repo.hooksCheck',
      params: { repo: 'repo-1' }
    })
    expect(runtimeLocalCall).toHaveBeenNthCalledWith(2, {
      method: 'repo.setupScriptImports',
      params: { repo: 'repo-1' }
    })
    expect(runtimeLocalCall).toHaveBeenNthCalledWith(3, {
      method: 'repo.issueCommandRead',
      params: { repo: 'repo-1' }
    })
    expect(runtimeLocalCall).toHaveBeenNthCalledWith(4, {
      method: 'repo.issueCommandWrite',
      params: { repo: 'repo-1', content: 'Fix it' }
    })
    expect(hooksCheck).not.toHaveBeenCalled()
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })

  it('checks hooks via the local hostId-aware ipc channel when a hostId is given', async () => {
    hooksCheck.mockResolvedValue({ hasHooks: false, hooks: null, mayNeedUpdate: false })

    await checkRuntimeHooks({ activeRuntimeEnvironmentId: null }, 'repo-1', 'ssh:host-1')

    expect(hooksCheck).toHaveBeenCalledWith({ repoId: 'repo-1', hostId: 'ssh:host-1' })
    expect(runtimeLocalCall).not.toHaveBeenCalled()
  })

  it('routes hook operations through the active runtime environment', async () => {
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-1',
      ok: true,
      result: { ok: true },
      _meta: { runtimeId: 'runtime-1' }
    })

    await checkRuntimeHooks({ activeRuntimeEnvironmentId: 'env-1' }, 'repo-1')
    await inspectRuntimeSetupScriptImports({ activeRuntimeEnvironmentId: 'env-1' }, 'repo-1')
    await readRuntimeIssueCommand({ activeRuntimeEnvironmentId: 'env-1' }, 'repo-1')
    await writeRuntimeIssueCommand({ activeRuntimeEnvironmentId: 'env-1' }, 'repo-1', 'Fix it')

    expect(runtimeEnvironmentCall).toHaveBeenNthCalledWith(1, {
      selector: 'env-1',
      method: 'repo.hooksCheck',
      params: { repo: 'repo-1' },
      timeoutMs: 15_000
    })
    expect(runtimeEnvironmentCall).toHaveBeenNthCalledWith(2, {
      selector: 'env-1',
      method: 'repo.setupScriptImports',
      params: { repo: 'repo-1' },
      timeoutMs: 15_000
    })
    expect(runtimeEnvironmentCall).toHaveBeenNthCalledWith(3, {
      selector: 'env-1',
      method: 'repo.issueCommandRead',
      params: { repo: 'repo-1' },
      timeoutMs: 15_000
    })
    expect(runtimeEnvironmentCall).toHaveBeenNthCalledWith(4, {
      selector: 'env-1',
      method: 'repo.issueCommandWrite',
      params: { repo: 'repo-1', content: 'Fix it' },
      timeoutMs: 15_000
    })
    expect(hooksCheck).not.toHaveBeenCalled()
    expect(runtimeLocalCall).not.toHaveBeenCalled()
  })
})
