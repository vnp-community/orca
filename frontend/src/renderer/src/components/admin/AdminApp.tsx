// TASK-FE-014: AdminApp — root component of the Admin SPA.
// Uses prop-driven state routing (no react-router-dom). Pages are loaded lazily.
import { useState, Suspense, lazy } from 'react'
import { AdminLayout, type AdminRoute } from './AdminLayout'
import { useAuthUser } from '../../hooks/useAuthSession'
import { useLogout } from '../../hooks/useLogout'
import { Tabs, TabsList, TabsTrigger } from '../ui/tabs'

// Lazy-load each page to keep the initial bundle small
const AdminDashboard = lazy(() => import('./AdminDashboard').then((m) => ({ default: m.AdminDashboard })))
const UsersPage      = lazy(() => import('./UsersPage').then((m) => ({ default: m.UsersPage })))
const UserForm       = lazy(() => import('./UserForm').then((m) => ({ default: m.UserForm })))
const PoliciesPage   = lazy(() => import('./PoliciesPage').then((m) => ({ default: m.PoliciesPage })))
const PolicyForm     = lazy(() => import('./PolicyForm').then((m) => ({ default: m.PolicyForm })))
const SessionsPage   = lazy(() => import('./SessionsPage').then((m) => ({ default: m.SessionsPage })))
const AuditPage      = lazy(() => import('./AuditPage').then((m) => ({ default: m.AuditPage })))
const CompanyProfileAdmin = lazy(() =>
  import('../profile/CompanyProfileAdmin').then((m) => ({ default: m.CompanyProfileAdmin }))
)
const DeptProfileAdmin = lazy(() =>
  import('../profile/DeptProfileAdmin').then((m) => ({ default: m.DeptProfileAdmin }))
)
const ProviderList = lazy(() =>
  import('../ai-provider/ProviderList').then((m) => ({ default: m.ProviderList }))
)
const FleetDashboard = lazy(() =>
  import('./fleet/fleet-dashboard').then((m) => ({ default: m.FleetDashboard }))
)

// TASK-FE-014: /profile hosts both company-wide and per-department profile
// admin — DeptProfileAdmin was never mounted anywhere, so it's exposed here
// as a tab rather than a new AdminRoute (keeps AdminLayout's route table untouched).
function ProfileAdminPage() {
  const [tab, setTab] = useState<'company' | 'departments'>('company')
  return (
    <div className="profile-admin-page">
      <Tabs value={tab} onValueChange={(v) => setTab(v as 'company' | 'departments')}>
        <TabsList>
          <TabsTrigger value="company">Company</TabsTrigger>
          <TabsTrigger value="departments">Departments</TabsTrigger>
        </TabsList>
      </Tabs>
      {tab === 'company' ? <CompanyProfileAdmin /> : <DeptProfileAdmin />}
    </div>
  )
}

function PageContent({
  route,
  onNavigate
}: {
  route: AdminRoute
  onNavigate: (r: AdminRoute) => void
}) {
  if (route === '/')         {return <AdminDashboard />}
  if (route === '/users')    {return <UsersPage onNavigate={onNavigate} />}
  if (route === '/users/new') {return <UserForm mode="create" onDone={() => onNavigate('/users')} />}
  if (route.startsWith('/users/') && route.endsWith('/edit')) {
    const id = route.split('/')[2]
    return <UserForm mode="edit" userId={id} onDone={() => onNavigate('/users')} />
  }
  if (route === '/policies') {return <PoliciesPage onNavigate={onNavigate} />}
  if (route === '/policies/new') {return <PolicyForm mode="create" onDone={() => onNavigate('/policies')} />}
  if (route.startsWith('/policies/') && route.endsWith('/edit')) {
    const id = route.split('/')[2]
    return <PolicyForm mode="edit" policyId={id} onDone={() => onNavigate('/policies')} />
  }
  if (route === '/sessions')      {return <SessionsPage />}
  if (route === '/audit')          {return <AuditPage />}
  if (route === '/profile')        {return <ProfileAdminPage />}
  if (route === '/ai-providers')   {return <ProviderList />}
  if (route === '/fleet')          {return <FleetDashboard />}
  return <AdminDashboard />
}

export function AdminApp() {
  const [currentRoute, setCurrentRoute] = useState<AdminRoute>('/')
  const user = useAuthUser()
  const logout = useLogout()

  // Guard: if not authenticated, this shouldn't render (backend redirects to /login)
  if (!user) {
    return (
      <div className="admin-auth-guard">
        <p>Not authenticated. Redirecting…</p>
      </div>
    )
  }

  return (
    <AdminLayout
      currentRoute={currentRoute}
      onNavigate={(r) => setCurrentRoute(r as AdminRoute)}
      userEmail={user.email}
      onLogout={logout}
    >
      <Suspense fallback={<div className="admin-page-loading" aria-label="Loading page…">Loading…</div>}>
        <PageContent route={currentRoute} onNavigate={(r) => setCurrentRoute(r as AdminRoute)} />
      </Suspense>
    </AdminLayout>
  )
}
