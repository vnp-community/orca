// DeptProfileAdmin.tsx — Department profile management for company admins (TDD-FE-11, TASK-FE-006)
import { useState, useEffect } from 'react'
import { ProfileEditor } from './ProfileEditor'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Badge } from '../ui/badge'
import { Skeleton } from '../ui/skeleton'
import type { Department } from '../../types/profile-types'

export function DeptProfileAdmin() {
  const [depts, setDepts]               = useState<Department[]>([])
  const [activeDeptId, setActiveDeptId] = useState<string | null>(null)
  const [isLoading, setIsLoading]       = useState(true)

  useEffect(() => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<Department[]>(target, 'profile.listDepts', {})
      .then(d => {
        setDepts(d)
        // Auto-select first department
        if (d.length > 0) {setActiveDeptId(d[0].id)}
      })
      .catch(() => setDepts([]))
      .finally(() => setIsLoading(false))
  }, [])

  if (isLoading) {
    return (
      <div className="dept-profile-admin p-4 space-y-2" data-testid="dept-profile-loading">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    )
  }

  if (depts.length === 0) {
    return (
      <div className="p-4 text-sm text-muted-foreground" data-testid="dept-profile-empty">
        No departments configured. Contact your organization admin.
      </div>
    )
  }

  return (
    <div className="dept-profile-admin space-y-4 p-4" data-testid="dept-profile-admin">
      {/* Department selector */}
      <div className="space-y-1">
        <p className="text-xs text-muted-foreground font-medium">Department</p>
        <div className="flex flex-wrap gap-2">
          {depts.map(dept => (
            <Badge
              key={dept.id}
              variant={activeDeptId === dept.id ? 'default' : 'outline'}
              className="cursor-pointer select-none"
              onClick={() => setActiveDeptId(dept.id)}
              data-testid={`dept-badge-${dept.id}`}
            >
              {dept.name}
              {dept.memberCount != null && (
                <span className="ml-1 opacity-60">({dept.memberCount})</span>
              )}
            </Badge>
          ))}
        </div>
      </div>

      {/* Department profile editor */}
      {activeDeptId ? (
        <div data-testid="dept-editor-container">
          <ProfileEditor scope="dept" scopeId={activeDeptId} />
        </div>
      ) : (
        <p className="text-sm text-muted-foreground" data-testid="dept-no-selection">
          Select a department above to edit its profile settings.
        </p>
      )}
    </div>
  )
}
