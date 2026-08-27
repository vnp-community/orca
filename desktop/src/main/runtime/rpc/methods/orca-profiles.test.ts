import { describe, expect, it, vi, beforeEach } from 'vitest'

const {
  mockGetActiveOrcaProfileHandlerContext,
  mockRunBeforeProfileRelaunch,
  mockScheduleProfileRelaunch,
  mockCreateLocalOrcaProfile,
  mockGetOrcaProfileListState,
  mockSeedNewOrcaProfileTelemetryConsent,
  mockSetActiveOrcaProfile,
  mockGetProfileUserDataPath,
  mockIsMultiProfileUiEnabled,
  mockTransferOrcaProfileProject,
  mockFindOrcaProfileProjectsByPath,
  mockCreateCloudLinkedOrcaProfile,
  mockConnectCurrentOrcaProfile,
  mockGetCurrentOrcaProfileAuthStatus,
  mockRefreshCurrentOrcaProfileAuth,
  mockSelectCurrentOrcaProfileOrg,
  mockSignOutCurrentOrcaProfile,
  mockChangeOrcaProfileOrgMemberRole,
  mockInviteOrcaProfileOrgMember,
  mockListOrcaProfileOrgMembers,
  mockRemoveOrcaProfileOrgMember,
  mockRevokeOrcaProfileOrgInvite
} = vi.hoisted(() => ({
  mockGetActiveOrcaProfileHandlerContext: vi.fn(),
  mockRunBeforeProfileRelaunch: vi.fn().mockResolvedValue(undefined),
  mockScheduleProfileRelaunch: vi.fn(),
  mockCreateLocalOrcaProfile: vi.fn(),
  mockGetOrcaProfileListState: vi.fn(),
  mockSeedNewOrcaProfileTelemetryConsent: vi.fn(),
  mockSetActiveOrcaProfile: vi.fn(),
  mockGetProfileUserDataPath: vi.fn().mockReturnValue('/fake/user-data'),
  mockIsMultiProfileUiEnabled: vi.fn().mockReturnValue(false),
  mockTransferOrcaProfileProject: vi.fn(),
  mockFindOrcaProfileProjectsByPath: vi.fn(),
  mockCreateCloudLinkedOrcaProfile: vi.fn(),
  mockConnectCurrentOrcaProfile: vi.fn(),
  mockGetCurrentOrcaProfileAuthStatus: vi.fn(),
  mockRefreshCurrentOrcaProfileAuth: vi.fn(),
  mockSelectCurrentOrcaProfileOrg: vi.fn(),
  mockSignOutCurrentOrcaProfile: vi.fn(),
  mockChangeOrcaProfileOrgMemberRole: vi.fn(),
  mockInviteOrcaProfileOrgMember: vi.fn(),
  mockListOrcaProfileOrgMembers: vi.fn(),
  mockRemoveOrcaProfileOrgMember: vi.fn(),
  mockRevokeOrcaProfileOrgInvite: vi.fn()
}))

vi.mock('../../../ipc/orca-profiles', () => ({
  getActiveOrcaProfileHandlerContext: mockGetActiveOrcaProfileHandlerContext,
  runBeforeProfileRelaunch: mockRunBeforeProfileRelaunch,
  scheduleProfileRelaunch: mockScheduleProfileRelaunch
}))

vi.mock('../../../orca-profiles/profile-index-store', () => ({
  createLocalOrcaProfile: mockCreateLocalOrcaProfile,
  getOrcaProfileListState: mockGetOrcaProfileListState,
  seedNewOrcaProfileTelemetryConsent: mockSeedNewOrcaProfileTelemetryConsent,
  setActiveOrcaProfile: mockSetActiveOrcaProfile
}))

vi.mock('../../../orca-profiles/profile-storage-paths', () => ({
  getProfileUserDataPath: mockGetProfileUserDataPath
}))

vi.mock('../../../orca-profiles/profile-ui-scope', () => ({
  isMultiProfileUiEnabled: mockIsMultiProfileUiEnabled
}))

vi.mock('../../../orca-profiles/profile-project-transfer', () => ({
  transferOrcaProfileProject: mockTransferOrcaProfileProject
}))

vi.mock('../../../orca-profiles/profile-project-presence', () => ({
  findOrcaProfileProjectsByPath: mockFindOrcaProfileProjectsByPath
}))

vi.mock('../../../orca-profiles/profile-cloud-service', () => ({
  createCloudLinkedOrcaProfile: mockCreateCloudLinkedOrcaProfile,
  connectCurrentOrcaProfile: mockConnectCurrentOrcaProfile,
  getCurrentOrcaProfileAuthStatus: mockGetCurrentOrcaProfileAuthStatus,
  refreshCurrentOrcaProfileAuth: mockRefreshCurrentOrcaProfileAuth,
  selectCurrentOrcaProfileOrg: mockSelectCurrentOrcaProfileOrg,
  signOutCurrentOrcaProfile: mockSignOutCurrentOrcaProfile
}))

vi.mock('../../../orca-profiles/profile-cloud-org-members-service', () => ({
  changeOrcaProfileOrgMemberRole: mockChangeOrcaProfileOrgMemberRole,
  inviteOrcaProfileOrgMember: mockInviteOrcaProfileOrgMember,
  listOrcaProfileOrgMembers: mockListOrcaProfileOrgMembers,
  removeOrcaProfileOrgMember: mockRemoveOrcaProfileOrgMember,
  revokeOrcaProfileOrgInvite: mockRevokeOrcaProfileOrgInvite
}))

import { RpcDispatcher } from '../dispatcher'
import type { RpcRequest } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { ORCA_PROFILES_METHODS } from './orca-profiles'

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function makeDispatcher() {
  const runtime = { getRuntimeId: () => 'test' } as unknown as OrcaRuntimeService
  return new RpcDispatcher({ runtime, methods: ORCA_PROFILES_METHODS })
}

function makeFakeStore(telemetry: unknown = { enabled: true }) {
  return {
    flush: vi.fn(),
    freezeWrites: vi.fn(),
    getSettings: vi.fn().mockReturnValue({ telemetry })
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetProfileUserDataPath.mockReturnValue('/fake/user-data')
  mockGetOrcaProfileListState.mockReturnValue({ activeProfileId: 'local-default', profiles: [] })
})

describe('orcaProfiles RPC methods', () => {
  it('orcaProfiles.list merges profile state with multiProfileUi flag', async () => {
    mockIsMultiProfileUiEnabled.mockReturnValue(true)
    const response = await makeDispatcher().dispatch(makeRequest('orcaProfiles.list'))
    expect(response).toMatchObject({
      ok: true,
      result: { activeProfileId: 'local-default', profiles: [], multiProfileUi: true }
    })
  })

  it('orcaProfiles.authStatus calls getCurrentOrcaProfileAuthStatus with the profile data path', async () => {
    mockGetCurrentOrcaProfileAuthStatus.mockReturnValue({ activeProfileId: 'a', configured: true })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.authStatus'))
    expect(mockGetCurrentOrcaProfileAuthStatus).toHaveBeenCalledWith('/fake/user-data')
  })

  it('orcaProfiles.createLocal seeds telemetry consent using the registered Store', async () => {
    const store = makeFakeStore({ enabled: false })
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue({ store, options: {} })
    mockCreateLocalOrcaProfile.mockReturnValue({
      activeProfileId: 'a',
      profiles: [],
      profile: { id: 'local-xyz' }
    })
    const response = await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.createLocal', { name: 'Work' })
    )
    expect(mockCreateLocalOrcaProfile).toHaveBeenCalledWith({ name: 'Work' })
    expect(mockSeedNewOrcaProfileTelemetryConsent).toHaveBeenCalledWith('local-xyz', {
      enabled: false
    })
    expect(response.ok).toBe(true)
  })

  it('orcaProfiles.createLocal throws when no handler context is registered', async () => {
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue(null)
    const response = await makeDispatcher().dispatch(makeRequest('orcaProfiles.createLocal', {}))
    expect(response.ok).toBe(false)
    if (!response.ok) {
      expect(response.error.message).toContain('orca_profile_handlers_unavailable')
    }
  })

  it('orcaProfiles.switch returns already-active without relaunching', async () => {
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue({ store: makeFakeStore(), options: {} })
    mockGetOrcaProfileListState.mockReturnValue({ activeProfileId: 'local-default', profiles: [] })
    const response = await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.switch', { profileId: 'local-default' })
    )
    expect(response).toMatchObject({ ok: true, result: { status: 'already-active' } })
    expect(mockScheduleProfileRelaunch).not.toHaveBeenCalled()
  })

  it('orcaProfiles.switch flushes, sets active profile, and schedules relaunch', async () => {
    const store = makeFakeStore()
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue({ store, options: {} })
    mockGetOrcaProfileListState.mockReturnValue({ activeProfileId: 'local-default', profiles: [] })
    const response = await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.switch', { profileId: 'cloud-1' })
    )
    expect(store.flush).toHaveBeenCalledTimes(1)
    expect(mockSetActiveOrcaProfile).toHaveBeenCalledWith('cloud-1')
    expect(mockScheduleProfileRelaunch).toHaveBeenCalledTimes(1)
    expect(response).toMatchObject({ ok: true, result: { status: 'relaunching' } })
  })

  it('orcaProfiles.transferProject delegates to transferOrcaProfileProject', async () => {
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue({ store: makeFakeStore(), options: {} })
    mockGetOrcaProfileListState.mockReturnValue({ activeProfileId: 'other', profiles: [] })
    mockTransferOrcaProfileProject.mockReturnValue({
      status: 'transferred',
      mode: 'copy',
      sourceProfileId: 'a',
      targetProfileId: 'b',
      sourceRepoId: 'r1',
      targetRepoId: 'r2',
      targetProjectId: null
    })
    const response = await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.transferProject', {
        sourceProfileId: 'a',
        targetProfileId: 'b',
        repoId: 'r1',
        mode: 'copy'
      })
    )
    expect(mockTransferOrcaProfileProject).toHaveBeenCalledWith(
      { sourceProfileId: 'a', targetProfileId: 'b', repoId: 'r1', mode: 'copy' },
      '/fake/user-data'
    )
    expect(response.ok).toBe(true)
  })

  it('orcaProfiles.findProjectProfiles normalizes args and delegates', async () => {
    mockFindOrcaProfileProjectsByPath.mockReturnValue({ projects: [] })
    await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.findProjectProfiles', { path: '/repos/foo' })
    )
    expect(mockFindOrcaProfileProjectsByPath).toHaveBeenCalledWith(
      {
        path: '/repos/foo',
        connectionId: null,
        executionHostId: null,
        excludeProfileId: null
      },
      '/fake/user-data'
    )
  })

  it('orcaProfiles.connectCurrent delegates to connectCurrentOrcaProfile', async () => {
    mockConnectCurrentOrcaProfile.mockResolvedValue({ status: 'cancelled', auth: {} })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.connectCurrent'))
    expect(mockConnectCurrentOrcaProfile).toHaveBeenCalledWith('/fake/user-data')
  })

  it('orcaProfiles.createCloudLinked seeds telemetry consent on created status', async () => {
    const store = makeFakeStore({ enabled: true })
    mockGetActiveOrcaProfileHandlerContext.mockReturnValue({ store, options: {} })
    mockCreateCloudLinkedOrcaProfile.mockResolvedValue({
      status: 'created',
      auth: {},
      activeProfileId: 'cloud-1',
      profiles: [],
      profile: { id: 'cloud-1' }
    })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.createCloudLinked', {}))
    expect(mockSeedNewOrcaProfileTelemetryConsent).toHaveBeenCalledWith('cloud-1', { enabled: true })
  })

  it('orcaProfiles.refreshAuth delegates to refreshCurrentOrcaProfileAuth', async () => {
    mockRefreshCurrentOrcaProfileAuth.mockResolvedValue({ status: 'local', auth: {} })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.refreshAuth'))
    expect(mockRefreshCurrentOrcaProfileAuth).toHaveBeenCalledWith('/fake/user-data')
  })

  it('orcaProfiles.signOutCurrent delegates to signOutCurrentOrcaProfile', async () => {
    mockSignOutCurrentOrcaProfile.mockResolvedValue({
      status: 'signed-out',
      auth: {},
      activeProfileId: 'local-default',
      profiles: []
    })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.signOutCurrent'))
    expect(mockSignOutCurrentOrcaProfile).toHaveBeenCalledWith('/fake/user-data')
  })

  it('orcaProfiles.selectOrg delegates to selectCurrentOrcaProfileOrg', async () => {
    mockSelectCurrentOrcaProfileOrg.mockResolvedValue({ status: 'failed', auth: {}, error: 'x' })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.selectOrg', { orgId: 'org-1' }))
    expect(mockSelectCurrentOrcaProfileOrg).toHaveBeenCalledWith('/fake/user-data', 'org-1')
  })

  it('orcaProfiles.orgMembersList delegates to listOrcaProfileOrgMembers', async () => {
    mockListOrcaProfileOrgMembers.mockResolvedValue({ status: 'unconfigured' })
    await makeDispatcher().dispatch(makeRequest('orcaProfiles.orgMembersList', { orgId: 'org-1' }))
    expect(mockListOrcaProfileOrgMembers).toHaveBeenCalledWith('/fake/user-data', 'org-1')
  })

  it('orcaProfiles.orgMemberInvite delegates to inviteOrcaProfileOrgMember', async () => {
    mockInviteOrcaProfileOrgMember.mockResolvedValue({ status: 'ok' })
    await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.orgMemberInvite', {
        orgId: 'org-1',
        email: 'a@example.com',
        role: 'member'
      })
    )
    expect(mockInviteOrcaProfileOrgMember).toHaveBeenCalledWith('/fake/user-data', {
      orgId: 'org-1',
      email: 'a@example.com',
      role: 'member'
    })
  })

  it('orcaProfiles.orgInviteRevoke delegates to revokeOrcaProfileOrgInvite', async () => {
    mockRevokeOrcaProfileOrgInvite.mockResolvedValue({ status: 'ok' })
    await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.orgInviteRevoke', { orgId: 'org-1', email: 'a@example.com' })
    )
    expect(mockRevokeOrcaProfileOrgInvite).toHaveBeenCalledWith('/fake/user-data', {
      orgId: 'org-1',
      email: 'a@example.com'
    })
  })

  it('orcaProfiles.orgMemberChangeRole delegates to changeOrcaProfileOrgMemberRole', async () => {
    mockChangeOrcaProfileOrgMemberRole.mockResolvedValue({ status: 'ok' })
    await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.orgMemberChangeRole', {
        orgId: 'org-1',
        userId: 'user-1',
        role: 'admin'
      })
    )
    expect(mockChangeOrcaProfileOrgMemberRole).toHaveBeenCalledWith('/fake/user-data', {
      orgId: 'org-1',
      userId: 'user-1',
      role: 'admin'
    })
  })

  it('orcaProfiles.orgMemberRemove delegates to removeOrcaProfileOrgMember', async () => {
    mockRemoveOrcaProfileOrgMember.mockResolvedValue({ status: 'ok' })
    await makeDispatcher().dispatch(
      makeRequest('orcaProfiles.orgMemberRemove', { orgId: 'org-1', userId: 'user-1' })
    )
    expect(mockRemoveOrcaProfileOrgMember).toHaveBeenCalledWith('/fake/user-data', {
      orgId: 'org-1',
      userId: 'user-1'
    })
  })
})
