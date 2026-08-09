// TASK-FE-011: UserAvatarMenu — avatar trigger + dropdown for the authenticated user.
// Standalone presentational component: receives user + onLogout from parent.
// No direct store access — call sites use useAuthUser() + useLogout().
import { useState, useEffect, useRef } from 'react'
import type { OrcaUser } from '../../store/slices/auth'
import { UserRoleBadge } from './UserRoleBadge'

type Props = {
  user: OrcaUser
  onLogout: () => Promise<void>
}

/** Returns up to 2 uppercase initials from a display name. */
function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export function UserAvatarMenu({ user, onLogout }: Props) {
  const [isOpen, setIsOpen] = useState(false)
  const [isLoggingOut, setIsLoggingOut] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  // Close on Escape key
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {setIsOpen(false)}
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  // Close when clicking outside
  useEffect(() => {
    if (!isOpen) {return}
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [isOpen])

  async function handleLogout() {
    setIsOpen(false)
    setIsLoggingOut(true)
    try {
      await onLogout()
    } finally {
      setIsLoggingOut(false)
    }
  }

  return (
    <div className="user-avatar-menu" ref={menuRef}>
      {/* Trigger */}
      <button
        type="button"
        className="user-avatar-menu__trigger"
        onClick={() => setIsOpen((prev) => !prev)}
        aria-label="Open user menu"
        aria-haspopup="menu"
        aria-expanded={isOpen}
        disabled={isLoggingOut}
      >
        {user.avatarUrl ? (
          <img
            src={user.avatarUrl}
            alt={user.name}
            className="user-avatar-menu__image"
          />
        ) : (
          <span className="user-avatar-menu__initials" aria-hidden="true">
            {getInitials(user.name)}
          </span>
        )}
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div className="user-avatar-menu__dropdown" role="menu">
          <div className="user-avatar-menu__user-info" role="presentation">
            <span className="user-avatar-menu__name">{user.name}</span>
            <span className="user-avatar-menu__email">{user.email}</span>
            <UserRoleBadge role={user.role} />
          </div>

          <hr className="user-avatar-menu__divider" />

          <button
            type="button"
            role="menuitem"
            className="user-avatar-menu__logout"
            onClick={handleLogout}
            disabled={isLoggingOut}
          >
            {isLoggingOut ? 'Logging out…' : 'Logout'}
          </button>
        </div>
      )}
    </div>
  )
}
