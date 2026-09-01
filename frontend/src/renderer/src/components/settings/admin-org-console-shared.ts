// Shared department-loading hook for AdminOrgConsole's Departments and Users
// tabs — split out of AdminOrgConsole.tsx (AGENTS.md max-lines budget) since
// both DepartmentsTab and UsersTab need the same live department list.
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import type { TenantDepartment } from '../../../../shared/tenant-user-profile-types'

export function useDepartments(): {
  departments: TenantDepartment[]
  loading: boolean
  reload: () => void
} {
  const [departments, setDepartments] = useState<TenantDepartment[]>([])
  const [loading, setLoading] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    setLoading(true)
    window.api.tenantProfile
      .listDepartments()
      .then(setDepartments)
      .catch(() => toast.error('Failed to load departments'))
      .finally(() => setLoading(false))
  }, [reloadToken])

  return { departments, loading, reload: () => setReloadToken((n) => n + 1) }
}
