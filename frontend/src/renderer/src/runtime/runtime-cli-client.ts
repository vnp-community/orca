// Why: CLI install/registration is desktop-local only (it manages the `orca`
// binary and shell PATH on the machine running Orca) — there is no
// remote-environment routing here, only a local call. This used to go
// through callRuntimeRpc({kind: 'local'}, 'cli.*'), i.e.
// window.api.runtime.call — on Electron desktop that's a same-machine IPC
// round trip that happens to reach the exact same getCliInstallStatus() (et
// al.) implementation window.api.cli.* calls directly, so the two were
// behaviorally identical there. On the web build, though,
// window.api.runtime.call is a REAL network call to backend-go, which
// rightly has no cli.* channels (installing a CLI on the user's own machine
// is not backend-go's or the browser's job) — every cli.* call always threw
// "channel not yet implemented in backend-go", including right after
// onboarding finishes (CliSection's status check). window.api.cli.* is
// already correctly implemented on both platforms (real IPC on desktop, an
// honest "unsupported in the browser" stub on web — see web-preload-api.ts's
// createCliApi) and needs no relay at all, so call it directly.
import type { CliInstallStatus } from '../../../shared/cli-install-types'

export function getRuntimeCliInstallStatus(): Promise<CliInstallStatus> {
  return window.api.cli.getInstallStatus()
}

export function installRuntimeCli(): Promise<CliInstallStatus> {
  return window.api.cli.install()
}

export function removeRuntimeCli(): Promise<CliInstallStatus> {
  return window.api.cli.remove()
}

export function getRuntimeWslCliInstallStatus(args?: {
  distro?: string | null
}): Promise<CliInstallStatus> {
  return window.api.cli.getWslInstallStatus(args)
}

export function installRuntimeWslCli(args?: { distro?: string | null }): Promise<CliInstallStatus> {
  return window.api.cli.installWsl(args)
}

export function removeRuntimeWslCli(args?: { distro?: string | null }): Promise<CliInstallStatus> {
  return window.api.cli.removeWsl(args)
}
