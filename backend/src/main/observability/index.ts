// Composition root for the diagnostics-bundle lane, adapted from
// desktop/src/main/observability/index.ts (diagnostics.* bundle RPC port).
//
// Known gap (documented, not fixed by this port — same pattern as the
// rateLimits.*/telemetry.* gaps noted in
// specs/backend/api/desktop-only-rpc-parity-gaps.md): nothing in
// server-bootstrap.ts calls initObservability() yet, so no local NDJSON sink
// is ever installed and `collectDiagnosticBundle()` always returns a
// header-only, zero-span bundle. The methods below are fully real — the
// upstream data source just isn't wired up yet. resolveObservabilityConsent()
// still governs whether the bundle button is enabled at all (CI / DNT /
// ORCA_TELEMETRY_DISABLED / ORCA_DIAGNOSTICS_DISABLED), matching desktop.

import { DEFAULT_MAX_FILES, getRotatedFamilySize } from './local-file-sink'
import { getDaemonLogFilePath, getTraceFilePath } from './logs-directory'
import { DAEMON_LOG_MAX_FILES } from '../daemon/daemon-file-log'
import { collectBundle as _collectBundle, type CollectBundleOptions, type CollectedBundle } from './bundle'
import {
  deleteBundle as _deleteBundle,
  uploadBundle as _uploadBundle,
  type DeleteBundleOptions,
  type UploadBundleOptions,
  type UploadBundleResult
} from './diagnostic-bundle-upload'

const CI_ENV_VARS = [
  'CI',
  'GITHUB_ACTIONS',
  'GITLAB_CI',
  'CIRCLECI',
  'TRAVIS',
  'BUILDKITE',
  'JENKINS_URL',
  'TEAMCITY_VERSION'
] as const

export type ObservabilityConsent = {
  readonly localFileEnabled: boolean
  readonly bundleEnabled: boolean
  readonly disabledReason?:
    | 'do_not_track'
    | 'orca_telemetry_disabled'
    | 'orca_diagnostics_disabled'
    | 'ci'
}

function envOn(name: string): boolean {
  const v = process.env[name]
  if (!v) {
    return false
  }
  const norm = v.trim().toLowerCase()
  return norm === '1' || norm === 'true'
}

function inCI(): boolean {
  return CI_ENV_VARS.some((v) => process.env[v] !== undefined && process.env[v] !== '')
}

export function resolveObservabilityConsent(): ObservabilityConsent {
  const dnt = envOn('DO_NOT_TRACK')
  const orcaDisabled = envOn('ORCA_TELEMETRY_DISABLED')
  const diagnosticsDisabled = envOn('ORCA_DIAGNOSTICS_DISABLED')
  const ci = inCI()

  if (ci) {
    return { localFileEnabled: false, bundleEnabled: false, disabledReason: 'ci' }
  }
  if (diagnosticsDisabled) {
    return { localFileEnabled: false, bundleEnabled: false, disabledReason: 'orca_diagnostics_disabled' }
  }
  if (dnt || orcaDisabled) {
    return {
      localFileEnabled: true,
      bundleEnabled: false,
      disabledReason: dnt ? 'do_not_track' : 'orca_telemetry_disabled'
    }
  }
  return { localFileEnabled: true, bundleEnabled: true }
}

export type DiagnosticsStatus = {
  readonly localFileEnabled: boolean
  readonly bundleEnabled: boolean
  readonly traceFilePath: string
  readonly traceFamilySize: number
  readonly disabledReason?: ObservabilityConsent['disabledReason']
}

export function getDiagnosticsStatus(): DiagnosticsStatus {
  const c = resolveObservabilityConsent()
  const traceFilePath = getTraceFilePath()
  const traceFamilySize = c.localFileEnabled ? getRotatedFamilySize(traceFilePath) : 0
  return {
    localFileEnabled: c.localFileEnabled,
    bundleEnabled: c.bundleEnabled,
    traceFilePath,
    traceFamilySize,
    ...(c.disabledReason ? { disabledReason: c.disabledReason } : {})
  }
}

export function collectDiagnosticBundle(
  meta: Pick<
    CollectBundleOptions,
    'appVersion' | 'platform' | 'arch' | 'osRelease' | 'orcaChannel' | 'lookbackMinutes'
  >
): CollectedBundle {
  return _collectBundle({
    traceFilePath: getTraceFilePath(),
    maxFiles: DEFAULT_MAX_FILES,
    daemonLogFilePath: getDaemonLogFilePath(),
    daemonLogMaxFiles: DAEMON_LOG_MAX_FILES,
    ...meta
  })
}

export async function uploadDiagnosticBundle(opts: UploadBundleOptions): Promise<UploadBundleResult> {
  return _uploadBundle(opts)
}

export async function deleteDiagnosticBundle(opts: DeleteBundleOptions): Promise<void> {
  return _deleteBundle(opts)
}
