import type { GlobalSettings, Repo } from './types'

export const LOCAL_EXECUTION_HOST_ID = 'local'
export const ALL_EXECUTION_HOSTS_SCOPE = 'all'

export type ExecutionHostKind = 'local' | 'ssh' | 'runtime' | 'devServer'
export type ExecutionHostId =
  | typeof LOCAL_EXECUTION_HOST_ID
  | `ssh:${string}`
  | `runtime:${string}`
  | `devServer:${string}`

export type ExecutionHostScope = typeof ALL_EXECUTION_HOSTS_SCOPE | ExecutionHostId

export type ParsedExecutionHost =
  | { kind: 'local'; id: typeof LOCAL_EXECUTION_HOST_ID }
  | { kind: 'ssh'; id: `ssh:${string}`; targetId: string }
  | { kind: 'runtime'; id: `runtime:${string}`; environmentId: string }
  | { kind: 'devServer'; id: `devServer:${string}`; devServerId: string }

function getCurrentLocalPlatform(): NodeJS.Platform | null {
  const globalNavigator = (globalThis as { navigator?: { userAgent?: string; platform?: string } })
    .navigator
  const userAgent = globalNavigator?.userAgent || globalNavigator?.platform || ''
  if (/Windows/i.test(userAgent)) {
    return 'win32'
  }
  if (/Mac/i.test(userAgent)) {
    return 'darwin'
  }
  if (/Linux|X11/i.test(userAgent)) {
    return 'linux'
  }
  return typeof process === 'undefined' ? null : process.platform
}

export function getLocalExecutionHostLabel(platform: NodeJS.Platform | null = null): string {
  const localPlatform = platform ?? getCurrentLocalPlatform()
  if (localPlatform === 'darwin') {
    return 'Local Mac'
  }
  if (localPlatform === 'win32') {
    return 'Local Windows'
  }
  if (localPlatform === 'linux') {
    return 'Local Linux'
  }
  return 'This computer'
}

function normalizeHostPart(value: string | null | undefined): string | null {
  const trimmed = value?.trim()
  return trimmed ? trimmed : null
}

export function toSshExecutionHostId(targetId: string): `ssh:${string}` {
  return `ssh:${encodeURIComponent(targetId)}`
}

export function toRuntimeExecutionHostId(environmentId: string): `runtime:${string}` {
  return `runtime:${encodeURIComponent(environmentId)}`
}

export function toDevServerExecutionHostId(devServerId: string): `devServer:${string}` {
  return `devServer:${encodeURIComponent(devServerId)}`
}

// Why: runtime-owned (ephemeral-VM) SSH targets are hidden from user-facing
// SSH/run-target surfaces. The renderer can't read the target.owner field, so it
// recognizes them by their deterministic id prefix. getRuntimeOwnedSshTargetId
// (main) builds on this same prefix to keep the two in sync.
export const RUNTIME_OWNED_SSH_TARGET_ID_PREFIX = 'runtime-ssh-'

export function isRuntimeOwnedSshTargetId(targetId: string | null | undefined): boolean {
  return typeof targetId === 'string' && targetId.startsWith(RUNTIME_OWNED_SSH_TARGET_ID_PREFIX)
}

export function parseExecutionHostId(value: string | null | undefined): ParsedExecutionHost | null {
  const normalized = normalizeHostPart(value)
  if (!normalized) {
    return null
  }
  if (normalized === LOCAL_EXECUTION_HOST_ID) {
    return { kind: 'local', id: LOCAL_EXECUTION_HOST_ID }
  }
  if (normalized.startsWith('ssh:')) {
    const encoded = normalized.slice('ssh:'.length)
    if (!encoded) {
      return null
    }
    try {
      const targetId = decodeURIComponent(encoded)
      return targetId ? { kind: 'ssh', id: `ssh:${encoded}`, targetId } : null
    } catch {
      return null
    }
  }
  if (normalized.startsWith('runtime:')) {
    const encoded = normalized.slice('runtime:'.length)
    if (!encoded) {
      return null
    }
    try {
      const environmentId = decodeURIComponent(encoded)
      return environmentId ? { kind: 'runtime', id: `runtime:${encoded}`, environmentId } : null
    } catch {
      return null
    }
  }
  if (normalized.startsWith('devServer:')) {
    const encoded = normalized.slice('devServer:'.length)
    if (!encoded) {
      return null
    }
    try {
      const devServerId = decodeURIComponent(encoded)
      return devServerId ? { kind: 'devServer', id: `devServer:${encoded}`, devServerId } : null
    } catch {
      return null
    }
  }
  return null
}

export function normalizeExecutionHostId(value: string | null | undefined): ExecutionHostId | null {
  return parseExecutionHostId(value)?.id ?? null
}

export function normalizeExecutionHostScope(value: string | null | undefined): ExecutionHostScope {
  const normalized = normalizeHostPart(value)
  if (!normalized || normalized === ALL_EXECUTION_HOSTS_SCOPE) {
    return ALL_EXECUTION_HOSTS_SCOPE
  }
  return normalizeExecutionHostId(normalized) ?? ALL_EXECUTION_HOSTS_SCOPE
}

export function normalizeVisibleExecutionHostIds(
  value: readonly string[] | null | undefined
): ExecutionHostId[] | null {
  if (!Array.isArray(value)) {
    return null
  }
  const ids: ExecutionHostId[] = []
  const seen = new Set<ExecutionHostId>()
  for (const raw of value) {
    const id = normalizeExecutionHostId(raw)
    if (!id || seen.has(id)) {
      continue
    }
    seen.add(id)
    ids.push(id)
  }
  return ids.length > 0 ? ids : null
}

export function normalizeExecutionHostOrder(
  value: readonly string[] | null | undefined
): ExecutionHostId[] {
  const normalized = normalizeVisibleExecutionHostIds(value)
  return normalized ?? []
}

export function getRepoExecutionHostId(
  repo: Pick<Repo, 'connectionId' | 'executionHostId' | 'devServerId'>
): ExecutionHostId {
  const executionHostId = normalizeExecutionHostId(repo.executionHostId)
  if (executionHostId) {
    return executionHostId
  }
  const connectionId = normalizeHostPart(repo.connectionId)
  if (connectionId) {
    return toSshExecutionHostId(connectionId)
  }
  const devServerId = normalizeHostPart(repo.devServerId)
  return devServerId ? toDevServerExecutionHostId(devServerId) : LOCAL_EXECUTION_HOST_ID
}

/**
 * The bare opaque key orca-runtime.ts's provider registries (ssh-filesystem-
 * dispatch.ts / ssh-git-dispatch.ts) are keyed by — distinct from the
 * prefixed, UI-facing ExecutionHostId. Those registries are transport-
 * agnostic (see dev-server-provider-lifecycle.ts): an SSH target id and a
 * Dev Server id both resolve through the same lookup, so a repo bound to
 * either one works without the ~40 call sites that read this key knowing
 * which transport backs it. Precedence matches getRepoExecutionHostId:
 * an explicit connectionId (SSH) wins over devServerId.
 */
export function getRepoProviderConnectionKey(
  repo: Pick<Repo, 'connectionId' | 'devServerId'>
): string | null {
  return normalizeHostPart(repo.connectionId) ?? normalizeHostPart(repo.devServerId) ?? null
}

export function getSettingsFocusedExecutionHostId(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): ExecutionHostId {
  const runtimeEnvironmentId = normalizeHostPart(settings?.activeRuntimeEnvironmentId)
  return runtimeEnvironmentId
    ? toRuntimeExecutionHostId(runtimeEnvironmentId)
    : LOCAL_EXECUTION_HOST_ID
}

export function getExecutionHostLabel(id: ExecutionHostScope): string {
  if (id === ALL_EXECUTION_HOSTS_SCOPE) {
    return 'All hosts'
  }
  const parsed = parseExecutionHostId(id)
  if (!parsed) {
    return 'All hosts'
  }
  switch (parsed.kind) {
    case 'local':
      return getLocalExecutionHostLabel()
    case 'ssh':
      return parsed.targetId
    case 'runtime':
      return parsed.environmentId
    case 'devServer':
      return parsed.devServerId
  }
}
