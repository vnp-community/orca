import { describe, expect, it, vi } from 'vitest'
import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { SSH_METHODS } from './ssh'

const {
  connectRegisteredSshTargetMock,
  getRegisteredSshStateMock,
  listRegisteredSshTargetsMock,
  listRegisteredRemovedSshTargetLabelsMock,
  disconnectRegisteredSshTargetMock,
  needsRegisteredSshPassphrasePromptMock,
  addRegisteredSshPortForwardMock,
  updateRegisteredSshPortForwardMock,
  removeRegisteredSshPortForwardMock,
  listRegisteredSshPortForwardsMock,
  listRegisteredSshDetectedPortsMock
} = vi.hoisted(() => ({
  connectRegisteredSshTargetMock: vi.fn(),
  getRegisteredSshStateMock: vi.fn(),
  listRegisteredSshTargetsMock: vi.fn(),
  listRegisteredRemovedSshTargetLabelsMock: vi.fn(),
  disconnectRegisteredSshTargetMock: vi.fn(),
  needsRegisteredSshPassphrasePromptMock: vi.fn(),
  addRegisteredSshPortForwardMock: vi.fn(),
  updateRegisteredSshPortForwardMock: vi.fn(),
  removeRegisteredSshPortForwardMock: vi.fn(),
  listRegisteredSshPortForwardsMock: vi.fn(),
  listRegisteredSshDetectedPortsMock: vi.fn()
}))

vi.mock('../../../ipc/ssh', () => ({
  connectRegisteredSshTarget: connectRegisteredSshTargetMock,
  getRegisteredSshState: getRegisteredSshStateMock,
  listRegisteredSshTargets: listRegisteredSshTargetsMock,
  listRegisteredRemovedSshTargetLabels: listRegisteredRemovedSshTargetLabelsMock,
  disconnectRegisteredSshTarget: disconnectRegisteredSshTargetMock,
  needsRegisteredSshPassphrasePrompt: needsRegisteredSshPassphrasePromptMock,
  addRegisteredSshPortForward: addRegisteredSshPortForwardMock,
  updateRegisteredSshPortForward: updateRegisteredSshPortForwardMock,
  removeRegisteredSshPortForward: removeRegisteredSshPortForwardMock,
  listRegisteredSshPortForwards: listRegisteredSshPortForwardsMock,
  listRegisteredSshDetectedPorts: listRegisteredSshDetectedPortsMock
}))

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

describe('ssh RPC methods', () => {
  it('returns the registered SSH target state', async () => {
    const state = {
      targetId: 'ssh-1',
      status: 'connected',
      error: null,
      reconnectAttempt: 0
    }
    getRegisteredSshStateMock.mockReturnValueOnce(state)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.getState', { targetId: 'ssh-1' }))

    expect(getRegisteredSshStateMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true, result: { state } })
  })

  it('connects through the registered desktop SSH lifecycle', async () => {
    const state = {
      targetId: 'ssh-1',
      status: 'connected',
      error: null,
      reconnectAttempt: 0
    }
    connectRegisteredSshTargetMock.mockResolvedValueOnce(state)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.connect', { targetId: 'ssh-1' }))

    expect(connectRegisteredSshTargetMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true, result: { state } })
  })

  it('returns null when the target has no registered state yet', async () => {
    getRegisteredSshStateMock.mockReturnValueOnce(undefined)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.getState', { targetId: 'ssh-1' }))

    expect(response).toMatchObject({ ok: true, result: { state: null } })
  })

  it('lists the registered SSH targets for paired clients', async () => {
    const targets = [{ id: 'ssh-1', label: 'Dev box', host: 'dev', port: 22, username: 'me' }]
    listRegisteredSshTargetsMock.mockReturnValueOnce(targets)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.listTargets'))

    expect(response).toMatchObject({ ok: true, result: { targets } })
  })

  it('lists removed-target labels for ghost-host display on paired clients', async () => {
    const labels = { 'ssh-old': 'Dev box' }
    listRegisteredRemovedSshTargetLabelsMock.mockReturnValueOnce(labels)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.listRemovedTargetLabels'))

    expect(response).toMatchObject({ ok: true, result: { labels } })
  })

  it('disconnects through the registered desktop SSH lifecycle', async () => {
    disconnectRegisteredSshTargetMock.mockResolvedValueOnce(undefined)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.disconnect', { targetId: 'ssh-1' }))

    expect(disconnectRegisteredSshTargetMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true })
  })

  it('reports whether a target needs an interactive passphrase prompt', async () => {
    needsRegisteredSshPassphrasePromptMock.mockReturnValueOnce(true)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('ssh.needsPassphrasePrompt', { targetId: 'ssh-1' })
    )

    expect(needsRegisteredSshPassphrasePromptMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true, result: { needsPrompt: true } })
  })

  it('adds a port forward through the registered SshPortForwardManager', async () => {
    const entry = {
      id: 'pf-1',
      connectionId: 'ssh-1',
      localPort: 3000,
      remoteHost: 'localhost',
      remotePort: 3000
    }
    addRegisteredSshPortForwardMock.mockResolvedValueOnce(entry)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('ssh.addPortForward', {
        targetId: 'ssh-1',
        localPort: 3000,
        remoteHost: 'localhost',
        remotePort: 3000
      })
    )

    expect(addRegisteredSshPortForwardMock).toHaveBeenCalledWith({
      targetId: 'ssh-1',
      localPort: 3000,
      remoteHost: 'localhost',
      remotePort: 3000
    })
    expect(response).toMatchObject({ ok: true, result: { entry } })
  })

  it('updates a port forward through the registered SshPortForwardManager', async () => {
    const entry = {
      id: 'pf-1',
      connectionId: 'ssh-1',
      localPort: 3001,
      remoteHost: 'localhost',
      remotePort: 3000
    }
    updateRegisteredSshPortForwardMock.mockResolvedValueOnce(entry)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('ssh.updatePortForward', {
        id: 'pf-1',
        targetId: 'ssh-1',
        localPort: 3001,
        remoteHost: 'localhost',
        remotePort: 3000
      })
    )

    expect(updateRegisteredSshPortForwardMock).toHaveBeenCalledWith({
      id: 'pf-1',
      targetId: 'ssh-1',
      localPort: 3001,
      remoteHost: 'localhost',
      remotePort: 3000
    })
    expect(response).toMatchObject({ ok: true, result: { entry } })
  })

  it('removes a port forward through the registered SshPortForwardManager', async () => {
    const entry = {
      id: 'pf-1',
      connectionId: 'ssh-1',
      localPort: 3000,
      remoteHost: 'localhost',
      remotePort: 3000
    }
    removeRegisteredSshPortForwardMock.mockResolvedValueOnce(entry)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(makeRequest('ssh.removePortForward', { id: 'pf-1' }))

    expect(removeRegisteredSshPortForwardMock).toHaveBeenCalledWith('pf-1')
    expect(response).toMatchObject({ ok: true, result: { entry } })
  })

  it('lists port forwards for a target', async () => {
    const forwards = [
      { id: 'pf-1', connectionId: 'ssh-1', localPort: 3000, remoteHost: 'localhost', remotePort: 3000 }
    ]
    listRegisteredSshPortForwardsMock.mockReturnValueOnce(forwards)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('ssh.listPortForwards', { targetId: 'ssh-1' })
    )

    expect(listRegisteredSshPortForwardsMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true, result: { forwards } })
  })

  it('lists detected ports for a target', async () => {
    const ports = [{ port: 3000, host: 'localhost' }]
    listRegisteredSshDetectedPortsMock.mockReturnValueOnce(ports)
    const runtime = { getRuntimeId: () => 'test-runtime' } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: SSH_METHODS })

    const response = await dispatcher.dispatch(
      makeRequest('ssh.listDetectedPorts', { targetId: 'ssh-1' })
    )

    expect(listRegisteredSshDetectedPortsMock).toHaveBeenCalledWith('ssh-1')
    expect(response).toMatchObject({ ok: true, result: { ports } })
  })
})
