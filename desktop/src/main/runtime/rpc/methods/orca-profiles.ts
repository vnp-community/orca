import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import type {
  ConnectCurrentOrcaProfileResult,
  CreateCloudLinkedOrcaProfileArgs,
  CreateCloudLinkedOrcaProfileResult,
  CreateLocalOrcaProfileArgs,
  CreateLocalOrcaProfileResult,
  FindOrcaProfileProjectsByPathArgs,
  FindOrcaProfileProjectsByPathResult,
  OrcaProfileAuthStatus,
  OrcaProfileListResult,
  OrcaProfileOrgInviteRevokeArgs,
  OrcaProfileOrgMemberChangeRoleArgs,
  OrcaProfileOrgMemberInviteArgs,
  OrcaProfileOrgMemberMutationResult,
  OrcaProfileOrgMemberRemoveArgs,
  OrcaProfileOrgMembersListResult,
  RefreshCurrentOrcaProfileAuthResult,
  SelectOrcaProfileOrgResult,
  SignOutCurrentOrcaProfileResult,
  SwitchOrcaProfileArgs,
  SwitchOrcaProfileResult,
  TransferOrcaProfileProjectArgs,
  TransferOrcaProfileProjectResult
} from '../../../../shared/orca-profiles'
import { normalizeExecutionHostId } from '../../../../shared/execution-host'
import {
  getActiveOrcaProfileHandlerContext,
  runBeforeProfileRelaunch,
  scheduleProfileRelaunch
} from '../../../ipc/orca-profiles'
import {
  createLocalOrcaProfile,
  getOrcaProfileListState,
  seedNewOrcaProfileTelemetryConsent,
  setActiveOrcaProfile
} from '../../../orca-profiles/profile-index-store'
import { getProfileUserDataPath } from '../../../orca-profiles/profile-storage-paths'
import { isMultiProfileUiEnabled } from '../../../orca-profiles/profile-ui-scope'
import { transferOrcaProfileProject } from '../../../orca-profiles/profile-project-transfer'
import { findOrcaProfileProjectsByPath } from '../../../orca-profiles/profile-project-presence'
import {
  createCloudLinkedOrcaProfile,
  connectCurrentOrcaProfile,
  getCurrentOrcaProfileAuthStatus,
  refreshCurrentOrcaProfileAuth,
  selectCurrentOrcaProfileOrg,
  signOutCurrentOrcaProfile
} from '../../../orca-profiles/profile-cloud-service'
import {
  changeOrcaProfileOrgMemberRole,
  inviteOrcaProfileOrgMember,
  listOrcaProfileOrgMembers,
  removeOrcaProfileOrgMember,
  revokeOrcaProfileOrgInvite
} from '../../../orca-profiles/profile-cloud-org-members-service'

// Why: methods that mutate the active-profile index (createLocal, switch,
// transferProject move-mode, createCloudLinked) need the exact Store +
// onBeforeRelaunch hook the real ipcMain handlers were registered with, so a
// profile switch/relaunch triggered over RPC runs the same pre-relaunch
// cleanup (renderer scrollback capture, PTY kill, stats flush) as the IPC
// path. See ipc/orca-profiles.ts#getActiveOrcaProfileHandlerContext.
function requireOrcaProfileHandlerContext(): NonNullable<
  ReturnType<typeof getActiveOrcaProfileHandlerContext>
> {
  const context = getActiveOrcaProfileHandlerContext()
  if (!context) {
    throw new Error('orca_profile_handlers_unavailable')
  }
  return context
}

const OrcaProfileIdParams = z.object({
  profileId: z.string().min(1, 'invalid_orca_profile_id')
})

const CreateLocalOrcaProfileParams = z
  .object({
    name: z.string().optional()
  })
  .nullish()

const CreateCloudLinkedOrcaProfileParams = z
  .object({
    orgId: z.string().optional(),
    name: z.string().optional()
  })
  .nullish()

const TransferOrcaProfileProjectParams = z.object({
  sourceProfileId: z.string().min(1, 'invalid_orca_profile_project_transfer'),
  targetProfileId: z.string().min(1, 'invalid_orca_profile_project_transfer'),
  repoId: z.string().min(1, 'invalid_orca_profile_project_transfer'),
  mode: z.enum(['move', 'copy'])
})

const FindProjectProfilesParams = z.object({
  path: z.string().min(1, 'invalid_orca_profile_project_path'),
  connectionId: z.string().nullish(),
  executionHostId: z.string().nullish(),
  excludeProfileId: z.string().nullish()
})

const OrgIdParams = z.object({
  orgId: z.string().min(1, 'invalid_orca_profile_org_selection')
})

const OrgRoleEnum = z.enum(['owner', 'admin', 'member'])

const OrgMemberInviteParams = z.object({
  orgId: z.string().min(1, 'invalid_orca_profile_org_selection'),
  email: z.string().min(1, 'invalid_orca_org_member_email'),
  role: OrgRoleEnum
})

const OrgInviteRevokeParams = z.object({
  orgId: z.string().min(1, 'invalid_orca_profile_org_selection'),
  email: z.string().min(1, 'invalid_orca_org_member_email')
})

const OrgMemberChangeRoleParams = z.object({
  orgId: z.string().min(1, 'invalid_orca_profile_org_selection'),
  userId: z.string().min(1, 'invalid_orca_org_member_user'),
  role: OrgRoleEnum
})

const OrgMemberRemoveParams = z.object({
  orgId: z.string().min(1, 'invalid_orca_profile_org_selection'),
  userId: z.string().min(1, 'invalid_orca_org_member_user')
})

function findProjectProfilesArgs(
  params: z.infer<typeof FindProjectProfilesParams>
): FindOrcaProfileProjectsByPathArgs {
  return {
    path: params.path.trim(),
    connectionId: params.connectionId?.trim() || null,
    executionHostId: params.executionHostId ? normalizeExecutionHostId(params.executionHostId) : null,
    excludeProfileId: params.excludeProfileId?.trim() || null
  }
}

// Why: one wrapper per `orcaProfiles:*` ipcMain channel (see
// desktop/src/main/ipc/orca-profiles.ts and
// desktop/src/main/ipc/orca-profile-org-members-handlers.ts), calling the
// exact same profile-store / profile-cloud-service functions so remote/mobile
// RPC callers observe identical state to the desktop's own renderer.
export const ORCA_PROFILES_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'orcaProfiles.list',
    params: null,
    handler: async (): Promise<OrcaProfileListResult> => ({
      ...getOrcaProfileListState(),
      multiProfileUi: isMultiProfileUiEnabled()
    })
  }),
  defineMethod({
    name: 'orcaProfiles.authStatus',
    params: null,
    handler: async (): Promise<OrcaProfileAuthStatus> =>
      getCurrentOrcaProfileAuthStatus(getProfileUserDataPath())
  }),
  defineMethod({
    name: 'orcaProfiles.createLocal',
    params: CreateLocalOrcaProfileParams,
    handler: async (params): Promise<CreateLocalOrcaProfileResult> => {
      const { store } = requireOrcaProfileHandlerContext()
      const args: CreateLocalOrcaProfileArgs | undefined = params ?? undefined
      const result = createLocalOrcaProfile(args)
      seedNewOrcaProfileTelemetryConsent(result.profile.id, store.getSettings().telemetry)
      return result
    }
  }),
  defineMethod({
    name: 'orcaProfiles.switch',
    params: OrcaProfileIdParams,
    handler: async (params: SwitchOrcaProfileArgs): Promise<SwitchOrcaProfileResult> => {
      const { store, options } = requireOrcaProfileHandlerContext()
      const profileId = params.profileId.trim()
      if (!profileId) {
        throw new Error('invalid_orca_profile_id')
      }
      const current = getOrcaProfileListState()
      if (profileId === current.activeProfileId) {
        return { status: 'already-active' }
      }
      // Why: the current profile must be persisted before the global index
      // points startup at the target profile.
      await runBeforeProfileRelaunch(options.onBeforeRelaunch)
      store.flush()
      setActiveOrcaProfile(profileId)
      scheduleProfileRelaunch()
      return { status: 'relaunching' }
    }
  }),
  defineMethod({
    name: 'orcaProfiles.transferProject',
    params: TransferOrcaProfileProjectParams,
    handler: async (
      params: TransferOrcaProfileProjectArgs
    ): Promise<TransferOrcaProfileProjectResult> => {
      const { store, options } = requireOrcaProfileHandlerContext()
      const current = getOrcaProfileListState()
      if (params.targetProfileId === current.activeProfileId) {
        throw new Error('active_target_orca_profile_transfer_requires_relaunch')
      }
      if (params.mode === 'move' && params.sourceProfileId === current.activeProfileId) {
        // Why: transfer before any relaunch side effect so a duplicate-target
        // or validation failure cannot strand the app in a quitting state.
        store.flush()
        const result = transferOrcaProfileProject(params, getProfileUserDataPath())
        if (result.status === 'transferred') {
          store.freezeWrites()
          await runBeforeProfileRelaunch(options.onBeforeRelaunch)
          setActiveOrcaProfile(params.targetProfileId)
          scheduleProfileRelaunch()
          return { ...result, willRelaunch: true }
        }
        return result
      }
      store.flush()
      return transferOrcaProfileProject(params, getProfileUserDataPath())
    }
  }),
  defineMethod({
    name: 'orcaProfiles.findProjectProfiles',
    params: FindProjectProfilesParams,
    handler: async (params): Promise<FindOrcaProfileProjectsByPathResult> =>
      findOrcaProfileProjectsByPath(findProjectProfilesArgs(params), getProfileUserDataPath())
  }),
  defineMethod({
    name: 'orcaProfiles.connectCurrent',
    params: null,
    handler: async (): Promise<ConnectCurrentOrcaProfileResult> =>
      connectCurrentOrcaProfile(getProfileUserDataPath())
  }),
  defineMethod({
    name: 'orcaProfiles.createCloudLinked',
    params: CreateCloudLinkedOrcaProfileParams,
    handler: async (params): Promise<CreateCloudLinkedOrcaProfileResult> => {
      const { store } = requireOrcaProfileHandlerContext()
      const args: CreateCloudLinkedOrcaProfileArgs = {
        ...(params?.orgId ? { orgId: params.orgId.trim() } : {}),
        ...(params?.name ? { name: params.name.trim() } : {})
      }
      const result = await createCloudLinkedOrcaProfile(getProfileUserDataPath(), args)
      if (result.status === 'created') {
        seedNewOrcaProfileTelemetryConsent(result.profile.id, store.getSettings().telemetry)
      }
      return result
    }
  }),
  defineMethod({
    name: 'orcaProfiles.refreshAuth',
    params: null,
    handler: async (): Promise<RefreshCurrentOrcaProfileAuthResult> =>
      refreshCurrentOrcaProfileAuth(getProfileUserDataPath())
  }),
  defineMethod({
    name: 'orcaProfiles.signOutCurrent',
    params: null,
    handler: async (): Promise<SignOutCurrentOrcaProfileResult> =>
      signOutCurrentOrcaProfile(getProfileUserDataPath())
  }),
  defineMethod({
    name: 'orcaProfiles.selectOrg',
    params: OrgIdParams,
    handler: async (params): Promise<SelectOrcaProfileOrgResult> =>
      selectCurrentOrcaProfileOrg(getProfileUserDataPath(), params.orgId.trim())
  }),
  defineMethod({
    name: 'orcaProfiles.orgMembersList',
    params: OrgIdParams,
    handler: async (params): Promise<OrcaProfileOrgMembersListResult> =>
      listOrcaProfileOrgMembers(getProfileUserDataPath(), params.orgId.trim())
  }),
  defineMethod({
    name: 'orcaProfiles.orgMemberInvite',
    params: OrgMemberInviteParams,
    handler: async (params: OrcaProfileOrgMemberInviteArgs): Promise<OrcaProfileOrgMemberMutationResult> =>
      inviteOrcaProfileOrgMember(getProfileUserDataPath(), params)
  }),
  defineMethod({
    name: 'orcaProfiles.orgInviteRevoke',
    params: OrgInviteRevokeParams,
    handler: async (
      params: OrcaProfileOrgInviteRevokeArgs
    ): Promise<OrcaProfileOrgMemberMutationResult> =>
      revokeOrcaProfileOrgInvite(getProfileUserDataPath(), params)
  }),
  defineMethod({
    name: 'orcaProfiles.orgMemberChangeRole',
    params: OrgMemberChangeRoleParams,
    handler: async (
      params: OrcaProfileOrgMemberChangeRoleArgs
    ): Promise<OrcaProfileOrgMemberMutationResult> =>
      changeOrcaProfileOrgMemberRole(getProfileUserDataPath(), params)
  }),
  defineMethod({
    name: 'orcaProfiles.orgMemberRemove',
    params: OrgMemberRemoveParams,
    handler: async (
      params: OrcaProfileOrgMemberRemoveArgs
    ): Promise<OrcaProfileOrgMemberMutationResult> =>
      removeOrcaProfileOrgMember(getProfileUserDataPath(), params)
  })
]
