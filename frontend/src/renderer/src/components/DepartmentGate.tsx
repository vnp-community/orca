// DepartmentGate.tsx — First-Login Department Gate (CR-DS-008).
// Blocks onboarding AND every other app feature until the caller's
// tenant-service department is set. Mounted as a full-screen sibling near
// the top of App.tsx's render tree (same spot as AgentHibernationGate/
// RetainedAgentsSyncGate) rather than wrapping App's own JSX — a
// position:fixed inset-0 overlay with a higher z-index than
// OnboardingFlow's (z-100) blocks it and everything else the same way
// OnboardingFlow itself already blocks the app without wrapping it.
import { useCallback, useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import type { TenantDepartment } from '../../../shared/tenant-user-profile-types'

type GateStatus = 'checking' | 'blocked' | 'clear'

export function DepartmentGate(): React.JSX.Element | null {
  // Why admin bypass: admins manage the org (approve agents, assign groups,
  // grant department access) — requiring them to first pick a department
  // they may not belong to would block the very console that assigns
  // everyone else's department. Documented default, reversible.
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  // Why gate on authStatus, not just currentUser: `currentUser` starts null
  // and `isAdmin` reads false for that whole window while checkSession()'s
  // GET /auth/me is still in flight — without this, the effect below fires
  // immediately on mount believing the caller is a non-admin, sends a
  // profile.getUserProfile call for a not-yet-known user, and (live-verified
  // against the bootstrap admin account, which has no tenant.user_profiles
  // row) logs a spurious TENANT_PROFILE_NOT_FOUND error before role even
  // resolves. Waiting for 'authenticated' means isAdmin is trustworthy by
  // the time this runs at all.
  const authStatus = useAppStore((s) => s.authStatus)

  const [status, setStatus] = useState<GateStatus>('checking')
  const [departments, setDepartments] = useState<TenantDepartment[]>([])
  const [selectedDeptId, setSelectedDeptId] = useState<string>('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (authStatus !== 'authenticated') {
      return
    }
    if (isAdmin) {
      setStatus('clear')
      return
    }
    let cancelled = false
    window.api.tenantProfile
      .getUserProfile()
      .then((profile) => {
        if (cancelled) {
          return
        }
        if (profile.departmentId) {
          setStatus('clear')
          return
        }
        return window.api.tenantProfile.listDepartments().then((depts) => {
          if (cancelled) {
            return
          }
          setDepartments(depts)
          setStatus('blocked')
        })
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        // Why fail-open: a transient profile-fetch error shouldn't brick the
        // whole app for an already-onboarded user — worst case, an ungated
        // user briefly reaches onboarding/features until the next check.
        console.error('[DepartmentGate] profile check failed:', err)
        setStatus('clear')
      })
    return () => {
      cancelled = true
    }
  }, [isAdmin, authStatus])

  const handleSubmit = useCallback(() => {
    if (!selectedDeptId || submitting) {
      return
    }
    setSubmitting(true)
    setError(null)
    window.api.tenantProfile
      .setUserDepartment({ departmentId: selectedDeptId })
      .then(() => setStatus('clear'))
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err))
      })
      .finally(() => setSubmitting(false))
  }, [selectedDeptId, submitting])

  if (status !== 'blocked') {
    return null
  }

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center overflow-hidden bg-black/50 p-4 text-foreground backdrop-blur-[2px]"
      data-department-gate-overlay
    >
      <section
        role="dialog"
        aria-label={translate('auto.components.DepartmentGate.title', 'Select your department')}
        aria-modal="true"
        className="relative flex w-full max-w-[480px] flex-col gap-5 rounded-xl border border-border bg-card p-8 text-card-foreground shadow-[0_10px_24px_rgba(0,0,0,0.18)]"
      >
        <div className="flex flex-col gap-2">
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            {translate('auto.components.DepartmentGate.title', 'Select your department')}
          </h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {translate(
              'auto.components.DepartmentGate.subtitle',
              'Orca uses your department to show you the dev servers your team has access to. This only needs to be set once.'
            )}
          </p>
        </div>

        <Select value={selectedDeptId} onValueChange={setSelectedDeptId}>
          <SelectTrigger>
            <SelectValue
              placeholder={translate(
                'auto.components.DepartmentGate.placeholder',
                'Choose a department'
              )}
            />
          </SelectTrigger>
          <SelectContent>
            {departments.map((dept) => (
              <SelectItem key={dept.id} value={dept.id}>
                {dept.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <Button disabled={!selectedDeptId || submitting} onClick={handleSubmit}>
          {submitting
            ? translate('auto.components.DepartmentGate.saving', 'Saving…')
            : translate('auto.components.DepartmentGate.continue', 'Continue')}
        </Button>
      </section>
    </div>
  )
}
