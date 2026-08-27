// TASK-FE-014: AdminLayout — shell for the Admin SPA.
// Uses simple hash-based routing (no react-router-dom dependency required).
import type { ReactNode } from 'react'

export type AdminRoute =
  | '/'
  | '/users'
  | '/users/new'
  | '/policies'
  | '/policies/new'
  | '/sessions'
  | '/audit'
  | '/profile'
  | '/ai-providers'
  | '/fleet'
  | '/teams'

type Props = {
  currentRoute: AdminRoute
  onNavigate: (route: AdminRoute) => void
  userEmail: string
  onLogout: () => void
  children: ReactNode
}

type NavItem = { route: AdminRoute; label: string; icon: string; exact?: boolean }

const NAV_ITEMS: NavItem[] = [
  { route: '/',              label: 'Dashboard',    icon: '📊', exact: true },
  { route: '/users',         label: 'Users',        icon: '👥' },
  { route: '/policies',      label: 'Policies',     icon: '🔐' },
  { route: '/sessions',      label: 'Sessions',     icon: '📡' },
  { route: '/audit',         label: 'Audit Log',    icon: '📋' },
  { route: '/ai-providers',  label: 'AI Providers', icon: '🤖' },
  { route: '/profile',       label: 'Profile',      icon: '🏢' },
  { route: '/teams',         label: 'Teams',        icon: '🧑‍🤝‍🧑' },
  { route: '/fleet',         label: 'Fleet',        icon: '🖥️' },
]

function isActive(item: NavItem, current: AdminRoute): boolean {
  if (item.exact) {return current === item.route}
  return current.startsWith(item.route)
}

export function AdminLayout({ currentRoute, onNavigate, userEmail, onLogout, children }: Props) {
  return (
    <div className="admin-layout">
      {/* Header */}
      <header className="admin-header">
        <span className="admin-header__logo">🔧 Orca Admin</span>
        <div className="admin-header__user">
          <span className="admin-header__email">{userEmail}</span>
          <button
            type="button"
            className="admin-header__logout"
            onClick={onLogout}
          >
            Logout
          </button>
        </div>
      </header>

      <div className="admin-body">
        {/* Sidebar nav */}
        <nav className="admin-nav" aria-label="Admin navigation">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.route}
              type="button"
              className={`admin-nav__item${isActive(item, currentRoute) ? ' admin-nav__item--active' : ''}`}
              aria-current={isActive(item, currentRoute) ? 'page' : undefined}
              onClick={() => onNavigate(item.route)}
            >
              <span aria-hidden="true">{item.icon}</span>
              {item.label}
            </button>
          ))}
        </nav>

        {/* Page content */}
        <main className="admin-content">
          {children}
        </main>
      </div>
    </div>
  )
}
