// Why: CLI install/registration manages the `orca` binary and shell PATH on
// the machine hosting a user's terminals — on desktop that's always the
// local machine, so window.api.cli.* is real same-machine IPC and
// devServerId is simply unused. On the web build there is no local machine
// to register anything on; devServerId tells web-preload-api.ts's
// createCliApi which connected Dev Server to relay the call to (backend-go's
// wscompat/channels_cli.go -> RelayByDevServer -> the agent's own
// cli.getInstallStatus/install/... — see that file's doc comment). Omitting
// devServerId on web (every caller that hasn't been updated to thread one
// through yet) keeps createCliApi's prior honest "unsupported in the
// browser" stub — no behavior change for those callers.
//
// This used to go through callRuntimeRpc({kind: 'local'}, 'cli.*'), i.e.
// window.api.runtime.call — on Electron desktop that's a same-machine IPC
// round trip that happens to reach the exact same getCliInstallStatus() (et
// al.) implementation window.api.cli.* calls directly, so the two were
// behaviorally identical there. On the web build, window.api.runtime.call
// is a REAL network call to backend-go, which at the time had no cli.*
// channels — every cli.* call always threw "channel not yet implemented in
// backend-go", including right after onboarding finishes (CliSection's
// status check). Calling window.api.cli.* directly (bypassing that generic
// relay) avoided the crash but left the web build permanently unable to
// register a CLI on a connected Dev Server, confirmed live on
// b15.openledger.vn as Settings > Orchestration's Install button always
// showing "Not installed" / "CLI registration is managed on the Orca
// server, not in the web browser". channels_cli.go now gives backend-go a
// real cli.* implementation, so devServerId-aware calls work end to end.
import type { CliInstallStatus } from '../../../shared/cli-install-types'

export function getRuntimeCliInstallStatus(devServerId?: string): Promise<CliInstallStatus> {
  return window.api.cli.getInstallStatus(devServerId ? { devServerId } : undefined)
}

export function installRuntimeCli(devServerId?: string): Promise<CliInstallStatus> {
  return window.api.cli.install(devServerId ? { devServerId } : undefined)
}

export function removeRuntimeCli(devServerId?: string): Promise<CliInstallStatus> {
  return window.api.cli.remove(devServerId ? { devServerId } : undefined)
}

export function getRuntimeWslCliInstallStatus(args?: {
  devServerId?: string
  distro?: string | null
}): Promise<CliInstallStatus> {
  return window.api.cli.getWslInstallStatus(args)
}

export function installRuntimeWslCli(args?: {
  devServerId?: string
  distro?: string | null
}): Promise<CliInstallStatus> {
  return window.api.cli.installWsl(args)
}

export function removeRuntimeWslCli(args?: {
  devServerId?: string
  distro?: string | null
}): Promise<CliInstallStatus> {
  return window.api.cli.removeWsl(args)
}
