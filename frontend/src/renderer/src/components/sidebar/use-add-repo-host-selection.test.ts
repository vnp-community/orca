// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type * as ReactModule from 'react'
import type { SidebarHostOption } from './sidebar-host-options'

const mocks = vi.hoisted(() => ({
  stateValues: [] as unknown[],
  stateSetters: [] as ReturnType<typeof vi.fn>[],
  stateIndex: 0,
  refValues: [] as unknown[],
  refIndex: 0,
  hostOptions: [] as SidebarHostOption[],
  isWeb: false,
  storeState: {
    settings: { activeRuntimeEnvironmentId: null as string | null },
    switchRuntimeEnvironment: vi.fn(),
    setActiveDevServerId: vi.fn(),
    devServers: [] as { id: string; name: string; status: string }[],
    setSshConnectionState: vi.fn(),
    sshConnectionStates: new Map(),
    runtimeEnvironments: [] as { id: string; name: string; source?: 'manual' | 'ephemeral-vm' }[]
  },
  sshConnect: vi.fn(),
  sshGetState: vi.fn()
}))

vi.mock('react', async (importOriginal) => {
  const actual = await importOriginal<typeof ReactModule>()
  return {
    ...actual,
    useCallback: <T extends (...args: never[]) => unknown>(fn: T) => fn,
    useMemo: <T>(factory: () => T) => factory(),
    useEffect: (effect: () => void | (() => void)) => {
      effect()
    },
    useRef: <T>(value: T) => {
      const index = mocks.refIndex++
      return {
        current: index in mocks.refValues ? (mocks.refValues[index] as T) : value
      }
    },
    useState: <T>(initial: T | (() => T)) => {
      const index = mocks.stateIndex++
      const value =
        index in mocks.stateValues
          ? mocks.stateValues[index]
          : typeof initial === 'function'
            ? (initial as () => T)()
            : initial
      const setter = vi.fn()
      mocks.stateSetters[index] = setter
      return [value as T, setter]
    }
  }
})

vi.mock('@/store', () => ({
  useAppStore: (selector: (state: typeof mocks.storeState) => unknown) => selector(mocks.storeState)
}))

vi.mock('@/lib/web-client-location', () => ({ isWebClientLocation: () => mocks.isWeb }))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn()
  }
}))

vi.mock('./use-sidebar-host-scope-options', () => ({
  useSidebarHostScopeOptions: () => ({ hostOptions: mocks.hostOptions })
}))

describe('useAddRepoHostSelection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.stateIndex = 0
    mocks.stateSetters = []
    mocks.refIndex = 0
    mocks.refValues = []
    mocks.isWeb = false
    mocks.storeState.devServers = []
    mocks.hostOptions = [
      {
        id: 'local',
        label: 'Local Mac',
        detail: 'This computer',
        kind: 'local',
        health: 'local',
        presence: 'local'
      },
      {
        id: 'ssh:ssh-1',
        label: 'Builder',
        detail: 'SSH',
        kind: 'ssh',
        health: 'available',
        presence: 'configured'
      },
      {
        id: 'runtime:env-1',
        label: 'Server',
        detail: 'Runtime',
        kind: 'runtime',
        health: 'available',
        presence: 'active'
      }
    ]
    mocks.storeState.settings = { activeRuntimeEnvironmentId: null }
    mocks.storeState.switchRuntimeEnvironment.mockResolvedValue(true)
    mocks.storeState.sshConnectionStates = new Map()
    mocks.storeState.runtimeEnvironments = []
    mocks.sshConnect.mockReset()
    mocks.sshGetState.mockReset()
    vi.stubGlobal('window', {
      api: {
        ssh: {
          connect: mocks.sshConnect,
          getState: mocks.sshGetState
        }
      }
    })
  })

  it('exposes the selected SSH target id', async () => {
    mocks.stateValues = ['ssh:ssh-1', false]
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(result.selectedHostId).toBe('ssh:ssh-1')
    expect(result.selectedParsedHost).toMatchObject({ kind: 'ssh', targetId: 'ssh-1' })
    expect(result.selectedSshTargetId).toBe('ssh-1')
  })

  it('switches runtime before selecting a runtime host', async () => {
    mocks.stateValues = ['local', false]
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })
    await result.handleSelectAddProjectHost('runtime:env-1')

    expect(mocks.storeState.switchRuntimeEnvironment).toHaveBeenCalledWith('env-1')
    expect(mocks.stateSetters[0]).toHaveBeenCalledWith('runtime:env-1')
    expect(setStep).toHaveBeenCalledWith('add')
  })

  it('clears the active runtime before selecting a local or SSH host', async () => {
    mocks.stateValues = ['runtime:env-1', false]
    mocks.storeState.settings = { activeRuntimeEnvironmentId: 'env-1' }
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })
    await result.handleSelectAddProjectHost('ssh:ssh-1')

    expect(mocks.storeState.switchRuntimeEnvironment).toHaveBeenCalledWith(null)
    expect(mocks.stateSetters[0]).toHaveBeenCalledWith('ssh:ssh-1')
    expect(setStep).toHaveBeenCalledWith('add')
  })

  it('falls back from a disconnected selected SSH host to Local Mac', async () => {
    mocks.stateValues = ['ssh:ssh-1', false]
    mocks.hostOptions[1] = {
      ...mocks.hostOptions[1],
      health: 'disconnected'
    }
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(result.selectedHostId).toBe('local')
    expect(result.selectedSshTargetId).toBeNull()
  })

  it('does not select a disconnected SSH host', async () => {
    mocks.stateValues = ['local', false]
    mocks.hostOptions[1] = {
      ...mocks.hostOptions[1],
      health: 'disconnected'
    }
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })
    await result.handleSelectAddProjectHost('ssh:ssh-1')

    expect(mocks.storeState.switchRuntimeEnvironment).not.toHaveBeenCalled()
    expect(mocks.stateSetters[0]).not.toHaveBeenCalledWith('ssh:ssh-1')
    expect(setStep).not.toHaveBeenCalled()
  })

  it('connects and selects a disconnected SSH host from Add Project', async () => {
    mocks.stateValues = ['local', true]
    mocks.hostOptions[1] = {
      ...mocks.hostOptions[1],
      health: 'disconnected'
    }
    mocks.sshConnect.mockResolvedValue({
      targetId: 'ssh-1',
      status: 'connected',
      error: null,
      reconnectAttempt: 0
    })
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })
    await result.handleConnectAddProjectHost('ssh:ssh-1')

    expect(mocks.storeState.setSshConnectionState).toHaveBeenCalledWith(
      'ssh-1',
      expect.objectContaining({ status: 'connecting' })
    )
    expect(mocks.sshConnect).toHaveBeenCalledWith({ targetId: 'ssh-1' })
    expect(mocks.storeState.setSshConnectionState).toHaveBeenCalledWith(
      'ssh-1',
      expect.objectContaining({ status: 'connected' })
    )
    expect(mocks.stateSetters[0]).toHaveBeenCalledWith('ssh:ssh-1')
    expect(mocks.stateSetters[1]).toHaveBeenCalledWith(false)
    expect(setStep).toHaveBeenCalledWith('add')
  })

  it('does not auto-select the active runtime host while it is unavailable', async () => {
    mocks.stateValues = ['local', false]
    mocks.hostOptions[2] = {
      ...mocks.hostOptions[2],
      health: 'blocked'
    }
    mocks.storeState.settings = { activeRuntimeEnvironmentId: 'env-1' }
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(mocks.stateSetters[0]).toHaveBeenCalledWith('local')
  })

  it('hides ephemeral VM runtime hosts from Add Project selection', async () => {
    mocks.stateValues = ['runtime:env-vm', false]
    mocks.hostOptions.push({
      id: 'runtime:env-vm',
      label: 'orca VM abc12345',
      detail: 'Runtime',
      kind: 'runtime',
      health: 'available',
      presence: 'project'
    })
    mocks.storeState.runtimeEnvironments = [
      { id: 'env-vm', name: 'orca VM abc12345', source: 'ephemeral-vm' }
    ]
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })

    expect(result.hostOptions.map((host) => host.id)).not.toContain('runtime:env-vm')
    expect(result.selectedHostId).toBe('local')
    await result.handleSelectAddProjectHost('runtime:env-vm')
    expect(mocks.storeState.switchRuntimeEnvironment).not.toHaveBeenCalledWith('env-vm')
    expect(setStep).not.toHaveBeenCalled()
  })
})

// SOL-FE-E2E-004: on web there is no local fallback; a dev server is chosen (or the dialog says why not).
describe('useAddRepoHostSelection on web', () => {
  const devServerOption = (id: string, name: string): SidebarHostOption => ({
    id: `devServer:${id}` as SidebarHostOption['id'],
    label: name,
    detail: 'Dev Server',
    kind: 'devServer',
    health: 'available',
    presence: 'project'
  })

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.isWeb = true
    mocks.stateIndex = 0
    mocks.stateSetters = []
    mocks.refIndex = 0
    mocks.refValues = []
    mocks.storeState.settings = { activeRuntimeEnvironmentId: 'session-auth' }
    mocks.storeState.runtimeEnvironments = []
    mocks.storeState.devServers = []
    vi.stubGlobal('window', { api: {} })
  })

  it('blocks instead of falling back to local when no dev server is connected', async () => {
    mocks.hostOptions = []
    mocks.stateValues = ['local', false, null]
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(result.selectedHostId).toBeNull()
    expect(result.selectedParsedHost).toBeNull()
    expect(result.hostGate).toEqual({ status: 'no-host' })
  })

  it('selects the only connected dev server and pins it', async () => {
    mocks.hostOptions = [devServerOption('ds-1', 'Mac Studio')]
    mocks.stateValues = ['local', false, null]
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(result.selectedHostId).toBe('devServer:ds-1')
    expect(result.selectedParsedHost).toMatchObject({ kind: 'devServer', devServerId: 'ds-1' })
    expect(result.hostGate).toEqual({ status: 'ready' })
    expect(mocks.stateSetters[2]).toHaveBeenCalledWith('devServer:ds-1')
  })

  it('does not choose for the user when several dev servers are connected', async () => {
    mocks.hostOptions = [
      devServerOption('ds-1', 'Mac Studio'),
      devServerOption('ds-2', 'Build Box')
    ]
    mocks.stateValues = ['local', false, null]
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    expect(result.selectedHostId).toBeNull()
    expect(result.hostGate).toEqual({ status: 'choose-host' })
    expect(mocks.stateSetters[2]).not.toHaveBeenCalled()
  })

  it('keeps the chosen host when it drops mid-way and reports it as unavailable', async () => {
    mocks.hostOptions = []
    mocks.storeState.devServers = [{ id: 'ds-1', name: 'Mac Studio', status: 'disconnected' }]
    mocks.stateValues = ['local', false, 'devServer:ds-1']
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep: vi.fn() })

    // The Browse step keeps its devServerId, so the dialog does not close or switch hosts silently.
    expect(result.selectedHostId).toBe('devServer:ds-1')
    expect(result.selectedParsedHost).toMatchObject({ kind: 'devServer', devServerId: 'ds-1' })
    expect(result.hostGate).toMatchObject({
      status: 'host-unavailable',
      label: 'Mac Studio',
      reason: 'disconnected'
    })
    expect(result.hostOptions.map((host) => host.id)).toEqual(['devServer:ds-1'])
    expect(result.hostOptions[0]).toMatchObject({ health: 'disconnected', label: 'Mac Studio' })
  })

  it('records the picked dev server and moves to the add step', async () => {
    mocks.hostOptions = [
      devServerOption('ds-1', 'Mac Studio'),
      devServerOption('ds-2', 'Build Box')
    ]
    mocks.stateValues = ['local', false, null]
    const setStep = vi.fn()
    const { useAddRepoHostSelection } = await import('./use-add-repo-host-selection')

    const result = useAddRepoHostSelection({ isOpen: true, setStep })
    await result.handleSelectAddProjectHost('devServer:ds-2')

    expect(mocks.storeState.setActiveDevServerId).toHaveBeenCalledWith('ds-2')
    expect(mocks.stateSetters[2]).toHaveBeenCalledWith('devServer:ds-2')
    expect(setStep).toHaveBeenCalledWith('add')
  })
})
