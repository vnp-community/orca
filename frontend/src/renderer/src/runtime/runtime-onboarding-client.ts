// Why: mirrors the git/preload API pattern (runtime-git-client.ts) so onboarding
// callers get one typed boundary instead of reimplementing local-vs-environment
// branching per call. onboarding.get/update/markChecklistItem read/write the
// active runtime's local persisted checklist state; the rest proxy a specific
// Dev Server (devServerId) through DevServerManager — both need the same
// local/environment split as every other runtime capability.
import type { GlobalSettings, OnboardingState } from '../../../shared/types'
import type {
  RemotePreflightStatus,
  WindowsTerminalCapabilities
} from '../../../shared/dev-server-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type RuntimeOnboardingSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export type RuntimeOnboardingUpdate = Partial<Omit<OnboardingState, 'checklist'>> & {
  checklist?: Partial<OnboardingState['checklist']>
}

export async function getRuntimeOnboardingState(
  settings: RuntimeOnboardingSettings | null | undefined
): Promise<OnboardingState> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.get()
  }
  return callRuntimeRpc<OnboardingState>(target, 'onboarding.get')
}

export async function updateRuntimeOnboardingState(
  settings: RuntimeOnboardingSettings | null | undefined,
  updates: RuntimeOnboardingUpdate
): Promise<OnboardingState> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.update(updates)
  }
  return callRuntimeRpc<OnboardingState>(target, 'onboarding.update', updates)
}

export async function markRuntimeOnboardingChecklistItem(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { item: string; devServerId?: string; value?: boolean }
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.onboarding.markChecklistItem(params)
    return
  }
  await callRuntimeRpc<{ marked: boolean }>(target, 'onboarding.markChecklistItem', params)
}

export async function detectRuntimeOnboardingAgents(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string | null }
): Promise<{ agents: string[]; platform: NodeJS.Platform | null; devServerId: string | null }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.detectAgents(params)
  }
  return callRuntimeRpc(target, 'onboarding.detectAgents', params)
}

export async function detectRuntimeOnboardingAgentsAllServers(
  settings: RuntimeOnboardingSettings | null | undefined
): Promise<Record<string, { agents: string[]; platform: NodeJS.Platform | null; error?: string }>> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.detectAgentsAllServers()
  }
  return callRuntimeRpc(target, 'onboarding.detectAgentsAllServers')
}

export async function getRuntimeOnboardingPreflightStatus(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string; force?: boolean }
): Promise<RemotePreflightStatus> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.getPreflightStatus(params)
  }
  return callRuntimeRpc(target, 'onboarding.getPreflightStatus', params)
}

export async function setRuntimeOnboardingGitIdentity(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string; name: string; email: string }
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.onboarding.setGitIdentity(params)
    return
  }
  await callRuntimeRpc(target, 'onboarding.setGitIdentity', params)
}

export async function detectRuntimeOnboardingGhosttyConfig(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string }
): Promise<{ configPath: string | null; themeDir: string | null }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.detectGhosttyConfig(params)
  }
  return callRuntimeRpc(target, 'onboarding.detectGhosttyConfig', params)
}

export async function openRuntimeOnboardingGhAuthTerminal(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string }
): Promise<{ ptyId: string; devServerId: string }> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.openGhAuthTerminal(params)
  }
  return callRuntimeRpc(target, 'onboarding.openGhAuthTerminal', params)
}

export async function detectRuntimeOnboardingWindowsCapabilities(
  settings: RuntimeOnboardingSettings | null | undefined,
  params: { devServerId: string }
): Promise<WindowsTerminalCapabilities> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.onboarding.detectWindowsCapabilities(params)
  }
  return callRuntimeRpc(target, 'onboarding.detectWindowsCapabilities', params)
}
