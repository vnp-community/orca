/* Why: mirrors runtime-github-client.ts's hybrid-routing shape for the
   `orcaProfiles:*` preload methods. Desktop-local calls stay on
   window.api.orcaProfiles (real IPC, unchanged behavior); paired/web callers
   route through the `orcaProfiles.*` runtime RPC instead — the RPC dispatcher
   runs in the same main process, so profile switch/relaunch and cloud-auth
   state stay identical either way. */
import type { GlobalSettings } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type RuntimeOrcaProfilesSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export function fetchRuntimeOrcaProfiles(
  settings: RuntimeOrcaProfilesSettings | null | undefined
): ReturnType<typeof window.api.orcaProfiles.list> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.list()
  }
  return callRuntimeRpc(target, 'orcaProfiles.list', {}, { timeoutMs: 15_000 })
}

export function fetchRuntimeOrcaProfileAuthStatus(
  settings: RuntimeOrcaProfilesSettings | null | undefined
): ReturnType<typeof window.api.orcaProfiles.authStatus> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.authStatus()
  }
  return callRuntimeRpc(target, 'orcaProfiles.authStatus', {}, { timeoutMs: 15_000 })
}

export function createRuntimeLocalOrcaProfile(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.createLocal>[0]
): ReturnType<typeof window.api.orcaProfiles.createLocal> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.createLocal(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.createLocal', args ?? {}, { timeoutMs: 15_000 })
}

export function createRuntimeCloudLinkedOrcaProfile(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.createCloudLinked>[0]
): ReturnType<typeof window.api.orcaProfiles.createCloudLinked> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.createCloudLinked(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.createCloudLinked', args ?? {}, { timeoutMs: 15_000 })
}

export function switchRuntimeOrcaProfile(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.switchProfile>[0]
): ReturnType<typeof window.api.orcaProfiles.switchProfile> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.switchProfile(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.switch', args, { timeoutMs: 15_000 })
}

export function transferRuntimeOrcaProfileProject(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.transferProject>[0]
): ReturnType<typeof window.api.orcaProfiles.transferProject> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.transferProject(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.transferProject', args, { timeoutMs: 15_000 })
}

export function findRuntimeOrcaProfileProjects(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.findProjectProfiles>[0]
): ReturnType<typeof window.api.orcaProfiles.findProjectProfiles> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.findProjectProfiles(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.findProjectProfiles', args, { timeoutMs: 15_000 })
}

export function connectRuntimeCurrentOrcaProfile(
  settings: RuntimeOrcaProfilesSettings | null | undefined
): ReturnType<typeof window.api.orcaProfiles.connectCurrent> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.connectCurrent()
  }
  return callRuntimeRpc(target, 'orcaProfiles.connectCurrent', {}, { timeoutMs: 30_000 })
}

export function refreshRuntimeCurrentOrcaProfileAuth(
  settings: RuntimeOrcaProfilesSettings | null | undefined
): ReturnType<typeof window.api.orcaProfiles.refreshAuth> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.refreshAuth()
  }
  return callRuntimeRpc(target, 'orcaProfiles.refreshAuth', {}, { timeoutMs: 15_000 })
}

export function signOutRuntimeCurrentOrcaProfile(
  settings: RuntimeOrcaProfilesSettings | null | undefined
): ReturnType<typeof window.api.orcaProfiles.signOutCurrent> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.signOutCurrent()
  }
  return callRuntimeRpc(target, 'orcaProfiles.signOutCurrent', {}, { timeoutMs: 15_000 })
}

export function selectRuntimeOrcaProfileOrg(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.selectOrg>[0]
): ReturnType<typeof window.api.orcaProfiles.selectOrg> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.selectOrg(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.selectOrg', args, { timeoutMs: 15_000 })
}

export function listRuntimeOrcaProfileOrgMembers(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.orgMembersList>[0]
): ReturnType<typeof window.api.orcaProfiles.orgMembersList> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.orgMembersList(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.orgMembersList', args, { timeoutMs: 15_000 })
}

export function inviteRuntimeOrcaProfileOrgMember(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.orgMemberInvite>[0]
): ReturnType<typeof window.api.orcaProfiles.orgMemberInvite> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.orgMemberInvite(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.orgMemberInvite', args, { timeoutMs: 15_000 })
}

export function revokeRuntimeOrcaProfileOrgInvite(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.orgInviteRevoke>[0]
): ReturnType<typeof window.api.orcaProfiles.orgInviteRevoke> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.orgInviteRevoke(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.orgInviteRevoke', args, { timeoutMs: 15_000 })
}

export function changeRuntimeOrcaProfileOrgMemberRole(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.orgMemberChangeRole>[0]
): ReturnType<typeof window.api.orcaProfiles.orgMemberChangeRole> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.orgMemberChangeRole(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.orgMemberChangeRole', args, { timeoutMs: 15_000 })
}

export function removeRuntimeOrcaProfileOrgMember(
  settings: RuntimeOrcaProfilesSettings | null | undefined,
  args: Parameters<typeof window.api.orcaProfiles.orgMemberRemove>[0]
): ReturnType<typeof window.api.orcaProfiles.orgMemberRemove> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.orcaProfiles.orgMemberRemove(args)
  }
  return callRuntimeRpc(target, 'orcaProfiles.orgMemberRemove', args, { timeoutMs: 15_000 })
}
