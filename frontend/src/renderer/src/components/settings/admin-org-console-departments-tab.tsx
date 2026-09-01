// DepartmentsTab — split out of AdminOrgConsole.tsx (AGENTS.md max-lines
// budget). Uses the shared useDepartments hook (also used by UsersTab, see
// admin-org-console-users-tab.tsx) so both tabs stay in sync on the same
// live department list.
import { useCallback, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'
import { useDepartments } from './admin-org-console-shared'

export function DepartmentsTab(): React.JSX.Element {
  const { departments, loading, reload } = useDepartments()
  const [newDeptName, setNewDeptName] = useState('')
  const [creating, setCreating] = useState(false)

  const handleCreate = useCallback(() => {
    if (!newDeptName.trim() || creating) {
      return
    }
    setCreating(true)
    window.api.tenantProfile
      .createDepartment({ name: newDeptName.trim() })
      .then(() => {
        setNewDeptName('')
        reload()
        toast.success('Department created')
      })
      .catch((err) => toast.error(err instanceof Error ? err.message : String(err)))
      .finally(() => setCreating(false))
  }, [newDeptName, creating, reload])

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-end gap-2">
        <Input
          value={newDeptName}
          onChange={(e) => setNewDeptName(e.target.value)}
          placeholder="New department name"
          className="max-w-xs"
        />
        <Button disabled={!newDeptName.trim() || creating} onClick={handleCreate}>
          Create department
        </Button>
      </div>

      {loading && departments.length === 0 ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : departments.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No departments yet — create one above so you can grant dev server group access to it.
        </p>
      ) : (
        <div className="flex flex-wrap gap-2">
          {departments.map((dept) => (
            <Badge key={dept.id} variant="secondary">
              {dept.name}
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}
