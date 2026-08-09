// ProfileFieldRow.tsx — Layout row for a profile setting with source badge (TDD-FE-11)
import type { ReactNode } from 'react'
import type { ProfileSource } from '../../types/profile-types'
import { ProfileSourceBadge } from './ProfileSourceBadge'

type ProfileFieldRowProps = {
  label:    string
  source?:  ProfileSource
  locked?:  boolean
  children: ReactNode
}

export function ProfileFieldRow({ label, source, locked, children }: ProfileFieldRowProps) {
  return (
    <div className="profile-field-row flex items-center gap-3 py-2">
      <label className="text-sm font-medium w-44 shrink-0">{label}</label>
      <div className="flex-1">{children}</div>
      <ProfileSourceBadge source={source} locked={locked} />
    </div>
  )
}
