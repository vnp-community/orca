// useProfile.ts — Profile read/write hooks (TDD-FE-11)
import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaProfile, ResolvedProfile } from '../types/profile-types'
import { toast } from 'sonner'

// --- Read hook ---

export function useProfile() {
  const { resolvedProfile, userProfile, profileIsLoading } = useAppStore(s => ({
    resolvedProfile:  (s as any).resolvedProfile as ResolvedProfile | null,
    userProfile:      (s as any).userProfile as OrcaProfile | null,
    profileIsLoading: (s as any).profileIsLoading as boolean,
  }))

  useEffect(() => {
    const store = useAppStore.getState() as any
    store.setProfileLoading(true)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)

    Promise.all([
      callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', {}),
      callRuntimeRpc<OrcaProfile>(target, 'profile.getUser', {}),
    ])
      .then(([resolved, user]) => {
        store.setResolved(resolved)
        store.setUserProfile(user)
      })
      .catch(err => {
        console.error('[useProfile] fetch failed:', err)
      })
      .finally(() => {
        store.setProfileLoading(false)
      })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  return { resolvedProfile, userProfile, profileIsLoading }
}

// --- Write hook ---

export function useProfileActions() {
  const saveProfile = useCallback(
    async (
      scope: 'user' | 'dept' | 'company',
      profile: OrcaProfile,
      scopeId?: string
    ) => {
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        if (scope === 'user') {
          await callRuntimeRpc(target, 'profile.updateUser', { profile })
          const resolved = await callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', {})
          const store = useAppStore.getState() as any
          store.setResolved(resolved)
          store.setUserProfile(profile)
          toast.success('Profile saved')
        } else if (scope === 'company') {
          await callRuntimeRpc(target, 'profile.updateCompany', { profile })
          toast.success('Company profile updated')
        } else if (scope === 'dept' && scopeId) {
          await callRuntimeRpc(target, 'profile.updateDept', { deptId: scopeId, profile })
          toast.success('Department profile updated')
        }
      } catch (err: any) {
        toast.error(err?.message ?? 'Failed to save profile')
        throw err
      }
    },
    []
  )

  return { saveProfile }
}
