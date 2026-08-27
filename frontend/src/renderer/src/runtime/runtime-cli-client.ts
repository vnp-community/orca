// Why: CLI install/registration is desktop-local only (it manages the `orca`
// binary and shell PATH on the machine running Orca) — there is no
// remote-environment routing here, only a local RPC call so the desktop
// process's own `orca serve` clients can reach it too.
import type { CliInstallStatus } from '../../../shared/cli-install-types'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export function getRuntimeCliInstallStatus(): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.getInstallStatus')
}

export function installRuntimeCli(): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.install')
}

export function removeRuntimeCli(): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.remove')
}

export function getRuntimeWslCliInstallStatus(args?: {
  distro?: string | null
}): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.getWslInstallStatus', args)
}

export function installRuntimeWslCli(args?: { distro?: string | null }): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.installWsl', args)
}

export function removeRuntimeWslCli(args?: { distro?: string | null }): Promise<CliInstallStatus> {
  return callRuntimeRpc<CliInstallStatus>(LOCAL_TARGET, 'cli.removeWsl', args)
}
