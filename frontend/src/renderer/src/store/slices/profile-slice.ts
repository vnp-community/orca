// profile-slice.ts — Profile hierarchy state (TDD-FE-11)
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { OrcaProfile, ResolvedProfile, Department } from '../../types/profile-types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type ProfileSlice = {
  /** User's own editable profile */
  userProfile: OrcaProfile | null
  /** Merged/inherited profile (company → dept → user) */
  resolvedProfile: ResolvedProfile | null
  /** Company-wide profile (admin only) */
  companyProfile: OrcaProfile | null
  /** Department list for company admin */
  depts: Department[]
  /** Loading state for profile fetch */
  profileIsLoading: boolean

  setUserProfile: (p: OrcaProfile) => void
  setResolved: (p: ResolvedProfile) => void
  setCompanyProfile: (p: OrcaProfile) => void
  setDepts: (depts: Department[]) => void
  setProfileLoading: (v: boolean) => void
  clearProfile: () => void
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createProfileSlice: StateCreator<AppState, [], [], ProfileSlice> = (set) => ({
  userProfile:      null,
  resolvedProfile:  null,
  companyProfile:   null,
  depts:            [],
  profileIsLoading: false,

  setUserProfile:    (p)     => set(() => ({ userProfile: p })),
  setResolved:       (p)     => set(() => ({ resolvedProfile: p })),
  setCompanyProfile: (p)     => set(() => ({ companyProfile: p })),
  setDepts:          (depts) => set(() => ({ depts })),
  setProfileLoading: (v)     => set(() => ({ profileIsLoading: v })),
  clearProfile:      ()      => set(() => ({
    userProfile:     null,
    resolvedProfile: null,
    companyProfile:  null,
    depts:           [],
  })),
})
