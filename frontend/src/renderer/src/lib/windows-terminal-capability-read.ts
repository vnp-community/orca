import { callRuntimeRpc, type RuntimeClientTarget } from '@/runtime/runtime-rpc-client'
import type { RuntimeStatus } from '../../../shared/runtime-types'
import type { WindowsTerminalCapabilities } from './windows-terminal-capabilities'

export type WindowsTerminalCapabilityLoadTarget = RuntimeClientTarget

export async function readWindowsTerminalCapabilities(
  target: WindowsTerminalCapabilityLoadTarget,
  sshConnectionId?: string | null
): Promise<WindowsTerminalCapabilities> {
  if (sshConnectionId) {
    const remoteCapabilityPromise =
      target.kind === 'environment'
        ? callRuntimeRpc<Omit<WindowsTerminalCapabilities, 'isLoading'>>(
            target,
            'preflight.detectRemoteWindowsTerminalCapabilities',
            { connectionId: sshConnectionId },
            { timeoutMs: 15_000 }
          )
        : window.api.preflight.detectRemoteWindowsTerminalCapabilities({
            connectionId: sshConnectionId
          })
    return remoteCapabilityPromise
      .then((capabilities) => ({
        ...capabilities,
        wslDistros: capabilities.wslDistros ?? [],
        isLoading: false
      }))
      .catch(() => ({
        wslAvailable: false,
        wslDistros: [],
        pwshAvailable: false,
        gitBashAvailable: false,
        hostPlatform: null,
        isLoading: false
      }))
  }

  // Why: local desktop and remote environments both expose the same host.*
  // RPC surface (host-capabilities.ts), so one call shape covers both —
  // callRuntimeRpc already branches window.api.runtime.call vs
  // window.api.runtimeEnvironments.call based on target.kind.
  const timeoutMs = target.kind === 'local' ? undefined : 15_000
  const [wslAvailable, wslDistros, pwshAvailable, gitBashAvailable, hostPlatform] =
    await Promise.all([
      callRuntimeRpc<boolean>(target, 'host.wsl.isAvailable', undefined, { timeoutMs }).catch(
        () => false
      ),
      callRuntimeRpc<string[]>(target, 'host.wsl.listDistros', undefined, { timeoutMs }).catch(
        () => []
      ),
      callRuntimeRpc<boolean>(target, 'host.pwsh.isAvailable', undefined, { timeoutMs }).catch(
        () => false
      ),
      callRuntimeRpc<boolean>(target, 'host.gitBash.isAvailable', undefined, { timeoutMs }).catch(
        () => false
      ),
      callRuntimeRpc<RuntimeStatus>(target, 'status.get', undefined, { timeoutMs })
        .then((status) => status.hostPlatform ?? null)
        .catch(() => null)
    ])
  return {
    wslAvailable,
    wslDistros,
    pwshAvailable,
    gitBashAvailable,
    hostPlatform,
    isLoading: false
  }
}
