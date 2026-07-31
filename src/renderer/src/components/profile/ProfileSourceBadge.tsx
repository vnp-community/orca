// ProfileSourceBadge.tsx — Visual indicator of profile field origin (TDD-FE-11)
import type { ReactNode } from 'react'
import type { ProfileSource } from '../../types/profile-types'
import { Building2, Users, User, GitMerge, Lock } from 'lucide-react'
import { cn } from '../../lib/utils'

interface ProfileSourceBadgeProps {
  source?: ProfileSource
  locked?: boolean
}

type SourceConfig = { label: string; className: string; icon: ReactNode }

const SOURCE_CONFIG: Record<ProfileSource, SourceConfig> = {
  company: { label: 'Company', className: 'bg-purple-100 text-purple-700', icon: <Building2 size={10} /> },
  dept:    { label: 'Dept',    className: 'bg-blue-100 text-blue-700',     icon: <Users size={10} /> },
  user:    { label: 'User',    className: 'bg-green-100 text-green-700',   icon: <User size={10} /> },
  concat:  { label: 'Concat',  className: 'bg-gray-100 text-gray-600',    icon: <GitMerge size={10} /> },
}

export function ProfileSourceBadge({ source, locked }: ProfileSourceBadgeProps) {
  if (locked) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 bg-red-50 text-red-600 text-xs font-normal">
        <Lock size={10} /> Company Only
      </span>
    )
  }
  if (!source) return null
  const { label, className, icon } = SOURCE_CONFIG[source]
  return (
    <span className={cn('inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-normal', className)}>
      {icon} {label}
    </span>
  )
}
