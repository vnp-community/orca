// CompanyProfileAdmin.tsx — Admin view for company + dept profile management (TDD-FE-11)
import { useEffect, useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { ProfileEditor } from './ProfileEditor'
import type { Department } from '../../types/profile-types'
import { Button } from '../ui/button'

export function CompanyProfileAdmin() {
  const [depts, setDepts]           = useState<Department[]>([])
  const [activeDeptId, setActiveDept] = useState<string | null>(null)

  useEffect(() => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<Department[]>(target, 'profile.listDepts', {})
      .then(d => setDepts(d))
      .catch(err => {
        console.error('[CompanyProfileAdmin] failed to load depts:', err)
      })
  }, [])

  return (
    <div className="company-profile-admin p-4 space-y-6">
      <h2 className="text-xl font-semibold">Company Profile</h2>

      {/* Dept selector */}
      <div className="flex gap-2 flex-wrap">
        {depts.map(d => (
          <Button
            key={d.id}
            variant={activeDeptId === d.id ? 'default' : 'outline'}
            size="sm"
            onClick={() => setActiveDept(d.id === activeDeptId ? null : d.id)}
          >
            {d.name}
          </Button>
        ))}
      </div>

      {/* Company-wide settings */}
      <div>
        <h3 className="text-base font-medium mb-2">Company-wide Settings</h3>
        <ProfileEditor scope="company" />
      </div>

      {/* Dept override */}
      {activeDeptId && (
        <div>
          <h3 className="text-base font-medium mb-2">
            Department Override — {depts.find(d => d.id === activeDeptId)?.name}
          </h3>
          <ProfileEditor scope="dept" scopeId={activeDeptId} />
        </div>
      )}
    </div>
  )
}
