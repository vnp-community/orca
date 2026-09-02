// useTenantMemberDirectory.ts — resolves userId -> {name, email} for the
// caller's own tenant, backing the project/repo member-picker UIs
// (MemberManager.tsx, RepoMemberManager.tsx). Those previously only had a
// raw userId to collect (a free-text input) and display (no way to tell
// who a userId actually was) — auth.listTenantMemberDirectory is the
// non-admin RPC that makes a real picker possible.
import { useEffect, useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'

export type TenantMemberDirectoryEntry = {
  id: string
  name: string
  email: string
}

export function useTenantMemberDirectory(): {
  directory: TenantMemberDirectoryEntry[]
  isLoading: boolean
} {
  const [directory, setDirectory] = useState<TenantMemberDirectoryEntry[]>([])
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setIsLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<TenantMemberDirectoryEntry[]>(target, 'auth.listTenantMemberDirectory', null)
      .then((result) => {
        if (!cancelled) {
          setDirectory(result ?? [])
        }
      })
      .catch(() => {
        // Why silent: a picker that fails to load falls back to the raw
        // userId (see describeMemberLabel below) — not a fatal error for
        // the member-management UI itself.
        if (!cancelled) {
          setDirectory([])
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  return { directory, isLoading }
}

// Why fall back to the raw id: a member added before the directory
// endpoint existed, or a user who has since left the tenant, may not
// resolve — showing the id is more honest than hiding the row.
export function describeMemberLabel(
  userId: string,
  directory: readonly TenantMemberDirectoryEntry[]
): string {
  const entry = directory.find((d) => d.id === userId)
  return entry ? `${entry.name} (${entry.email})` : userId
}
