// useProfile.ts — Profile read/write hooks (TDD-FE-11)
import { useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
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
    // BL-PRF-02: browser tạo traceId TRƯỚC khi gọi RPC (CR-TRACE-000 §3.3 hàng 1).
    // Đây là root span của cả 2 lời gọi song song bên dưới — dùng chung 1 id vì về
    // nghiệp vụ đây là MỘT thao tác "load profile" duy nhất, không phải 2 flow riêng.
    const span = Tracers.uiProfileResolveFlow.start()

    Promise.all([
      callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', { traceId: span.id }),
      callRuntimeRpc<OrcaProfile>(target, 'profile.getUser', { traceId: span.id }),
    ])
      .then(([resolved, user]) => {
        store.setResolved(resolved)
        store.setUserProfile(user)
        span.ok({ hasSecurityLock: resolved?.security !== undefined })
      })
      .catch(err => {
        console.error('[useProfile] fetch failed:', err)
        span.fail(err)
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
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // BL-PRF-01: 1 span bao phủ toàn bộ save + refetch resolved (nếu scope='user'),
      // field `scope` phân biệt 3 nhánh.
      const span = Tracers.uiProfileUpdateFlow.start({ scope, targetId: scopeId })
      try {
        if (scope === 'user') {
          await callRuntimeRpc(target, 'profile.updateUser', { profile, traceId: span.id })
          const resolved = await callRuntimeRpc<ResolvedProfile>(target, 'profile.getResolved', {})
          const store = useAppStore.getState() as any
          store.setResolved(resolved)
          store.setUserProfile(profile)
          toast.success('Profile saved')
        } else if (scope === 'company') {
          await callRuntimeRpc(target, 'profile.updateCompany', { profile, traceId: span.id })
          toast.success('Company profile updated')
        } else if (scope === 'dept' && scopeId) {
          await callRuntimeRpc(target, 'profile.updateDept', { deptId: scopeId, profile, traceId: span.id })
          toast.success('Department profile updated')
        }
        span.ok({ scope })
      } catch (err: any) {
        span.fail(err, { scope })
        toast.error(err?.message ?? 'Failed to save profile')
        throw err
      }
    },
    []
  )

  return { saveProfile }
}
